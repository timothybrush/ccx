package autopilot

import (
	"testing"
)

// ── EvaluateShadow 表驱动测试 ──

func TestEvaluateShadow_DisabledState(t *testing.T) {
	advisor := NewTrustedRoutingAdvisor()
	_ = advisor.SetState(AdvisorStateDisabled)

	input := AdvisorInput{
		RequestKind:      "messages",
		InputTokenBucket: "<1k",
	}
	hint, err := advisor.EvaluateShadow(input)
	if err != nil {
		t.Fatalf("disabled 状态不应返回错误: %v", err)
	}
	if hint != nil {
		t.Fatal("disabled 状态应返回 nil hint")
	}
}

func TestEvaluateShadow_StateMachine(t *testing.T) {
	advisor := NewTrustedRoutingAdvisor()

	// Phase 2 起支持全部五态
	tests := []struct {
		name    string
		state   AdvisorState
		wantErr bool
	}{
		{"disabled 合法", AdvisorStateDisabled, false},
		{"shadow 合法", AdvisorStateShadow, false},
		{"candidate 合法", AdvisorStateCandidate, false},
		{"active 合法", AdvisorStateActive, false},
		{"rolled_back 合法", AdvisorStateRolledBack, false},
		{"未知状态 不允许", AdvisorState("unknown"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := advisor.SetState(tt.state)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetState(%s) error=%v, wantErr=%v", tt.state, err, tt.wantErr)
			}
		})
	}
}

func TestEvaluateShadow_TaskClassRouting(t *testing.T) {
	advisor := NewTrustedRoutingAdvisor()
	// 默认已是 shadow

	tests := []struct {
		name          string
		input         AdvisorInput
		wantTaskClass TaskClass
		wantNeverDown bool
		wantMinTier   QualityTier
		wantConfMin   float64
	}{
		{
			name: "原生生图 → image_generation",
			input: AdvisorInput{
				RequestKind:          "images",
				Operation:            "generations",
				InputTokenBucket:     "<1k",
				CandidateTaskClasses: []TaskClass{TaskClassImageGen},
			},
			wantTaskClass: TaskClassImageGen,
			wantNeverDown: true,
			wantMinTier:   QualityTierNormal,
			wantConfMin:   0.9,
		},
		{
			name: "embedding 任务",
			input: AdvisorInput{
				RequestKind:          "vectors",
				InputTokenBucket:     "1-10k",
				CandidateTaskClasses: []TaskClass{TaskClassEmbedding},
			},
			wantTaskClass: TaskClassEmbedding,
			wantNeverDown: true,
			wantMinTier:   QualityTierLow,
			wantConfMin:   0.9,
		},
		{
			name: "识图任务",
			input: AdvisorInput{
				RequestKind:          "messages",
				HasImage:             true,
				InputTokenBucket:     "1-10k",
				CandidateTaskClasses: []TaskClass{TaskClassVision},
			},
			wantTaskClass: TaskClassVision,
			wantNeverDown: true,
			wantMinTier:   QualityTierNormal,
			wantConfMin:   0.85,
		},
		{
			name: "长上下文任务",
			input: AdvisorInput{
				RequestKind:          "messages",
				NeedsLongContext:     true,
				InputTokenBucket:     "50k+",
				CandidateTaskClasses: []TaskClass{TaskClassLongContext},
			},
			wantTaskClass: TaskClassLongContext,
			wantNeverDown: true,
			wantMinTier:   QualityTierHigh,
			wantConfMin:   0.8,
		},
		{
			name: "轻任务 → 可降级",
			input: AdvisorInput{
				RequestKind:          "chat",
				Operation:            "count_tokens",
				InputTokenBucket:     "<1k",
				CandidateTaskClasses: []TaskClass{TaskClassLightweight},
			},
			wantTaskClass: TaskClassLightweight,
			wantNeverDown: false,
			wantMinTier:   QualityTierLow,
			wantConfMin:   0.8,
		},
		{
			name: "子代理 worker → 可降级",
			input: AdvisorInput{
				RequestKind:          "messages",
				AgentRole:            "subagent",
				InputTokenBucket:     "1-10k",
				CandidateTaskClasses: []TaskClass{TaskClassWorker},
			},
			wantTaskClass: TaskClassWorker,
			wantNeverDown: false,
			wantMinTier:   QualityTierNormal,
			wantConfMin:   0.6,
		},
		{
			name: "主代理 supervisor → 不降级",
			input: AdvisorInput{
				RequestKind:          "messages",
				AgentRole:            "main",
				InputTokenBucket:     "1-10k",
				CandidateTaskClasses: []TaskClass{TaskClassSupervisor},
			},
			wantTaskClass: TaskClassSupervisor,
			wantNeverDown: true,
			wantMinTier:   QualityTierHigh,
			wantConfMin:   0.75,
		},
		{
			name: "未知任务类型 → 保守",
			input: AdvisorInput{
				RequestKind:      "messages",
				InputTokenBucket: "1-10k",
			},
			wantTaskClass: TaskClassSupervisor, // 默认回退到 supervisor
			wantNeverDown: true,
			wantMinTier:   QualityTierHigh,
			wantConfMin:   0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint, err := advisor.EvaluateShadow(tt.input)
			if err != nil {
				t.Fatalf("EvaluateShadow 失败: %v", err)
			}
			if hint == nil {
				t.Fatal("shadow 状态不应返回 nil hint")
			}

			if hint.TaskClass != tt.wantTaskClass {
				t.Errorf("TaskClass = %s, want %s", hint.TaskClass, tt.wantTaskClass)
			}
			if hint.NeverDemote != tt.wantNeverDown {
				t.Errorf("NeverDemote = %v, want %v", hint.NeverDemote, tt.wantNeverDown)
			}
			if qualityTierRank(hint.SuggestedMinQualityTier) < qualityTierRank(tt.wantMinTier) {
				t.Errorf("SuggestedMinQualityTier = %s, want >= %s", hint.SuggestedMinQualityTier, tt.wantMinTier)
			}
			if hint.Confidence < tt.wantConfMin {
				t.Errorf("Confidence = %f, want >= %f", hint.Confidence, tt.wantConfMin)
			}
			if hint.GenerationMs < 0 {
				t.Errorf("GenerationMs = %d, want >= 0", hint.GenerationMs)
			}
			if hint.BackendType != "heuristic" {
				t.Errorf("BackendType = %s, want heuristic", hint.BackendType)
			}
			if len(hint.Reasons) == 0 {
				t.Error("Reasons 不应为空")
			}
		})
	}
}

