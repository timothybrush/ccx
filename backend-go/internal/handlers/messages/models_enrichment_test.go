package messages

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
)

// TestModelsHandler_FillsMaxOutputTokensFromBuiltinRegistry 验证 /v1/models 响应中
// 命中模型注册表的模型自动填充 max_output_tokens 字段。
// 回归 GitHub issue #266：Codex Desktop 依赖此字段识别模型输出上限。
func TestModelsHandler_FillsMaxOutputTokensFromBuiltinRegistry(t *testing.T) {
	tests := []struct {
		name                string
		modelID             string
		wantMaxOutputTokens bool // true 表示期望 > 0
	}{
		{
			name:                "Claude Opus 4.6 有 max_output_tokens",
			modelID:             "claude-opus-4-6",
			wantMaxOutputTokens: true,
		},
		{
			name:                "DeepSeek V4 Pro 有 max_output_tokens",
			modelID:             "deepseek-v4-pro",
			wantMaxOutputTokens: true,
		},
		{
			name:                "GPT-5.4 有 max_output_tokens",
			modelID:             "gpt-5.4",
			wantMaxOutputTokens: true,
		},
		{
			name:                "未知模型无 max_output_tokens",
			modelID:             "unknown-model-xyz",
			wantMaxOutputTokens: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"` + tt.modelID + `","object":"model"}]}`))
			}))
			defer upstream.Close()

			cfgManager := setupModelsConfigManager(t, config.Config{
				Upstream: []config.UpstreamConfig{{
					Name:        "test-channel",
					BaseURL:     upstream.URL,
					APIKeys:     []string{"sk-test"},
					ServiceType: "claude",
				}},
			})
			sch := newModelsTestScheduler(cfgManager)
			router := newModelsRouterForAggregate(&config.EnvConfig{ProxyAccessKey: "test-key"}, cfgManager, sch)

			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			req.Header.Set("Authorization", "Bearer test-key")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
			}

			var resp ModelsResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("解析响应失败: %v", err)
			}

			model := findModelEntry(resp.Data, tt.modelID)
			if model == nil {
				t.Fatalf("缺少模型 %s: %#v", tt.modelID, resp.Data)
			}

			if tt.wantMaxOutputTokens {
				if model.MaxOutputTokens <= 0 {
					t.Fatalf("max_output_tokens = %d, want > 0", model.MaxOutputTokens)
				}
			} else {
				if model.MaxOutputTokens != 0 {
					t.Fatalf("max_output_tokens = %d, want 0 for unknown model", model.MaxOutputTokens)
				}
			}
		})
	}
}

// TestModelsHandler_MaxOutputTokensNotOverriddenByUpstream 验证当上游已返回
// max_output_tokens 时，不被注册表覆盖。
func TestModelsHandler_MaxOutputTokensNotOverriddenByUpstream(t *testing.T) {
	// 上游返回带自定义 max_output_tokens 的模型
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"claude-opus-4-6","object":"model","max_output_tokens":999}]}`))
	}))
	defer upstream.Close()

	cfgManager := setupModelsConfigManager(t, config.Config{
		Upstream: []config.UpstreamConfig{{
			Name:        "test-channel",
			BaseURL:     upstream.URL,
			APIKeys:     []string{"sk-test"},
			ServiceType: "claude",
		}},
	})
	sch := newModelsTestScheduler(cfgManager)
	router := newModelsRouterForAggregate(&config.EnvConfig{ProxyAccessKey: "test-key"}, cfgManager, sch)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	var resp ModelsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	model := findModelEntry(resp.Data, "claude-opus-4-6")
	if model == nil {
		t.Fatalf("缺少模型: %#v", resp.Data)
	}
	// 上游已提供值应保留，不被注册表覆盖
	if model.MaxOutputTokens != 999 {
		t.Fatalf("max_output_tokens = %d, want 999（上游值应保留）", model.MaxOutputTokens)
	}
}

// TestMergeModels_MaxOutputTokensPropagated 验证 mergeModels 在去重时
// 正确传播 max_output_tokens。
func TestMergeModels_MaxOutputTokensPropagated(t *testing.T) {
	result := mergeModels(
		[]ModelEntry{{
			ID:              "model-a",
			Object:          "model",
			MaxOutputTokens: 128000,
		}},
		[]ModelEntry{{
			ID:     "model-a",
			Object: "model",
			// 第二个条目没有 max_output_tokens
		}},
	)

	if len(result) != 1 {
		t.Fatalf("结果数量 = %d, want 1", len(result))
	}
	if result[0].MaxOutputTokens != 128000 {
		t.Fatalf("max_output_tokens = %d, want 128000", result[0].MaxOutputTokens)
	}
}

