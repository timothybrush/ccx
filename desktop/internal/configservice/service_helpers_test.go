package configservice

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

// ── Part B: Service 集成测试（t.TempDir） ─────────────────

func newTestService(t *testing.T) *Service {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	dataDir := filepath.Join(t.TempDir(), "ccx-data")
	svc, err := New(dataDir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return svc
}

func assertDiffContains(t *testing.T, result ConfigDiffResult, want string) {
	t.Helper()
	for _, file := range result.Files {
		for _, line := range file.Lines {
			if strings.Contains(line.Content, want) {
				return
			}
		}
	}
	t.Fatalf("diff does not contain %q", want)
}

func assertDiffDoesNotLeak(t *testing.T, result ConfigDiffResult, rawValues ...string) {
	t.Helper()
	for _, file := range result.Files {
		for _, line := range file.Lines {
			for _, raw := range rawValues {
				if strings.Contains(line.Content, raw) {
					t.Fatalf("diff for %s leaked raw sensitive value %q in line: %q", file.Path, raw, line.Content)
				}
			}
		}
	}
}

func writeJSON(path string, data any) {
	b, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, append(b, '\n'), 0o644)
}

func readJSON(path string, dest any) {
	b, _ := os.ReadFile(path)
	json.Unmarshal(b, dest)
}

func writeTextForTest(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s failed: %v", path, err)
	}
}

func writeCodexSession(t *testing.T, path, provider, body string) {
	t.Helper()
	firstLine, err := json.Marshal(map[string]any{
		"type": "session_meta",
		"payload": map[string]any{
			"id":             filepath.Base(path),
			"model_provider": provider,
		},
	})
	if err != nil {
		t.Fatalf("marshal session failed: %v", err)
	}
	writeTextForTest(t, path, string(firstLine)+"\n"+body+"\n")
}

func readCodexSessionProvider(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s failed: %v", path, err)
	}
	firstLine := strings.SplitN(string(content), "\n", 2)[0]
	var meta map[string]any
	if err := json.Unmarshal([]byte(firstLine), &meta); err != nil {
		t.Fatalf("unmarshal first line failed: %v", err)
	}
	payload, ok := meta["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload missing in %s", path)
	}
	provider, ok := payload["model_provider"].(string)
	if !ok {
		t.Fatalf("model_provider missing in %s", path)
	}
	return provider
}

func openTestSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir sqlite dir failed: %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	return db
}
