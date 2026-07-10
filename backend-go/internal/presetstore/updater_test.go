package presetstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestPresetUpdaterCheckOnceUpdatesStoreAndCache(t *testing.T) {
	cacheDir := t.TempDir()
	bundle := validBundle()
	bundle.DataVersion = "2026.07.10-2"

	server := newPresetTestServer(t, bundle, false, false)
	defer server.Close()

	store := NewPresetStore(nil)
	updater := NewPresetUpdater(store, UpdaterConfig{
		Enabled:                 true,
		IndexURL:                server.URL + "/index.json",
		Interval:                time.Minute,
		CacheDir:                cacheDir,
		AllowInsecureForTesting: true,
	})

	if err := updater.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}
	if store.DataVersion() != bundle.DataVersion {
		t.Fatalf("store version = %q, want %q", store.DataVersion(), bundle.DataVersion)
	}
	if updater.Status().Source != "remote" {
		t.Fatalf("status source = %q, want remote", updater.Status().Source)
	}
	loaded, err := LoadCache(cacheDir)
	if err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}
	if loaded.DataVersion != bundle.DataVersion {
		t.Fatalf("cached version = %q, want %q", loaded.DataVersion, bundle.DataVersion)
	}
}

func TestPresetUpdaterCheckOnceRejectsTamperedShard(t *testing.T) {
	bundle := validBundle()
	bundle.DataVersion = "2026.07.10-2"
	server := newPresetTestServer(t, bundle, true, false)
	defer server.Close()

	store := NewPresetStore(nil)
	updater := NewPresetUpdater(store, UpdaterConfig{
		Enabled:                 true,
		IndexURL:                server.URL + "/index.json",
		Interval:                time.Minute,
		AllowInsecureForTesting: true,
	})

	if err := updater.CheckOnce(context.Background()); err == nil {
		t.Fatal("CheckOnce() error = nil, want hash mismatch")
	}
	if store.DataVersion() != "" {
		t.Fatalf("store version = %q, want embedded empty version", store.DataVersion())
	}
}

func TestPresetUpdaterCheckOnceRejectsHigherSchema(t *testing.T) {
	bundle := validBundle()
	bundle.DataVersion = "2026.07.10-2"
	server := newPresetTestServer(t, bundle, false, true)
	defer server.Close()

	store := NewPresetStore(nil)
	updater := NewPresetUpdater(store, UpdaterConfig{
		Enabled:                 true,
		IndexURL:                server.URL + "/index.json",
		Interval:                time.Minute,
		AllowInsecureForTesting: true,
	})

	if err := updater.CheckOnce(context.Background()); err == nil {
		t.Fatal("CheckOnce() error = nil, want higher schema error")
	}
}

func TestPresetUpdaterCheckOnceSkipsOlderVersion(t *testing.T) {
	store := NewPresetStore(validBundle())
	incoming := validBundle()
	incoming.DataVersion = "2026.07.10-0"
	server := newPresetTestServer(t, incoming, false, false)
	defer server.Close()

	updater := NewPresetUpdater(store, UpdaterConfig{
		Enabled:                 true,
		IndexURL:                server.URL + "/index.json",
		Interval:                time.Minute,
		AllowInsecureForTesting: true,
	})

	if err := updater.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}
	if store.DataVersion() != "2026.07.10-1" {
		t.Fatalf("store version = %q, want unchanged 2026.07.10-1", store.DataVersion())
	}
}

