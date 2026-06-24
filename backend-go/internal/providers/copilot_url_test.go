package providers

import (
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
)

func TestBuildTargetURL_CopilotSkipsVersionPrefix(t *testing.T) {
	p := &ResponsesProvider{}

	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"default_base", "https://api.githubcopilot.com", "https://api.githubcopilot.com/responses"},
		{"hash_skip", "https://api.githubcopilot.com#", "https://api.githubcopilot.com/responses"},
		{"trailing_slash", "https://api.githubcopilot.com/", "https://api.githubcopilot.com/responses"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &config.UpstreamConfig{
				BaseURL:     tt.baseURL,
				ServiceType: "copilot",
			}
			if got := p.buildTargetURL(upstream); got != tt.want {
				t.Errorf("buildTargetURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
