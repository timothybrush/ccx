package autopilot

import (
	"database/sql"
	"github.com/BenedictKing/ccx/internal/errutil"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ── ProfileChangelogStore（Phase 3A：环形内存 + SQLite 落盘，仿 TraceStore 形状）──

const (
	// changelogMaxRecords 内存环形保留最大记录数。
	changelogMaxRecords = 500

	// changelogRetentionDays 数据保留天数（设计 §3.8：30 天滚动清理）。
	changelogRetentionDays = 30

	// changelogPruneEveryNWrites 每写入 N 次机会性触发一次过期清理，
	// 不单开定时任务，避免为低频事件引入额外后台 goroutine。
	changelogPruneEveryNWrites = 100
)

// ProfileChangelogStore 管理 ProfileChangeEvent 的内存环形缓存与 SQLite 持久化。
// 全部写入落盘（变更事件本身就是低频信号，不需要 TraceStore 那种抽样策略）。
type ProfileChangelogStore struct {
	db *sql.DB

	records []*ProfileChangeEvent // 按时间排序，最新在末尾
	mu      sync.RWMutex

	writeCount atomic.Int64
}

// NewProfileChangelogStoreWithDB 使用外部 *sql.DB 创建 ProfileChangelogStore。
// db 为 nil 时只使用内存环形缓存，不落盘（不影响调用方，fail-safe）。
func NewProfileChangelogStoreWithDB(db *sql.DB) (*ProfileChangelogStore, error) {
	if db != nil {
		if err := initProfileChangelogSchema(db); err != nil {
			return nil, err
		}
	}

	store := &ProfileChangelogStore{
		db:      db,
		records: make([]*ProfileChangeEvent, 0, changelogMaxRecords),
	}

	if db != nil {
		if err := store.loadRecent(); err != nil {
			log.Printf("[ProfileChangelog-Init] 警告: 加载历史记录失败: %v", err)
		}
	}

	return store, nil
}

// initProfileChangelogSchema 建表迁移。
func initProfileChangelogSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS profile_changelog (
    event_uid    TEXT PRIMARY KEY,
    channel_uid  TEXT NOT NULL,
    channel_kind TEXT NOT NULL,
    endpoint_uid TEXT NOT NULL DEFAULT '',
    metrics_key  TEXT NOT NULL DEFAULT '',
    event_type   TEXT NOT NULL,
    summary      TEXT NOT NULL DEFAULT '',
    old_value    TEXT NOT NULL DEFAULT '',
    new_value    TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_profile_changelog_channel  ON profile_changelog(channel_uid, created_at);
CREATE INDEX IF NOT EXISTS idx_profile_changelog_endpoint ON profile_changelog(endpoint_uid, created_at);
`
	_, err := db.Exec(schema)
	return err
}

// loadRecent 从 SQLite 加载最近 changelogMaxRecords 条到内存。
func (s *ProfileChangelogStore) loadRecent() error {
	rows, err := s.db.Query(`
		SELECT event_uid, channel_uid, channel_kind, endpoint_uid, metrics_key,
		       event_type, summary, old_value, new_value, created_at
		FROM profile_changelog
		ORDER BY created_at DESC
		LIMIT ?`, changelogMaxRecords)
	if err != nil {
		return err
	}
	defer errutil.IgnoreDeferred(rows.Close)

	var loaded []*ProfileChangeEvent
	for rows.Next() {
		ev, err := scanChangelogRow(rows)
		if err != nil {
			log.Printf("[ProfileChangelog-LoadRecent] 跳过损坏行: %v", err)
			continue
		}
		loaded = append(loaded, ev)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// 反转为时间升序（SQL 用 DESC 取最新，内存按 ASC 存储）
	s.mu.Lock()
	for i := len(loaded) - 1; i >= 0; i-- {
		s.records = append(s.records, loaded[i])
	}
	s.mu.Unlock()

	return nil
}

func scanChangelogRow(rows *sql.Rows) (*ProfileChangeEvent, error) {
	var ev ProfileChangeEvent
	var eventType, createdAt string

	err := rows.Scan(
		&ev.EventUID, &ev.ChannelUID, &ev.ChannelKind, &ev.EndpointUID, &ev.MetricsKey,
		&eventType, &ev.Summary, &ev.OldValue, &ev.NewValue, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	ev.EventType = ProfileChangeEventType(eventType)
	ev.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &ev, nil
}

// ── CRUD ──

// Record 记录一条画像变更事件。自动填充缺失的 EventUID/CreatedAt。
// 超出内存上限时淘汰最旧记录；SQLite 落盘失败只记日志，不影响调用方（fail-open）。
func (s *ProfileChangelogStore) Record(ev ProfileChangeEvent) {
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	if ev.EventUID == "" {
		ev.EventUID = GenerateChangeEventUID(ev.EndpointUID, ev.EventType, ev.CreatedAt)
	}

	rec := ev
	s.mu.Lock()
	s.records = append(s.records, &rec)
	if len(s.records) > changelogMaxRecords {
		excess := len(s.records) - changelogMaxRecords
		s.records = s.records[excess:]
	}
	s.mu.Unlock()

	if s.db == nil {
		return
	}
	if err := s.persist(&rec); err != nil {
		log.Printf("[ProfileChangelog-Record] 警告: 落盘失败 uid=%s: %v", rec.EventUID, err)
	}

	if count := s.writeCount.Add(1); count%changelogPruneEveryNWrites == 0 {
		if err := s.pruneExpired(); err != nil {
			log.Printf("[ProfileChangelog-Prune] 警告: 清理过期数据失败: %v", err)
		}
	}
}

// ListRecent 返回最近 N 条记录（时间降序）。n<=0 或超过总量时返回全部。
func (s *ProfileChangelogStore) ListRecent(n int) []*ProfileChangeEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.records)
	if n <= 0 || n > total {
		n = total
	}

	start := total - n
	result := make([]*ProfileChangeEvent, n)
	for i := 0; i < n; i++ {
		cp := *s.records[start+i]
		result[n-1-i] = &cp
	}
	return result
}

// ListByChannel 返回指定 channelUID 下最近 N 条记录（时间降序）。
func (s *ProfileChangelogStore) ListByChannel(channelUID string, n int) []*ProfileChangeEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n <= 0 {
		n = changelogMaxRecords
	}

	var result []*ProfileChangeEvent
	for i := len(s.records) - 1; i >= 0 && len(result) < n; i-- {
		if s.records[i].ChannelUID != channelUID {
			continue
		}
		cp := *s.records[i]
		result = append(result, &cp)
	}
	return result
}

// ── 持久化辅助 ──

func (s *ProfileChangelogStore) persist(ev *ProfileChangeEvent) error {
	_, err := s.db.Exec(`
INSERT INTO profile_changelog
    (event_uid, channel_uid, channel_kind, endpoint_uid, metrics_key,
     event_type, summary, old_value, new_value, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(event_uid) DO NOTHING
`,
		ev.EventUID,
		ev.ChannelUID,
		ev.ChannelKind,
		ev.EndpointUID,
		ev.MetricsKey,
		string(ev.EventType),
		ev.Summary,
		ev.OldValue,
		ev.NewValue,
		ev.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// pruneExpired 删除超过 changelogRetentionDays 天的记录。
func (s *ProfileChangelogStore) pruneExpired() error {
	if s.db == nil {
		return nil
	}
	cutoff := time.Now().UTC().Add(-changelogRetentionDays * 24 * time.Hour).Format(time.RFC3339)
	_, err := s.db.Exec(`DELETE FROM profile_changelog WHERE created_at < ?`, cutoff)
	return err
}

// Close 关闭 ProfileChangelogStore。当前无需特殊清理（复用外部传入的 db 连接）。
func (s *ProfileChangelogStore) Close() error {
	return nil
}
