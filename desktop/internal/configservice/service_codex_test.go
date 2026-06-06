package configservice

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetStatusCodex_NoConfig(t *testing.T) {
	svc := newTestService(t)
	status, err := svc.GetStatus(PlatformCodex, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Configured {
		t.Error("should not be configured when no config.toml exists")
	}
}

func TestApplyAndRestoreCodex(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	err := svc.Apply(ApplyAgentConfigRequest{Platform: PlatformCodex}, 3688, "test-key")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// 验证 config.toml — 新格式 openai_base_url 模式
	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), `model_provider = "openai"`) {
		t.Errorf("config.toml should contain model_provider = openai")
	}
	if !strings.Contains(string(content), `openai_base_url = "http://127.0.0.1:3688/v1"`) {
		t.Errorf("config.toml should contain openai_base_url")
	}
	if strings.Contains(string(content), `[model_providers.ccx]`) {
		t.Errorf("config.toml should NOT contain legacy [model_providers.ccx] block")
	}

	// 验证 auth.json
	var authData map[string]any
	readJSON(authPath, &authData)
	if authData["OPENAI_API_KEY"] != "test-key" {
		t.Errorf("auth OPENAI_API_KEY = %v", authData["OPENAI_API_KEY"])
	}

	// Restore
	err = svc.Restore(PlatformCodex)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
}

func TestGetStatusCodex_ThirdPartyProvider(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	// 模拟已配置 dashscope 的 config.toml
	tomlContent := `model_provider = "dashscope"

[model_providers.dashscope]
base_url = "https://dashscope.aliyuncs.com/compatible-mode/v1"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "sk-ds-key"})

	status, err := svc.GetStatus(PlatformCodex, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderDashScope {
		t.Errorf("Provider = %q, want %q", status.Provider, ProviderDashScope)
	}
	if !status.Configured {
		t.Error("Configured should be true for third-party provider")
	}
}

func TestGetStatusCodex_OpenCodeZenProvider(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	tomlContent := `model_provider = "opencode-zen"

[model_providers.opencode-zen]
base_url = "https://api.opencode.ai/v1"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "sk-zen-key"})

	status, err := svc.GetStatus(PlatformCodex, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderOpenCodeZen {
		t.Errorf("Provider = %q, want %q", status.Provider, ProviderOpenCodeZen)
	}
	if !status.Configured {
		t.Error("Configured should be true for opencode-zen provider")
	}
}

// ── P1 回归: Codex 恢复清理第三方 provider block ──

func TestApplyAndRestoreCodex_ThirdPartyCleanup(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	// 先 Apply 第三方 provider
	err := svc.Apply(ApplyAgentConfigRequest{
		Platform: PlatformCodex,
		Provider: ProviderDashScope,
		APIKey:   "sk-ds-key",
		BaseURL:  "https://dashscope.aliyuncs.com/compatible-mode/v1",
	}, 0, "")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	s := string(content)
	if !strings.Contains(s, `model_provider = "dashscope"`) {
		t.Error("config.toml should contain model_provider = dashscope after apply")
	}
	if !strings.Contains(s, `[model_providers.dashscope]`) {
		t.Error("config.toml should contain dashscope provider block after apply")
	}
	if !strings.Contains(s, `experimental_bearer_token = "sk-ds-key"`) {
		t.Error("config.toml should contain experimental_bearer_token for third-party plugin mode")
	}
	if strings.Contains(s, `env_key = "OPENAI_API_KEY"`) {
		t.Error("config.toml should not contain env_key in third-party plugin mode")
	}
	authData, _, _ := readJSONMap(authPath)
	if authData["OPENAI_API_KEY"] != "sk-ds-key" {
		t.Errorf("OPENAI_API_KEY = %v, want sk-ds-key", authData["OPENAI_API_KEY"])
	}
	if authData["auth_mode"] != "chatgpt" {
		t.Errorf("auth_mode = %v, want chatgpt", authData["auth_mode"])
	}

	// Restore
	err = svc.Restore(PlatformCodex)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	content, _ = os.ReadFile(configPath)
	s = string(content)
	if strings.Contains(s, `[model_providers.dashscope]`) {
		t.Error("config.toml should NOT contain dashscope provider block after restore")
	}
}

