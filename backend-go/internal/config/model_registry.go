package config

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/BenedictKing/ccx/internal/presetstore"
)

type compiledBuiltinPattern struct {
	regex               *regexp.Regexp
	hasSuffixConstraint bool
}

// compileBuiltinPattern 将 pattern 编译为 Go RE2 兼容的正则。
// 对于包含 (?=$|@) 等 lookahead 的模式，提取主正则并标记需要后缀检查。
func compileBuiltinPattern(pattern string) (*compiledBuiltinPattern, error) {
	// 分离主模式和后缀 lookahead
	// 常见形式：^主模式(?:可选后缀)(?=$|@)
	// 用前缀 (?i) 加强，Go RE2 支持 (?i)
	rePattern := "(?i)" + pattern

	// 去除所有 (?=...) / (?!) 等 lookahead，记录是否有后缀约束
	// 正则：找到最后一个 (?=...) 部分
	hasSuffixConstraint := false
	if idx := strings.LastIndex(rePattern, "(?="); idx >= 0 {
		suffix := rePattern[idx:]
		if strings.HasSuffix(suffix, ")") {
			// 去掉 (?=...)，但保留主模式
			rePattern = rePattern[:idx]
			// 检查 lookahead 内容是否包含 $（字符串结束断言）
			hasSuffixConstraint = strings.Contains(suffix, "$") || strings.Contains(suffix, "@")
		}
	}

	re, err := regexp.Compile(rePattern)
	if err != nil {
		return nil, err
	}
	return &compiledBuiltinPattern{regex: re, hasSuffixConstraint: hasSuffixConstraint}, nil
}

func matchBuiltinRegexPatternWithCache(pattern, model string, patternCache map[string]*compiledBuiltinPattern) bool {
	if len(patternCache) == 0 {
		return false
	}
	compiled, ok := patternCache[pattern]
	if !ok {
		return false
	}
	if !compiled.regex.MatchString(model) {
		return false
	}
	if compiled.hasSuffixConstraint {
		loc := compiled.regex.FindStringIndex(model)
		if loc == nil {
			return false
		}
		endIdx := loc[1]
		if endIdx < len(model) && model[endIdx] != '@' {
			return false
		}
	}
	return true
}

const (
	DefaultOutputReserveTokens     = 8192
	DefaultUnknownSafeWindowTokens = 200000
)

// ResolvedAgentModelProfile 描述下游 agent 模型解析结果。
type ResolvedAgentModelProfile struct {
	Profile        AgentModelProfile
	MatchedPattern string
	Source         string
	Known          bool
}

// ResolvedUpstreamCapability 描述实际模型能力解析结果。
type ResolvedUpstreamCapability struct {
	Capability     UpstreamModelCapability
	RequestModel   string
	ActualModel    string
	MatchedPattern string
	Source         string
	Known          bool
}

// ResolvedModelBenchmarkProfile 描述规范模型能力基准的匹配结果。
type ResolvedModelBenchmarkProfile struct {
	Profile        ModelBenchmarkProfile
	Model          string
	MatchedPattern string
	Source         string
	Known          bool
}

// IsContextRoutingEnabled 返回上下文路由是否启用，默认启用。
func (c ContextRoutingConfig) IsContextRoutingEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// EffectiveOutputReserveTokens 返回未显式请求输出上限时的预留 token。
func (c ContextRoutingConfig) EffectiveOutputReserveTokens() int {
	if c.DefaultOutputReserveTokens > 0 {
		return c.DefaultOutputReserveTokens
	}
	return DefaultOutputReserveTokens
}

// EffectiveUnknownSafeWindowTokens 返回未知能力渠道可接受的安全窗口。
func (c ContextRoutingConfig) EffectiveUnknownSafeWindowTokens() int {
	if c.UnknownSafeWindowTokens > 0 {
		return c.UnknownSafeWindowTokens
	}
	return DefaultUnknownSafeWindowTokens
}

