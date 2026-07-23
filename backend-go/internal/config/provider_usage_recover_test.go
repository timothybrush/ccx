package config

import (
	"testing"
	"time"
)

func isTestKeyDisabled(cm *ConfigManager, apiKey string) bool {
	cfg := cm.GetConfig()
	return cfg.Upstream[0].IsKeyDisabledNow(apiKey, time.Now())
}

func testConfigManager(t *testing.T) (*ConfigManager, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	cm, err := NewConfigManager(tmpDir+"/config.json", tmpDir)
	if err != nil {
		t.Fatalf("NewConfigManager 失败: %v", err)
	}
	return cm, func() { _ = cm.Close() }
}

func seedManagedProvider(cm *ConfigManager, accountUID, credentialUID, providerID, apiKey string) {
	cm.config.ManagedAccounts = []ManagedAccountConfig{{
		AccountUID: accountUID,
		ProviderID: providerID,
		Name:       providerID + "-test",
		Credentials: []ManagedAccountCredential{{
			CredentialUID: credentialUID,
			APIKey:        apiKey,
		}},
	}}
	cm.config.Upstream = []UpstreamConfig{{
		Name:          providerID + "-claude",
		ChannelUID:    "ch-" + providerID + "-001",
		AccountUID:    accountUID,
		ProviderID:    providerID,
		AutoManaged:   true,
		Status:        "active",
		APIKeys:       []string{apiKey},
		APIKeyConfigs: []APIKeyConfig{{Key: apiKey, CredentialUID: credentialUID}},
	}}
}

func blacklistTestKey(t *testing.T, cm *ConfigManager, apiKey, reason string) {
	t.Helper()
	if err := cm.BlacklistKey("Messages", 0, apiKey, reason, reason+"-msg"); err != nil {
		t.Fatalf("BlacklistKey 失败: %v", err)
	}
}

// ── Kimi ──────────────────────────────────────────────────────────

func TestTryRestore_Kimi_FiveHourReset(t *testing.T) {
	cm, cleanup := testConfigManager(t)
	defer cleanup()

	seedManagedProvider(cm, "acct-kr", "c-kr", "kimi", "sk-kr")
	blacklistTestKey(t, cm, "sk-kr", "insufficient_balance")
	if !isTestKeyDisabled(cm, "sk-kr") {
		t.Fatal("拉黑后 Key 应被禁用")
	}

	pastReset := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	_ = cm.BindManagedAccountKimiConsole("acct-kr", "c-kr", KimiConsoleCredential{
		AccessToken: "tok",
		Usage: KimiCodeUsageSnapshot{
			CodeFiveHour: &KimiCodeRatioWindow{Ratio: 0.3, Enabled: true, ResetTime: pastReset},
			WeeklyUsage:  KimiCodeQuotaWindow{Remaining: 50},
		},
	})

	TryRestoreDisabledKeysByUsage(cm, "acct-kr", "sk-kr", "c-kr")

	cfg := cm.GetConfig()
	if len(cfg.Upstream[0].DisabledAPIKeys) != 0 {
		t.Fatalf("Key 应已恢复: DisabledAPIKeys=%+v", cfg.Upstream[0].DisabledAPIKeys)
	}
	if len(cfg.Upstream[0].APIKeys) != 1 || cfg.Upstream[0].APIKeys[0] != "sk-kr" {
		t.Fatalf("Key 未恢复: APIKeys=%v", cfg.Upstream[0].APIKeys)
	}
}

func TestTryRestore_Kimi_NotReset(t *testing.T) {
	cm, cleanup := testConfigManager(t)
	defer cleanup()

	seedManagedProvider(cm, "acct-kn", "c-kn", "kimi", "sk-kn")
	blacklistTestKey(t, cm, "sk-kn", "insufficient_balance")

	futureReset := time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	_ = cm.BindManagedAccountKimiConsole("acct-kn", "c-kn", KimiConsoleCredential{
		AccessToken: "tok",
		Usage: KimiCodeUsageSnapshot{
			CodeFiveHour: &KimiCodeRatioWindow{Ratio: 1.0, Enabled: true, ResetTime: futureReset},
			WeeklyUsage:  KimiCodeQuotaWindow{Remaining: 0},
		},
	})

	TryRestoreDisabledKeysByUsage(cm, "acct-kn", "sk-kn", "c-kn")

	if !isTestKeyDisabled(cm, "sk-kn") {
		t.Fatal("限额未重置时 Key 不应自动恢复")
	}
}

