package autopilot

import (
	"crypto/sha256"
	"encoding/binary"
	"log"
	"sort"
)

// IntentMatchResult 人工意图匹配结果。
type IntentMatchResult struct {
	Intent       *ManualRoutingIntent // 命中的意图
	ChannelUID   string               // 目标渠道 UID
	Reasons      []string             // 命中原因列表
	Specificity  int                  // 匹配维度越多越具体，用于多意图排序
	FallbackUsed bool                 // 是否因 fallback 而退回到默认排序
}

// IntentMatchContext 意图匹配上下文，聚合请求画像中的可用维度。
// 不要求全部字段都有值；缺失字段（零值）跳过对应匹配维度。
type IntentMatchContext struct {
	ChannelKind string    // 必填
	Model       string    // model_trial 匹配
	TaskClass   TaskClass // TaskClasses 过滤
	AgentRole   string    // AgentRoles 过滤
	SessionID   string    // session_pin 匹配
	PromptHash  string    // 确定性流量分配
}

// MatchIntent 从活跃意图列表中查找最匹配的意图。
// 返回 nil 表示无匹配。多意图命中时按 Specificity 降序取最具体的。
// 匹配维度：ChannelKind → IntentType 特定字段 → TaskClasses/AgentRoles → TrafficPercent。
func MatchIntent(ctx *IntentMatchContext, intents []*ManualRoutingIntent) *IntentMatchResult {
	if ctx == nil || len(intents) == 0 {
		return nil
	}

	var candidates []IntentMatchResult

	for _, intent := range intents {
		if intent == nil {
			continue
		}

		// 1. ChannelKind 必须匹配
		if intent.ChannelKind != ctx.ChannelKind {
			continue
		}

		// 2. IntentType 特定匹配
		reasons, ok := matchByType(ctx, intent)
		if !ok {
			continue
		}

		spec := len(reasons)

		// 3. TaskClasses 作用范围过滤
		if len(intent.TaskClasses) > 0 {
			if ctx.TaskClass == "" || !containsTaskClass(intent.TaskClasses, ctx.TaskClass) {
				continue
			}
			reasons = append(reasons, "task_class="+string(ctx.TaskClass))
			spec++
		}

		// 4. AgentRoles 作用范围过滤
		if len(intent.AgentRoles) > 0 && ctx.AgentRole != "" {
			if !containsString(intent.AgentRoles, ctx.AgentRole) {
				continue
			}
			reasons = append(reasons, "agent_role="+ctx.AgentRole)
			spec++
		}

		// 5. 确定性 TrafficPercent 过滤
		if !passesTrafficPercent(ctx, intent.TrafficPercent) {
			continue
		}
		if intent.TrafficPercent > 0 && intent.TrafficPercent < 100 {
			reasons = append(reasons, "traffic_percent")
			spec++
		}

		candidates = append(candidates, IntentMatchResult{
			Intent:      intent,
			ChannelUID:  intent.ChannelUID,
			Reasons:     reasons,
			Specificity: spec,
		})
	}

	if len(candidates) == 0 {
		return nil
	}

	// 按 Specificity 降序，相同时按 ChannelKind + IntentUID 稳定排序
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Specificity != candidates[j].Specificity {
			return candidates[i].Specificity > candidates[j].Specificity
		}
		if candidates[i].Intent.ChannelKind != candidates[j].Intent.ChannelKind {
			return candidates[i].Intent.ChannelKind < candidates[j].Intent.ChannelKind
		}
		return candidates[i].Intent.IntentUID < candidates[j].Intent.IntentUID
	})

	best := candidates[0]
	log.Printf("[IntentMatcher-Match] uid=%s type=%s specificity=%d reasons=%v",
		best.Intent.IntentUID, string(best.Intent.IntentType),
		best.Specificity, best.Reasons)

	return &best
}

// matchByType 按 IntentType 的特定字段进行匹配。
// 返回匹配原因列表和是否匹配成功。
func matchByType(ctx *IntentMatchContext, intent *ManualRoutingIntent) ([]string, bool) {
	switch intent.IntentType {
	case IntentTypeModelTrial:
		return matchModelTrial(ctx, intent)
	case IntentTypeChannelTrial:
		return matchChannelTrial(ctx, intent)
	case IntentTypeEndpointTrial:
		return matchEndpointTrial(ctx, intent)
	case IntentTypeSessionPin:
		return matchSessionPin(ctx, intent)
	default:
		return nil, false
	}
}

// matchModelTrial 模型试用匹配：请求模型与意图模型相同。
func matchModelTrial(ctx *IntentMatchContext, intent *ManualRoutingIntent) ([]string, bool) {
	if intent.Model == "" || ctx.Model == "" {
		return nil, false
	}
	if intent.Model != ctx.Model {
		return nil, false
	}
	return []string{"model_trial=" + intent.Model}, true
}

// matchChannelTrial 渠道试用匹配：无特定字段条件，由后续 TaskClasses/AgentRoles/TrafficPercent 过滤。
func matchChannelTrial(ctx *IntentMatchContext, intent *ManualRoutingIntent) ([]string, bool) {
	return []string{"channel_trial"}, true
}

// matchEndpointTrial endpoint 试用匹配：等价于 ChannelTrial，精确到 MetricsKey 由意图的目标渠道保证。
func matchEndpointTrial(ctx *IntentMatchContext, intent *ManualRoutingIntent) ([]string, bool) {
	return []string{"endpoint_trial"}, true
}

// matchSessionPin 会话级排障匹配：SessionID 精确匹配。
func matchSessionPin(ctx *IntentMatchContext, intent *ManualRoutingIntent) ([]string, bool) {
	if intent.SessionID == "" || ctx.SessionID == "" {
		return nil, false
	}
	if intent.SessionID != ctx.SessionID {
		return nil, false
	}
	return []string{"session_pin=" + intent.SessionID}, true
}

// passesTrafficPercent 确定性流量百分比检查。
// 使用 PromptHash（优先）或 SessionID 做确定性哈希取模，不用 rand。
// trafficPercent <= 0 或 >= 100 时匹配全部流量。
func passesTrafficPercent(ctx *IntentMatchContext, trafficPercent int) bool {
	if trafficPercent <= 0 || trafficPercent >= 100 {
		return true
	}

	// 选择哈希输入源
	input := ctx.PromptHash
	if input == "" {
		input = ctx.SessionID
	}
	if input == "" {
		// 无可哈希输入：退化为不匹配（无法确定性分配）
		return false
	}

	bucket := deterministicBucket(input)
	return bucket < uint32(trafficPercent)
}

// deterministicBucket 将字符串哈希映射到 [0, 100) 范围。
// 使用 SHA256 确保跨平台、跨机器的一致性。
func deterministicBucket(input string) uint32 {
	h := sha256.Sum256([]byte(input))
	return binary.BigEndian.Uint32(h[:4]) % 100
}

// ── 辅助函数 ──

// containsTaskClass 检查 TaskClass 列表中是否包含指定值。
func containsTaskClass(list []TaskClass, target TaskClass) bool {
	for _, tc := range list {
		if tc == target {
			return true
		}
	}
	return false
}

// containsString 检查字符串列表中是否包含指定值。
func containsString(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

// intentExplicitlyTargetsSupervisor 检查意图是否显式声明包含 supervisor 任务类别。
// 用于 supervisor 保护：third-party 渠道的 model_trial 不覆盖 supervisor，
// 除非意图 TaskClasses 显式包含 supervisor。
func intentExplicitlyTargetsSupervisor(intent *ManualRoutingIntent) bool {
	return containsTaskClass(intent.TaskClasses, TaskClassSupervisor)
}