// BuiltinAgentModelProfiles 返回 CCX 内置的下游 agent 模型知识库。
func BuiltinAgentModelProfiles() map[string]AgentModelProfile {
	return map[string]AgentModelProfile{
		"gpt-5.2": {
			DisplayName:            "GPT-5.5 / gpt-5.2",
			ContextWindowTokens:    272000,
			MaxContextWindowTokens: 272000,
			EffectiveContextRatio:  0.95,
			AutoCompactRatio:       0.90,
			TruncationMode:         "bytes",
			TruncationLimit:        10000,
			ReasoningEfforts:       []string{"low", "medium", "high", "xhigh"},
		},
		"gpt-5.4": {
			DisplayName:            "gpt-5.4",
			ContextWindowTokens:    272000,
			MaxContextWindowTokens: 1000000,
			EffectiveContextRatio:  0.95,
			AutoCompactRatio:       0.90,
			TruncationMode:         "tokens",
			TruncationLimit:        10000,
			ReasoningEfforts:       []string{"low", "medium", "high", "xhigh"},
			SupportsPriorityTier:   true,
		},
		"gpt-5.4-mini": {
			DisplayName:            "gpt-5.4-mini",
			ContextWindowTokens:    272000,
			MaxContextWindowTokens: 272000,
			EffectiveContextRatio:  0.95,
			AutoCompactRatio:       0.90,
			TruncationMode:         "tokens",
			TruncationLimit:        10000,
			ReasoningEfforts:       []string{"low", "medium", "high", "xhigh"},
		},
		"gpt-5.3-codex": {
			DisplayName:            "gpt-5.3-codex",
			ContextWindowTokens:    272000,
			MaxContextWindowTokens: 272000,
			EffectiveContextRatio:  0.95,
			AutoCompactRatio:       0.90,
			TruncationMode:         "tokens",
			TruncationLimit:        10000,
			ReasoningEfforts:       []string{"low", "medium", "high", "xhigh"},
		},
		"codex-auto-review": {
			DisplayName:            "Codex Auto Review",
			ContextWindowTokens:    272000,
			MaxContextWindowTokens: 1000000,
			EffectiveContextRatio:  0.95,
			AutoCompactRatio:       0.90,
			TruncationMode:         "tokens",
			TruncationLimit:        10000,
			ReasoningEfforts:       []string{"low", "medium", "high", "xhigh"},
		},
		"gpt-5.5": {
			DisplayName:            "GPT-5.5",
			ContextWindowTokens:    272000,
			MaxContextWindowTokens: 272000,
			EffectiveContextRatio:  0.95,
			AutoCompactRatio:       0.90,
			TruncationMode:         "tokens",
			TruncationLimit:        10000,
			ReasoningEfforts:       []string{"low", "medium", "high", "xhigh"},
			SupportsPriorityTier:   true,
		},
		"gpt-5.6-*": {
			DisplayName:            "Amazon Bedrock GPT-5.6",
			ContextWindowTokens:    272000,
			MaxContextWindowTokens: 272000,
			EffectiveContextRatio:  0.95,
			AutoCompactRatio:       0.90,
			TruncationMode:         "tokens",
			TruncationLimit:        10000,
			ReasoningEfforts:       []string{"low", "medium", "high", "xhigh", "max"},
		},
		"claude-haiku-4-5*": {
			DisplayName:         "Claude Haiku 4.5",
			ContextWindowTokens: 200000,
			MaxOutputTokens:     64000,
			ReasoningEfforts:    []string{"extended"},
		},
		"claude-sonnet-4-5*": {
			DisplayName:         "Claude Sonnet 4.5",
			ContextWindowTokens: 200000,
			MaxOutputTokens:     64000,
			ReasoningEfforts:    []string{"extended"},
		},
		"claude-opus-4-5*": {
			DisplayName:         "Claude Opus 4.5",
			ContextWindowTokens: 200000,
			MaxOutputTokens:     64000,
			ReasoningEfforts:    []string{"low", "medium", "high"},
		},
		"claude-sonnet-4-6*": {
			DisplayName:         "Claude Sonnet 4.6",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     64000,
			ReasoningEfforts:    []string{"low", "medium", "high", "max"},
		},
		"claude-opus-4-6*": {
			DisplayName:         "Claude Opus 4.6",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     128000,
			ReasoningEfforts:    []string{"low", "medium", "high", "max"},
		},
		"claude-opus-4-7*": {
			DisplayName:         "Claude Opus 4.7",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     128000,
			ReasoningEfforts:    []string{"low", "medium", "high", "xhigh", "max"},
		},
		"claude-opus-4-8*": {
			DisplayName:         "Claude Opus 4.8",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     128000,
			ReasoningEfforts:    []string{"low", "medium", "high", "xhigh", "max"},
		},
		"claude-sonnet-5*": {
			DisplayName:         "Claude Sonnet 5",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     128000,
			ReasoningEfforts:    []string{"low", "medium", "high", "xhigh", "max"},
		},
		"claude-fable-5*": {
			DisplayName:         "Claude Fable 5",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     128000,
			ReasoningEfforts:    []string{"low", "medium", "high", "xhigh", "max"},
		},
		"claude-mythos-5*": {
			DisplayName:         "Claude Mythos 5",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     128000,
			ReasoningEfforts:    []string{"low", "medium", "high", "xhigh", "max"},
		},
		"claude-mythos-preview*": {
			DisplayName:         "Claude Mythos Preview",
			ContextWindowTokens: 1000000,
			ReasoningEfforts:    []string{"max"},
		},
		"fable": {
			DisplayName:         "Claude Fable alias",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     128000,
		},
		"mythos": {
			DisplayName:         "Claude Mythos alias",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     128000,
		},
		"opus": {
			DisplayName:         "Claude Opus alias",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     128000,
		},
		"sonnet": {
			DisplayName:         "Claude Sonnet alias",
			ContextWindowTokens: 1000000,
			MaxOutputTokens:     64000,
		},
		"haiku": {
			DisplayName:         "Claude Haiku alias",
			ContextWindowTokens: 200000,
			MaxOutputTokens:     64000,
		},
		"*": {
			DisplayName:            "Codex fallback",
			ContextWindowTokens:    272000,
			MaxContextWindowTokens: 272000,
			EffectiveContextRatio:  0.95,
			AutoCompactRatio:       0.90,
			TruncationMode:         "bytes",
			TruncationLimit:        10000,
		},
	}
}

