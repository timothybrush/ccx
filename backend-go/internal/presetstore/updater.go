package presetstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultUpdaterInterval = 30 * time.Minute
	defaultShardSizeLimit  = 2 << 20
	defaultHTTPTimeout     = 10 * time.Second
)

// UpdaterConfig 控制远程预置更新器行为。
type UpdaterConfig struct {
	Enabled bool

	IndexURL string
	Interval time.Duration
	CacheDir string

	HTTPClient *http.Client

	// AllowInsecureForTesting 仅供测试允许 HTTP，生产必须保持 false。
	AllowInsecureForTesting bool
}

type PresetIndex struct {
	SchemaVersion int                `json:"schemaVersion"`
	DataVersion   string             `json:"dataVersion"`
	PublishedAt   time.Time          `json:"publishedAt"`
	Shards        []PresetIndexShard `json:"shards"`
}

type PresetIndexShard struct {
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type UpdaterStatus struct {
	Enabled       bool       `json:"enabled"`
	Running       bool       `json:"running"`
	Checking      bool       `json:"checking"`
	Source        string     `json:"source,omitempty"`
	DataVersion   string     `json:"dataVersion,omitempty"`
	LastCheckAt   *time.Time `json:"lastCheckAt,omitempty"`
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`
	NextCheckAt   *time.Time `json:"nextCheckAt,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
	CacheValid    bool       `json:"cacheValid"`
}

type PresetUpdater struct {
	store  *PresetStore
	config UpdaterConfig

	client *http.Client

	mu            sync.RWMutex
	running       bool
	checking      bool
	source        string
	dataVersion   string
	lastCheckAt   *time.Time
	lastSuccessAt *time.Time
	nextCheckAt   *time.Time
	lastError     string
	cacheValid    bool

	startStopMu sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
}

func NewPresetUpdater(store *PresetStore, cfg UpdaterConfig) *PresetUpdater {
	if store == nil {
		store = NewPresetStore(nil)
	}
	if cfg.Interval <= 0 {
		cfg.Interval = defaultUpdaterInterval
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	} else if client.Timeout <= 0 {
		clone := *client
		clone.Timeout = defaultHTTPTimeout
		client = &clone
	}

	u := &PresetUpdater{
		store:  store,
		config: cfg,
		client: client,
		source: "embedded",
	}
	u.dataVersion = store.DataVersion()
	return u
}

// LoadCacheAtStartup 尝试从磁盘缓存恢复 bundle；成功时立即切换为 cache 源。
func (u *PresetUpdater) LoadCacheAtStartup() error {
	if strings.TrimSpace(u.config.CacheDir) == "" {
		return fmt.Errorf("[PresetUpdater-Cache] cacheDir 未配置")
	}
	bundle, err := LoadCache(u.config.CacheDir)
	if err != nil {
		u.setCacheValid(false)
		return err
	}
	u.store.Swap(bundle)
	u.mu.Lock()
	u.source = "cache"
	u.dataVersion = bundle.DataVersion
	u.cacheValid = true
	u.lastError = ""
	u.mu.Unlock()
	return nil
}

func (u *PresetUpdater) Start(ctx context.Context) {
	u.startStopMu.Lock()
	defer u.startStopMu.Unlock()
	if u.running {
		return
	}
	if !u.config.Enabled {
		u.mu.Lock()
		u.running = false
		u.mu.Unlock()
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	u.cancel = cancel
	u.done = make(chan struct{})
	now := time.Now()
	next := now.Add(u.config.Interval)

	u.mu.Lock()
	u.running = true
	u.lastError = ""
	u.nextCheckAt = &next
	u.mu.Unlock()

	go u.loop(runCtx)
}

func (u *PresetUpdater) Stop() {
	u.startStopMu.Lock()
	cancel := u.cancel
	done := u.done
	if !u.running {
		u.startStopMu.Unlock()
		return
	}
	u.cancel = nil
	u.done = nil
	u.startStopMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}

	u.mu.Lock()
	u.running = false
	u.checking = false
	u.nextCheckAt = nil
	u.mu.Unlock()
}

func (u *PresetUpdater) Status() UpdaterStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return UpdaterStatus{
		Enabled:       u.config.Enabled,
		Running:       u.running,
		Checking:      u.checking,
		Source:        u.source,
		DataVersion:   u.dataVersion,
		LastCheckAt:   cloneTimePtr(u.lastCheckAt),
		LastSuccessAt: cloneTimePtr(u.lastSuccessAt),
		NextCheckAt:   cloneTimePtr(u.nextCheckAt),
		LastError:     u.lastError,
		CacheValid:    u.cacheValid,
	}
}

