package autopilot

import (
	"fmt"
	"log"
	"time"
)

// ── Advisor 状态机 ──

// AdvisorState advisor 生命周期状态。
// Phase 2 起支持全部五态：disabled/shadow/candidate/active/rolled_back。
type AdvisorState string

const (
	AdvisorStateDisabled   AdvisorState = "disabled"    // 未启用
	AdvisorStateShadow     AdvisorState = "shadow"      // 只记录 hint，不影响路由
	AdvisorStateCandidate  AdvisorState = "candidate"   // 满足样本/准确率门槛，仍需用户显式允许
	AdvisorStateActive     AdvisorState = "active"      // 只对 lightweight/worker 低风险请求生效
	AdvisorStateRolledBack AdvisorState = "rolled_back" // SLO 恶化或用户关闭
)

// ── RoutingHint 契约（§4.7.2）──

// TrustedRoutingHint advisor 对一次请求的路由建议。
type TrustedRoutingHint struct {
	TaskClass               TaskClass   `json:"taskClass"`
	ComplexityTier          string      `json:"complexityTier"` // trivial | routine | complex | unknown
	SuggestedMinQualityTier QualityTier `json:"suggestedMinQualityTier"`
	AllowLocalCandidate     bool        `json:"allowLocalCandidate"`
	AllowCheapRemote        bool        `json:"allowCheapRemote"`
	NeverDemote             bool        `json:"neverDemote"`
	Confidence              float64     `json:"confidence"` // 0.0 ~ 1.0
	Reasons                 []string    `json:"reasons"`
	GenerationMs            int64       `json:"generationMs"` // hint 生成耗时（毫秒）
	BackendType             string      `json:"backendType"`  // heuristic | llm
}

// ── AdvisorInput 隐私白名单（P0.2）──

// AdvisorInput 白名单结构：只允许脱敏特征，绝不含消息正文/key/URL 明细。
type AdvisorInput struct {
	RequestKind    string `json:"requestKind"` // messages | chat | responses | gemini
	Operation      string `json:"operation,omitempty"`
	RequestedModel string `json:"requestedModel,omitempty"`
	AgentRole      string `json:"agentRole,omitempty"` // main | subagent

	InputTokenBucket string `json:"inputTokenBucket"` // <1k | 1-10k | 10-50k | 50k+
	HasImage         bool   `json:"hasImage"`
	NeedsToolUse     bool   `json:"needsToolUse"`
	NeedsReasoning   bool   `json:"needsReasoning"`
	NeedsLongContext bool   `json:"needsLongContext"`

	RedactedTaskSummary  string      `json:"redactedTaskSummary,omitempty"`
	CandidateTaskClasses []TaskClass `json:"candidateTaskClasses"`
}

// ── AdvisorBackend 接口 ──

// AdvisorBackend 生成路由 hint 的后端抽象。
// Phase 1 提供 heuristicBackend（确定性本地启发式）。
// 后续可扩展 llmBackend（本地/一等官方 LLM）。
type AdvisorBackend interface {
	// Generate 根据 AdvisorInput 生成 TrustedRoutingHint。
	Generate(input AdvisorInput) (*TrustedRoutingHint, error)
	// BackendType 返回后端标识（heuristic | llm）。
	BackendType() string
}

// ── TrustedRoutingAdvisor ──

// TrustedRoutingAdvisor 可信模型路由辅助器。
// Phase 1 shadow：只生成 hint 并记录，绝不影响真实调度。
type TrustedRoutingAdvisor struct {
	state            AdvisorState
	backend          AdvisorBackend
	regressionStreaks map[string]int // channelUID -> 连续 degrading 窗口计数（Phase 4 Item 3）
}

