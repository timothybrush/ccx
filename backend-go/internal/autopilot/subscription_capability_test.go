package autopilot

import (
	"testing"
)

// ── BuildSharedCapability 测试 ──

func TestBuildSharedCapability_NilEndpoints(t *testing.T) {
	got := BuildSharedCapability(nil)
	if got != nil {
		t.Errorf("BuildSharedCapability(nil) 应返回 nil, 得到 %+v", got)
	}
}

func TestBuildSharedCapability_EmptyEndpoints(t *testing.T) {
	got := BuildSharedCapability([]*KeyEndpointProfile{})
	if got != nil {
		t.Errorf("BuildSharedCapability(空) 应返回 nil, 得到 %+v", got)
	}
}

func TestBuildSharedCapability_SingleEndpoint(t *testing.T) {
	ep := &KeyEndpointProfile{
		EndpointUID:       "ep-001",
		AvailableModels:   []string{"gpt-4o", "gpt-4o-mini"},
		SupportsVision:    true,
		SupportsToolCalls: true,
	}

	sc := BuildSharedCapability([]*KeyEndpointProfile{ep})
	if sc == nil {
		t.Fatal("单 endpoint 不应返回 nil")
	}
	if sc.TotalEndpoints != 1 {
		t.Errorf("TotalEndpoints = %d, 期望 1", sc.TotalEndpoints)
	}
	if sc.ConsistentCount != 1 {
		t.Errorf("ConsistentCount = %d, 期望 1", sc.ConsistentCount)
	}
	if !sc.SupportsVision {
		t.Error("SupportsVision 应为 true")
	}
	if !sc.SupportsToolCalls {
		t.Error("SupportsToolCalls 应为 true")
	}
	if sc.SupportsReasoning {
		t.Error("SupportsReasoning 应为 false")
	}
	if sc.ModelListHash == "" {
		t.Error("ModelListHash 不应为空")
	}
	if len(sc.ModelList) != 2 {
		t.Errorf("ModelList 长度 = %d, 期望 2", len(sc.ModelList))
	}
	if sc.ProbedAt.IsZero() {
		t.Error("ProbedAt 不应为零值")
	}
}

func TestBuildSharedCapability_MajorityVote(t *testing.T) {
	// 3 个 endpoint，2 个有相同的模型列表，1 个不同
	cases := []struct {
		name             string
		endpoints        []*KeyEndpointProfile
		wantModels       int
		wantConsistent   int
		wantInconsistent int
		wantVision       bool
		wantReasoning    bool
	}{
		{
			name: "多数派一致",
			endpoints: []*KeyEndpointProfile{
				{EndpointUID: "ep-001", AvailableModels: []string{"gpt-4o", "gpt-4o-mini"}, SupportsVision: true},
				{EndpointUID: "ep-002", AvailableModels: []string{"gpt-4o", "gpt-4o-mini"}, SupportsVision: true},
				{EndpointUID: "ep-003", AvailableModels: []string{"claude-3", "claude-3-haiku"}, SupportsVision: false},
			},
			wantModels:       2, // 多数派 (gpt-4o, gpt-4o-mini)
			wantConsistent:   2,
			wantInconsistent: 1,
			wantVision:       true,
			wantReasoning:    false,
		},
		{
			name: "全部一致",
			endpoints: []*KeyEndpointProfile{
				{EndpointUID: "ep-001", AvailableModels: []string{"a", "b"}, SupportsToolCalls: true},
				{EndpointUID: "ep-002", AvailableModels: []string{"a", "b"}},
				{EndpointUID: "ep-003", AvailableModels: []string{"a", "b"}},
			},
			wantModels:       2,
			wantConsistent:   3,
			wantInconsistent: 0,
			wantVision:       false,
			wantReasoning:    false,
		},
		{
			name: "全不相同取并集",
			endpoints: []*KeyEndpointProfile{
				{EndpointUID: "ep-001", AvailableModels: []string{"a"}, SupportsReasoning: true},
				{EndpointUID: "ep-002", AvailableModels: []string{"b"}},
				{EndpointUID: "ep-003", AvailableModels: []string{"c"}},
			},
			wantModels:       3, // 并集 {a,b,c}
			wantConsistent:   0, // 全部与并集哈希不一致
			wantInconsistent: 3,
			wantVision:       false,
			wantReasoning:    true,
		},
		{
			name: "能力并集",
			endpoints: []*KeyEndpointProfile{
				{EndpointUID: "ep-001", AvailableModels: []string{"x"}, SupportsVision: true},
				{EndpointUID: "ep-002", AvailableModels: []string{"x"}, SupportsToolCalls: true},
				{EndpointUID: "ep-003", AvailableModels: []string{"x"}, SupportsReasoning: true, SupportsLongCtx: true},
			},
			wantModels:       1,
			wantConsistent:   3,
			wantInconsistent: 0,
			wantVision:       true,
			wantReasoning:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := BuildSharedCapability(tc.endpoints)
			if sc == nil {
				t.Fatal("BuildSharedCapability 返回 nil")
			}
			if len(sc.ModelList) != tc.wantModels {
				t.Errorf("ModelList 长度 = %d, 期望 %d, 内容: %v", len(sc.ModelList), tc.wantModels, sc.ModelList)
			}
			if sc.ConsistentCount != tc.wantConsistent {
				t.Errorf("ConsistentCount = %d, 期望 %d", sc.ConsistentCount, tc.wantConsistent)
			}
			if len(sc.InconsistentKeys) != tc.wantInconsistent {
				t.Errorf("InconsistentKeys 长度 = %d, 期望 %d", len(sc.InconsistentKeys), tc.wantInconsistent)
			}
			if sc.SupportsVision != tc.wantVision {
				t.Errorf("SupportsVision = %v, 期望 %v", sc.SupportsVision, tc.wantVision)
			}
			if sc.SupportsReasoning != tc.wantReasoning {
				t.Errorf("SupportsReasoning = %v, 期望 %v", sc.SupportsReasoning, tc.wantReasoning)
			}
			// ModelList 已排序
			for i := 1; i < len(sc.ModelList); i++ {
				if sc.ModelList[i-1] >= sc.ModelList[i] {
					t.Errorf("ModelList 未排序: %v", sc.ModelList)
					break
				}
			}
		})
	}
}