// TestMergeModels_MaxOutputTokensFilledWhenFirstEmpty 验证当第一个条目缺少
// max_output_tokens 但第二个有值时，合并后保留有值的。
func TestMergeModels_MaxOutputTokensFilledWhenFirstEmpty(t *testing.T) {
	result := mergeModels(
		[]ModelEntry{{
			ID:     "model-b",
			Object: "model",
		}},
		[]ModelEntry{{
			ID:              "model-b",
			Object:          "model",
			MaxOutputTokens: 64000,
		}},
	)

	if len(result) != 1 {
		t.Fatalf("结果数量 = %d, want 1", len(result))
	}
	if result[0].MaxOutputTokens != 64000 {
		t.Fatalf("max_output_tokens = %d, want 64000", result[0].MaxOutputTokens)
	}
}

// TestModelsHandler_BothContextWindowAndMaxOutputTokensFilled 验证 context_window
// 和 max_output_tokens 同时填充。
func TestModelsHandler_BothContextWindowAndMaxOutputTokensFilled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"claude-opus-4-8","object":"model"}]}`))
	}))
	defer upstream.Close()

	cfgManager := setupModelsConfigManager(t, config.Config{
		Upstream: []config.UpstreamConfig{{
			Name:        "test-channel",
			BaseURL:     upstream.URL,
			APIKeys:     []string{"sk-test"},
			ServiceType: "claude",
		}},
	})
	sch := newModelsTestScheduler(cfgManager)
	router := newModelsRouterForAggregate(&config.EnvConfig{ProxyAccessKey: "test-key"}, cfgManager, sch)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	var resp ModelsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	model := findModelEntry(resp.Data, "claude-opus-4-8")
	if model == nil {
		t.Fatalf("缺少模型: %#v", resp.Data)
	}
	if model.ContextWindow <= 0 {
		t.Fatalf("context_window = %d, want > 0", model.ContextWindow)
	}
	if model.MaxOutputTokens <= 0 {
		t.Fatalf("max_output_tokens = %d, want > 0", model.MaxOutputTokens)
	}
	if !model.Supports1M {
		t.Fatalf("supports1m = false, want true")
	}
}

// TestModelsHandler_MaxOutputTokensInJSONResponse 验证 JSON 响应中
// max_output_tokens 字段名正确（小写下划线，与 Codex Desktop 期望一致）。
func TestModelsHandler_MaxOutputTokensInJSONResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"claude-opus-4-6","object":"model"}]}`))
	}))
	defer upstream.Close()

	cfgManager := setupModelsConfigManager(t, config.Config{
		Upstream: []config.UpstreamConfig{{
			Name:        "test-channel",
			BaseURL:     upstream.URL,
			APIKeys:     []string{"sk-test"},
			ServiceType: "claude",
		}},
	})
	sch := newModelsTestScheduler(cfgManager)
	router := newModelsRouterForAggregate(&config.EnvConfig{ProxyAccessKey: "test-key"}, cfgManager, sch)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	// 用 map[string]interface{} 解析以验证 JSON key 名称
	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := raw["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Fatalf("data 为空或格式错误: %#v", raw)
	}

	model, ok := data[0].(map[string]interface{})
	if !ok {
		t.Fatalf("data[0] 格式错误: %#v", data[0])
	}

	// 验证 max_output_tokens 字段存在且为数字
	maxTokens, exists := model["max_output_tokens"]
	if !exists {
		t.Fatal("JSON 响应中缺少 max_output_tokens 字段")
	}
	if _, ok := maxTokens.(float64); !ok {
		t.Fatalf("max_output_tokens 类型 = %T, want float64 (JSON number)", maxTokens)
	}

	// 验证 context_window 字段存在
	_, exists = model["context_window"]
	if !exists {
		t.Fatal("JSON 响应中缺少 context_window 字段")
	}
}

// TestModelsHandler_ConfiguredModelsFallbackIncludesMaxOutputTokens 验证在
// 实时发现失败、回退到 configuredModels 时，也填充 max_output_tokens。
func TestModelsHandler_ConfiguredModelsFallbackIncludesMaxOutputTokens(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	cfgManager := setupModelsConfigManager(t, config.Config{
		Upstream: []config.UpstreamConfig{{
			Name:            "test-channel",
			BaseURL:         upstream.URL,
			APIKeys:         []string{"sk-test"},
			ServiceType:     "claude",
			SupportedModels: []string{"claude-opus-4-8"},
		}},
	})
	sch := newModelsTestScheduler(cfgManager)
	router := newModelsRouterForAggregate(&config.EnvConfig{ProxyAccessKey: "test-key"}, cfgManager, sch)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	var resp ModelsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	model := findModelEntry(resp.Data, "claude-opus-4-8")
	if model == nil {
		t.Fatalf("缺少配置回退模型: %#v", resp.Data)
	}
	if model.ContextWindow <= 0 {
		t.Fatalf("context_window = %d, want > 0（配置回退也应填充）", model.ContextWindow)
	}
	if model.MaxOutputTokens <= 0 {
		t.Fatalf("max_output_tokens = %d, want > 0（配置回退也应填充）", model.MaxOutputTokens)
	}
}

// verify 模型列表排序使用独立的 scheduler 初始化
