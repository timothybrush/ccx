package autopilot

import (
	"strings"
	"time"
)

// ── 质量档 ──

// QualityTier 表示模型或 endpoint 的质量档位。
// 基于模型族推导（opus=premium, sonnet=high, haiku=normal），不来自 OriginTier。
type QualityTier string

const (
	QualityTierPremium QualityTier = "premium" // 旗舰：claude-opus, gpt-5.5, gpt-5.4
	QualityTierHigh    QualityTier = "high"    // 高端：claude-sonnet, gpt-5.3-codex
	QualityTierNormal  QualityTier = "normal"  // 标准：claude-haiku, gpt-5.4-mini
	QualityTierLow     QualityTier = "low"     // 低端：其他
)

// ── 稳定性档 ──

// StabilityTier 表示 endpoint 的稳定性档位。
// 基于最近 1 小时的成功率和 429 率推导。
type StabilityTier string

const (
	StabilityTierStable   StabilityTier = "stable"   // 成功率 >= 95% 且 429 率 < 5%
	StabilityTierNormal   StabilityTier = "normal"   // 成功率 >= 80% 且 429 率 < 20%
	StabilityTierUnstable StabilityTier = "unstable" // 其他
)

// ── 速度档 ──

// SpeedTier 表示 endpoint 的速度档位。
// 基于最近 100 次请求的 p95 首 token 延迟推导。
type SpeedTier string

const (
	SpeedTierFast   SpeedTier = "fast"   // p95 < 500ms
	SpeedTierNormal SpeedTier = "normal" // p95 < 2000ms
	SpeedTierSlow   SpeedTier = "slow"   // p95 >= 2000ms
)

// ── 成本档 ──

// CostTier 表示 endpoint 的成本档位。
type CostTier string

const (
	CostTierFree      CostTier = "free"      // Input/Output 都是 0
	CostTierCheap     CostTier = "cheap"     // EffectiveInput < $1/M 且 EffectiveOutput < $5/M
	CostTierNormal    CostTier = "normal"    // EffectiveInput < $10/M 且 EffectiveOutput < $30/M
	CostTierExpensive CostTier = "expensive" // 其他
)

// ── ModelFamily 模型派系 ──

// ModelFamily 表示模型派系（厂商系列）。
// 用于派系偏好排序和质量档推导的基础分类。
type ModelFamily string

const (
	// ── 国际主流 ──
	ModelFamilyClaude  ModelFamily = "claude"  // claude-*，Anthropic
	ModelFamilyOpenAI  ModelFamily = "openai"  // gpt-*, o*, codex-*，OpenAI / Amazon Bedrock
	ModelFamilyGemini  ModelFamily = "gemini"  // gemini-*，Google
	ModelFamilyMistral ModelFamily = "mistral" // mistral-*, mixtral-*，Mistral AI

	// ── 国产主流 ──
	ModelFamilyDeepSeek  ModelFamily = "deepseek"  // DeepSeek V3/V4，DeepSeek
	ModelFamilyQwen      ModelFamily = "qwen"      // qwen3-*，通义千问，DashScope
	ModelFamilyGLM       ModelFamily = "glm"       // glm-5-*，智谱 AI
	ModelFamilyKimi      ModelFamily = "kimi"      // kimi-k2-*，月之暗面 Moonshot
	ModelFamilyMiMo      ModelFamily = "mimo"      // mimo-v2-*，小米
	ModelFamilyERNIE     ModelFamily = "ernie"     // ernie-4.5，百度
	ModelFamilyDoubao    ModelFamily = "doubao"    // doubao-seed-*，字节豆包 Volcengine
	ModelFamilyMiniMax   ModelFamily = "minimax"   // minimax-m*，MiniMax
	ModelFamilyYi        ModelFamily = "yi"        // yi-*，零一万物 01.ai
	ModelFamilyBaichuan  ModelFamily = "baichuan"  // baichuan-m*，百川智能
	ModelFamilyStep      ModelFamily = "step"      // step-3.*，阶跃星辰 StepFun
	ModelFamilySenseNova ModelFamily = "sensenova" // sensenova-6.*，商汤 SenseTime
	ModelFamilyAgnes     ModelFamily = "agnes"     // agnes-2.*，Sapiens AI（小米独立系列）
	ModelFamilyLongCat   ModelFamily = "longcat"   // longcat-2.*，京东

	// ── 特殊 ──
	ModelFamilyLocal   ModelFamily = "local"   // ollama/lmstudio/llama-server 本地运行时
	ModelFamilyUnknown ModelFamily = "unknown" // 无法识别
)

