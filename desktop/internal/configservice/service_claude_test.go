package configservice

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNew(t *testing.T) {
	svc := newTestService(t)
	if svc.homeDir == "" {
		t.Error("homeDir should not be empty")
	}
	if svc.stateDir == "" {
		t.Error("stateDir should not be empty")
	}
	if _, err := os.Stat(svc.stateDir); os.IsNotExist(err) {
		t.Error("stateDir should be created")
	}
}

func TestNewWithDefaultDataDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_STATE_HOME", "")
	}
	svc, err := New("")
	if err != nil {
		t.Fatalf("New('') failed: %v", err)
	}
	expected := filepath.Join(tmpHome, ".config", "ccx-desktop", "agent-config-state")
	if runtime.GOOS == "linux" {
		expected = filepath.Join(tmpHome, ".local", "state", "ccx", "agent-config-state")
	}
	if svc.stateDir != expected {
		t.Errorf("stateDir = %q, want %q", svc.stateDir, expected)
	}
}

func TestGetStatusClaude_NoConfig(t *testing.T) {
	svc := newTestService(t)
	status, err := svc.GetStatus(PlatformClaude, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Configured {
		t.Error("should not be configured when no settings.json exists")
	}
	// 空 base_url 时 detectClaudeProvider 返回 ""
	if status.Provider != "" {
		t.Errorf("provider = %q, want empty", status.Provider)
	}
}

func TestGetStatusClaude_CCXProvider(t *testing.T) {
	svc := newTestService(t)
	settingsPath := filepath.Join(svc.homeDir, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(settingsPath), 0o755)
	data := map[string]any{
		"env": map[string]any{
			"ANTHROPIC_BASE_URL": "http://127.0.0.1:3688",
		},
	}
	writeJSON(settingsPath, data)

	status, err := svc.GetStatus(PlatformClaude, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderCCX {
		t.Errorf("provider = %q, want %q", status.Provider, ProviderCCX)
	}
	if !status.MatchesCurrentPort {
		t.Error("should match current port")
	}
	if status.NeedsUpdate {
		t.Error("should not need update when port matches")
	}
}

func TestGetStatusClaude_DeepSeekProvider(t *testing.T) {
	svc := newTestService(t)
	settingsPath := filepath.Join(svc.homeDir, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(settingsPath), 0o755)
	data := map[string]any{
		"env": map[string]any{
			"ANTHROPIC_BASE_URL": "https://api.deepseek.com/anthropic",
		},
	}
	writeJSON(settingsPath, data)

	status, err := svc.GetStatus(PlatformClaude, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderDeepSeek {
		t.Errorf("provider = %q, want %q", status.Provider, ProviderDeepSeek)
	}
	if !status.Configured {
		t.Error("should be configured for DeepSeek")
	}
}

func TestGetStatusClaude_CompshareProvider(t *testing.T) {
	svc := newTestService(t)
	settingsPath := filepath.Join(svc.homeDir, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(settingsPath), 0o755)
	data := map[string]any{
		"env": map[string]any{
			"ANTHROPIC_BASE_URL": "https://cp.compshare.cn",
		},
	}
	writeJSON(settingsPath, data)

	status, err := svc.GetStatus(PlatformClaude, 3688)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Provider != ProviderCompshare {
		t.Errorf("provider = %q, want %q", status.Provider, ProviderCompshare)
	}
	if !status.Configured {
		t.Error("should be configured for Compshare")
	}
}

func TestApplyAndRestoreClaude(t *testing.T) {
	svc := newTestService(t)
	settingsPath := filepath.Join(svc.homeDir, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(settingsPath), 0o755)
	original := map[string]any{
		"env": map[string]any{
			"ANTHROPIC_BASE_URL": "https://original.example.com",
		},
	}
	writeJSON(settingsPath, original)

	err := svc.Apply(ApplyAgentConfigRequest{Platform: PlatformClaude}, 3688, "test-key")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// 验证注入后配置
	var after map[string]any
	readJSON(settingsPath, &after)
	env := after["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "http://127.0.0.1:3688" {
		t.Errorf("base_url = %v", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "test-key" {
		t.Errorf("auth_token = %v", env["ANTHROPIC_AUTH_TOKEN"])
	}

	// Restore
	err = svc.Restore(PlatformClaude)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	var restored map[string]any
	readJSON(settingsPath, &restored)
	env = restored["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] != "https://original.example.com" {
		t.Errorf("restored base_url = %v", env["ANTHROPIC_BASE_URL"])
	}
}

func TestSaveAndLoadProviderKeys(t *testing.T) {
	svc := newTestService(t)

	err := svc.SaveProviderKeyAsset(ProviderKeyAsset{
		Provider: ProviderDeepSeek,
		APIKey:   "sk-test-key",
		BaseURL:  "https://api.deepseek.com",
	})
	if err != nil {
		t.Fatalf("SaveProviderKeyAsset failed: %v", err)
	}
	err = svc.SaveProviderKeyAsset(ProviderKeyAsset{
		Provider: ProviderCompshare,
		APIKey:   "cs-test-key",
		BaseURL:  "https://cp.compshare.cn",
	})
	if err != nil {
		t.Fatalf("SaveProviderKeyAsset failed: %v", err)
	}

	keys := svc.GetSavedProviderKeys()
	if keys["channel:"+ProviderDeepSeek] != "sk-test-key" {
		t.Errorf("channel key = %q", keys["channel:"+ProviderDeepSeek])
	}
	if keys[PlatformClaude+":"+ProviderDeepSeek] != "sk-test-key" {
		t.Errorf("claude key = %q", keys[PlatformClaude+":"+ProviderDeepSeek])
	}
	if keys["channel:"+ProviderCompshare] != "cs-test-key" {
		t.Errorf("compshare channel key = %q", keys["channel:"+ProviderCompshare])
	}
	if keys[PlatformClaude+":"+ProviderCompshare] != "cs-test-key" {
		t.Errorf("compshare claude key = %q", keys[PlatformClaude+":"+ProviderCompshare])
	}

	assets := svc.GetProviderKeyAssets()
	foundDeepSeek := false
	foundCompshare := false
	for _, a := range assets {
		switch a.Provider {
		case ProviderDeepSeek:
			foundDeepSeek = true
			if a.APIKey != "sk-test-key" {
				t.Errorf("asset APIKey = %q", a.APIKey)
			}
		case ProviderCompshare:
			foundCompshare = true
			if a.APIKey != "cs-test-key" {
				t.Errorf("compshare asset APIKey = %q", a.APIKey)
			}
		}
	}
	if !foundDeepSeek {
		t.Error("DeepSeek asset not found")
	}
	if !foundCompshare {
		t.Error("Compshare asset not found")
	}
}

// ── P1 回归: Codex 第三方 provider 状态识别 ──