func (u *PresetUpdater) CheckOnce(ctx context.Context) error {
	if err := u.validateIndexURL(); err != nil {
		u.finishCheck(time.Now(), err, false)
		return err
	}

	startedAt := time.Now()
	u.beginCheck()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("[PresetUpdater-Check] panic: %v", r)
			u.finishCheck(startedAt, err, false)
			panic(r)
		}
	}()

	indexURL, err := url.Parse(u.config.IndexURL)
	if err != nil {
		err = fmt.Errorf("[PresetUpdater-Check] 解析 index URL 失败: %w", err)
		u.finishCheck(startedAt, err, false)
		return err
	}

	indexBytes, err := u.fetchBytes(ctx, indexURL.String(), 0)
	if err != nil {
		err = fmt.Errorf("[PresetUpdater-Check] 拉取 index 失败: %w", err)
		u.finishCheck(startedAt, err, false)
		return err
	}

	var index PresetIndex
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		err = fmt.Errorf("[PresetUpdater-Check] 解析 index 失败: %w", err)
		u.finishCheck(startedAt, err, false)
		return err
	}
	if index.SchemaVersion > CurrentSchemaVersion {
		err = fmt.Errorf("[PresetUpdater-Check] schemaVersion %d 高于本二进制支持的 %d", index.SchemaVersion, CurrentSchemaVersion)
		u.finishCheck(startedAt, err, false)
		return err
	}

	currentVersion := u.store.DataVersion()
	if compareDataVersion(index.DataVersion, currentVersion) <= 0 {
		log.Printf("[PresetUpdater-Check] 跳过非新版本 index: current=%q incoming=%q", currentVersion, index.DataVersion)
		u.finishCheck(startedAt, nil, false)
		return nil
	}

	bundle, err := u.buildBundleFromIndex(ctx, indexURL, &index)
	if err != nil {
		u.finishCheck(startedAt, err, false)
		return err
	}
	if err := Validate(bundle); err != nil {
		err = fmt.Errorf("[PresetUpdater-Check] 校验 bundle 失败: %w", err)
		u.finishCheck(startedAt, err, false)
		return err
	}
	if strings.TrimSpace(u.config.CacheDir) != "" {
		if err := SaveCache(u.config.CacheDir, bundle); err != nil {
			err = fmt.Errorf("[PresetUpdater-Cache] 写缓存失败: %w", err)
			u.finishCheck(startedAt, err, false)
			return err
		}
	}

	u.store.Swap(bundle)
	u.finishCheck(startedAt, nil, true)
	log.Printf("[PresetUpdater-Check] 预置更新成功: version=%q", bundle.DataVersion)
	return nil
}

func (u *PresetUpdater) loop(ctx context.Context) {
	defer close(u.done)

	if err := u.CheckOnce(ctx); err != nil {
		log.Printf("[PresetUpdater-Start] 首次检查失败: %v", err)
	}

	ticker := time.NewTicker(u.config.Interval)
	defer ticker.Stop()

	for {
		next := time.Now().Add(u.config.Interval)
		u.mu.Lock()
		u.nextCheckAt = &next
		u.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := u.CheckOnce(ctx); err != nil {
				log.Printf("[PresetUpdater-Loop] 周期检查失败: %v", err)
			}
		}
	}
}