// ── Codex openai_base_url 新格式测试 ──

func TestGetStatusCodex_NewStyleCCXProxy(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	tomlContent := `model_provider = "openai"
openai_base_url = "http://127.0.0.1:3688/v1"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "test-key"})

	status, err := svc.GetStatus(PlatformCodex, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderCCX {
		t.Errorf("Provider = %q, want %q", status.Provider, ProviderCCX)
	}
	if !status.Configured {
		t.Error("Configured should be true for new-style CCX proxy")
	}
	if !status.MatchesCurrentPort {
		t.Error("MatchesCurrentPort should be true when port matches")
	}
}

func TestGetStatusCodex_NewStyleCCXProxy_WrongPort(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	tomlContent := `model_provider = "openai"
openai_base_url = "http://127.0.0.1:9999/v1"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "test-key"})

	status, err := svc.GetStatus(PlatformCodex, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderCCX {
		t.Errorf("Provider = %q, want %q", status.Provider, ProviderCCX)
	}
	if !status.NeedsUpdate {
		t.Error("NeedsUpdate should be true when port mismatches")
	}
	if status.MatchesCurrentPort {
		t.Error("MatchesCurrentPort should be false when port mismatches")
	}
}

func TestGetStatusCodex_OpenAIDirect(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	tomlContent := `model_provider = "openai"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "sk-openai-key"})

	status, err := svc.GetStatus(PlatformCodex, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderOpenAI {
		t.Errorf("Provider = %q, want %q", status.Provider, ProviderOpenAI)
	}
	if !status.Configured {
		t.Error("Configured should be true for OpenAI direct")
	}
}

func TestGetStatusCodex_LegacyCCX(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	tomlContent := `model_provider = "ccx"

[model_providers.ccx]
name = "CCX Proxy"
base_url = "http://127.0.0.1:3688/v1"
wire_api = "responses"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "test-key"})

	status, err := svc.GetStatus(PlatformCodex, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderCCX {
		t.Errorf("Provider = %q, want %q", status.Provider, ProviderCCX)
	}
	if !status.MatchesCurrentPort {
		t.Error("MatchesCurrentPort should be true for legacy CCX with matching port")
	}
}

func TestApplyCodexOpenAI_RemovesBaseURL(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	// 先写入一个含 openai_base_url 的 config
	tomlContent := `model_provider = "openai"
openai_base_url = "http://127.0.0.1:3688/v1"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "sk-openai-key"})

	// 应用 OpenAI direct（apiKey 须通过 req.APIKey 传入）
	err := svc.Apply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderOpenAI, APIKey: "sk-openai-key"}, 0, "")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	s := string(content)
	if strings.Contains(s, "openai_base_url") {
		t.Error("config.toml should NOT contain openai_base_url after switching to OpenAI direct")
	}
	if !strings.Contains(s, `model_provider = "openai"`) {
		t.Error("config.toml should contain model_provider = openai")
	}
}

func TestApplyCodexOpenAI_OAuthMode(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	// 先写入一个含 openai_base_url 的 config（CCX 代理残留）
	tomlContent := `model_provider = "openai"
openai_base_url = "http://127.0.0.1:3688/v1"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "ccx-proxy-key", "auth_mode": "chatgpt"})

	// 应用 OpenAI direct，不提供 apiKey → 应走 OAuth 模式
	err := svc.Apply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderOpenAI}, 0, "")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// 验证 config.toml 清理了 proxy 残留
	configContent, _ := os.ReadFile(configPath)
	s := string(configContent)
	if strings.Contains(s, "openai_base_url") {
		t.Error("config.toml should NOT contain openai_base_url after switching to OpenAI direct")
	}
	if !strings.Contains(s, `model_provider = "openai"`) {
		t.Error("config.toml should contain model_provider = openai")
	}

	// 验证 auth.json：auth_mode = "chatgpt"，OPENAI_API_KEY = nil
	authContent, _ := os.ReadFile(authPath)
	var authData map[string]any
	json.Unmarshal(authContent, &authData)
	if authData["auth_mode"] != "chatgpt" {
		t.Errorf("auth.json should have auth_mode = chatgpt, got %v", authData["auth_mode"])
	}
	if authData["OPENAI_API_KEY"] != nil {
		t.Errorf("auth.json should have OPENAI_API_KEY = null, got %v", authData["OPENAI_API_KEY"])
	}
}

