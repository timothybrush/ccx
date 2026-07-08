package autopilot

import (
	"net/http"
	"testing"
)

// TestObserveRateLimitSignal_FeedsDiscovererAndBuckets 验证 ObserveRateLimitSignal
// 同时喂 RateLimitDiscoverer 和 TimeBucketStore。
func TestObserveRateLimitSignal_FeedsDiscovererAndBuckets(t *testing.T) {
	store := NewTimeBucketStore()
	mgr := &Manager{
		store:               nil, // collectAll 不调用，不需要
		rateLimitDiscoverer: NewRateLimitDiscoverer(RateLimitDiscovererConfig{}),
		timeBucketStore:     store,
	}

	endpointUID := "ep-test-001"
	metricsKey := "mk-test-001"

	// 模拟 2xx 成功响应（带 Anthropic limit header）
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-requests-limit", "60")
	headers.Set("anthropic-ratelimit-requests-remaining", "40")
	headers.Set("anthropic-ratelimit-requests-reset", "2026-01-01T00:01:00Z")

	mgr.ObserveRateLimitSignal(endpointUID, 1, metricsKey, false, 200, headers, http.StatusOK)

	// 验证 RateLimitDiscoverer 收到信号
	state := mgr.rateLimitDiscoverer.GetState(endpointUID)
	if state == nil {
		t.Fatal("RateLimitDiscoverer 未收到信号")
	}
	if state.ObserveCount != 1 {
		t.Errorf("ObserveCount = %d, want 1", state.ObserveCount)
	}
	// header limit=60 → RPM 应为 60（1 分钟窗口）
	if state.EstimatedRPM != 60 {
		t.Errorf("EstimatedRPM = %d, want 60", state.EstimatedRPM)
	}
	if state.Source != RateLimitSourceHeader {
		t.Errorf("Source = %s, want %s", state.Source, RateLimitSourceHeader)
	}

	// 验证 TimeBucketStore 收到信号
	buckets := store.GetBuckets(endpointUID, 1)
	if len(buckets) == 0 {
		t.Fatal("TimeBucketStore 未收到信号")
	}
	b := buckets[0]
	if b.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", b.RequestCount)
	}
	if b.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", b.SuccessCount)
	}
}

// TestObserveRateLimitSignal_429FeedsDiscoverer 验证 429 信号喂发现器并记录为失败。
func TestObserveRateLimitSignal_429FeedsDiscoverer(t *testing.T) {
	store := NewTimeBucketStore()
	mgr := &Manager{
		rateLimitDiscoverer: NewRateLimitDiscoverer(RateLimitDiscovererConfig{}),
		timeBucketStore:     store,
	}

	endpointUID := "ep-test-429"
	headers := http.Header{}
	headers.Set("Retry-After", "30")

	mgr.ObserveRateLimitSignal(endpointUID, 1, "mk-429", false, 100, headers, http.StatusTooManyRequests)

	// 验证发现器收到 429 信号
	state := mgr.rateLimitDiscoverer.GetState(endpointUID)
	if state == nil {
		t.Fatal("RateLimitDiscoverer 未收到 429 信号")
	}
	if state.Last429At == nil {
		t.Error("Last429At 应非 nil")
	}
	if state.Source != RateLimitSourcePassiveAIMD {
		t.Errorf("Source = %s, want %s", state.Source, RateLimitSourcePassiveAIMD)
	}
	// 429+Retry-After: RPM 应被减半（从 MaxAutoRPM/2=60 降到 30）
	if state.EstimatedRPM <= 0 {
		t.Errorf("EstimatedRPM = %d, 应 > 0", state.EstimatedRPM)
	}

	// 验证时间桶记录为失败
	buckets := store.GetBuckets(endpointUID, 1)
	if len(buckets) == 0 {
		t.Fatal("TimeBucketStore 未收到 429 信号")
	}
	b := buckets[0]
	if b.FailureCount != 1 {
		t.Errorf("FailureCount = %d, want 1", b.FailureCount)
	}
	if b.SuccessCount != 0 {
		t.Errorf("SuccessCount = %d, want 0", b.SuccessCount)
	}
}

// TestObserveRateLimitSignal_Non2xxNon429RecordsBucketOnly 验证非 2xx 非 429 只记录时间桶。
func TestObserveRateLimitSignal_Non2xxNon429RecordsBucketOnly(t *testing.T) {
	store := NewTimeBucketStore()
	mgr := &Manager{
		rateLimitDiscoverer: NewRateLimitDiscoverer(RateLimitDiscovererConfig{}),
		timeBucketStore:     store,
	}

	endpointUID := "ep-test-500"
	headers := http.Header{}

	mgr.ObserveRateLimitSignal(endpointUID, 1, "mk-500", false, 50, headers, http.StatusInternalServerError)

	// 发现器不应收到信号
	state := mgr.rateLimitDiscoverer.GetState(endpointUID)
	if state != nil {
		t.Error("RateLimitDiscoverer 不应收到 500 信号")
	}

	// 时间桶应记录为失败
	buckets := store.GetBuckets(endpointUID, 1)
	if len(buckets) == 0 {
		t.Fatal("TimeBucketStore 应收到 500 信号")
	}
	if buckets[0].FailureCount != 1 {
		t.Errorf("FailureCount = %d, want 1", buckets[0].FailureCount)
	}
}

// TestObserveRateLimitSignal_EmptyEndpointUIDNoop 验证空 endpointUID 安全跳过。
func TestObserveRateLimitSignal_EmptyEndpointUIDNoop(t *testing.T) {
	store := NewTimeBucketStore()
	mgr := &Manager{
		rateLimitDiscoverer: NewRateLimitDiscoverer(RateLimitDiscovererConfig{}),
		timeBucketStore:     store,
	}

	headers := http.Header{}
	mgr.ObserveRateLimitSignal("", 1, "mk", false, 100, headers, http.StatusOK)

	if mgr.rateLimitDiscoverer.StateCount() != 0 {
		t.Error("空 endpointUID 不应触发发现器")
	}
	if len(store.GetBuckets("", 1)) != 0 {
		t.Error("空 endpointUID 不应触发时间桶")
	}
}

// TestObserveRateLimitSignal_NilHeadersNoop 验证 nil headers 安全跳过。
func TestObserveRateLimitSignal_NilHeadersNoop(t *testing.T) {
	store := NewTimeBucketStore()
	mgr := &Manager{
		rateLimitDiscoverer: NewRateLimitDiscoverer(RateLimitDiscovererConfig{}),
		timeBucketStore:     store,
	}

	// nil headers 不应 panic
	mgr.ObserveRateLimitSignal("ep", 1, "mk", false, 100, nil, http.StatusOK)
}
