package common

import (
	"testing"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/scheduler"
)

func TestShouldNormalizeMetadataUserIDOnlyMessages(t *testing.T) {
	enabled := true
	disabled := false

	tests := []struct {
		name     string
		kind     scheduler.ChannelKind
		upstream *config.UpstreamConfig
		want     bool
	}{
		{
			name:     "messages inherits default enabled",
			kind:     scheduler.ChannelKindMessages,
			upstream: &config.UpstreamConfig{},
			want:     true,
		},
		{
			name:     "messages honors disabled switch",
			kind:     scheduler.ChannelKindMessages,
			upstream: &config.UpstreamConfig{NormalizeMetadataUserID: &disabled},
			want:     false,
		},
		{
			name:     "responses ignores enabled switch",
			kind:     scheduler.ChannelKindResponses,
			upstream: &config.UpstreamConfig{NormalizeMetadataUserID: &enabled},
			want:     false,
		},
		{
			name:     "nil upstream",
			kind:     scheduler.ChannelKindMessages,
			upstream: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldNormalizeMetadataUserID(tt.kind, tt.upstream); got != tt.want {
				t.Fatalf("shouldNormalizeMetadataUserID() = %v, want %v", got, tt.want)
			}
		})
	}
}