// BuiltinUpstreamModelCapabilities 返回 CCX 内置的实际上游模型能力知识库。
var (
	builtinOnce       sync.Once
	builtinRebuildMu  sync.Mutex
	builtinSnapshotMu sync.RWMutex
	builtinSnapshot   upstreamCapabilitySnapshot
	builtinObservers  sync.Map
)

type upstreamCapabilitySnapshot struct {
	store                 *presetstore.PresetStore
	capabilities          map[string]UpstreamModelCapability
	patternCache          map[string]*compiledBuiltinPattern
	benchmarks            map[string]ModelBenchmarkProfile
	benchmarkPatternCache map[string]*compiledBuiltinPattern
}

func BuiltinUpstreamModelCapabilities() map[string]UpstreamModelCapability {
	return cloneCapabilitiesMap(currentBuiltinSnapshot().capabilities)
}

// BuiltinModelBenchmarkProfiles 返回规范模型能力基准的深拷贝。
func BuiltinModelBenchmarkProfiles() map[string]ModelBenchmarkProfile {
	return cloneBenchmarkProfilesMap(currentBuiltinSnapshot().benchmarks)
}

func currentBuiltinSnapshot() upstreamCapabilitySnapshot {
	builtinOnce.Do(func() {
		rebuildBuiltinSnapshotForStore(presetstore.Default())
	})
	store := presetstore.Default()
	snapshot := getBuiltinSnapshot()
	if shouldRebuildBuiltinSnapshot(snapshot, store) {
		rebuildBuiltinSnapshotForStore(store)
	}
	return getBuiltinSnapshot()
}

func shouldRebuildBuiltinSnapshot(snapshot upstreamCapabilitySnapshot, store *presetstore.PresetStore) bool {
	if store == nil {
		return snapshot.store != nil || len(snapshot.capabilities) == 0
	}
	return snapshot.store != store || len(snapshot.capabilities) == 0
}

