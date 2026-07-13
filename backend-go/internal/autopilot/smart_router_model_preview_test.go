package autopilot

import (
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
)

func newModelPreviewStore(t *testing.T, profiles ...ModelProfile) *ModelProfileStore {
	t.Helper()
	store := &ModelProfileStore{
		cache:     make(map[string]*ModelProfile),
		dirtyKeys: make(map[string]struct{}),
	}
	for i := range profiles {
		profile := profiles[i]
		if err := store.Upsert(&profile); err != nil {
			t.Fatalf("ModelProfileStore.Upsert() error = %v", err)
		}
	}
	return store
}

func modelPreviewConfig(mode string) config.Config {
	return config.Config{
		Upstream: []config.UpstreamConfig{{
			Name:            "glm-auto",
			ChannelUID:      "ch_glm_auto",
			BaseURL:         "https://glm.example.com",
			APIKeys:         []string{"sk-glm"},
			Status:          "active",
			AutoManaged:     true,
			SupportedModels: []string{"glm-5.2"},
		}},
		AutopilotRouting: config.AutopilotRoutingConfig{
			RoutingMode: mode,
			ModelMapping: config.ModelMappingRoutingConfig{
				AutoResolve:            true,
				CapabilityFloorEnabled: true,
			},
		},
	}
}

func glmPreviewProfile() ModelProfile {
	return ModelProfile{
		ChannelUID: "ch_glm_auto", ChannelKind: "messages", MetricsKey: "metrics_glm",
		ModelID: "glm-5.2", ModelFamily: ModelFamilyGLM, QualityTier: QualityTierHigh,
		ContextTokens: 1_048_576, SupportsToolCalls: true, SupportsReasoning: true,
		ProbeSuccess: true,
	}
}

func TestBuildPlanPreviewsAutoResolvedModel(t *testing.T) {
	cfgManager, cleanup := createTestConfigManager(t, modelPreviewConfig("shadow"))
	defer cleanup()
	store := newModelPreviewStore(t, glmPreviewProfile())
	resolver := NewModelResolver(store, cfgManager)
	router := NewSmartRouter(nil, nil, nil, cfgManager)
	router.SetModelResolver(resolver)

	profile := BuildRequestProfile(RequestProfileFeatures{
		Model: "claude-sonnet-5", ChannelKind: "messages", Operation: "completion",
		EstTokens: 20_000, ToolUseNeed: true, ReasoningNeed: true,
	})
	plan := router.BuildPlan(&profile)
	if len(plan.Candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1: %+v", len(plan.Candidates), plan.Candidates)
	}
	candidate := plan.Candidates[0]
	if candidate.ChannelUID != "ch_glm_auto" || candidate.MappedModel != "glm-5.2" {
		t.Fatalf("candidate = %+v, want ch_glm_auto mapped to glm-5.2", candidate)
	}
	if candidate.MappingSource != "auto_resolve_preview" || candidate.MappingReason == "" {
		t.Fatalf("mapping metadata = %q/%q", candidate.MappingSource, candidate.MappingReason)
	}
	if plan.SelectedChannelUID != "ch_glm_auto" || plan.SelectedModel != "glm-5.2" {
		t.Fatalf("selection = %q/%q, want ch_glm_auto/glm-5.2",
			plan.SelectedChannelUID, plan.SelectedModel)
	}
	if !containsString(plan.SortReasons, "dryrun_auto_resolve_preview") {
		t.Fatalf("sort reasons = %v, want auto-resolve preview marker", plan.SortReasons)
	}
}

func TestBuildPlanAutoResolvePreviewRespectsCapabilityFloor(t *testing.T) {
	cfgManager, cleanup := createTestConfigManager(t, modelPreviewConfig("shadow"))
	defer cleanup()
	profile := glmPreviewProfile()
	profile.SupportsToolCalls = false
	store := newModelPreviewStore(t, profile)
	router := NewSmartRouter(nil, nil, nil, cfgManager)
	router.SetModelResolver(NewModelResolver(store, cfgManager))

	requestProfile := BuildRequestProfile(RequestProfileFeatures{
		Model: "claude-sonnet-5", ChannelKind: "messages", Operation: "completion",
		EstTokens: 20_000, ToolUseNeed: true,
	})
	plan := router.BuildPlan(&requestProfile)
	if len(plan.Candidates) != 0 {
		t.Fatalf("ineligible auto-resolve candidates = %+v, want none", plan.Candidates)
	}
}

func TestResolveModelSupportDoesNotExpandRealCandidatesInShadow(t *testing.T) {
	cfgManager, cleanup := createTestConfigManager(t, modelPreviewConfig("shadow"))
	defer cleanup()
	resolver := NewModelResolver(newModelPreviewStore(t, glmPreviewProfile()), cfgManager)
	manager := &Manager{cfgManager: cfgManager, modelResolver: resolver}
	upstream := cfgManager.GetConfig().Upstream[0]

	supported, _, source, _ := manager.ResolveModelSupport("messages", &upstream, "claude-sonnet-5")
	if supported || source != "explain" {
		t.Fatalf("shadow support = %v source=%q, want false/explain", supported, source)
	}

	if err := cfgManager.SetAutopilotRoutingMode("assist"); err != nil {
		t.Fatalf("SetAutopilotRoutingMode() error = %v", err)
	}
	supported, mapped, source, reason := manager.ResolveModelSupport("messages", &upstream, "claude-sonnet-5")
	if !supported || mapped != "glm-5.2" || source != "auto_resolve" || reason == "" {
		t.Fatalf("assist support = %v mapped=%q source=%q reason=%q",
			supported, mapped, source, reason)
	}
}

func TestWireSmartRouterInjectsDryRunModelResolver(t *testing.T) {
	resolver := &ModelResolver{}
	router := &SmartRouter{}
	manager := &Manager{smartRouter: router, modelResolver: resolver}
	manager.WireSmartRouter()
	if router.modelResolver != resolver {
		t.Fatal("WireSmartRouter() did not inject ModelResolver")
	}
}
