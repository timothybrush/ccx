package config

import (
	"math"
	"testing"
)

func TestIsAPIKeyConfigGroupMultiplierAllowed(t *testing.T) {
	one, two := 1.0, 2.0
	tests := []struct {
		name string
		cfg  APIKeyConfig
		want bool
	}{
		{name: "legacy config", cfg: APIKeyConfig{}, want: true},
		{name: "at limit", cfg: APIKeyConfig{GroupMultiplier: &one, MaxGroupMultiplier: &one}, want: true},
		{name: "above limit", cfg: APIKeyConfig{GroupMultiplier: &two, MaxGroupMultiplier: &one}, want: false},
		{name: "incomplete metadata", cfg: APIKeyConfig{GroupMultiplier: &one}, want: false},
		{name: "invalid metadata", cfg: APIKeyConfig{GroupMultiplier: &one, MaxGroupMultiplier: ptrFloat64(math.NaN())}, want: false},
	}
	for _, tt := range tests {
		if got := IsAPIKeyConfigGroupMultiplierAllowed(tt.cfg); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestGetNextAPIKeySkipsKeysAboveGroupMultiplierLimit(t *testing.T) {
	safeRatio, unsafeRatio, limit := 1.0, 2.0, 1.0
	cm := &ConfigManager{}
	upstream := &UpstreamConfig{
		Name:    "newapi",
		APIKeys: []string{"unsafe", "safe"},
		APIKeyConfigs: []APIKeyConfig{
			{Key: "unsafe", GroupMultiplier: &unsafeRatio, MaxGroupMultiplier: &limit},
			{Key: "safe", GroupMultiplier: &safeRatio, MaxGroupMultiplier: &limit},
		},
	}

	key, err := cm.GetNextAPIKey(upstream, nil, "Responses")
	if err != nil || key != "safe" {
		t.Fatalf("GetNextAPIKey() = %q, %v; want safe key", key, err)
	}

	if _, err := cm.GetNextAPIKey(upstream, map[string]bool{"safe": true}, "Responses"); err == nil {
		t.Fatal("GetNextAPIKey() must not fall back to an over-limit key")
	}
}

func TestGetAdminAPIKeySkipsDisabledKeyAboveGroupMultiplierLimit(t *testing.T) {
	unsafeRatio, limit := 2.0, 1.0
	cm := &ConfigManager{}
	upstream := &UpstreamConfig{
		Name: "newapi",
		DisabledAPIKeys: []DisabledKeyInfo{{
			Key: "unsafe",
			Config: &APIKeyConfig{
				Key:                "unsafe",
				GroupMultiplier:    &unsafeRatio,
				MaxGroupMultiplier: &limit,
			},
		}},
	}

	if _, _, err := cm.GetAdminAPIKey(upstream, nil, "Responses"); err == nil {
		t.Fatal("GetAdminAPIKey() must not borrow an over-limit key")
	}
}

func ptrFloat64(value float64) *float64 {
	return &value
}