func getBuiltinSnapshot() upstreamCapabilitySnapshot {
	builtinSnapshotMu.RLock()
	defer builtinSnapshotMu.RUnlock()
	// snapshot 发布后保持不可变；浅拷贝持有旧 map 引用在并发替换后仍然安全，
	// 避免每次模型解析都深拷贝整个注册表。
	return builtinSnapshot
}

func rebuildBuiltinSnapshotForStore(store *presetstore.PresetStore) {
	builtinRebuildMu.Lock()
	defer builtinRebuildMu.Unlock()
	if store == nil {
		store = presetstore.Default()
	}
	if _, loaded := builtinObservers.LoadOrStore(store, struct{}{}); !loaded {
		store.RegisterOnChange(func(*presetstore.PresetBundle) {
			rebuildBuiltinSnapshotForStore(store)
		})
	}
	bundle := store.Get()
	capabilities := generatedBuiltinUpstreamModelCapabilities()
	if runtimeCapabilities := convertRuntimeCapabilities(bundle.ModelRegistry); len(runtimeCapabilities) > 0 {
		capabilities = runtimeCapabilities
	}
	benchmarks := generatedBuiltinModelBenchmarkProfiles()
	if runtimeBenchmarks := convertRuntimeBenchmarkProfiles(bundle.ModelRegistry); len(runtimeBenchmarks) > 0 {
		benchmarks = runtimeBenchmarks
	}
	snapshot := upstreamCapabilitySnapshot{
		store:                 store,
		capabilities:          cloneCapabilitiesMap(capabilities),
		patternCache:          buildPatternCache(precisionKeys(capabilities)),
		benchmarks:            cloneBenchmarkProfilesMap(benchmarks),
		benchmarkPatternCache: buildPatternCache(benchmarkPatternKeys(benchmarks)),
	}
	builtinSnapshotMu.Lock()
	builtinSnapshot = snapshot
	builtinSnapshotMu.Unlock()
}

func precisionKeys(m map[string]UpstreamModelCapability) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func benchmarkPatternKeys(m map[string]ModelBenchmarkProfile) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func buildPatternCache(patterns []string) map[string]*compiledBuiltinPattern {
	cache := make(map[string]*compiledBuiltinPattern, len(patterns))
	for _, p := range patterns {
		compiled, err := compileBuiltinPattern(p)
		if err != nil {
			panic("invalid builtin model pattern regex: " + p + ": " + err.Error())
		}
		cache[p] = compiled
	}
	return cache
}

func cloneCapabilitiesMap(src map[string]UpstreamModelCapability) map[string]UpstreamModelCapability {
	if len(src) == 0 {
		return map[string]UpstreamModelCapability{}
	}
	dst := make(map[string]UpstreamModelCapability, len(src))
	for pattern, capability := range src {
		dst[pattern] = cloneCapability(capability)
	}
	return dst
}

func cloneCapability(src UpstreamModelCapability) UpstreamModelCapability {
	dst := src
	if len(src.ReasoningEfforts) > 0 {
		dst.ReasoningEfforts = append([]string(nil), src.ReasoningEfforts...)
	}
	if len(src.Sources) > 0 {
		dst.Sources = append([]string(nil), src.Sources...)
	}
	if len(src.Capabilities) > 0 {
		dst.Capabilities = make(map[string]bool, len(src.Capabilities))
		for key, value := range src.Capabilities {
			dst.Capabilities[key] = value
		}
	}
	if src.Pricing != nil {
		pricing := *src.Pricing
		if pricing.InputCacheHitPrice != nil {
			v := *pricing.InputCacheHitPrice
			pricing.InputCacheHitPrice = &v
		}
		if pricing.InputCacheMissPrice != nil {
			v := *pricing.InputCacheMissPrice
			pricing.InputCacheMissPrice = &v
		}
		if pricing.OutputPrice != nil {
			v := *pricing.OutputPrice
			pricing.OutputPrice = &v
		}
		if len(src.Pricing.Tiers) > 0 {
			pricing.Tiers = make([]ModelPricingTier, len(src.Pricing.Tiers))
			for i, tier := range src.Pricing.Tiers {
				pricing.Tiers[i] = clonePricingTier(tier)
			}
		}
		dst.Pricing = &pricing
	}
	return dst
}