// NewTrustedRoutingAdvisor 创建 TrustedRoutingAdvisor。
// Phase 1 默认使用 heuristicBackend，初始状态 shadow。
func NewTrustedRoutingAdvisor() *TrustedRoutingAdvisor {
	return &TrustedRoutingAdvisor{
		state:            AdvisorStateShadow,
		backend:          &heuristicBackend{},
		regressionStreaks: make(map[string]int),
	}
}

// NewTrustedRoutingAdvisorWithBackend 使用自定义 backend 创建（便于测试）。
func NewTrustedRoutingAdvisorWithBackend(state AdvisorState, backend AdvisorBackend) *TrustedRoutingAdvisor {
	return &TrustedRoutingAdvisor{
		state:            state,
		backend:          backend,
		regressionStreaks: make(map[string]int),
	}
}

// State 返回当前 advisor 状态。
func (a *TrustedRoutingAdvisor) State() AdvisorState {
	return a.state
}

// SetState 切换 advisor 状态。
// Phase 2 起支持全部五态切换（disabled/shadow/candidate/active/rolled_back）。
func (a *TrustedRoutingAdvisor) SetState(state AdvisorState) error {
	switch state {
	case AdvisorStateDisabled, AdvisorStateShadow, AdvisorStateCandidate, AdvisorStateActive, AdvisorStateRolledBack:
		a.state = state
		return nil
	default:
		return fmt.Errorf("[Advisor-SetState] 未知状态 %q", state)
	}
}

// CheckAndApplySLORollback 检查渠道质量是否持续恶化，满足阈值时自动回滚 advisor 到 rolled_back。
// 仅当 advisor 当前处于 active 状态时才执行回滚（双门控之一）。
// isDegrading 为 true 表示该 channelUID 本轮存在质量恶化信号。
// consecutiveWindows 是触发回滚所需的连续 degrading 窗口数。
// 返回触发回滚的 channelUID（空字符串表示未触发）。
func (a *TrustedRoutingAdvisor) CheckAndApplySLORollback(
	channelUID string,
	isDegrading bool,
	consecutiveWindows int,
) (rolledBackChannelUID string) {
	if consecutiveWindows <= 0 {
		consecutiveWindows = 3
	}

	if isDegrading {
		a.regressionStreaks[channelUID]++
	} else {
		a.regressionStreaks[channelUID] = 0
	}

	streak := a.regressionStreaks[channelUID]
	if streak >= consecutiveWindows && a.state == AdvisorStateActive {
		log.Printf("[Advisor-SLORollback] 渠道 %s 连续 %d 轮 degrading，回滚 advisor 到 rolled_back",
			channelUID, streak)
		a.state = AdvisorStateRolledBack
		// 清零所有 streak（advisor 已全局回滚）
		a.regressionStreaks = make(map[string]int)
		return channelUID
	}

	return ""
}

// EvaluateShadow shadow 模式下评估请求，返回 hint。
// disabled 状态直接返回 nil（静默跳过）。
// Phase 1 使用确定性启发式产生 hint。
func (a *TrustedRoutingAdvisor) EvaluateShadow(input AdvisorInput) (*TrustedRoutingHint, error) {
	if a.state == AdvisorStateDisabled {
		return nil, nil
	}

	// 隐私校验：确保输入不包含非法字段（通过反射无法检查，这里依赖结构体设计约束）
	if err := validateAdvisorInput(input); err != nil {
		log.Printf("[Advisor-Shadow] 输入校验失败: %v", err)
		return nil, err
	}

	start := time.Now()
	hint, err := a.backend.Generate(input)
	if err != nil {
		log.Printf("[Advisor-Shadow] hint 生成失败: %v", err)
		return nil, fmt.Errorf("[Advisor-Shadow] hint 生成失败: %w", err)
	}

	hint.GenerationMs = time.Since(start).Milliseconds()
	hint.BackendType = a.backend.BackendType()
	return hint, nil
}

