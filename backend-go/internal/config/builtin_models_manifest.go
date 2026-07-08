package config

import "strings"

// BuiltinModelsManifest 内置模型清单条目。
// 对部分官方订阅入口（如 Claude OAuth 订阅、Codex plan 入口），
// 上游 models 接口可能不存在（404）或返回错误清单。
// 命中时直接使用 ModelIDs 作为该入口的可用模型列表，跳过上游探测。
type BuiltinModelsManifest struct {
	// BaseURLPattern 精确 host 或 host+path 前缀匹配，
	// 如 "api.anthropic.com" 或 "api.anthropic.com/v1"。
	BaseURLPattern string `json:"baseUrlPattern"`

	// ServiceType 渠道协议类型：messages/responses/chat/gemini/images/vectors。
	ServiceType string `json:"serviceType"`

	// PlanHint 订阅类型提示，仅用于文档和日志，不影响匹配逻辑。
	PlanHint string `json:"planHint,omitempty"`

	// ModelIDs 该入口实际可用的模型 ID 清单。
	ModelIDs []string `json:"modelIds"`

	// DisableProbe 为 true 时 Discovery 流程跳过 GET /v1/models，
	// 直接用 ModelIDs 生成 ModelProfile[]。
	DisableProbe bool `json:"disableProbe"`
}

// builtinModelsManifests 内置模型清单。
// 当前仅收录从仓库代码/模板/测试中能确认的官方入口。
// 新增条目须附注证据来源，不臆造 baseURL。
var builtinModelsManifests = []BuiltinModelsManifest{
	// Anthropic 官方 API 入口（来源：buildClaudeCompatibleModelsURLs 测试、
	// expectedRequestUrls 测试、前端 locale placeholder 确认 api.anthropic.com）。
	// 该入口的 /v1/models 正常可用，DisableProbe=false；
	// 当上游探测失败时回退使用此清单。
	{
		BaseURLPattern: "api.anthropic.com",
		ServiceType:    "messages",
		PlanHint:       "anthropic_api",
		ModelIDs: []string{
			"claude-fable-5",
			"claude-mythos-5",
			"claude-opus-4-8",
			"claude-opus-4-7",
			"claude-opus-4-6",
			"claude-sonnet-5",
			"claude-sonnet-4-6",
			"claude-sonnet-4-5",
			"claude-opus-4-5",
			"claude-haiku-4-5",
		},
		DisableProbe: false,
	},
}

// LookupBuiltinManifest 根据上游 baseURL 和 serviceType 查找匹配的内置清单。
// 匹配规则：BaseURLPattern 作为 host 或 host+path 前缀匹配。
// 优先精确 host 匹配，其次 host+path 前缀匹配（最长前缀优先）。
// 返回匹配的清单和 true；未命中返回零值和 false。
func LookupBuiltinManifest(baseURL string, serviceType string) (BuiltinModelsManifest, bool) {
	if len(builtinModelsManifests) == 0 {
		return BuiltinModelsManifest{}, false
	}

	normalized := normalizeBaseURLForManifest(baseURL)
	if normalized == "" {
		return BuiltinModelsManifest{}, false
	}

	var bestMatch BuiltinModelsManifest
	var bestMatchLen int
	found := false

	for _, manifest := range builtinModelsManifests {
		if manifest.ServiceType != serviceType {
			continue
		}
		pattern := strings.ToLower(manifest.BaseURLPattern)
		if !matchManifestPattern(normalized, pattern) {
			continue
		}
		// 最长前缀优先：避免 "api.anthropic.com" 误匹配到更具体的 pattern
		if len(pattern) > bestMatchLen {
			bestMatch = manifest
			bestMatchLen = len(pattern)
			found = true
		}
	}

	return bestMatch, found
}

// matchManifestPattern 检查 normalized baseURL 是否匹配 manifest pattern。
// pattern 可以是纯 host（精确匹配）或 host+path 前缀匹配。
func matchManifestPattern(normalized string, pattern string) bool {
	// 精确匹配
	if normalized == pattern {
		return true
	}
	// host 精确匹配（pattern 不含路径，normalized 可能含路径）
	if !strings.Contains(pattern, "/") && strings.HasPrefix(normalized, pattern) {
		rest := normalized[len(pattern):]
		return rest == "" || rest[0] == '/'
	}
	// host+path 前缀匹配
	return strings.HasPrefix(normalized, pattern)
}

// normalizeBaseURLForManifest 将 baseURL 规范化为 host 或 host+path 形式用于匹配。
// 去掉 scheme（https://）、尾部 /、# 标记、版本段（/v1 等）。
func normalizeBaseURLForManifest(baseURL string) string {
	s := strings.ToLower(strings.TrimSpace(baseURL))
	if s == "" {
		return ""
	}
	// 去掉 scheme
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	// 去掉 #
	s = strings.TrimSuffix(s, "#")
	// 去掉尾部 /
	s = strings.TrimRight(s, "/")
	return s
}