// Provider → ModelFamily 映射表（从 generated_model_registry.go 提取）。
// providerFamilyMap 是全局只读映射，init 时构建。
var providerFamilyMap = map[string]ModelFamily{
	"anthropic":      ModelFamilyClaude,
	"openai":         ModelFamilyOpenAI,
	"amazon-bedrock": ModelFamilyOpenAI,
	"dashscope":      ModelFamilyQwen,
	"volcengine":     ModelFamilyDoubao,
	"xiaomi":         ModelFamilyMiMo, // 具体子系列需按前缀再细分
	"baidu":          ModelFamilyERNIE,
	"01-ai":          ModelFamilyYi,
	"moonshot":       ModelFamilyKimi,
	"zai":            ModelFamilyGLM,
	"deepseek":       ModelFamilyDeepSeek,
	"minimax":        ModelFamilyMiniMax,
	"baichuan":       ModelFamilyBaichuan,
	"stepfun":        ModelFamilyStep,
	"sensenova":      ModelFamilySenseNova,
	"agnes":          ModelFamilyAgnes,
	"longcat":        ModelFamilyLongCat,
	"mistral":        ModelFamilyMistral,
	"google":         ModelFamilyGemini,
}

// modelIDPrefixRules 定义模型 ID 前缀到派系的映射（兜底规则，优先级低于 provider 映射）。
// 按长度降序排列以确保最长前缀优先匹配。
var modelIDPrefixRules = []struct {
	prefix string
	family ModelFamily
}{
	// 国际
	{"claude-", ModelFamilyClaude},
	{"codex-", ModelFamilyOpenAI},
	{"gpt-", ModelFamilyOpenAI},
	{"o1-", ModelFamilyOpenAI},
	{"o3-", ModelFamilyOpenAI},
	{"o4-", ModelFamilyOpenAI},
	{"gemini-", ModelFamilyGemini},
	{"mistral-", ModelFamilyMistral},
	{"mixtral-", ModelFamilyMistral},
	// 国产
	{"deepseek-", ModelFamilyDeepSeek},
	{"qwen3-", ModelFamilyQwen},
	{"qwen-", ModelFamilyQwen},
	{"glm-", ModelFamilyGLM},
	{"kimi-", ModelFamilyKimi},
	{"mimo-", ModelFamilyMiMo},
	{"ernie-", ModelFamilyERNIE},
	{"doubao-", ModelFamilyDoubao},
	{"minimax-", ModelFamilyMiniMax},
	{"yi-", ModelFamilyYi},
	{"baichuan-", ModelFamilyBaichuan},
	{"step-", ModelFamilyStep},
	{"sensenova-", ModelFamilySenseNova},
	{"agnes-", ModelFamilyAgnes},
	{"longcat-", ModelFamilyLongCat},
	// 特殊
	{"ollama/", ModelFamilyLocal},
	{"lmstudio/", ModelFamilyLocal},
}

// InferModelFamily 从 provider 字段或模型 ID 前缀推导模型派系。
// 优先使用 provider 映射（来自模型注册表的显式标注），回退到 modelID 前缀匹配。
// provider 参数可为空，此时仅依赖 modelID 前缀匹配。
func InferModelFamily(modelID, provider string) ModelFamily {
	// 优先级 1：provider 显式映射
	if provider != "" {
		normalized := strings.ToLower(strings.TrimSpace(provider))
		if family, ok := providerFamilyMap[normalized]; ok {
			// xiaomi 的子系列细分：mimo-v2-* 归 mimo，agnes-* 归 agnes
			if family == ModelFamilyMiMo {
				if strings.HasPrefix(strings.ToLower(modelID), "agnes-") {
					return ModelFamilyAgnes
				}
			}
			return family
		}
	}

	// 优先级 2：modelID 前缀匹配
	lowerID := strings.ToLower(strings.TrimSpace(modelID))
	for _, rule := range modelIDPrefixRules {
		if strings.HasPrefix(lowerID, rule.prefix) {
			return rule.family
		}
	}

	return ModelFamilyUnknown
}

