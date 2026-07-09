package autopilot

import (
	"sort"
	"sync"
	"time"
)

// ── 用量画像累积（Phase 4 Item 4：渠道推荐）──
//
// UsagePatternAccumulator 按 proxyKeyMask × TaskDomain × channelUID 滚动累积请求次数，
// 用于渠道推荐的"该用户常用领域 + 常用渠道"画像。纯内存、Phase 1 shadow 风格（不持久化，
// 进程重启后清零，与 UsageMeter/RateLimitDiscoverer 的既有惯例一致）。
//
// 数据来源：由调用方（Manager.RecordUsagePattern）在请求成功完成后调用 RecordUsage 喂入，
// 不参与任何调度/候选过滤决策——纯观测性累积，与 FastDecayScorer/UsageMeter 的定位一致。

// defaultUsagePatternRetentionDays 数据保留天数（滚动窗口上限）。
const defaultUsagePatternRetentionDays = 30

// dayKeyLayout 用于生成按天分桶的 key。
const dayKeyLayout = "2006-01-02"

// UsagePatternAccumulator 内存中按 proxyKeyMask 管理用量分布。
type UsagePatternAccumulator struct {
	mu sync.RWMutex
	// data[proxyKeyMask][domain][channelUID][dayKey] = count
	data          map[string]map[TaskDomain]map[string]map[string]int
	retentionDays int
	nowFunc       func() time.Time
	lastPruneDay  string
}

// NewUsagePatternAccumulator 创建 UsagePatternAccumulator，retentionDays<=0 时使用默认值 30。
func NewUsagePatternAccumulator(retentionDays int) *UsagePatternAccumulator {
	if retentionDays <= 0 {
		retentionDays = defaultUsagePatternRetentionDays
	}
	return &UsagePatternAccumulator{
		data:          make(map[string]map[TaskDomain]map[string]map[string]int),
		retentionDays: retentionDays,
		nowFunc:       time.Now,
	}
}

// RecordUsage 记录一次请求：proxyKeyMask 使用 domain 领域，最终落到 channelUID。
// proxyKeyMask 或 channelUID 为空时静默跳过（无法归因）。并发安全。
func (a *UsagePatternAccumulator) RecordUsage(proxyKeyMask string, domain TaskDomain, channelUID string) {
	if proxyKeyMask == "" || channelUID == "" {
		return
	}
	if domain == "" {
		domain = TaskDomainGeneral
	}

	now := a.nowFunc().UTC()
	dayKey := now.Format(dayKeyLayout)

	a.mu.Lock()
	defer a.mu.Unlock()

	byDomain, ok := a.data[proxyKeyMask]
	if !ok {
		byDomain = make(map[TaskDomain]map[string]map[string]int)
		a.data[proxyKeyMask] = byDomain
	}
	byChannel, ok := byDomain[domain]
	if !ok {
		byChannel = make(map[string]map[string]int)
		byDomain[domain] = byChannel
	}
	byDay, ok := byChannel[channelUID]
	if !ok {
		byDay = make(map[string]int)
		byChannel[channelUID] = byDay
	}
	byDay[dayKey]++

	// 摊销清理：一天最多触发一次全量清理，避免无界内存增长。
	if a.lastPruneDay != dayKey {
		a.lastPruneDay = dayKey
		a.pruneLocked(now)
	}
}

// pruneLocked 清除超出保留窗口的天级桶。调用方必须持有写锁。
func (a *UsagePatternAccumulator) pruneLocked(now time.Time) {
	cutoff := now.AddDate(0, 0, -a.retentionDays).Format(dayKeyLayout)
	for mask, byDomain := range a.data {
		for domain, byChannel := range byDomain {
			for channelUID, byDay := range byChannel {
				for day := range byDay {
					if day < cutoff {
						delete(byDay, day)
					}
				}
				if len(byDay) == 0 {
					delete(byChannel, channelUID)
				}
			}
			if len(byChannel) == 0 {
				delete(byDomain, domain)
			}
		}
		if len(byDomain) == 0 {
			delete(a.data, mask)
		}
	}
}

// DomainUsageStat 单个 TaskDomain 的用量统计（用于排序展示）。
type DomainUsageStat struct {
	Domain TaskDomain `json:"domain"`
	Count  int        `json:"count"`
}

// ChannelUsageStat 单个 channelUID 的用量统计（用于排序展示）。
type ChannelUsageStat struct {
	ChannelUID string `json:"channelUid"`
	Count      int    `json:"count"`
}

// windowCutoffDay 返回窗口起始天的 dayKey（含当天）。
func (a *UsagePatternAccumulator) windowCutoffDay(windowDays int) string {
	if windowDays <= 0 {
		windowDays = defaultUsagePatternRetentionDays
	}
	return a.nowFunc().UTC().AddDate(0, 0, -windowDays+1).Format(dayKeyLayout)
}

// DomainDistribution 返回指定 proxyKeyMask 在窗口内各 TaskDomain 的累计请求数，按数量降序排列。
func (a *UsagePatternAccumulator) DomainDistribution(proxyKeyMask string, windowDays int) []DomainUsageStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	byDomain, ok := a.data[proxyKeyMask]
	if !ok {
		return nil
	}
	cutoff := a.windowCutoffDay(windowDays)

	result := make([]DomainUsageStat, 0, len(byDomain))
	for domain, byChannel := range byDomain {
		total := 0
		for _, byDay := range byChannel {
			for day, count := range byDay {
				if day >= cutoff {
					total += count
				}
			}
		}
		if total > 0 {
			result = append(result, DomainUsageStat{Domain: domain, Count: total})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Domain < result[j].Domain
	})
	return result
}

// ChannelDistribution 返回指定 proxyKeyMask + domain 在窗口内各 channelUID 的累计请求数，按数量降序排列。
func (a *UsagePatternAccumulator) ChannelDistribution(proxyKeyMask string, domain TaskDomain, windowDays int) []ChannelUsageStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	byDomain, ok := a.data[proxyKeyMask]
	if !ok {
		return nil
	}
	byChannel, ok := byDomain[domain]
	if !ok {
		return nil
	}
	cutoff := a.windowCutoffDay(windowDays)

	result := make([]ChannelUsageStat, 0, len(byChannel))
	for channelUID, byDay := range byChannel {
		total := 0
		for day, count := range byDay {
			if day >= cutoff {
				total += count
			}
		}
		if total > 0 {
			result = append(result, ChannelUsageStat{ChannelUID: channelUID, Count: total})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].ChannelUID < result[j].ChannelUID
	})
	return result
}

// AllProxyKeyMasks 返回当前累积器中出现过的全部 proxyKeyMask（用于全局聚合粒度）。
func (a *UsagePatternAccumulator) AllProxyKeyMasks() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]string, 0, len(a.data))
	for mask := range a.data {
		result = append(result, mask)
	}
	sort.Strings(result)
	return result
}