func TestApplyCodexOpenAI_WithApiKey(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	tomlContent := `model_provider = "ccx"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": nil, "auth_mode": "chatgpt"})

	// 应用 OpenAI direct，提供了 apiKey → 应写入 key 并设置 auth_mode = "apikey"
	err := svc.Apply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderOpenAI, APIKey: "sk-my-openai-key"}, 0, "")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// 验证 auth.json：OPENAI_API_KEY 写入，auth_mode = "apikey"
	authContent, _ := os.ReadFile(authPath)
	var authData map[string]any
	json.Unmarshal(authContent, &authData)
	if authData["OPENAI_API_KEY"] != "sk-my-openai-key" {
		t.Errorf("auth.json should have OPENAI_API_KEY = sk-my-openai-key, got %v", authData["OPENAI_API_KEY"])
	}
	if authData["auth_mode"] != "apikey" {
		t.Errorf("auth.json should have auth_mode = apikey, got %v", authData["auth_mode"])
	}
}

func TestApplyCodexOpenAI_DoesNotPersistProviderKey(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte("model_provider = \"ccx\"\n"), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": nil, "auth_mode": "chatgpt"})

	// OpenAI 直连的 key 只落 auth.json，不应再写入 provider-keys 存储
	err := svc.Apply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderOpenAI, APIKey: "sk-my-openai-key"}, 0, "")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if saved := svc.GetSavedProviderKeys()["codex:openai"]; saved != "" {
		t.Errorf("OpenAI direct key should not be persisted as provider key, got %q", saved)
	}
}

func TestApplyCodex_MigratesFromLegacyCCX(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	// 写入旧格式 ccx 配置
	tomlContent := `model_provider = "ccx"

[model_providers.ccx]
name = "CCX Proxy"
base_url = "http://127.0.0.1:9999/v1"
wire_api = "responses"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "old-key"})

	// 应用新格式
	err := svc.Apply(ApplyAgentConfigRequest{Platform: PlatformCodex}, 3688, "new-key")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	s := string(content)
	if !strings.Contains(s, `model_provider = "openai"`) {
		t.Error("config.toml should contain model_provider = openai after migration")
	}
	if !strings.Contains(s, `openai_base_url = "http://127.0.0.1:3688/v1"`) {
		t.Error("config.toml should contain openai_base_url with correct port")
	}
	if strings.Contains(s, `[model_providers.ccx]`) {
		t.Error("config.toml should NOT contain legacy [model_providers.ccx] block after migration")
	}
}

// ── Codex 模式切换测试 ──

func TestGetStatusCodex_QuickMode(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	tomlContent := `model_provider = "openai"
openai_base_url = "http://127.0.0.1:3688/v1"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "test-key"})

	status, err := svc.GetStatus(PlatformCodex, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderCCX {
		t.Errorf("Provider = %q, want %q", status.Provider, ProviderCCX)
	}
	if status.Mode != "" {
		t.Errorf("Mode = %q, want empty for quick openai_base_url mode", status.Mode)
	}
	if !status.MatchesCurrentPort {
		t.Error("MatchesCurrentPort should be true")
	}
}

func TestGetStatusCodex_PluginMode(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	tomlContent := `model_provider = "ccx"

[model_providers.ccx]
name = "CCX Proxy"
base_url = "http://127.0.0.1:3688/v1"
wire_api = "responses"
requires_openai_auth = true
experimental_bearer_token = "test-key"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "test-key", "auth_mode": "chatgpt"})

	status, err := svc.GetStatus(PlatformCodex, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderCCX {
		t.Errorf("Provider = %q, want %q", status.Provider, ProviderCCX)
	}
	if status.Mode != "plugin" {
		t.Errorf("Mode = %q, want plugin", status.Mode)
	}
	if !status.MatchesCurrentPort {
		t.Error("MatchesCurrentPort should be true")
	}
}