// TestEvaluateShadow_50kBucket 测试 50k+ token 强制升级。
func TestEvaluateShadow_50kBucket(t *testing.T) {
	advisor := NewTrustedRoutingAdvisor()

	input := AdvisorInput{
		RequestKind:          "messages",
		AgentRole:            "subagent",
		InputTokenBucket:     "50k+",
		CandidateTaskClasses: []TaskClass{TaskClassWorker},
	}

	hint, err := advisor.EvaluateShadow(input)
	if err != nil {
		t.Fatalf("EvaluateShadow 失败: %v", err)
	}

	// 50k+ 应强制升级到 high 并设置 NeverDemote
	if hint.SuggestedMinQualityTier != QualityTierHigh && hint.SuggestedMinQualityTier != QualityTierPremium {
		t.Errorf("50k+ 应强制升级到 high+, got %s", hint.SuggestedMinQualityTier)
	}
	if !hint.NeverDemote {
		t.Error("50k+ 应设置 NeverDemote=true")
	}
}

// TestEvaluateShadow_ReasoningAdjustment 测试 reasoning 需求降低置信度。
func TestEvaluateShadow_ReasoningAdjustment(t *testing.T) {
	advisor := NewTrustedRoutingAdvisor()

	// 不含 reasoning
	inputBase := AdvisorInput{
		RequestKind:          "chat",
		Operation:            "count_tokens",
		InputTokenBucket:     "<1k",
		CandidateTaskClasses: []TaskClass{TaskClassLightweight},
	}
	hintBase, _ := advisor.EvaluateShadow(inputBase)

	// 含 reasoning
	inputReasoning := inputBase
	inputReasoning.NeedsReasoning = true
	inputReasoning.CandidateTaskClasses = []TaskClass{TaskClassLightweight}
	hintReasoning, _ := advisor.EvaluateShadow(inputReasoning)

	if hintReasoning.Confidence >= hintBase.Confidence {
		t.Errorf("含 reasoning 时置信度应降低: base=%f, reasoning=%f",
			hintBase.Confidence, hintReasoning.Confidence)
	}
}