func cloneBenchmarkProfilesMap(src map[string]ModelBenchmarkProfile) map[string]ModelBenchmarkProfile {
	if len(src) == 0 {
		return map[string]ModelBenchmarkProfile{}
	}
	dst := make(map[string]ModelBenchmarkProfile, len(src))
	for pattern, profile := range src {
		dst[pattern] = cloneBenchmarkProfile(profile)
	}
	return dst
}

func cloneBenchmarkProfile(src ModelBenchmarkProfile) ModelBenchmarkProfile {
	dst := src
	if len(src.CategoryScores) > 0 {
		dst.CategoryScores = make(map[string]float64, len(src.CategoryScores))
		for category, score := range src.CategoryScores {
			dst.CategoryScores[category] = score
		}
	}
	if len(src.Sources) > 0 {
		dst.Sources = append([]string(nil), src.Sources...)
	}
	return dst
}

func clonePricingTier(src ModelPricingTier) ModelPricingTier {
	dst := src
	if src.InputCacheHitPrice != nil {
		v := *src.InputCacheHitPrice
		dst.InputCacheHitPrice = &v
	}
	if src.InputCacheMissPrice != nil {
		v := *src.InputCacheMissPrice
		dst.InputCacheMissPrice = &v
	}
	if src.OutputPrice != nil {
		v := *src.OutputPrice
		dst.OutputPrice = &v
	}
	return dst
}

func convertRuntimeCapabilities(preset *presetstore.ModelRegistryPreset) map[string]UpstreamModelCapability {
	if preset == nil || len(preset.UpstreamCapabilities) == 0 {
		return nil
	}
	capabilities := make(map[string]UpstreamModelCapability)
	for _, entry := range preset.UpstreamCapabilities {
		capability := UpstreamModelCapability{
			ContextWindowTokens:     entry.ContextWindowTokens,
			MaxOutputTokens:         entry.MaxOutputTokens,
			DefaultOutputTokens:     entry.DefaultOutputTokens,
			RecommendedOutputTokens: entry.RecommendedOutputTokens,
			ThinkingMode:            entry.ThinkingMode,
			ReasoningEfforts:        append([]string(nil), entry.ReasoningEfforts...),
			Provider:                entry.Provider,
			DisplayName:             entry.DisplayName,
			Description:             entry.Description,
			Sources:                 append([]string(nil), entry.Sources...),
		}
		if len(entry.Capabilities) > 0 {
			capability.Capabilities = make(map[string]bool, len(entry.Capabilities))
			for key, value := range entry.Capabilities {
				capability.Capabilities[key] = value
			}
		}
		if entry.Pricing != nil {
			capability.Pricing = &ModelPricing{
				Unit:                coalesceString(entry.Pricing.Unit, preset.PricingUnit),
				Currency:            entry.Pricing.Currency,
				InputCacheHitPrice:  cloneFloatPointer(entry.Pricing.InputCacheHitPrice),
				InputCacheMissPrice: cloneFloatPointer(entry.Pricing.InputCacheMissPrice),
				OutputPrice:         cloneFloatPointer(entry.Pricing.OutputPrice),
			}
			if len(entry.Pricing.Tiers) > 0 {
				capability.Pricing.Tiers = make([]ModelPricingTier, len(entry.Pricing.Tiers))
				for i, tier := range entry.Pricing.Tiers {
					capability.Pricing.Tiers[i] = ModelPricingTier{
						Label:               tier.Label,
						InputTokensAbove:    tier.InputTokensAbove,
						InputTokensUpTo:     tier.InputTokensUpTo,
						InputCacheHitPrice:  cloneFloatPointer(tier.InputCacheHitPrice),
						InputCacheMissPrice: cloneFloatPointer(tier.InputCacheMissPrice),
						OutputPrice:         cloneFloatPointer(tier.OutputPrice),
					}
				}
			}
		}
		for _, pattern := range entry.Patterns {
			capabilities[pattern] = cloneCapability(capability)
		}
	}
	return capabilities
}

