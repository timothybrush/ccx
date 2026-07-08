package autopilot

import (
	"log"
	"sort"
	"time"
)

// ── 订阅级能力共享推导（设计 §3.2.3，shadow 模式）──
//
// 同 SubscriptionUID 下的所有 endpoint 共享一份「订阅级能力画像」。
// BuildSharedCapability 从 endpoint 画像聚合（多数派投票），结果存入 SubscriptionProfile。
// DetectCapabilityDrift 检测单个 key 与订阅级能力是否一致。

// BuildSharedCapability 从同订阅的 endpoint 画像列表聚合出共享能力。
// 规则：
//   - 模型列表：取出现次数最多的列表（多数派），如全不相同则取并集
//   - 能力标签：任一 endpoint 支持即为 true（并集策略，订阅级能力取所有 key 的上界）
//   - 协议开关：同上，取并集
//
// endpoints 为空时返回 nil。
func BuildSharedCapability(endpoints []*KeyEndpointProfile) *SharedCapability {
	if len(endpoints) == 0 {
		return nil
	}

	now := time.Now()

	// ── 模型列表多数派 ──
	modelList := majorityModelList(endpoints)
	listHash := hashModelList(modelList)

	// ── 能力标签并集 ──
	var supportsVision, supportsToolCalls, supportsReasoning bool
	var supportsStreaming, supportsLongCtx, supportsMultiModal bool

	for _, ep := range endpoints {
		if ep.SupportsVision {
			supportsVision = true
		}
		if ep.SupportsToolCalls {
			supportsToolCalls = true
		}
		if ep.SupportsReasoning {
			supportsReasoning = true
		}
		if ep.SupportsLongCtx {
			supportsLongCtx = true
		}
	}

	// 流式和多模态从 AvailableModels 推断：有模型即视为支持流式（默认）
	// Phase 1 shadow 简化：有模型列表则标记 SupportsStreaming
	if len(modelList) > 0 {
		supportsStreaming = true
	}

	// ── 一致性统计 ──
	var inconsistentKeys []string
	for _, ep := range endpoints {
		epHash := ep.ModelListHash
		if epHash == "" {
			epHash = hashModelList(ep.AvailableModels)
		}
		if epHash != listHash {
			inconsistentKeys = append(inconsistentKeys, ep.EndpointUID)
		}
	}
	consistentCount := len(endpoints) - len(inconsistentKeys)

	sc := &SharedCapability{
		ModelListHash:      listHash,
		ModelList:          modelList,
		SupportsVision:     supportsVision,
		SupportsToolCalls:  supportsToolCalls,
		SupportsReasoning:  supportsReasoning,
		SupportsStreaming:  supportsStreaming,
		SupportsLongCtx:    supportsLongCtx,
		SupportsMultiModal: supportsMultiModal,
		TotalEndpoints:     len(endpoints),
		ConsistentCount:    consistentCount,
		InconsistentKeys:   inconsistentKeys,
		ProbedAt:           now,
	}

	log.Printf("[SubscriptionCapability-Build] 总 endpoint=%d 一致=%d 不一致=%d 模型数=%d",
		len(endpoints), consistentCount, len(inconsistentKeys), len(modelList))

	return sc
}

// DetectCapabilityDrift 检测单个 endpoint 与订阅级共享能力的差异。
// 返回中文差异描述列表；空列表表示无差异。
// shared 为 nil 时视为无共享能力，不报告差异。
func DetectCapabilityDrift(shared *SharedCapability, endpoint *KeyEndpointProfile) []string {
	if shared == nil || endpoint == nil {
		return nil
	}

	var diffs []string

	// ── 模型列表差异 ──
	epHash := endpoint.ModelListHash
	if epHash == "" {
		epHash = hashModelList(endpoint.AvailableModels)
	}
	if epHash != "" && epHash != shared.ModelListHash {
		// 计算详细差异
		added, removed := diffModelLists(shared.ModelList, endpoint.AvailableModels)
		if len(added) > 0 {
			diffs = append(diffs, "模型列表新增: "+joinStrings(added, ", "))
		}
		if len(removed) > 0 {
			diffs = append(diffs, "模型列表缺失: "+joinStrings(removed, ", "))
		}
		if len(added) == 0 && len(removed) == 0 && epHash != shared.ModelListHash {
			diffs = append(diffs, "模型列表哈希不一致")
		}
	}

	// ── 能力标签差异（endpoint 缺少共享能力中为 true 的标签）──
	if shared.SupportsVision && !endpoint.SupportsVision {
		diffs = append(diffs, "缺少视觉能力（订阅级支持）")
	}
	if shared.SupportsToolCalls && !endpoint.SupportsToolCalls {
		diffs = append(diffs, "缺少工具调用能力（订阅级支持）")
	}
	if shared.SupportsReasoning && !endpoint.SupportsReasoning {
		diffs = append(diffs, "缺少推理能力（订阅级支持）")
	}
	if shared.SupportsLongCtx && !endpoint.SupportsLongCtx {
		diffs = append(diffs, "缺少长上下文能力（订阅级支持）")
	}

	return diffs
}

// ── 内部辅助 ──

// majorityModelList 从 endpoints 中选出出现次数最多的模型列表。
// 如多个列表出现次数相同，取最长的那个（能力取并集倾向）。
// 如所有 endpoint 的列表各不相同，取并集。
func majorityModelList(endpoints []*KeyEndpointProfile) []string {
	if len(endpoints) == 0 {
		return nil
	}
	if len(endpoints) == 1 {
		return sortedCopy(endpoints[0].AvailableModels)
	}

	// 按哈希分组
	hashGroups := make(map[string]int)      // hash -> count
	hashModels := make(map[string][]string) // hash -> models
	for _, ep := range endpoints {
		hash := ep.ModelListHash
		if hash == "" {
			hash = hashModelList(ep.AvailableModels)
		}
		hashGroups[hash]++
		if _, exists := hashModels[hash]; !exists {
			hashModels[hash] = sortedCopy(ep.AvailableModels)
		}
	}

	// 找出现次数最多的哈希
	maxCount := 0
	var bestHash string
	for hash, count := range hashGroups {
		if count > maxCount || (count == maxCount && len(hashModels[hash]) > len(hashModels[bestHash])) {
			maxCount = count
			bestHash = hash
		}
	}

	// 超过半数取多数派；票数相同且各有多个时取最长列表（能力取并集倾向）；否则取并集
	half := len(endpoints) / 2
	if maxCount > half || (maxCount == half && maxCount > 1) {
		return hashModels[bestHash]
	}

	// 取并集
	unionSet := make(map[string]struct{})
	for _, ep := range endpoints {
		for _, m := range ep.AvailableModels {
			unionSet[m] = struct{}{}
		}
	}
	union := make([]string, 0, len(unionSet))
	for m := range unionSet {
		union = append(union, m)
	}
	sort.Strings(union)
	return union
}

// joinStrings 将字符串切片用分隔符连接。
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += sep + ss[i]
	}
	return result
}