// ── MiMo ──────────────────────────────────────────────────────────

func TestTryRestore_MiMo_HasQuota(t *testing.T) {
	cm, cleanup := testConfigManager(t)
	defer cleanup()

	seedManagedProvider(cm, "acct-mr", "c-mr", "mimo", "sk-mr")
	blacklistTestKey(t, cm, "sk-mr", "insufficient_balance")

	_ = cm.BindManagedAccountMiMoConsole("acct-mr", "c-mr", "", MiMoConsoleCredential{
		Cookie:       "cookie",
		CurrentUsage: MiMoTokenPlanUsageQuota{Used: 500, Limit: 1000},
		ValidatedAt:  time.Now(),
	})

	TryRestoreDisabledKeysByUsage(cm, "acct-mr", "sk-mr", "c-mr")

	cfg := cm.GetConfig()
	if len(cfg.Upstream[0].DisabledAPIKeys) != 0 {
		t.Fatalf("MiMo 有余量时 Key 应恢复: DisabledAPIKeys=%+v", cfg.Upstream[0].DisabledAPIKeys)
	}
}

// ── Volcengine ────────────────────────────────────────────────────

func TestTryRestore_Volcengine_ResetPassed(t *testing.T) {
	cm, cleanup := testConfigManager(t)
	defer cleanup()

	seedManagedProvider(cm, "acct-vr", "c-vr", "volcengine", "sk-vr")
	blacklistTestKey(t, cm, "sk-vr", "insufficient_balance")

	passedMs := time.Now().Add(-time.Minute).UnixMilli()
	pct := 50.0
	cm.config.ManagedAccounts[0].Credentials[0].VolcengineAccessKey = &VolcengineAccessKeyPair{
		AccessKeyID: "ak", SecretAccessKey: "sk", Plan: "coding",
		PlanTier: "basic", PlanStatus: "active",
		Usage: &VolcenginePlanUsage{
			FiveHour:  &VolcenginePlanUsageWindow{UsedPercent: &pct, ResetTime: passedMs},
			FetchedAt: time.Now(),
		},
	}

	TryRestoreDisabledKeysByUsage(cm, "acct-vr", "sk-vr", "c-vr")

	cfg := cm.GetConfig()
	if len(cfg.Upstream[0].DisabledAPIKeys) != 0 {
		t.Fatalf("火山 Coding Plan 重置后 Key 应恢复: DisabledAPIKeys=%+v", cfg.Upstream[0].DisabledAPIKeys)
	}
}

func TestTryRestore_Volcengine_CodingPlanRemainingUsageBeforeLongerWindowReset(t *testing.T) {
	cm, cleanup := testConfigManager(t)
	defer cleanup()

	seedManagedProvider(cm, "acct-vr", "c-vr", "volcengine", "sk-vr")
	blacklistTestKey(t, cm, "sk-vr", "insufficient_quota")

	fiveHourPct := 0.0
	weeklyPct := 58.0
	monthlyPct := 33.0
	futureMs := time.Now().Add(7 * 24 * time.Hour).UnixMilli()
	cm.config.ManagedAccounts[0].Credentials[0].VolcengineAccessKey = &VolcengineAccessKeyPair{
		AccessKeyID: "ak", SecretAccessKey: "sk", Plan: "coding",
		PlanTier: "basic", PlanStatus: "active",
		Usage: &VolcenginePlanUsage{
			FiveHour:  &VolcenginePlanUsageWindow{UsedPercent: &fiveHourPct},
			Weekly:    &VolcenginePlanUsageWindow{UsedPercent: &weeklyPct, ResetTime: futureMs},
			Monthly:   &VolcenginePlanUsageWindow{UsedPercent: &monthlyPct, ResetTime: futureMs},
			FetchedAt: time.Now(),
		},
	}

	TryRestoreDisabledKeysByUsage(cm, "acct-vr", "sk-vr", "c-vr")

	if isTestKeyDisabled(cm, "sk-vr") {
		t.Fatal("五小时窗口已恢复且周/月仍有余量时，火山 Coding Plan Key 应恢复")
	}
}

