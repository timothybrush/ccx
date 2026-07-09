package config

import (
	"reflect"
	"testing"
)

func TestParseExtraProxyAccessKeys(t *testing.T) {
	got := parseExtraProxyAccessKeys(" key-a, ,key-b,key-a, key-c ")
	want := []string{"key-a", "key-b", "key-c"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %#v, want %#v", got, want)
	}
}

func TestEnvConfig_IsValidProxyAccessKey(t *testing.T) {
	envCfg := &EnvConfig{
		ProxyAccessKey:       "primary-key",
		ExtraProxyAccessKeys: []string{"extra-a", "extra-b"},
	}

	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "primary key", key: "primary-key", want: true},
		{name: "extra key", key: "extra-a", want: true},
		{name: "unknown key", key: "unknown", want: false},
		{name: "empty key", key: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envCfg.IsValidProxyAccessKey(tt.key); got != tt.want {
				t.Fatalf("IsValidProxyAccessKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestEnvConfig_IsValidAdminAccessKey(t *testing.T) {
	t.Run("falls back to primary proxy key without extra keys", func(t *testing.T) {
		envCfg := &EnvConfig{ProxyAccessKey: "primary-key"}

		if !envCfg.IsValidAdminAccessKey("primary-key") {
			t.Fatal("primary proxy key should be valid admin key without extra keys")
		}
	})

	t.Run("requires admin key when extra keys exist", func(t *testing.T) {
		envCfg := &EnvConfig{
			ProxyAccessKey:       "primary-key",
			ExtraProxyAccessKeys: []string{"extra-a"},
			AdminAccessKey:       "admin-key",
		}

		if !envCfg.IsValidAdminAccessKey("admin-key") {
			t.Fatal("admin key should be valid")
		}
		if envCfg.IsValidAdminAccessKey("primary-key") {
			t.Fatal("primary proxy key should not be valid admin key when extra keys exist")
		}
		if envCfg.IsValidAdminAccessKey("extra-a") {
			t.Fatal("extra proxy key should not be valid admin key")
		}
	})
}

func TestEnvConfig_ValidateAccessKeys(t *testing.T) {
	tests := []struct {
		name    string
		envCfg  *EnvConfig
		wantErr bool
	}{
		{
			name:    "no extra keys allows fallback",
			envCfg:  &EnvConfig{ProxyAccessKey: "primary-key"},
			wantErr: false,
		},
		{
			name: "extra keys require admin key",
			envCfg: &EnvConfig{
				ProxyAccessKey:       "primary-key",
				ExtraProxyAccessKeys: []string{"extra-a"},
			},
			wantErr: true,
		},
		{
			name: "admin key must differ from primary proxy key",
			envCfg: &EnvConfig{
				ProxyAccessKey:       "same-key",
				ExtraProxyAccessKeys: []string{"extra-a"},
				AdminAccessKey:       "same-key",
			},
			wantErr: true,
		},
		{
			name: "extra key must differ from primary proxy key",
			envCfg: &EnvConfig{
				ProxyAccessKey:       "primary-key",
				ExtraProxyAccessKeys: []string{"primary-key"},
				AdminAccessKey:       "admin-key",
			},
			wantErr: true,
		},
		{
			name: "extra keys must not contain default placeholder",
			envCfg: &EnvConfig{
				ProxyAccessKey:       "primary-key",
				ExtraProxyAccessKeys: []string{"your-proxy-access-key"},
				AdminAccessKey:       "admin-key",
			},
			wantErr: true,
		},
		{
			name: "admin key must differ from extra proxy keys",
			envCfg: &EnvConfig{
				ProxyAccessKey:       "primary-key",
				ExtraProxyAccessKeys: []string{"admin-key"},
				AdminAccessKey:       "admin-key",
			},
			wantErr: true,
		},
		{
			name: "valid extra keys",
			envCfg: &EnvConfig{
				ProxyAccessKey:       "primary-key",
				ExtraProxyAccessKeys: []string{"extra-a"},
				AdminAccessKey:       "admin-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.envCfg.ValidateAccessKeys()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateAccessKeys() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewEnvConfig_ParsesExtraProxyAccessKeys(t *testing.T) {
	t.Setenv("EXTRA_PROXY_ACCESS_KEYS", "extra-a, extra-b")

	envCfg := NewEnvConfig()
	want := []string{"extra-a", "extra-b"}

	if !reflect.DeepEqual(envCfg.ExtraProxyAccessKeys, want) {
		t.Fatalf("ExtraProxyAccessKeys = %#v, want %#v", envCfg.ExtraProxyAccessKeys, want)
	}
}

func TestEnvConfig_ProxyKeyMaskForRequest(t *testing.T) {
	envCfg := &EnvConfig{
		ProxyAccessKey:       "sk-aaaaaaaaaaaaaaaaaaaa",        // 23 chars
		ExtraProxyAccessKeys: []string{"sk-bbbbbbbbbbbbbbbbbbbb"}, // 23 chars
	}

	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "primary key", key: "sk-aaaaaaaaaaaaaaaaaaaa", want: "sk-a***************aaaa"},
		{name: "extra key", key: "sk-bbbbbbbbbbbbbbbbbbbb", want: "sk-b***************bbbb"},
		{name: "unknown key", key: "unknown", want: ""},
		{name: "empty key", key: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := envCfg.ProxyKeyMaskForRequest(tt.key)
			if got != tt.want {
				t.Fatalf("ProxyKeyMaskForRequest(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestMaskKeyForIdentity(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "long key 19 chars", key: "sk-abcdefghijklmnop", want: "sk-a***********mnop"}, // 19 chars
		{name: "short key 8 chars", key: "12345678", want: "********"},
		{name: "short key 4 chars", key: "abcd", want: "****"},
		{name: "key 9 chars", key: "123456789", want: "1234*6789"},
		{name: "empty key", key: "", want: ""},
		{name: "key 23 chars", key: "sk-aaaaaaaaaaaaaaaaaaaa", want: "sk-a***************aaaa"}, // 23 chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskKeyForIdentity(tt.key)
			if got != tt.want {
				t.Fatalf("maskKeyForIdentity(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
