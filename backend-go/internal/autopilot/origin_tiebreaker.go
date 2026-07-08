// origin_tiebreaker.go — OriginTier tie-breaker 纯函数（渠道级排序）
//
// 设计意图（§5.4.1 / §9.1）：仅在同质量/同健康/同成本档时使用信任等级，
// 不把信任等级当质量。此函数独立于 endpoint_policy.go 的 endpoint 级排序，
// 供 SmartRouter 层的 channel 级排序使用。

package autopilot

import "sort"

// BreakTieByOriginTier 对分数相同（严格相等）的候选按 OriginTier 重新排序，
// 不影响分数不同的候选相对顺序。
//
// originTiers: key=ChannelUID, value=该渠道的 OriginTier。
// 缺失的 ChannelUID 按 unknown（rank 0）处理，不 panic。
//
// 算法：单轮 sort.SliceStable，Less 在 Score 降序主序的基础上，
// 同分时追加 OriginTier rank 降序作为次序。稳定排序保证：
// 同分同 rank 的候选保持输入相对顺序不变。
func BreakTieByOriginTier(candidates []ScoredCandidate, originTiers map[string]ChannelOriginTier) []ScoredCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		ci, cj := candidates[i], candidates[j]
		// 主序：Score 降序
		if ci.Score != cj.Score {
			return ci.Score > cj.Score
		}
		// 同分 tie-breaker：OriginTier rank 降序
		rankI := originTierRank(lookupOriginTier(ci.ChannelUID, originTiers))
		rankJ := originTierRank(lookupOriginTier(cj.ChannelUID, originTiers))
		return rankI > rankJ
	})

	return candidates
}

// lookupOriginTier 从 map 查找 ChannelUID 对应的 OriginTier，缺失返回 unknown。
func lookupOriginTier(channelUID string, originTiers map[string]ChannelOriginTier) ChannelOriginTier {
	if tier, ok := originTiers[channelUID]; ok {
		return tier
	}
	return OriginTierUnknown
}