func TestApplyCodex_PluginMode(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	err := svc.Apply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderCCX, Mode: "plugin"}, 3688, "test-key")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	s := string(content)
	if !strings.Contains(s, `model_provider = "ccx"`) {
		t.Error("config.toml should contain model_provider = ccx")
	}
	if !strings.Contains(s, `[model_providers.ccx]`) {
		t.Error("config.toml should contain [model_providers.ccx]")
	}
	if !strings.Contains(s, `requires_openai_auth = true`) {
		t.Error("config.toml should contain requires_openai_auth = true")
	}
	if !strings.Contains(s, `experimental_bearer_token = "test-key"`) {
		t.Error("config.toml should contain experimental_bearer_token")
	}
	blockIndex := strings.Index(s, `[model_providers.ccx]`)
	tokenIndex := strings.Index(s, `experimental_bearer_token = "test-key"`)
	if tokenIndex < blockIndex {
		t.Error("experimental_bearer_token should be inside [model_providers.ccx] block")
	}
	if strings.Contains(s, "openai_base_url") {
		t.Error("config.toml should not contain openai_base_url in plugin mode")
	}

	authData, _, _ := readJSONMap(authPath)
	if authData["auth_mode"] != "chatgpt" {
		t.Errorf("auth_mode = %v, want chatgpt", authData["auth_mode"])
	}
	if authData["OPENAI_API_KEY"] != "test-key" {
		t.Errorf("OPENAI_API_KEY = %v, want test-key", authData["OPENAI_API_KEY"])
	}
}

func TestApplyCodex_SwitchFromPluginToQuick(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)

	tomlContent := `model_provider = "ccx"

[model_providers.ccx]
name = "CCX Proxy"
base_url = "http://127.0.0.1:3688/v1"
wire_api = "responses"
requires_openai_auth = true
experimental_bearer_token = "old-key"
`
	os.WriteFile(configPath, []byte(tomlContent), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "old-key", "auth_mode": "chatgpt"})

	err := svc.Apply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderCCX, Mode: "quick"}, 3688, "new-key")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	s := string(content)
	if !strings.Contains(s, `model_provider = "openai"`) {
		t.Error("config.toml should contain model_provider = openai")
	}
	if !strings.Contains(s, `openai_base_url = "http://127.0.0.1:3688/v1"`) {
		t.Error("config.toml should contain openai_base_url")
	}
	if strings.Contains(s, "experimental_bearer_token") {
		t.Error("config.toml should not contain experimental_bearer_token after switching to quick")
	}
	if strings.Contains(s, `[model_providers.ccx]`) {
		t.Error("config.toml should not contain ccx provider block after switching to quick")
	}

	authData, _, _ := readJSONMap(authPath)
	if authData["OPENAI_API_KEY"] != "new-key" {
		t.Errorf("OPENAI_API_KEY = %v, want new-key", authData["OPENAI_API_KEY"])
	}
	if authData["auth_mode"] != "apikey" {
		t.Errorf("auth_mode = %v, want apikey", authData["auth_mode"])
	}
}