// ModelProfileQualityTierFromFamily 根据 ModelFamily 和 modelID 推导该模型的 QualityTier。
// 遵循设计 §3.4 QualityTier 推导规则（优先级 1：模型注册表中的模型族）。
func ModelProfileQualityTierFromFamily(family ModelFamily, modelID string) QualityTier {
	lowerID := strings.ToLower(modelID)

	switch family {
	case ModelFamilyClaude:
		if strings.Contains(lowerID, "opus") || strings.Contains(lowerID, "mythos") || strings.Contains(lowerID, "fable") {
			return QualityTierPremium
		}
		if strings.Contains(lowerID, "sonnet") {
			return QualityTierHigh
		}
		if strings.Contains(lowerID, "haiku") {
			return QualityTierNormal
		}
		return QualityTierNormal

	case ModelFamilyOpenAI:
		if strings.Contains(lowerID, "gpt-5.5") || strings.Contains(lowerID, "gpt-5.4") && !strings.Contains(lowerID, "mini") {
			return QualityTierPremium
		}
		if strings.Contains(lowerID, "gpt-5.3") || strings.Contains(lowerID, "gpt-5.2") {
			return QualityTierHigh
		}
		if strings.Contains(lowerID, "mini") {
			return QualityTierNormal
		}
		return QualityTierNormal

	case ModelFamilyGemini:
		if strings.Contains(lowerID, "ultra") || strings.Contains(lowerID, "pro") {
			return QualityTierHigh
		}
		return QualityTierNormal

	case ModelFamilyDeepSeek:
		if strings.Contains(lowerID, "v4-pro") {
			return QualityTierHigh
		}
		return QualityTierNormal

	case ModelFamilyKimi:
		if strings.Contains(lowerID, "k2.7") || strings.Contains(lowerID, "k2.6") {
			return QualityTierHigh
		}
		return QualityTierNormal

	case ModelFamilyGLM:
		if strings.Contains(lowerID, "glm-5") || strings.Contains(lowerID, "glm-5p2") {
			return QualityTierHigh
		}
		return QualityTierNormal

	case ModelFamilyQwen:
		if strings.Contains(lowerID, "max") {
			return QualityTierHigh
		}
		return QualityTierNormal

	default:
		return QualityTierLow
	}
}

// ── ModelProfile ──

// ModelProfile 是每个 (KeyEndpoint + 模型) 组合的画像。
type ModelProfile struct {
	// ── 锚定到 KeyEndpoint ──
	ChannelUID  string    `json:"channelUid"`
	ChannelID   int       `json:"channelId"` // 当前配置数组 index，仅用于展示/兼容
	ChannelKind string    `json:"channelKind"`
	ServiceType string    `json:"serviceType"`
	MetricsKey  string    `json:"metricsKey"` // 精确到 identityBaseURL + key + serviceType
	ModelID     string    `json:"modelId"`    // 该 endpoint 内的实际模型名
	UpdatedAt   time.Time `json:"updatedAt"`

	// ── 能力 ──
	ModelFamily       ModelFamily `json:"modelFamily"` // 派系，从注册表推导
	QualityTier       QualityTier `json:"qualityTier"` // 基于模型族的质量档
	SpeedTier         SpeedTier   `json:"speedTier"`
	ContextTokens     int         `json:"contextTokens"`
	SupportsVision    bool        `json:"supportsVision"`
	SupportsToolCalls bool        `json:"supportsToolCalls"`
	SupportsReasoning bool        `json:"supportsReasoning"`

	// ── 上游供应商质量（同模型在不同上游的质量差异）──
	ProviderQualityScore      float64 `json:"providerQualityScore,omitempty"`      // 0.0-1.0
	ProviderQualitySource     string  `json:"providerQualitySource,omitempty"`     // probe | user_feedback | inferred | default
	ProviderQualityConfidence float64 `json:"providerQualityConfidence,omitempty"` // 置信度

	// ── 探测结果 ──
	ProbeSuccess    bool      `json:"probeSuccess"`
	LastProbeAt     time.Time `json:"lastProbeAt"`
	ProbeLatencyMs  int64     `json:"probeLatencyMs"`
	ProbeConfidence float64   `json:"probeConfidence"`

	// ── 来源 ──
	Source string `json:"source"` // builtin_registry | auto_probe | capability_test | manual
}