// TestNewTrustedRoutingAdvisorWithBackend 测试自定义 backend。
func TestNewTrustedRoutingAdvisorWithBackend(t *testing.T) {
	customHint := &TrustedRoutingHint{
		TaskClass:  TaskClassLightweight,
		Confidence: 0.99,
		Reasons:    []string{"custom backend"},
	}
	backend := &mockBackend{hint: customHint}

	advisor := NewTrustedRoutingAdvisorWithBackend(AdvisorStateShadow, backend)
	input := AdvisorInput{
		RequestKind:      "messages",
		InputTokenBucket: "<1k",
	}

	hint, err := advisor.EvaluateShadow(input)
	if err != nil {
		t.Fatalf("EvaluateShadow 失败: %v", err)
	}
	if hint.Confidence != 0.99 {
		t.Errorf("自定义 backend 未生效: Confidence = %f, want 0.99", hint.Confidence)
	}
	if hint.BackendType != "mock" {
		t.Errorf("BackendType = %s, want mock", hint.BackendType)
	}
}

// mockBackend 测试用 mock。
type mockBackend struct {
	hint *TrustedRoutingHint
	err  error
}

func (m *mockBackend) Generate(input AdvisorInput) (*TrustedRoutingHint, error) {
	if m.err != nil {
		return nil, m.err
	}
	cp := *m.hint
	return &cp, nil
}

func (m *mockBackend) BackendType() string { return "mock" }

// ── CheckAndApplySLORollback 表驱动测试 ──

func TestCheckAndApplySLORollback_StreakBelowThreshold(t *testing.T) {
	advisor := NewTrustedRoutingAdvisorWithBackend(AdvisorStateActive, &mockBackend{})

	// 连续 2 轮 degrading（阈值=3），不应触发回滚
	for i := 0; i < 2; i++ {
		rolledBack := advisor.CheckAndApplySLORollback("ch-1", true, 3)
		if rolledBack != "" {
			t.Fatalf("第 %d 轮不应触发回滚，streak=%d < threshold=3", i+1, i+1)
		}
	}

	if advisor.State() != AdvisorStateActive {
		t.Errorf("2 轮 degrading 后 advisor 状态应保持 active, got %s", advisor.State())
	}
}

func TestCheckAndApplySLORollback_StreakReachingThreshold_Active(t *testing.T) {
	advisor := NewTrustedRoutingAdvisorWithBackend(AdvisorStateActive, &mockBackend{})

	// 连续 3 轮 degrading（阈值=3），第 3 轮应触发回滚
	for i := 0; i < 2; i++ {
		advisor.CheckAndApplySLORollback("ch-1", true, 3)
	}

	rolledBack := advisor.CheckAndApplySLORollback("ch-1", true, 3)
	if rolledBack != "ch-1" {
		t.Errorf("第 3 轮应回滚 ch-1, got %q", rolledBack)
	}
	if advisor.State() != AdvisorStateRolledBack {
		t.Errorf("触发回滚后 advisor 状态应为 rolled_back, got %s", advisor.State())
	}
}

func TestCheckAndApplySLORollback_StreakReachingThreshold_NotActive(t *testing.T) {
	tests := []struct {
		name  string
		state AdvisorState
	}{
		{"shadow 状态不触发回滚", AdvisorStateShadow},
		{"candidate 状态不触发回滚", AdvisorStateCandidate},
		{"disabled 状态不触发回滚", AdvisorStateDisabled},
		{"rolled_back 状态不触发回滚", AdvisorStateRolledBack},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			advisor := NewTrustedRoutingAdvisorWithBackend(tt.state, &mockBackend{})

			// 连续 3 轮 degrading，但 advisor 不在 active 状态
			for i := 0; i < 3; i++ {
				rolledBack := advisor.CheckAndApplySLORollback("ch-1", true, 3)
				if rolledBack != "" {
					t.Fatalf("非 active 状态不应触发回滚, got %q", rolledBack)
				}
			}

			if advisor.State() != tt.state {
				t.Errorf("状态不应改变: 原=%s, 现=%s", tt.state, advisor.State())
			}
		})
	}
}