func TestBuildSharedCapability_UsesModelListHash(t *testing.T) {
	// endpoint 已有 ModelListHash，应直接使用
	ep := &KeyEndpointProfile{
		EndpointUID:     "ep-hash",
		AvailableModels: []string{"a", "b"},
		ModelListHash:   hashModelList([]string{"a", "b"}),
	}

	sc := BuildSharedCapability([]*KeyEndpointProfile{ep})
	if sc.ModelListHash != ep.ModelListHash {
		t.Errorf("应复用已有 ModelListHash: got=%s want=%s", sc.ModelListHash, ep.ModelListHash)
	}
}

// ── DetectCapabilityDrift 测试 ──

func TestDetectCapabilityDrift(t *testing.T) {
	shared := &SharedCapability{
		ModelListHash:     hashModelList([]string{"a", "b", "c"}),
		ModelList:         []string{"a", "b", "c"},
		SupportsVision:    true,
		SupportsToolCalls: true,
		SupportsReasoning: false,
		SupportsLongCtx:   true,
	}

	cases := []struct {
		name         string
		shared       *SharedCapability
		endpoint     *KeyEndpointProfile
		wantDiffs    int
		wantContains string
	}{
		{
			name:   "完全一致无差异",
			shared: shared,
			endpoint: &KeyEndpointProfile{
				EndpointUID:       "ep-001",
				AvailableModels:   []string{"a", "b", "c"},
				ModelListHash:     hashModelList([]string{"a", "b", "c"}),
				SupportsVision:    true,
				SupportsToolCalls: true,
				SupportsReasoning: false,
				SupportsLongCtx:   true,
			},
			wantDiffs: 0,
		},
		{
			name:   "模型列表有新增",
			shared: shared,
			endpoint: &KeyEndpointProfile{
				EndpointUID:       "ep-002",
				AvailableModels:   []string{"a", "b", "c", "d"},
				SupportsVision:    true,
				SupportsToolCalls: true,
				SupportsLongCtx:   true,
			},
			wantDiffs:    1,
			wantContains: "新增",
		},
		{
			name:   "模型列表有缺失",
			shared: shared,
			endpoint: &KeyEndpointProfile{
				EndpointUID:       "ep-003",
				AvailableModels:   []string{"a", "b"},
				SupportsVision:    true,
				SupportsToolCalls: true,
				SupportsLongCtx:   true,
			},
			wantDiffs:    1,
			wantContains: "缺失",
		},
		{
			name:   "能力标签不一致",
			shared: shared,
			endpoint: &KeyEndpointProfile{
				EndpointUID:       "ep-004",
				AvailableModels:   []string{"a", "b", "c"},
				ModelListHash:     hashModelList([]string{"a", "b", "c"}),
				SupportsVision:    false, // 缺少 vision
				SupportsToolCalls: false, // 缺少 tool calls
				SupportsLongCtx:   true,
			},
			wantDiffs:    2,
			wantContains: "视觉",
		},
		{
			name:   "模型和能力都不一致",
			shared: shared,
			endpoint: &KeyEndpointProfile{
				EndpointUID:       "ep-005",
				AvailableModels:   []string{"x"},
				SupportsVision:    false,
				SupportsToolCalls: false,
				SupportsLongCtx:   false,
			},
			wantDiffs: 5, // 模型新增+缺失 + vision + tool + longctx
		},
		{
			name:      "shared 为 nil 不报告差异",
			shared:    nil,
			endpoint:  &KeyEndpointProfile{EndpointUID: "ep-nil"},
			wantDiffs: 0,
		},
		{
			name:      "endpoint 为 nil 不报告差异",
			shared:    shared,
			endpoint:  nil,
			wantDiffs: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diffs := DetectCapabilityDrift(tc.shared, tc.endpoint)
			if len(diffs) != tc.wantDiffs {
				t.Errorf("差异条数 = %d, 期望 %d: %v", len(diffs), tc.wantDiffs, diffs)
			}
			if tc.wantContains != "" && len(diffs) > 0 {
				found := false
				for _, d := range diffs {
					if contains(d, tc.wantContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("期望差异包含 %q, 实际: %v", tc.wantContains, diffs)
				}
			}
		})
	}
}