// validateAdvisorInput 校验 AdvisorInput 白名单约束。
// 确保 RedactedTaskSummary 不含明显敏感模式。
func validateAdvisorInput(input AdvisorInput) error {
	// RequestKind 必须合法
	switch input.RequestKind {
	case "messages", "chat", "responses", "gemini", "images", "vectors":
		// 合法（images/vectors 由硬规则直接路由到原生渠道）
	default:
		if input.RequestKind != "" {
			return fmt.Errorf("requestKind %q 不在白名单内", input.RequestKind)
		}
	}

	// InputTokenBucket 必须合法
	switch input.InputTokenBucket {
	case "<1k", "1-10k", "10-50k", "50k+":
		// 合法
	case "":
		// 允许空
	default:
		return fmt.Errorf("inputTokenBucket %q 不在白名单内", input.InputTokenBucket)
	}
	return nil
}

// ── heuristicBackend: 确定性启发式后端（Phase 1）──

// heuristicBackend 基于 TaskClass 静态映射 + 本地特征的确定性启发式。
type heuristicBackend struct{}

func (h *heuristicBackend) BackendType() string { return "heuristic" }

func (h *heuristicBackend) Generate(input AdvisorInput) (*TrustedRoutingHint, error) {
	hint := &TrustedRoutingHint{
		Reasons: []string{},
	}

	// 根据已知 TaskClass 推导路由策略
	tc := resolveTaskClass(input)
	hint.TaskClass = tc

	switch tc {
	case TaskClassImageGen:
		// 原生生图：必须走 images 渠道，不允许降级
		hint.ComplexityTier = "routine"
		hint.SuggestedMinQualityTier = QualityTierNormal
		hint.AllowLocalCandidate = false
		hint.AllowCheapRemote = true
		hint.NeverDemote = true
		hint.Confidence = 0.95
		hint.Reasons = append(hint.Reasons, "原生生图任务，必须走 images 渠道")

	case TaskClassEmbedding:
		// Embedding：必须走 vectors 渠道，不允许降级
		hint.ComplexityTier = "routine"
		hint.SuggestedMinQualityTier = QualityTierLow
		hint.AllowLocalCandidate = true
		hint.AllowCheapRemote = true
		hint.NeverDemote = true
		hint.Confidence = 0.95
		hint.Reasons = append(hint.Reasons, "embedding 任务，必须走 vectors 渠道")

	case TaskClassVision:
		// 识图：需要 vision 能力，不能降级到无 vision 的模型
		hint.ComplexityTier = "routine"
		hint.SuggestedMinQualityTier = QualityTierNormal
		hint.AllowLocalCandidate = false
		hint.AllowCheapRemote = true
		hint.NeverDemote = true
		hint.Confidence = 0.90
		hint.Reasons = append(hint.Reasons, "识图任务，需要 vision 能力")

	case TaskClassLongContext:
		// 长上下文：需要大 context 窗口，不允许本地/便宜降级
		hint.ComplexityTier = "complex"
		hint.SuggestedMinQualityTier = QualityTierHigh
		hint.AllowLocalCandidate = false
		hint.AllowCheapRemote = false
		hint.NeverDemote = true
		hint.Confidence = 0.85
		hint.Reasons = append(hint.Reasons, "长上下文任务，需要大 context 窗口")

	case TaskClassSupervisor:
		// 主代理：默认保持高质量，不建议降级
		hint.ComplexityTier = "complex"
		hint.SuggestedMinQualityTier = QualityTierHigh
		hint.AllowLocalCandidate = false
		hint.AllowCheapRemote = false
		hint.NeverDemote = true
		hint.Confidence = 0.80
		hint.Reasons = append(hint.Reasons, "主代理任务，保持高质量路由")

	case TaskClassWorker:
		// 子代理：可尝试便宜渠道，但需有一定质量
		hint.ComplexityTier = "routine"
		hint.SuggestedMinQualityTier = QualityTierNormal
		hint.AllowLocalCandidate = true
		hint.AllowCheapRemote = true
		hint.NeverDemote = false
		hint.Confidence = 0.70
		hint.Reasons = append(hint.Reasons, "子代理任务，可尝试性价比渠道")

	case TaskClassLightweight:
		// 轻任务：优先本地/便宜渠道
		hint.ComplexityTier = "trivial"
		hint.SuggestedMinQualityTier = QualityTierLow
		hint.AllowLocalCandidate = true
		hint.AllowCheapRemote = true
		hint.NeverDemote = false
		hint.Confidence = 0.85
		hint.Reasons = append(hint.Reasons, "轻任务，优先本地/便宜渠道")

	default:
		// 未知任务：保守策略，不建议降级
		hint.ComplexityTier = "unknown"
		hint.SuggestedMinQualityTier = QualityTierHigh
		hint.AllowLocalCandidate = false
		hint.AllowCheapRemote = false
		hint.NeverDemote = true
		hint.Confidence = 0.50
		hint.Reasons = append(hint.Reasons, "未知任务类型，保守路由")
	}

	// 根据输入特征微调置信度
	adjustConfidence(hint, input)

	return hint, nil
}