func TestCheckAndApplySLORollback_NonDegradingResetsStreak(t *testing.T) {
	advisor := NewTrustedRoutingAdvisorWithBackend(AdvisorStateActive, &mockBackend{})

	// 连续 2 轮 degrading
	advisor.CheckAndApplySLORollback("ch-1", true, 3)
	advisor.CheckAndApplySLORollback("ch-1", true, 3)

	// 第 3 轮非 degrading → streak 归零
	advisor.CheckAndApplySLORollback("ch-1", false, 3)

	// 再连续 2 轮 degrading → 仍不应触发（streak 重置后从 1 开始）
	for i := 0; i < 2; i++ {
		rolledBack := advisor.CheckAndApplySLORollback("ch-1", true, 3)
		if rolledBack != "" {
			t.Fatalf("streak 重置后第 %d 轮不应触发回滚", i+1)
		}
	}

	if advisor.State() != AdvisorStateActive {
		t.Errorf("streak 重置后 2 轮 degrading 不应回滚, got %s", advisor.State())
	}
}

func TestCheckAndApplySLORollback_MultipleChannels(t *testing.T) {
	advisor := NewTrustedRoutingAdvisorWithBackend(AdvisorStateActive, &mockBackend{})

	// ch-1 连续 2 轮 degrading，ch-2 连续 2 轮 degrading
	advisor.CheckAndApplySLORollback("ch-1", true, 3)
	advisor.CheckAndApplySLORollback("ch-2", true, 3)
	advisor.CheckAndApplySLORollback("ch-1", true, 3)
	advisor.CheckAndApplySLORollback("ch-2", true, 3)

	// ch-2 第 3 轮触发回滚
	rolledBack := advisor.CheckAndApplySLORollback("ch-2", true, 3)
	if rolledBack != "ch-2" {
		t.Errorf("ch-2 第 3 轮应回滚, got %q", rolledBack)
	}

	if advisor.State() != AdvisorStateRolledBack {
		t.Errorf("任一渠道达到阈值应回滚 advisor, got %s", advisor.State())
	}
}

func TestCheckAndApplySLORollback_ZeroThresholdDefaultsTo3(t *testing.T) {
	advisor := NewTrustedRoutingAdvisorWithBackend(AdvisorStateActive, &mockBackend{})

	// threshold=0 应默认为 3
	for i := 0; i < 2; i++ {
		advisor.CheckAndApplySLORollback("ch-1", true, 0)
	}

	// 第 3 轮应触发（0 被当作 3）
	rolledBack := advisor.CheckAndApplySLORollback("ch-1", true, 0)
	if rolledBack != "ch-1" {
		t.Errorf("threshold=0 应默认为 3, 第 3 轮应回滚, got %q", rolledBack)
	}
}

func TestCheckAndApplySLORollback_RollbackClearsAllStreaks(t *testing.T) {
	advisor := NewTrustedRoutingAdvisorWithBackend(AdvisorStateActive, &mockBackend{})

	// ch-1 连续 3 轮触发回滚
	for i := 0; i < 3; i++ {
		advisor.CheckAndApplySLORollback("ch-1", true, 3)
	}

	// 回滚后 advisor 状态变为 rolled_back
	if advisor.State() != AdvisorStateRolledBack {
		t.Fatalf("应已回滚, got %s", advisor.State())
	}
	_ =

		// 手动重新激活
		advisor.SetState(AdvisorStateActive)

	// ch-2 从零开始计数（回滚时清零了所有 streaks）
	rolledBack := advisor.CheckAndApplySLORollback("ch-2", true, 3)
	if rolledBack != "" {
		t.Error("回滚清零后 ch-2 第 1 轮不应触发")
	}
}
