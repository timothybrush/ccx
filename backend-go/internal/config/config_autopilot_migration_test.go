package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigMigratesMissingAutopilotBlock(t *testing.T) {
	configPath := writeAutopilotMigrationConfig(t, nil)

	cm, err := NewConfigManager(configPath, "")
	if err != nil {
		t.Fatalf("NewConfigManager() error = %v", err)
	}
	cm.CloseWatcher()

	cfg := cm.GetAutopilotRouting()
	assertCurrentAutopilotDefaults(t, cfg)

	persisted := readPersistedAutopilotConfig(t, configPath)
	assertCurrentAutopilotDefaults(t, persisted)
}

func TestLoadConfigUpgradesPartialAutopilotAndPreservesExplicitValues(t *testing.T) {
	legacy := json.RawMessage(`{
		"mode": "ASSIST",
		"modelFamilyPreference": {
			"enabled": false,
			"weight": 0,
			"globalOrder": []
		},
		"reasoningEffort": {
			"enabled": false,
			"perTaskClass": {}
		},
		"modelMapping": {
			"autoResolve": true
		},
		"trustedRoutingAdvisor": {
			"enabled": false
		}
	}`)
	configPath := writeAutopilotMigrationConfig(t, legacy)

	cm, err := NewConfigManager(configPath, "")
	if err != nil {
		t.Fatalf("NewConfigManager() error = %v", err)
	}
	cm.CloseWatcher()

	assertLegacyAutopilotOverrides(t, cm.GetAutopilotRouting())
	assertLegacyAutopilotOverrides(t, readPersistedAutopilotConfig(t, configPath))

	// schemaVersion 防止 omitempty 删除显式零值后，下次启动又把它们误判为缺失字段。
	reloaded, err := NewConfigManager(configPath, "")
	if err != nil {
		t.Fatalf("重新加载迁移后的配置失败: %v", err)
	}
	reloaded.CloseWatcher()
	assertLegacyAutopilotOverrides(t, reloaded.GetAutopilotRouting())
}

func TestLoadConfigDoesNotPersistAutopilotEnvKillSwitch(t *testing.T) {
	t.Setenv(autopilotKillSwitchEnv, "true")
	configPath := writeAutopilotMigrationConfig(t, json.RawMessage(`{"mode":"auto"}`))

	cm, err := NewConfigManager(configPath, "")
	if err != nil {
		t.Fatalf("NewConfigManager() error = %v", err)
	}
	cm.CloseWatcher()

	runtimeCfg := cm.GetAutopilotRouting()
	if !runtimeCfg.KillSwitch || runtimeCfg.EffectiveRoutingMode() != AutopilotModeOff {
		t.Fatalf("运行态环境急停未生效: %+v", runtimeCfg)
	}

	persisted := readPersistedAutopilotConfig(t, configPath)
	if persisted.KillSwitch {
		t.Fatal("AUTOPILOT_KILL_SWITCH 不应写回 config.json")
	}
	if persisted.RoutingMode != AutopilotModeAuto {
		t.Fatalf("持久化 mode = %q, want %q", persisted.RoutingMode, AutopilotModeAuto)
	}
}

func writeAutopilotMigrationConfig(t *testing.T, autopilot json.RawMessage) string {
	t.Helper()

	root := map[string]any{
		"upstream":          []any{},
		"responsesUpstream": []any{},
		"geminiUpstream":    []any{},
		"fuzzyModeEnabled":  true,
		"thinkingCache": map[string]any{
			"ttlHours": ThinkingCacheDefaultTTLHours,
		},
	}
	if autopilot != nil {
		var value any
		if err := json.Unmarshal(autopilot, &value); err != nil {
			t.Fatalf("解析测试 autopilot 配置失败: %v", err)
		}
		root["autopilot"] = value
	}

	data, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("序列化测试配置失败: %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}
	return configPath
}

func readPersistedAutopilotConfig(t *testing.T, configPath string) AutopilotRoutingConfig {
	t.Helper()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取迁移后配置失败: %v", err)
	}
	var persisted struct {
		Autopilot AutopilotRoutingConfig `json:"autopilot"`
	}
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("解析迁移后配置失败: %v", err)
	}
	return persisted.Autopilot
}

func assertCurrentAutopilotDefaults(t *testing.T, cfg AutopilotRoutingConfig) {
	t.Helper()
	if cfg.SchemaVersion != currentAutopilotConfigSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", cfg.SchemaVersion, currentAutopilotConfigSchemaVersion)
	}
	if cfg.RoutingMode != AutopilotModeShadow {
		t.Fatalf("mode = %q, want %q", cfg.RoutingMode, AutopilotModeShadow)
	}
	if !cfg.HealthCheck.Enabled || !cfg.ModelMapping.CapabilityFloorEnabled {
		t.Fatalf("缺失当前默认能力配置: health=%v modelMapping=%v", cfg.HealthCheck, cfg.ModelMapping)
	}
	if cfg.SLORollback.ConsecutiveWindows != 3 || cfg.ABTest.ShadowCandidateCount != 1 {
		t.Fatalf("缺失新增子配置默认值: slo=%+v abTest=%+v", cfg.SLORollback, cfg.ABTest)
	}
}

func assertLegacyAutopilotOverrides(t *testing.T, cfg AutopilotRoutingConfig) {
	t.Helper()
	if cfg.SchemaVersion != currentAutopilotConfigSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", cfg.SchemaVersion, currentAutopilotConfigSchemaVersion)
	}
	if cfg.RoutingMode != AutopilotModeAssist {
		t.Fatalf("mode = %q, want %q", cfg.RoutingMode, AutopilotModeAssist)
	}
	if cfg.ModelFamilyPreference.Enabled || cfg.ModelFamilyPreference.Weight != 0 || len(cfg.ModelFamilyPreference.GlobalOrder) != 0 {
		t.Fatalf("显式模型派系零值未保留: %+v", cfg.ModelFamilyPreference)
	}
	if cfg.ReasoningEffort.Enabled || len(cfg.ReasoningEffort.PerTaskClass) != 0 {
		t.Fatalf("显式 reasoningEffort 配置未保留: %+v", cfg.ReasoningEffort)
	}
	if !cfg.ModelMapping.AutoResolve || !cfg.ModelMapping.CapabilityFloorEnabled {
		t.Fatalf("modelMapping 显式值或新增默认值错误: %+v", cfg.ModelMapping)
	}
	if cfg.TrustedRoutingAdvisor.Enabled {
		t.Fatal("显式关闭 trustedRoutingAdvisor 未保留")
	}
	if !cfg.HealthCheck.Enabled || cfg.SLORollback.ConsecutiveWindows != 3 {
		t.Fatalf("缺失字段未升级到当前默认值: health=%+v slo=%+v", cfg.HealthCheck, cfg.SLORollback)
	}
}
