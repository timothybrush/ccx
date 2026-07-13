package autopilot

import (
	"path/filepath"
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/scheduler"
)

func TestBuildChannelEntryUsesRegistryCapabilities(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		upstream      config.UpstreamConfig
		wantVision    bool
		wantTools     bool
		wantReasoning bool
	}{
		{
			name: "多模态模型使用内置能力", model: "mimo-v2.5",
			wantVision: true, wantTools: true, wantReasoning: true,
		},
		{
			name: "文本模型不会误报视觉", model: "mimo-v2.5-pro",
			wantTools: true, wantReasoning: true,
		},
		{
			name: "NoVision 强制覆盖注册表", model: "mimo-v2.5",
			upstream:  config.UpstreamConfig{NoVision: true},
			wantTools: true, wantReasoning: true,
		},
		{
			name: "映射后的 NoVisionModels 强制覆盖注册表", model: "alias-model",
			upstream: config.UpstreamConfig{
				ModelMapping:   map[string]string{"alias-model": "mimo-v2.5"},
				NoVisionModels: []string{"mimo-v2.5"},
			},
			wantTools: true, wantReasoning: true,
		},
		{
			name: "渠道能力覆盖可提供正向能力", model: "custom-model",
			upstream: config.UpstreamConfig{ModelCapabilities: map[string]config.UpstreamModelCapability{
				"custom-model": {
					ThinkingMode: "thinking",
					Capabilities: map[string]bool{"vision": true, "toolCalls": true},
				},
			}},
			wantVision: true, wantTools: true, wantReasoning: true,
		},
	}

	router := NewSmartRouter(nil, nil, nil, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := tt.upstream
			upstream.ChannelUID = "ch_registry"
			entry := router.buildChannelEntry(
				scheduler.ChannelInfo{Index: 0, Name: "registry", Status: "active"},
				&upstream, "messages", tt.model, nil,
			)
			if entry.SupportsVision != tt.wantVision || entry.SupportsToolCalls != tt.wantTools ||
				entry.SupportsReasoning != tt.wantReasoning {
				t.Fatalf("capabilities vision=%v tools=%v reasoning=%v, want %v/%v/%v",
					entry.SupportsVision, entry.SupportsToolCalls, entry.SupportsReasoning,
					tt.wantVision, tt.wantTools, tt.wantReasoning)
			}
		})
	}
}

func TestBuildChannelEntryMergesRegistryAndEndpointCapabilities(t *testing.T) {
	store, err := NewProfileStore(filepath.Join(t.TempDir(), "profiles.db"))
	if err != nil {
		t.Fatalf("创建 ProfileStore 失败: %v", err)
	}
	defer store.Close()

	profiles := []*KeyEndpointProfile{
		{
			EndpointUID: "ep_registry", ChannelUID: "ch_registry", ChannelKind: "messages",
			HealthState: HealthStateUnknown, QualityTier: QualityTierNormal,
			StabilityTier: StabilityTierNormal, SpeedTier: SpeedTierNormal, CostTier: CostTierNormal,
		},
		{
			EndpointUID: "ep_profile", ChannelUID: "ch_profile", ChannelKind: "messages",
			HealthState: HealthStateUnknown, QualityTier: QualityTierNormal,
			StabilityTier: StabilityTierNormal, SpeedTier: SpeedTierNormal, CostTier: CostTierNormal,
			SupportsVision: true, SupportsToolCalls: true, SupportsReasoning: true,
		},
	}
	for _, profile := range profiles {
		if err := store.Upsert(profile); err != nil {
			t.Fatalf("写入 profile 失败: %v", err)
		}
	}

	router := NewSmartRouter(store, nil, nil, nil)
	registryEntry := router.buildChannelEntry(
		scheduler.ChannelInfo{Index: 0, Name: "registry", Status: "active"},
		&config.UpstreamConfig{ChannelUID: "ch_registry"}, "messages", "glm-5.2", nil,
	)
	if !registryEntry.SupportsToolCalls || !registryEntry.SupportsReasoning {
		t.Fatalf("空画像不应抹掉注册表能力: %+v", registryEntry)
	}
	if reasons := routingHardConstraintReasons(&RequestProfile{ToolUseNeed: true, ReasoningNeed: true}, &registryEntry); len(reasons) != 0 {
		t.Fatalf("注册表已知能力不应触发硬约束: %v", reasons)
	}

	profileEntry := router.buildChannelEntry(
		scheduler.ChannelInfo{Index: 1, Name: "profile", Status: "active"},
		&config.UpstreamConfig{ChannelUID: "ch_profile"}, "messages", "unknown-model", nil,
	)
	if !profileEntry.SupportsVision || !profileEntry.SupportsToolCalls || !profileEntry.SupportsReasoning {
		t.Fatalf("画像正向能力未合并: %+v", profileEntry)
	}
}