func TestPreviewApplyCodex_QuickModeMasksProxyKey(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte(`model_provider = "openai"

[model_providers.ccx]
name = "CCX Proxy"
base_url = "http://127.0.0.1:3688/v1"
wire_api = "responses"
requires_openai_auth = true
experimental_bearer_token = "old-plugin-secret-value"
`), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "old-auth-secret-value", "auth_mode": "chatgpt"})

	result, err := svc.PreviewApply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderCCX, Mode: "quick"}, 3688, "local-proxy-secret-value")
	if err != nil {
		t.Fatalf("PreviewApply failed: %v", err)
	}

	assertDiffDoesNotLeak(t, result, "local-proxy-secret-value", "old-plugin-secret-value", "old-auth-secret-value")
}

func TestPreviewApplyCodex_PluginModeMasksProxyKey(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte(`model_provider = "openai"
openai_base_url = "http://127.0.0.1:3688/v1"
`), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "old-auth-secret-value", "auth_mode": "chatgpt"})

	result, err := svc.PreviewApply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderCCX, Mode: "plugin"}, 3688, "local-proxy-secret-value")
	if err != nil {
		t.Fatalf("PreviewApply failed: %v", err)
	}

	assertDiffDoesNotLeak(t, result, "local-proxy-secret-value", "old-auth-secret-value")
}

func TestPreviewApplyCodex_ThirdPartyPluginMasksRemovedCCXToken(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte(`model_provider = "ccx"

[model_providers.ccx]
name = "CCX Proxy"
base_url = "http://127.0.0.1:3688/v1"
wire_api = "responses"
requires_openai_auth = true
experimental_bearer_token = "old-plugin-secret-value"
`), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "old-auth-secret-value", "auth_mode": "chatgpt"})

	result, err := svc.PreviewApply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderDashScope, Mode: "plugin"}, 3688, "new-provider-secret-value")
	if err != nil {
		t.Fatalf("PreviewApply failed: %v", err)
	}

	assertDiffDoesNotLeak(t, result, "new-provider-secret-value", "old-plugin-secret-value", "old-auth-secret-value")
}

func TestPreviewApplyCodex_ThirdPartyQuickMasksRemovedCCXToken(t *testing.T) {
	svc := newTestService(t)
	configPath := filepath.Join(svc.homeDir, ".codex", "config.toml")
	authPath := filepath.Join(svc.homeDir, ".codex", "auth.json")
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte(`model_provider = "ccx"

[model_providers.ccx]
name = "CCX Proxy"
base_url = "http://127.0.0.1:3688/v1"
wire_api = "responses"
requires_openai_auth = true
experimental_bearer_token = "key"
`), 0o644)
	writeJSON(authPath, map[string]any{"OPENAI_API_KEY": "old-auth-secret-value", "auth_mode": "chatgpt"})

	result, err := svc.PreviewApply(ApplyAgentConfigRequest{Platform: PlatformCodex, Provider: ProviderDashScope, Mode: "quick"}, 3688, "new-provider-secret-value")
	if err != nil {
		t.Fatalf("PreviewApply failed: %v", err)
	}

	assertDiffDoesNotLeak(t, result, `"key"`, "new-provider-secret-value", "old-auth-secret-value")
	assertDiffContains(t, result, `experimental_bearer_token = "k***y"`)
}

