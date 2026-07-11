package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBindManagedAccountMiMoConsoleReplacesKeyAcrossRoutes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{
  "managedAccounts":[{"accountUid":"acct_mimo","providerId":"mimo","name":"mimo","credentials":[{"credentialUid":"cred_mimo","apiKey":"tp-old-key"}]}],
  "upstream":[{"accountUid":"acct_mimo","channelUid":"ch_msg","providerId":"mimo","autoManaged":true,"serviceType":"claude","baseUrl":"https://token-plan-cn.xiaomimimo.com/anthropic","apiKeyConfigs":[{"credentialUid":"cred_mimo","baseUrl":"https://token-plan-cn.xiaomimimo.com/anthropic"}]}],
  "chatUpstream":[{"accountUid":"acct_mimo","channelUid":"ch_chat","providerId":"mimo","autoManaged":true,"serviceType":"openai","baseUrl":"https://token-plan-cn.xiaomimimo.com/v1","apiKeyConfigs":[{"credentialUid":"cred_mimo","baseUrl":"https://token-plan-cn.xiaomimimo.com/v1"}]}],
  "responsesUpstream":[{"accountUid":"acct_mimo","channelUid":"ch_resp","providerId":"mimo","autoManaged":true,"serviceType":"openai","baseUrl":"https://token-plan-cn.xiaomimimo.com/v1","apiKeyConfigs":[{"credentialUid":"cred_mimo","baseUrl":"https://token-plan-cn.xiaomimimo.com/v1"}]}],
  "geminiUpstream":[{"accountUid":"acct_mimo","channelUid":"ch_gem","providerId":"mimo","autoManaged":true,"serviceType":"openai","baseUrl":"https://token-plan-cn.xiaomimimo.com/v1","apiKeyConfigs":[{"credentialUid":"cred_mimo","baseUrl":"https://token-plan-cn.xiaomimimo.com/v1"}]}],
  "imagesUpstream":[],"vectorsUpstream":[]
}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewConfigManager(path, filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	snapshot := MiMoConsoleCredential{Cookie: "api-platform_serviceToken=session", PlanCode: "max", ValidatedAt: time.Now()}
	if err := manager.BindManagedAccountMiMoConsole("acct_mimo", "cred_mimo", "tp-cookie-key", snapshot); err != nil {
		t.Fatal(err)
	}
	channels := manager.GetAccountChannels("acct_mimo")
	if len(channels) != 4 {
		t.Fatalf("channels=%d", len(channels))
	}
	for _, channel := range channels {
		if len(channel.Upstream.APIKeys) != 1 || channel.Upstream.APIKeys[0] != "tp-cookie-key" {
			t.Fatalf("route %s key 未替换: %+v", channel.Kind, channel.Upstream.APIKeys)
		}
		if len(channel.Upstream.APIKeyConfigs) != 1 || channel.Upstream.APIKeyConfigs[0].CredentialUID != "cred_mimo" || channel.Upstream.APIKeyConfigs[0].Key != "tp-cookie-key" {
			t.Fatalf("route %s credential 绑定丢失: %+v", channel.Kind, channel.Upstream.APIKeyConfigs)
		}
		if !accountContainsString(channel.Upstream.HistoricalAPIKeys, "tp-old-key") {
			t.Fatalf("route %s 未保留旧 Key 指标身份", channel.Kind)
		}
	}
	credential, ok := manager.GetManagedAccountCredential("acct_mimo", "cred_mimo")
	if !ok || credential.APIKey != "tp-cookie-key" || credential.MiMoConsole == nil || credential.MiMoConsole.Cookie != snapshot.Cookie {
		t.Fatalf("credential=%+v", credential)
	}
	manager.Close()
	reloaded, err := NewConfigManager(path, filepath.Join(dir, "backups-reloaded"))
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	reloadedCredential, ok := reloaded.GetManagedAccountCredential("acct_mimo", "cred_mimo")
	if !ok || reloadedCredential.APIKey != "tp-cookie-key" || reloadedCredential.MiMoConsole == nil || reloadedCredential.MiMoConsole.Cookie != snapshot.Cookie {
		t.Fatalf("持久化回读失败: %+v", reloadedCredential)
	}
	for _, channel := range reloaded.GetAccountChannels("acct_mimo") {
		if len(channel.Upstream.APIKeys) != 1 || channel.Upstream.APIKeys[0] != "tp-cookie-key" {
			t.Fatalf("route %s 重载后 Key 错误: %+v", channel.Kind, channel.Upstream.APIKeys)
		}
	}
}
