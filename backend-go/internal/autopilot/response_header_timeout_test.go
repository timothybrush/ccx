package autopilot

import (
	"testing"
	"time"
)

func TestSuggestResponseHeaderTimeout(t *testing.T) {
	lightweight := &RequestProfile{TaskClass: TaskClassLightweight, ContextNeed: 500}
	now := time.Now()
	freshProfile := func(samples, p95Ms int64) *KeyEndpointProfile {
		updatedAt := now
		return &KeyEndpointProfile{
			FirstByteSampleCount:    samples,
			P95FirstByteLatencyMs:   p95Ms,
			FirstByteStatsUpdatedAt: &updatedAt,
		}
	}
	profile := freshProfile(20, 2_000)
	staleAt := now.Add(-adaptiveFirstByteStatsMaxAge - time.Minute)
	futureAt := now.Add(time.Minute)

	tests := []struct {
		name      string
		profile   *KeyEndpointProfile
		request   *RequestProfile
		inherited int
		isStream  bool
		want      int
	}{
		{name: "streaming fast endpoint uses conservative floor", profile: profile, request: lightweight, inherited: 120_000, isStream: true, want: 30_000},
		{name: "bounded non-stream operation uses conservative floor", profile: profile, request: &RequestProfile{TaskClass: TaskClassLightweight, Operation: "title_generation"}, inherited: 120_000, want: 30_000},
		{name: "generic non-stream completion fails open", profile: profile, request: lightweight, inherited: 120_000},
		{name: "slower endpoint uses buffered p95", profile: freshProfile(30, 10_000), request: lightweight, inherited: 120_000, isStream: true, want: 45_000},
		{name: "insufficient samples fail open", profile: freshProfile(19, 2_000), request: lightweight, inherited: 120_000, isStream: true},
		{name: "missing freshness timestamp fails open", profile: &KeyEndpointProfile{FirstByteSampleCount: 20, P95FirstByteLatencyMs: 2_000}, request: lightweight, inherited: 120_000, isStream: true},
		{name: "stale stats fail open", profile: &KeyEndpointProfile{FirstByteSampleCount: 20, P95FirstByteLatencyMs: 2_000, FirstByteStatsUpdatedAt: &staleAt}, request: lightweight, inherited: 120_000, isStream: true},
		{name: "future timestamp fails open", profile: &KeyEndpointProfile{FirstByteSampleCount: 20, P95FirstByteLatencyMs: 2_000, FirstByteStatsUpdatedAt: &futureAt}, request: lightweight, inherited: 120_000, isStream: true},
		{name: "supervisor remains inherited", profile: profile, request: &RequestProfile{TaskClass: TaskClassSupervisor}, inherited: 120_000, isStream: true},
		{name: "reasoning remains inherited", profile: profile, request: &RequestProfile{TaskClass: TaskClassLightweight, ReasoningNeed: true}, inherited: 120_000, isStream: true},
		{name: "tool call remains inherited", profile: profile, request: &RequestProfile{TaskClass: TaskClassLightweight, ToolUseNeed: true}, inherited: 120_000, isStream: true},
		{name: "longer context remains inherited", profile: profile, request: &RequestProfile{TaskClass: TaskClassLightweight, ContextNeed: 10_000}, inherited: 120_000, isStream: true},
		{name: "suggestion never lengthens inherited timeout", profile: freshProfile(20, 30_000), request: lightweight, inherited: 120_000, isStream: true},
		{name: "nil profile", request: lightweight, inherited: 120_000, isStream: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SuggestResponseHeaderTimeout(tt.profile, tt.request, tt.inherited, tt.isStream); got != tt.want {
				t.Fatalf("SuggestResponseHeaderTimeout() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildResponseHeaderTimeoutLookupUsesEndpointProfile(t *testing.T) {
	store := newTestProfileStore(t)
	if store == nil {
		t.Skip("profile store unavailable")
	}
	updatedAt := time.Now()
	profile := &KeyEndpointProfile{
		EndpointUID:             "ep-fast",
		FirstByteSampleCount:    25,
		P95FirstByteLatencyMs:   1_500,
		FirstByteStatsUpdatedAt: &updatedAt,
	}
	if err := store.Upsert(profile); err != nil {
		t.Fatal(err)
	}
	lookup := buildResponseHeaderTimeoutLookup(store, &RequestProfile{TaskClass: TaskClassLightweight, ContextNeed: 100})
	if lookup == nil {
		t.Fatal("lookup should not be nil")
	}
	if got := lookup(profile.EndpointUID, 120_000, true); got != 30_000 {
		t.Fatalf("lookup timeout = %d, want 30000", got)
	}
	if got := lookup("missing", 120_000, true); got != 0 {
		t.Fatalf("missing endpoint timeout = %d, want 0", got)
	}
}