func TestPresetUpdaterCheckOnceAcceptsLegacySubscriptionKind(t *testing.T) {
	bundle := validBundle()
	bundle.DataVersion = "2026.07.10-2"
	subscriptionBytes, err := json.Marshal(bundle.Subscription)
	if err != nil {
		t.Fatalf("json.Marshal(subscription) error = %v", err)
	}
	index := PresetIndex{
		SchemaVersion: bundle.SchemaVersion,
		DataVersion:   bundle.DataVersion,
		Shards: []PresetIndexShard{{
			Kind:   "subscription",
			URL:    "./subscription.json",
			SHA256: sha256Hex(subscriptionBytes),
		}},
	}
	indexBytes, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("json.Marshal(index) error = %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = w.Write(indexBytes)
		case "/subscription.json":
			_, _ = w.Write(subscriptionBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := NewPresetStore(nil)
	updater := NewPresetUpdater(store, UpdaterConfig{
		Enabled:                 true,
		IndexURL:                server.URL + "/index.json",
		Interval:                time.Minute,
		AllowInsecureForTesting: true,
	})

	if err := updater.CheckOnce(context.Background()); err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}
	if store.DataVersion() != bundle.DataVersion {
		t.Fatalf("store version = %q, want %q", store.DataVersion(), bundle.DataVersion)
	}
}

func TestPresetUpdaterLoadCacheAtStartupFallbackOnCorruption(t *testing.T) {
	cacheDir := t.TempDir()
	bundle := validBundle()
	if err := SaveCache(cacheDir, bundle); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}
	bundlePath := filepath.Join(cacheDir, bundleFileName)
	if err := os.WriteFile(bundlePath, []byte("broken"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := NewPresetStore(nil)
	updater := NewPresetUpdater(store, UpdaterConfig{CacheDir: cacheDir})
	if err := updater.LoadCacheAtStartup(); err == nil {
		t.Fatal("LoadCacheAtStartup() error = nil, want corruption error")
	}
	if updater.Status().CacheValid {
		t.Fatal("CacheValid = true, want false")
	}
	if store.DataVersion() != "" {
		t.Fatalf("store version = %q, want embedded version", store.DataVersion())
	}
}

func TestPresetUpdaterStopIsIdempotent(t *testing.T) {
	bundle := validBundle()
	bundle.DataVersion = "2026.07.10-2"
	server := newPresetTestServer(t, bundle, false, false)
	defer server.Close()

	store := NewPresetStore(nil)
	updater := NewPresetUpdater(store, UpdaterConfig{
		Enabled:                 true,
		IndexURL:                server.URL + "/index.json",
		Interval:                20 * time.Millisecond,
		AllowInsecureForTesting: true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updater.Start(ctx)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if updater.Status().LastCheckAt != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if updater.Status().LastCheckAt == nil {
		t.Fatal("updater did not perform initial check")
	}

	updater.Stop()
	updater.Stop()
	if updater.Status().Running {
		t.Fatal("Running = true, want false")
	}
}

func TestStatusHandlerNilUpdater(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/presets/status", StatusHandler(nil))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/presets/status", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码=%d，期望 200", w.Code)
	}
	var status UpdaterStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("解析状态响应失败: %v", err)
	}
	if status.Enabled || status.Source != "embedded" {
		t.Fatalf("nil updater 状态=%+v，期望 disabled/embedded", status)
	}
}

func newPresetTestServer(t *testing.T, bundle *PresetBundle, tamperHash bool, higherSchema bool) *httptest.Server {
	t.Helper()

	subscriptionBytes, err := json.Marshal(bundle.Subscription)
	if err != nil {
		t.Fatalf("json.Marshal(subscription) error = %v", err)
	}
	index := PresetIndex{
		SchemaVersion: bundle.SchemaVersion,
		DataVersion:   bundle.DataVersion,
		PublishedAt:   time.Now().UTC(),
		Shards: []PresetIndexShard{{
			Kind:   "subscriptionPreset",
			URL:    "./subscription.json",
			SHA256: sha256Hex(subscriptionBytes),
		}},
	}
	if tamperHash {
		index.Shards[0].SHA256 = "deadbeef"
	}
	if higherSchema {
		index.SchemaVersion = CurrentSchemaVersion + 1
	}
	indexBytes, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("json.Marshal(index) error = %v", err)
	}

	var indexHits atomic.Int32
	var shardHits atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			indexHits.Add(1)
			_, _ = w.Write(indexBytes)
		case "/subscription.json":
			shardHits.Add(1)
			_, _ = w.Write(subscriptionBytes)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestCompareDataVersion(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "equal", a: "2026.07.10-1", b: "2026.07.10-1", want: 0},
		{name: "empty old", a: "2026.07.10-1", b: "", want: 1},
		{name: "older", a: "2026.07.10-0", b: "2026.07.10-1", want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareDataVersion(tt.a, tt.b); got != tt.want {
				t.Fatalf("compareDataVersion(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestPresetUpdaterLoadCacheAtStartupSetsCacheSource(t *testing.T) {
	cacheDir := t.TempDir()
	bundle := validBundle()
	bundle.DataVersion = "2026.07.10-9"
	if err := SaveCache(cacheDir, bundle); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	store := NewPresetStore(nil)
	updater := NewPresetUpdater(store, UpdaterConfig{CacheDir: cacheDir})
	if err := updater.LoadCacheAtStartup(); err != nil {
		t.Fatalf("LoadCacheAtStartup() error = %v", err)
	}
	status := updater.Status()
	if status.Source != "cache" {
		t.Fatalf("source = %q, want cache", status.Source)
	}
	if status.DataVersion != bundle.DataVersion {
		t.Fatalf("dataVersion = %q, want %q", status.DataVersion, bundle.DataVersion)
	}
}

func TestPresetUpdaterCheckOnceRequiresHTTPSByDefault(t *testing.T) {
	bundle := validBundle()
	server := newPresetTestServer(t, bundle, false, false)
	defer server.Close()

	updater := NewPresetUpdater(NewPresetStore(nil), UpdaterConfig{
		Enabled:  true,
		IndexURL: server.URL + "/index.json",
		Interval: time.Minute,
	})
	if err := updater.CheckOnce(context.Background()); err == nil {
		t.Fatal("CheckOnce() error = nil, want HTTPS validation error")
	}
}

func TestPresetUpdaterStartDisabledIsNoop(t *testing.T) {
	updater := NewPresetUpdater(NewPresetStore(nil), UpdaterConfig{Enabled: false})
	updater.Start(context.Background())
	if updater.Status().Running {
		t.Fatal("Running = true, want false")
	}
}

func ExamplePresetUpdater_Status() {
	updater := NewPresetUpdater(NewPresetStore(nil), UpdaterConfig{Enabled: true})
	fmt.Println(updater.Status().Enabled)
	// Output: true
}
