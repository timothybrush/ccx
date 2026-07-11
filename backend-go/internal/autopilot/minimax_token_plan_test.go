package autopilot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiniMaxTokenPlanFetcherFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-cp-test" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "model_remains": [{
    "model_name": "MiniMax-M3",
    "current_interval_usage_count": 12,
    "current_interval_total_count": 100,
    "current_interval_remaining_percent": 88,
    "current_weekly_usage_count": 120,
    "current_weekly_total_count": 1000,
    "current_weekly_remaining_percent": 88,
    "remains_time": 3600000,
    "weekly_start_time": "2026-07-06T00:00:00Z",
    "weekly_end_time": "2026-07-13T00:00:00Z"
  }],
  "base_resp": {"status_code": 0, "status_msg": "success"}
}`))
	}))
	defer server.Close()

	fetcher := &MiniMaxTokenPlanFetcher{HTTPClient: server.Client(), EndpointOverride: server.URL}
	usage, err := fetcher.Fetch(context.Background(), "https://api.minimaxi.com/anthropic", "sk-cp-test")
	if err != nil {
		t.Fatalf("Fetch 返回错误: %v", err)
	}
	if len(usage.Models) != 1 {
		t.Fatalf("models = %d, want 1", len(usage.Models))
	}
	model := usage.Models[0]
	if model.ModelName != "MiniMax-M3" || model.CurrentIntervalRemainingPercent != 88 || model.RemainsTimeMs != 3600000 {
		t.Fatalf("模型用量解析错误: %+v", model)
	}
}

func TestMiniMaxTokenPlanFetcherRejectsNonPlanKey(t *testing.T) {
	_, err := (&MiniMaxTokenPlanFetcher{}).Fetch(context.Background(), "https://api.minimaxi.com/anthropic", "sk-payg")
	if err == nil {
		t.Fatal("普通 API Key 不应触发 Token Plan 用量查询")
	}
}

func TestMiniMaxTokenPlanUsageEndpoints(t *testing.T) {
	tests := []struct {
		baseURL  string
		wantHost string
	}{
		{"https://api.minimaxi.com/anthropic", "https://api.minimaxi.com/"},
		{"https://api.minimax.chat", "https://api.minimaxi.com/"},
		{"https://api.minimax.io/anthropic", "https://api.minimax.io/"},
	}
	for _, tt := range tests {
		endpoints := miniMaxTokenPlanUsageEndpoints(tt.baseURL)
		if len(endpoints) < 1 || len(endpoints[0]) < len(tt.wantHost) || endpoints[0][:len(tt.wantHost)] != tt.wantHost {
			t.Fatalf("baseURL=%s endpoints=%v, want host %s", tt.baseURL, endpoints, tt.wantHost)
		}
	}
	if endpoints := miniMaxTokenPlanUsageEndpoints("https://relay.example.com/minimax"); len(endpoints) != 0 {
		t.Fatalf("中转站不应命中官方用量接口: %v", endpoints)
	}
}

func TestMiniMaxRemainingPercentFallsBackToCounts(t *testing.T) {
	if got := miniMaxRemainingPercent(nil, 25, 100); got != 75 {
		t.Fatalf("remaining percent = %v, want 75", got)
	}
	reported := 120.0
	if got := miniMaxRemainingPercent(&reported, 0, 0); got != 100 {
		t.Fatalf("reported percent should be clamped: %v", got)
	}
}