func TestDetectCapabilityDrift_ModelListHashConsistency(t *testing.T) {
	// 确保用 ModelListHash 和用 AvailableModels 计算的哈希一致
	models := []string{"gpt-4o", "claude-3", "gemini-pro"}
	shared := &SharedCapability{
		ModelListHash: hashModelList(models),
		ModelList:     sortedCopy(models),
	}

	// endpoint 没有 ModelListHash，但 AvailableModels 相同
	ep := &KeyEndpointProfile{
		EndpointUID:     "ep-no-hash",
		AvailableModels: []string{"gemini-pro", "gpt-4o", "claude-3"}, // 未排序
	}

	diffs := DetectCapabilityDrift(shared, ep)
	if len(diffs) != 0 {
		t.Errorf("同模型不同顺序应无差异, 实际: %v", diffs)
	}
}

// ── majorityModelList 测试 ──

func TestMajorityModelList(t *testing.T) {
	cases := []struct {
		name      string
		endpoints []*KeyEndpointProfile
		wantCount int
	}{
		{
			name:      "单个 endpoint",
			endpoints: []*KeyEndpointProfile{{AvailableModels: []string{"a"}}},
			wantCount: 1,
		},
		{
			name: "2:1 多数派",
			endpoints: []*KeyEndpointProfile{
				{AvailableModels: []string{"a", "b"}},
				{AvailableModels: []string{"a", "b"}},
				{AvailableModels: []string{"x"}},
			},
			wantCount: 2,
		},
		{
			name: "全不同时取并集",
			endpoints: []*KeyEndpointProfile{
				{AvailableModels: []string{"a"}},
				{AvailableModels: []string{"b"}},
				{AvailableModels: []string{"c"}},
			},
			wantCount: 3,
		},
		{
			name: "2:2 相同票数取较长",
			endpoints: []*KeyEndpointProfile{
				{AvailableModels: []string{"a", "b"}},
				{AvailableModels: []string{"a", "b"}},
				{AvailableModels: []string{"x", "y", "z"}},
				{AvailableModels: []string{"x", "y", "z"}},
			},
			wantCount: 3, // 取较长的 {x,y,z}
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := majorityModelList(tc.endpoints)
			if len(result) != tc.wantCount {
				t.Errorf("结果长度 = %d, 期望 %d: %v", len(result), tc.wantCount, result)
			}
		})
	}
}

// ── joinStrings 测试 ──

func TestJoinStrings(t *testing.T) {
	cases := []struct {
		name     string
		ss       []string
		sep      string
		expected string
	}{
		{"空切片", nil, ", ", ""},
		{"单元素", []string{"a"}, ", ", "a"},
		{"多元素", []string{"a", "b", "c"}, ", ", "a, b, c"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := joinStrings(tc.ss, tc.sep)
			if got != tc.expected {
				t.Errorf("joinStrings(%v, %q) = %q, 期望 %q", tc.ss, tc.sep, got, tc.expected)
			}
		})
	}
}

// ── 测试辅助 ──

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