func convertRuntimeBenchmarkProfiles(preset *presetstore.ModelRegistryPreset) map[string]ModelBenchmarkProfile {
	if preset == nil || len(preset.BenchmarkProfiles) == 0 {
		return nil
	}
	profiles := make(map[string]ModelBenchmarkProfile)
	for _, entry := range preset.BenchmarkProfiles {
		profile := ModelBenchmarkProfile{
			CanonicalModel:       entry.CanonicalModel,
			OverallScore:         entry.OverallScore,
			Sources:              append([]string(nil), entry.Sources...),
			VerifiedAt:           entry.VerifiedAt,
			Lane:                 entry.Lane,
			SharedResults:        entry.SharedResults,
			ComparableCategories: entry.ComparableCategories,
			TotalCategories:      entry.TotalCategories,
		}
		if len(entry.CategoryScores) > 0 {
			profile.CategoryScores = make(map[string]float64, len(entry.CategoryScores))
			for category, score := range entry.CategoryScores {
				profile.CategoryScores[category] = score
			}
		}
		for _, pattern := range entry.Patterns {
			profiles[pattern] = cloneBenchmarkProfile(profile)
		}
	}
	return profiles
}

func cloneFloatPointer(src *float64) *float64 {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func coalesceString(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

// ResolveAgentModelProfile 解析下游 agent 模型语义。
func ResolveAgentModelProfile(requestModel string, global map[string]AgentModelProfile) ResolvedAgentModelProfile {
	if profile, pattern, ok := resolvePatternValue(requestModel, global); ok {
		return ResolvedAgentModelProfile{Profile: profile, MatchedPattern: pattern, Source: "global", Known: true}
	}
	if profile, pattern, ok := resolvePatternValue(requestModel, BuiltinAgentModelProfiles()); ok {
		return ResolvedAgentModelProfile{Profile: profile, MatchedPattern: pattern, Source: "builtin", Known: true}
	}
	return ResolvedAgentModelProfile{}
}

// ResolveUpstreamCapability 解析渠道中实际模型的能力。
func ResolveUpstreamCapability(requestModel string, upstream *UpstreamConfig, global map[string]UpstreamModelCapability) ResolvedUpstreamCapability {
	actualModel := requestModel
	if upstream != nil {
		actualModel = RedirectModel(requestModel, upstream)
		if capability, pattern, ok := resolveCapabilityForModels(actualModel, requestModel, upstream.ModelCapabilities); ok {
			return ResolvedUpstreamCapability{Capability: capability, RequestModel: requestModel, ActualModel: actualModel, MatchedPattern: pattern, Source: "channel", Known: true}
		}
	}
	if capability, pattern, ok := resolveCapabilityForModels(actualModel, requestModel, global); ok {
		return ResolvedUpstreamCapability{Capability: capability, RequestModel: requestModel, ActualModel: actualModel, MatchedPattern: pattern, Source: "global", Known: true}
	}
	snapshot := currentBuiltinSnapshot()
	if capability, pattern, ok := resolveCapabilityForModelsFold(actualModel, requestModel, snapshot.capabilities, snapshot.patternCache); ok {
		return ResolvedUpstreamCapability{Capability: cloneCapability(capability), RequestModel: requestModel, ActualModel: actualModel, MatchedPattern: pattern, Source: "builtin", Known: true}
	}
	if upstream != nil && (upstream.DefaultCapability.ContextWindowTokens > 0 || upstream.DefaultCapability.MaxOutputTokens > 0) {
		return ResolvedUpstreamCapability{Capability: upstream.DefaultCapability, RequestModel: requestModel, ActualModel: actualModel, Source: "channel_default", Known: true}
	}
	return ResolvedUpstreamCapability{RequestModel: requestModel, ActualModel: actualModel}
}

// ResolveModelBenchmarkProfile 解析规范模型的能力上界证据。
// 基准只提供软评分依据，不参与 supportedModels 或能力下界判断。
func ResolveModelBenchmarkProfile(model string) ResolvedModelBenchmarkProfile {
	model = strings.TrimSpace(model)
	if model == "" {
		return ResolvedModelBenchmarkProfile{}
	}
	snapshot := currentBuiltinSnapshot()
	if profile, pattern, ok := resolvePatternValueFold(model, snapshot.benchmarks, snapshot.benchmarkPatternCache); ok {
		return ResolvedModelBenchmarkProfile{
			Profile:        cloneBenchmarkProfile(profile),
			Model:          model,
			MatchedPattern: pattern,
			Source:         "builtin",
			Known:          true,
		}
	}
	return ResolvedModelBenchmarkProfile{Model: model}
}

func resolveCapabilityForModels(actualModel, requestModel string, capabilities map[string]UpstreamModelCapability) (UpstreamModelCapability, string, bool) {
	if capability, pattern, ok := resolvePatternValue(actualModel, capabilities); ok {
		return capability, pattern, true
	}
	if requestModel != actualModel {
		if capability, pattern, ok := resolvePatternValue(requestModel, capabilities); ok {
			return capability, pattern, true
		}
	}
	return UpstreamModelCapability{}, "", false
}

func resolveCapabilityForModelsFold(actualModel, requestModel string, capabilities map[string]UpstreamModelCapability, patternCache map[string]*compiledBuiltinPattern) (UpstreamModelCapability, string, bool) {
	if capability, pattern, ok := resolvePatternValueFold(actualModel, capabilities, patternCache); ok {
		return capability, pattern, true
	}
	if requestModel != actualModel {
		if capability, pattern, ok := resolvePatternValueFold(requestModel, capabilities, patternCache); ok {
			return capability, pattern, true
		}
	}
	return UpstreamModelCapability{}, "", false
}

func resolvePatternValue[T any](model string, values map[string]T) (T, string, bool) {
	var zero T
	model = strings.TrimSpace(model)
	if model == "" || len(values) == 0 {
		return zero, "", false
	}
	if value, ok := values[model]; ok {
		return value, model, true
	}

	patterns := make([]string, 0, len(values))
	for pattern := range values {
		if pattern == model {
			continue
		}
		if isValidSupportedModelPattern(pattern) {
			patterns = append(patterns, pattern)
		}
	}
	sort.Slice(patterns, func(i, j int) bool {
		if len(patterns[i]) == len(patterns[j]) {
			return patterns[i] < patterns[j]
		}
		return len(patterns[i]) > len(patterns[j])
	})

	for _, pattern := range patterns {
		if matchSupportedModelPattern(pattern, model) {
			return values[pattern], pattern, true
		}
	}
	return zero, "", false
}

func resolvePatternValueFold[T any](model string, values map[string]T, patternCache ...map[string]*compiledBuiltinPattern) (T, string, bool) {
	var zero T
	model = strings.TrimSpace(model)
	if model == "" || len(values) == 0 {
		return zero, "", false
	}
	if value, ok := values[model]; ok {
		return value, model, true
	}
	for pattern, value := range values {
		if strings.EqualFold(pattern, model) {
			return value, pattern, true
		}
	}

	patterns := make([]string, 0, len(values))
	for pattern := range values {
		if strings.EqualFold(pattern, model) {
			continue
		}
		patterns = append(patterns, pattern)
	}
	sort.Slice(patterns, func(i, j int) bool {
		if len(patterns[i]) == len(patterns[j]) {
			return patterns[i] < patterns[j]
		}
		return len(patterns[i]) > len(patterns[j])
	})

	for _, pattern := range patterns {
		var cache map[string]*compiledBuiltinPattern
		if len(patternCache) > 0 {
			cache = patternCache[0]
		}
		// 优先用正则匹配（builtin 正则），失败再回退通配符
		if matchBuiltinRegexPatternWithCache(pattern, model, cache) {
			return values[pattern], pattern, true
		}
		if matchSupportedModelPatternFold(pattern, model, cache) {
			return values[pattern], pattern, true
		}
	}
	return zero, "", false
}

func matchSupportedModelPatternFold(pattern, model string, patternCache map[string]*compiledBuiltinPattern) bool {
	return matchSupportedModelPattern(strings.ToLower(pattern), strings.ToLower(model))
}