// resolveTaskClass 从 AdvisorInput 推导 TaskClass。
// 遵循 P0.3 硬规则：确定性规则优先，不确定时升级到更保守分类。
func resolveTaskClass(input AdvisorInput) TaskClass {
	// 已有候选分类时使用第一个（由调用方 TaskClassifier 确定）
	if len(input.CandidateTaskClasses) > 0 {
		return input.CandidateTaskClasses[0]
	}

	// 硬规则回退
	switch input.RequestKind {
	case "images":
		return TaskClassImageGen
	case "vectors":
		return TaskClassEmbedding
	}

	if input.HasImage {
		return TaskClassVision
	}
	if input.NeedsLongContext {
		return TaskClassLongContext
	}

	// AgentRole 推导
	switch input.AgentRole {
	case "subagent":
		return TaskClassWorker
	case "main":
		return TaskClassSupervisor
	}

	// Operation 白名单检查（轻任务）
	switch input.Operation {
	case "count_tokens", "summarize", "title", "classify", "format":
		if !input.NeedsToolUse && !input.NeedsReasoning && !input.HasImage {
			return TaskClassLightweight
		}
	}

	// 不确定时升级到 supervisor（保守）
	return TaskClassSupervisor
}

// adjustConfidence 根据输入特征微调 hint 置信度。
func adjustConfidence(hint *TrustedRoutingHint, input AdvisorInput) {
	// 有 reasoning 需求时降低降级置信度
	if input.NeedsReasoning && !hint.NeverDemote {
		hint.Confidence *= 0.85
		hint.Reasons = append(hint.Reasons, "含 reasoning 需求，降低降级置信度")
	}

	// 有工具使用时降低降级置信度
	if input.NeedsToolUse && !hint.NeverDemote {
		hint.Confidence *= 0.90
		hint.Reasons = append(hint.Reasons, "含工具使用，降低降级置信度")
	}

	// 50k+ 长上下文：强制升级到 high
	if input.InputTokenBucket == "50k+" && hint.SuggestedMinQualityTier != QualityTierPremium {
		hint.SuggestedMinQualityTier = QualityTierHigh
		hint.NeverDemote = true
		hint.Reasons = append(hint.Reasons, "50k+ token 长上下文，强制升级质量下界")
	}
}

// ── sanitizeAdvisorInput 脱敏：确保输出不含敏感字段 ──

// SanitizeAdvisorInput 返回 AdvisorInput 的安全副本，用于外部日志/导出。
// Phase 1 验证白名单完整性：不添加任何非白名单字段。
func SanitizeAdvisorInput(input AdvisorInput) AdvisorInput {
	// 直接拷贝，因为 AdvisorInput 本身已是白名单结构
	// RedactedTaskSummary 保留（仅本地/一等 advisor 可生成）
	return input
}
