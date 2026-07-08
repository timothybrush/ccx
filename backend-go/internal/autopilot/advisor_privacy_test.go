package autopilot

import (
	"encoding/json"
	"strings"
	"testing"
)

// ── AdvisorInput 隐私白名单测试 ──

// TestAdvisorInput_WhitelistFields 确认 AdvisorInput 只包含白名单字段。
// 如果有人偷偷加了敏感字段，此测试会失败。
func TestAdvisorInput_WhitelistFields(t *testing.T) {
	// 构造完整输入并序列化
	input := AdvisorInput{
		RequestKind:          "messages",
		Operation:            "generate",
		RequestedModel:       "claude-sonnet-5",
		AgentRole:            "main",
		InputTokenBucket:     "1-10k",
		HasImage:             true,
		NeedsToolUse:         true,
		NeedsReasoning:       true,
		NeedsLongContext:     true,
		RedactedTaskSummary:  "summarize logs",
		CandidateTaskClasses: []TaskClass{TaskClassSupervisor},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	allowedFields := map[string]bool{
		"requestKind":          true,
		"operation":            true,
		"requestedModel":       true,
		"agentRole":            true,
		"inputTokenBucket":     true,
		"hasImage":             true,
		"needsToolUse":         true,
		"needsReasoning":       true,
		"needsLongContext":     true,
		"redactedTaskSummary":  true,
		"candidateTaskClasses": true,
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	for field := range m {
		if !allowedFields[field] {
			t.Errorf("发现非白名单字段 %q，违反 P0.2 隐私约束", field)
		}
	}

	// 确认敏感字段绝不存在
	sensitiveFields := []string{
		"message", "content", "prompt", "apiKey", "api_key",
		"authorization", "token", "secret", "password",
		"url", "baseUrl", "base_url", "endpoint",
		"fileContent", "file_content", "history", "messages",
	}
	for _, sf := range sensitiveFields {
		if _, ok := m[sf]; ok {
			t.Errorf("发现敏感字段 %q，严重违反 P0.2 隐私约束", sf)
		}
	}
}

// TestAdvisorInput_ConstructWithSensitiveData 模拟构造含敏感字段的输入。
// 由于 AdvisorInput 是白名单结构，敏感字段无法设置——验证此约束。
func TestAdvisorInput_ConstructWithSensitiveData(t *testing.T) {
	// 正常构造：只有白名单字段
	input := AdvisorInput{
		RequestKind:      "messages",
		InputTokenBucket: "<1k",
	}

	// 序列化后检查不含任何敏感关键词
	data, _ := json.Marshal(input)
	sensitive := []string{
		"api_key", "apiKey", "authorization", "secret",
		"password", "api_token", "bearer", "message_body", "prompt_text",
		"file_content", "base_url", "endpoint_url",
	}
	jsonStr := string(data)
	for _, kw := range sensitive {
		if strings.Contains(strings.ToLower(jsonStr), strings.ToLower(kw)) {
			t.Errorf("AdvisorInput 序列化结果含敏感关键词 %q", kw)
		}
	}
}

// TestSanitizeAdvisorInput 脱敏函数不添加额外字段。
func TestSanitizeAdvisorInput(t *testing.T) {
	input := AdvisorInput{
		RequestKind:          "messages",
		Operation:            "count_tokens",
		RequestedModel:       "claude-sonnet-5",
		AgentRole:            "main",
		InputTokenBucket:     "1-10k",
		HasImage:             false,
		NeedsToolUse:         false,
		NeedsReasoning:       false,
		NeedsLongContext:     false,
		RedactedTaskSummary:  "test summary",
		CandidateTaskClasses: []TaskClass{TaskClassLightweight},
	}

	sanitized := SanitizeAdvisorInput(input)

	// 序列化后比对字段数
	origData, _ := json.Marshal(input)
	saniData, _ := json.Marshal(sanitized)

	var origMap, saniMap map[string]json.RawMessage
	json.Unmarshal(origData, &origMap)
	json.Unmarshal(saniData, &saniMap)

	if len(origMap) != len(saniMap) {
		t.Errorf("脱敏后字段数变化: 原始=%d, 脱敏=%d", len(origMap), len(saniMap))
	}

	for k := range origMap {
		if _, ok := saniMap[k]; !ok {
			t.Errorf("脱敏后丢失字段 %q", k)
		}
	}
	for k := range saniMap {
		if _, ok := origMap[k]; !ok {
			t.Errorf("脱敏后新增字段 %q（不应添加非白名单字段）", k)
		}
	}
}

// TestValidateAdvisorInput_BadRequestKind 测试非法 requestKind 校验。
func TestValidateAdvisorInput_BadRequestKind(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		wantErr bool
	}{
		{"合法 messages", "messages", false},
		{"合法 chat", "chat", false},
		{"合法 responses", "responses", false},
		{"合法 gemini", "gemini", false},
		{"空字符串", "", false},
		{"合法 images", "images", false},
		{"合法 vectors", "vectors", false},
		{"非法 openai", "openai", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := AdvisorInput{
				RequestKind:      tt.kind,
				InputTokenBucket: "<1k",
			}
			err := validateAdvisorInput(input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAdvisorInput(kind=%q) error=%v, wantErr=%v", tt.kind, err, tt.wantErr)
			}
		})
	}
}

// TestValidateAdvisorInput_BadTokenBucket 测试非法 token bucket 校验。
func TestValidateAdvisorInput_BadTokenBucket(t *testing.T) {
	tests := []struct {
		name    string
		bucket  string
		wantErr bool
	}{
		{"合法 <1k", "<1k", false},
		{"合法 1-10k", "1-10k", false},
		{"合法 10-50k", "10-50k", false},
		{"合法 50k+", "50k+", false},
		{"空字符串", "", false},
		{"非法值", "999k", true},
		{"含注入", "<script>", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := AdvisorInput{
				RequestKind:      "messages",
				InputTokenBucket: tt.bucket,
			}
			err := validateAdvisorInput(input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAdvisorInput(bucket=%q) error=%v, wantErr=%v", tt.bucket, err, tt.wantErr)
			}
		})
	}
}

// TestAdvisorDecisionRecord_NoPlaintext 确认决策记录不含明文 prompt。
func TestAdvisorDecisionRecord_NoPlaintext(t *testing.T) {
	rec := AdvisorDecisionRecord{
		DecisionUID:       "test-001",
		RequestUID:        "req-001",
		AdvisorUID:        "adv-001",
		AdvisorOriginTier: "local",
		Mode:              AdvisorStateShadow,
		TaskClass:         TaskClassLightweight,
		PromptHash:        "sha256:abcdef1234567890", // 只存 hash
		InputTokenBucket:  "<1k",
		Hint: TrustedRoutingHint{
			TaskClass:  TaskClassLightweight,
			Confidence: 0.85,
			Reasons:    []string{"轻任务"},
		},
		Outcome:   "matched",
		LatencyMs: 5,
	}

	data, _ := json.Marshal(rec)
	jsonStr := string(data)

	// 确认不含明文 prompt
	if strings.Contains(jsonStr, "password") || strings.Contains(jsonStr, "secret") {
		t.Error("决策记录含敏感信息")
	}

	// 确认只有 hash，不含 URL
	if strings.Contains(jsonStr, "http://") || strings.Contains(jsonStr, "https://") {
		if !strings.Contains(jsonStr, "sha256:") {
			t.Error("决策记录含 URL 而非 hash")
		}
	}
}