func TestTryRestore_Volcengine_CodingPlanExhaustedWindowNotReset(t *testing.T) {
	cm, cleanup := testConfigManager(t)
	defer cleanup()

	seedManagedProvider(cm, "acct-vr", "c-vr", "volcengine", "sk-vr")
	blacklistTestKey(t, cm, "sk-vr", "insufficient_quota")

	exhaustedPct := 100.0
	cm.config.ManagedAccounts[0].Credentials[0].VolcengineAccessKey = &VolcengineAccessKeyPair{
		AccessKeyID: "ak", SecretAccessKey: "sk", Plan: "coding",
		PlanTier: "basic", PlanStatus: "active",
		Usage: &VolcenginePlanUsage{
			FiveHour:  &VolcenginePlanUsageWindow{UsedPercent: &exhaustedPct, ResetTime: time.Now().Add(time.Hour).UnixMilli()},
			FetchedAt: time.Now(),
		},
	}

	TryRestoreDisabledKeysByUsage(cm, "acct-vr", "sk-vr", "c-vr")

	if !isTestKeyDisabled(cm, "sk-vr") {
		t.Fatal("仍耗尽且五小时窗口未重置时，火山 Coding Plan Key 不应恢复")
	}
}

// ── 非自动恢复原因 ───────────────────────────────────────────────

func TestTryRestore_SkipsNonAutoRecoverable(t *testing.T) {
	cm, cleanup := testConfigManager(t)
	defer cleanup()

	seedManagedProvider(cm, "acct-skip", "c-skip", "kimi", "sk-skip")
	blacklistTestKey(t, cm, "sk-skip", "authentication_error")

	pastReset := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	_ = cm.BindManagedAccountKimiConsole("acct-skip", "c-skip", KimiConsoleCredential{
		AccessToken: "tok",
		Usage: KimiCodeUsageSnapshot{
			CodeFiveHour: &KimiCodeRatioWindow{Ratio: 0.1, Enabled: true, ResetTime: pastReset},
			WeeklyUsage:  KimiCodeQuotaWindow{Remaining: 50},
		},
	})

	TryRestoreDisabledKeysByUsage(cm, "acct-skip", "sk-skip", "c-skip")

	if !isTestKeyDisabled(cm, "sk-skip") {
		t.Fatal("authentication_error 不应自动恢复")
	}
}

// ── 无渠道/无账号 ────────────────────────────────────────────────

func TestTryRestore_NoChannels(t *testing.T) {
	cm, cleanup := testConfigManager(t)
	defer cleanup()
	TryRestoreDisabledKeysByUsage(cm, "nonexistent", "sk-anything", "c-anything")
}

// ── syncManagedAccountsFromChannels: DisabledAPIKeys 凭证保留 ────

func TestSyncManagedAccounts_PreservesDisabledKeyCredentials(t *testing.T) {
	cm, cleanup := testConfigManager(t)
	defer cleanup()

	seedManagedProvider(cm, "acct-sync", "c-sync", "kimi", "sk-sync")

	pastReset := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	_ = cm.BindManagedAccountKimiConsole("acct-sync", "c-sync", KimiConsoleCredential{
		AccessToken: "tok",
		Usage: KimiCodeUsageSnapshot{
			CodeFiveHour: &KimiCodeRatioWindow{Ratio: 0.3, Enabled: true, ResetTime: pastReset},
		},
	})

	blacklistTestKey(t, cm, "sk-sync", "insufficient_balance")

	cfg := cm.GetConfig()
	found := false
	for _, account := range cfg.ManagedAccounts {
		for _, c := range account.Credentials {
			if c.CredentialUID == "c-sync" {
				found = true
				if c.KimiConsole == nil {
					t.Fatal("DisabledAPIKeys 凭证丢失: KimiConsole = nil")
				}
				if c.KimiConsole.AccessToken != "tok" {
					t.Fatalf("KimiConsole.AccessToken = %q, want tok", c.KimiConsole.AccessToken)
				}
				if c.APIKey != "sk-sync" {
					t.Fatalf("凭证 APIKey = %q, want sk-sync", c.APIKey)
				}
			}
		}
	}
	if !found {
		t.Fatal("凭证未在 ManagedAccounts 中找到")
	}
}
