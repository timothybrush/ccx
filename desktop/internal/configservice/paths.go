package configservice

import (
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) claudeSettingsPath() string {
	return filepath.Join(s.homeDir, ".claude", "settings.json")
}

func (s *Service) claudeConfigPath() string {
	return filepath.Join(s.homeDir, ".claude", "config.json")
}

func (s *Service) claudeRootConfigPath() string {
	return filepath.Join(s.homeDir, ".claude.json")
}

func (s *Service) codexConfigPath() string {
	return filepath.Join(s.homeDir, ".codex", "config.toml")
}

func (s *Service) codexAuthPath() string {
	return filepath.Join(s.homeDir, ".codex", "auth.json")
}

func (s *Service) codexSessionsDir() string {
	return filepath.Join(s.homeDir, ".codex", "sessions")
}

func (s *Service) codexArchivedSessionsDir() string {
	return filepath.Join(s.homeDir, ".codex", "archived_sessions")
}

func (s *Service) codexStateDBPath() string {
	return filepath.Join(s.homeDir, ".codex", "state_5.sqlite")
}

func (s *Service) claudeStatePath() string {
	return filepath.Join(s.stateDir, "claude.json")
}

func (s *Service) codexStatePath() string {
	return filepath.Join(s.stateDir, "codex.json")
}

func (s *Service) providerKeysPath() string {
	return filepath.Join(s.stateDir, "provider-keys.json")
}

func (s *Service) currentDataDir() string {
	return filepath.Dir(s.stateDir)
}

func (s *Service) readCurrentProxyAccessKey() string {
	if key := readEnvValueFromFile(filepath.Join(s.currentDataDir(), ".env"), "PROXY_ACCESS_KEY"); strings.TrimSpace(key) != "" {
		return key
	}
	if key := strings.TrimSpace(os.Getenv("PROXY_ACCESS_KEY")); key != "" {
		return key
	}
	return ""
}

func (s *Service) openCodeConfigPath() string {
	return filepath.Join(s.homeDir, ".config", "opencode", "opencode.jsonc")
}

func (s *Service) openCodeAuthPath() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dir != "" {
		return filepath.Join(dir, "opencode", "auth.json")
	}
	return filepath.Join(s.homeDir, ".local", "share", "opencode", "auth.json")
}

func (s *Service) openCodeStatePath() string {
	return filepath.Join(s.stateDir, "opencode.json")
}
