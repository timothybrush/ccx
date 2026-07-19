package config

import "math"

// IsAPIKeyConfigGroupMultiplierAllowed 判断 Key 的分组倍率是否在其持久化上限内。
// 两个字段均缺失时保持历史配置的兼容行为；只配置其中一个或值非法时 fail-closed。
func IsAPIKeyConfigGroupMultiplierAllowed(cfg APIKeyConfig) bool {
	if cfg.GroupMultiplier == nil && cfg.MaxGroupMultiplier == nil {
		return true
	}
	if cfg.GroupMultiplier == nil || cfg.MaxGroupMultiplier == nil {
		return false
	}
	ratio, limit := *cfg.GroupMultiplier, *cfg.MaxGroupMultiplier
	if !isFiniteNonNegative(ratio) || !isFiniteNonNegative(limit) {
		return false
	}
	return ratio <= limit
}

// IsAPIKeyGroupMultiplierAllowed 返回渠道中某个 Key 是否满足其成本安全约束。
// 找不到对应配置时按历史手工 Key 处理，保持现有配置的兼容行为。
func (u *UpstreamConfig) IsAPIKeyGroupMultiplierAllowed(apiKey string) bool {
	if u == nil {
		return false
	}
	for _, cfg := range u.APIKeyConfigs {
		if cfg.Key == apiKey {
			return IsAPIKeyConfigGroupMultiplierAllowed(cfg)
		}
	}
	return true
}

func isFiniteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}