func (u *PresetUpdater) buildBundleFromIndex(ctx context.Context, indexURL *url.URL, index *PresetIndex) (*PresetBundle, error) {
	var subscription SubscriptionPreset
	foundSubscription := false
	for _, shard := range index.Shards {
		if shard.Kind != "subscriptionPreset" && shard.Kind != "subscription" {
			continue
		}
		if strings.TrimSpace(shard.SHA256) == "" {
			return nil, fmt.Errorf("[PresetUpdater-Check] subscription shard 缺少 sha256")
		}
		resolved, err := indexURL.Parse(shard.URL)
		if err != nil {
			return nil, fmt.Errorf("[PresetUpdater-Check] 解析 shard URL 失败: %w", err)
		}
		body, err := u.fetchBytes(ctx, resolved.String(), defaultShardSizeLimit)
		if err != nil {
			return nil, fmt.Errorf("[PresetUpdater-Check] 拉取 subscription shard 失败: %w", err)
		}
		if !strings.EqualFold(strings.TrimSpace(shard.SHA256), sha256Hex(body)) {
			return nil, fmt.Errorf("[PresetUpdater-Check] subscription shard SHA256 校验失败")
		}
		if err := json.Unmarshal(body, &subscription); err != nil {
			return nil, fmt.Errorf("[PresetUpdater-Check] 解析 subscription shard 失败: %w", err)
		}
		foundSubscription = true
		break
	}
	if !foundSubscription {
		return nil, fmt.Errorf("[PresetUpdater-Check] index 缺少 subscription shard")
	}

	return &PresetBundle{
		SchemaVersion: index.SchemaVersion,
		DataVersion:   index.DataVersion,
		Subscription:  subscription,
	}, nil
}

func (u *PresetUpdater) fetchBytes(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	if err := u.validateRemoteURL(rawURL); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	reader := io.Reader(resp.Body)
	if limit > 0 {
		reader = io.LimitReader(resp.Body, limit+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if limit > 0 && int64(len(body)) > limit {
		return nil, fmt.Errorf("响应体超过上限 %d bytes", limit)
	}
	return body, nil
}

func (u *PresetUpdater) validateIndexURL() error {
	if strings.TrimSpace(u.config.IndexURL) == "" {
		return fmt.Errorf("[PresetUpdater-Config] indexURL 不能为空")
	}
	return u.validateRemoteURL(u.config.IndexURL)
}

func (u *PresetUpdater) validateRemoteURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("[PresetUpdater-Config] 非法 URL: %w", err)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && u.config.AllowInsecureForTesting {
		return nil
	}
	return fmt.Errorf("[PresetUpdater-Config] 仅允许 HTTPS URL")
}

func (u *PresetUpdater) beginCheck() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.checking = true
}

func (u *PresetUpdater) finishCheck(at time.Time, err error, updated bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.checking = false
	u.lastCheckAt = cloneTimePtr(&at)
	next := at.Add(u.config.Interval)
	u.nextCheckAt = &next
	if err != nil {
		u.lastError = err.Error()
		return
	}
	u.lastError = ""
	if updated {
		u.source = "remote"
		u.dataVersion = u.store.DataVersion()
		u.cacheValid = strings.TrimSpace(u.config.CacheDir) != ""
		u.lastSuccessAt = cloneTimePtr(&at)
	}
}

func (u *PresetUpdater) setCacheValid(valid bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.cacheValid = valid
}

func cloneTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}

func compareDataVersion(a, b string) int {
	as := strings.TrimSpace(a)
	bs := strings.TrimSpace(b)
	switch {
	case as == bs:
		return 0
	case as == "":
		return -1
	case bs == "":
		return 1
	case as < bs:
		return -1
	default:
		return 1
	}
}

// StatusHandler 返回 GET /api/presets/status 的只读处理器。
// updater 为 nil 时返回 disabled 状态而非错误，避免预置更新功能影响服务健康。
func StatusHandler(updater *PresetUpdater) gin.HandlerFunc {
	return func(c *gin.Context) {
		if updater == nil {
			c.JSON(http.StatusOK, UpdaterStatus{Enabled: false, Source: "embedded"})
			return
		}
		c.JSON(http.StatusOK, updater.Status())
	}
}
