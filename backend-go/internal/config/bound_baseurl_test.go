package config

import "testing"

// TestBoundBaseURLForKey 覆盖 per-key baseURL 绑定查找。
// 归一化结果与 GetAllBaseURLs 同源，故用后者的元素作为期望值，避免硬编码归一化形式。
func TestBoundBaseURLForKey(t *testing.T) {
	upstream := UpstreamConfig{
		ServiceType: "claude",
		BaseURLs: []string{
			"https://api.xiaomimimo.com/anthropic",
			"https://token-plan-cn.xiaomimimo.com/anthropic",
		},
		APIKeys: []string{"sk-payg", "tp-token", "unbound"},
		APIKeyConfigs: []APIKeyConfig{
			{Key: "sk-payg", BaseURL: "https://api.xiaomimimo.com/anthropic"},
			{Key: "tp-token", BaseURL: "https://token-plan-cn.xiaomimimo.com/anthropic"},
			{Key: "unbound"}, // 无 BaseURL，保持笛卡尔积
		},
	}

	all := upstream.GetAllBaseURLs()
	if len(all) != 2 {
		t.Fatalf("GetAllBaseURLs 返回 %d 个端点，期望 2: %v", len(all), all)
	}

	// sk-payg 绑定首个端点，归一化后应等于 all[0]
	if got := upstream.BoundBaseURLForKey("sk-payg"); got != all[0] {
		t.Errorf("BoundBaseURLForKey(sk-payg) = %q, want %q", got, all[0])
	}
	// tp-token 绑定第二个端点
	if got := upstream.BoundBaseURLForKey("tp-token"); got != all[1] {
		t.Errorf("BoundBaseURLForKey(tp-token) = %q, want %q", got, all[1])
	}
	// unbound：无绑定，返回空串（保持原有笛卡尔积行为）
	if got := upstream.BoundBaseURLForKey("unbound"); got != "" {
		t.Errorf("BoundBaseURLForKey(unbound) = %q, want \"\"", got)
	}
	// 不存在的 key：返回空串
	if got := upstream.BoundBaseURLForKey("missing"); got != "" {
		t.Errorf("BoundBaseURLForKey(missing) = %q, want \"\"", got)
	}
}