func TestMigrateCodexSessions_RewritesJSONLAndSQLite(t *testing.T) {
	svc := newTestService(t)
	sessionsDir := svc.codexSessionsDir()
	archivedDir := svc.codexArchivedSessionsDir()
	activeChanged := filepath.Join(sessionsDir, "active.jsonl")
	activeCurrent := filepath.Join(sessionsDir, "current.jsonl")
	archivedChanged := filepath.Join(archivedDir, "archived.jsonl")
	invalid := filepath.Join(archivedDir, "invalid.jsonl")
	writeCodexSession(t, activeChanged, ProviderOpenAI, `{"role":"user","content":"model_provider openai should stay in body"}`)
	writeCodexSession(t, activeCurrent, ProviderCCX, `{"role":"user","content":"already current"}`)
	writeCodexSession(t, archivedChanged, "local", `{"role":"user","content":"archived"}`)
	writeTextForTest(t, invalid, `{"type":"message","payload":{"model_provider":"openai"}}
`)

	db := openTestSQLite(t, svc.codexStateDBPath())
	_, err := db.Exec(`CREATE TABLE threads (id TEXT PRIMARY KEY, model_provider TEXT)`)
	if err != nil {
		t.Fatalf("create threads failed: %v", err)
	}
	_, err = db.Exec(`INSERT INTO threads (id, model_provider) VALUES ('1', 'local'), ('2', 'ccx'), ('3', 'openai'), ('4', NULL)`)
	if err != nil {
		t.Fatalf("insert threads failed: %v", err)
	}
	db.Close()

	result, err := svc.MigrateCodexSessions(MigrateCodexSessionsRequest{Provider: ProviderCCX, Mode: "plugin"})
	if err != nil {
		t.Fatalf("MigrateCodexSessions failed: %v", err)
	}
	if result.TargetProvider != ProviderCCX {
		t.Fatalf("target provider = %q, want %q", result.TargetProvider, ProviderCCX)
	}
	if result.TotalFiles != 4 || result.MigratedFiles != 2 || result.SkippedFiles != 2 || result.FailedFiles != 0 {
		t.Fatalf("unexpected file result: %+v", result)
	}
	if result.SQLiteSkipped || result.SQLiteRowsUpdated != 3 {
		t.Fatalf("unexpected sqlite result: %+v", result)
	}
	if got := readCodexSessionProvider(t, activeChanged); got != ProviderCCX {
		t.Fatalf("active provider = %q, want ccx", got)
	}
	if got := readCodexSessionProvider(t, activeCurrent); got != ProviderCCX {
		t.Fatalf("current provider = %q, want ccx", got)
	}
	if got := readCodexSessionProvider(t, archivedChanged); got != ProviderCCX {
		t.Fatalf("archived provider = %q, want ccx", got)
	}
	content, err := os.ReadFile(activeChanged)
	if err != nil {
		t.Fatalf("read migrated session failed: %v", err)
	}
	if !strings.Contains(string(content), `model_provider openai should stay in body`) {
		t.Fatalf("conversation body was unexpectedly changed: %s", string(content))
	}

	db = openTestSQLite(t, svc.codexStateDBPath())
	defer db.Close()
	rows, err := db.Query(`SELECT model_provider, COUNT(*) FROM threads GROUP BY model_provider`)
	if err != nil {
		t.Fatalf("query threads failed: %v", err)
	}
	defer rows.Close()
	providers := map[string]int{}
	for rows.Next() {
		var provider string
		var count int
		if err := rows.Scan(&provider, &count); err != nil {
			t.Fatalf("scan threads failed: %v", err)
		}
		providers[provider] = count
	}
	if providers[ProviderCCX] != 4 || len(providers) != 1 {
		t.Fatalf("providers after sqlite migration = %#v, want only ccx=4", providers)
	}
}

func TestResolveCodexSessionModelProvider(t *testing.T) {
	cases := []struct {
		name string
		req  MigrateCodexSessionsRequest
		want string
	}{
		{"openai", MigrateCodexSessionsRequest{Provider: ProviderOpenAI}, ProviderOpenAI},
		{"ccx quick", MigrateCodexSessionsRequest{Provider: ProviderCCX, Mode: "quick"}, ProviderOpenAI},
		{"ccx plugin", MigrateCodexSessionsRequest{Provider: ProviderCCX, Mode: "plugin"}, ProviderCCX},
		{"dashscope quick", MigrateCodexSessionsRequest{Provider: ProviderDashScope, Mode: "quick"}, ProviderOpenAI},
		{"dashscope plugin", MigrateCodexSessionsRequest{Provider: ProviderDashScope, Mode: "plugin"}, ProviderDashScope},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveCodexSessionModelProvider(c.req)
			if err != nil {
				t.Fatalf("resolve failed: %v", err)
			}
			if got != c.want {
				t.Fatalf("provider = %q, want %q", got, c.want)
			}
		})
	}
}

// ── helpers ──
