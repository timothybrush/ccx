package autopilot

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/BenedictKing/ccx/internal/errutil"
	"log"
	"sync"
	"time"
)

// ── AdvisorDecisionRecord（P0.1）──

// AdvisorDecisionRecord advisor shadow 决策记录。
// 记录 hint 与实际调度结果对比，用于准确率统计。
// 只保存 hash、bucket、结构化特征和结果，不保存 prompt 明文。
type AdvisorDecisionRecord struct {
	DecisionUID       string       `json:"decisionUid"`
	RequestUID        string       `json:"requestUid,omitempty"`
	AdvisorUID        string       `json:"advisorUid"`
	AdvisorOriginTier string       `json:"advisorOriginTier"` // first | local
	Mode              AdvisorState `json:"mode"`              // shadow | candidate | active
	TaskClass         TaskClass    `json:"taskClass"`
	PromptHash        string       `json:"promptHash,omitempty"` // 不存明文 prompt
	InputTokenBucket  string       `json:"inputTokenBucket"`

	Hint            TrustedRoutingHint `json:"hint"`
	DefaultPlanHash string             `json:"defaultPlanHash"`
	Applied         bool               `json:"applied"`

	Outcome              string    `json:"outcome"`                    // matched | fallback | user_override | upstream_error | timeout
	Reason               string    `json:"reason,omitempty"`           // slo_regression | manual | 自动回滚触发原因
	MisrouteSeverity     string    `json:"misrouteSeverity,omitempty"` // none | minor | major | critical
	LatencyMs            int64     `json:"latencyMs"`
	EstimatedAdvisorCost float64   `json:"estimatedAdvisorCost,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
}

// ── AdvisorStats 准确率统计 ──

// AdvisorStats advisor 决策统计汇总。
type AdvisorStats struct {
	TotalSamples       int `json:"totalSamples"`
	MatchedCount       int `json:"matchedCount"`       // hint 与实际路由一致
	FallbackCount      int `json:"fallbackCount"`      // 降级到默认路由
	UserOverrideCount  int `json:"userOverrideCount"`  // 用户覆盖
	UpstreamErrorCount int `json:"upstreamErrorCount"` // 上游错误
	TimeoutCount       int `json:"timeoutCount"`       // 超时

	MatchRate      float64 `json:"matchRate"`    // 命中率 = matched / total
	AvgLatencyMs   float64 `json:"avgLatencyMs"` // 平均 hint 生成耗时
	TotalLatencyMs int64   `json:"totalLatencyMs"`

	CriticalMisrouteCount int     `json:"criticalMisrouteCount"` // 严重误判数
	CriticalMisrouteRate  float64 `json:"criticalMisrouteRate"`  // 严重误判率
	FalseDemotionCount    int     `json:"falseDemotionCount"`    // 错误降级数
	FalseDemotionRate     float64 `json:"falseDemotionRate"`     // 错误降级率
}

// ── AdvisorDecisionStore ──

const (
	// advisorDecisionMaxRecords 环形保留最大记录数。
	advisorDecisionMaxRecords = 1000
)

// AdvisorDecisionStore 管理 AdvisorDecisionRecord 的内存缓存与 SQLite 持久化。
// 环形保留最近 1000 条，超出时淘汰最旧记录。
type AdvisorDecisionStore struct {
	db *sql.DB

	records []*AdvisorDecisionRecord // 按时间排序，最新在末尾
	mu      sync.RWMutex
}

// NewAdvisorDecisionStoreWithDB 使用外部 *sql.DB 创建 AdvisorDecisionStore。
func NewAdvisorDecisionStoreWithDB(db *sql.DB) (*AdvisorDecisionStore, error) {
	if err := initAdvisorDecisionSchema(db); err != nil {
		return nil, fmt.Errorf("[AdvisorDecisionStore-Init] 建表失败: %w", err)
	}

	store := &AdvisorDecisionStore{
		db:      db,
		records: make([]*AdvisorDecisionRecord, 0, advisorDecisionMaxRecords),
	}

	if err := store.loadAll(); err != nil {
		return nil, fmt.Errorf("[AdvisorDecisionStore-Init] 加载历史记录失败: %w", err)
	}

	log.Printf("[AdvisorDecisionStore-Init] 初始化完成，已加载 %d 条决策记录", len(store.records))
	return store, nil
}

// initAdvisorDecisionSchema 建表迁移。
func initAdvisorDecisionSchema(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS autopilot_advisor_decisions (
    decision_uid        TEXT PRIMARY KEY,
    request_uid         TEXT,
    advisor_uid         TEXT    NOT NULL,
    advisor_origin_tier TEXT    NOT NULL,
    mode                TEXT    NOT NULL,
    task_class          TEXT    NOT NULL,
    prompt_hash         TEXT,
    input_token_bucket  TEXT    NOT NULL DEFAULT '',
    hint_json           TEXT    NOT NULL,
    default_plan_hash   TEXT    NOT NULL DEFAULT '',
    applied             INTEGER NOT NULL DEFAULT 0,
    outcome             TEXT    NOT NULL DEFAULT '',
    misroute_severity   TEXT    NOT NULL DEFAULT '',
    latency_ms          INTEGER NOT NULL DEFAULT 0,
    estimated_advisor_cost REAL NOT NULL DEFAULT 0,
    reason              TEXT    NOT NULL DEFAULT '',
    created_at          TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_advisor_decisions_created
    ON autopilot_advisor_decisions(created_at);
CREATE INDEX IF NOT EXISTS idx_advisor_decisions_outcome
    ON autopilot_advisor_decisions(outcome);
`
	_, err := db.Exec(schema)
	return err
}

// loadAll 从 SQLite 加载全部记录到内存（按 created_at 排序）。
func (s *AdvisorDecisionStore) loadAll() error {
	rows, err := s.db.Query(`
		SELECT decision_uid, request_uid, advisor_uid, advisor_origin_tier,
		       mode, task_class, prompt_hash, input_token_bucket,
		       hint_json, default_plan_hash, applied,
		       outcome, misroute_severity, latency_ms, estimated_advisor_cost,
		       reason, created_at
		FROM autopilot_advisor_decisions
		ORDER BY created_at ASC`)
	if err != nil {
		return err
	}
	defer errutil.IgnoreDeferred(rows.Close)

	s.mu.Lock()
	defer s.mu.Unlock()

	for rows.Next() {
		rec, err := scanDecisionRow(rows)
		if err != nil {
			log.Printf("[AdvisorDecisionStore-LoadAll] 跳过损坏行: %v", err)
			continue
		}
		s.records = append(s.records, rec)
	}

	// 超出上限时只保留最新
	if len(s.records) > advisorDecisionMaxRecords {
		excess := len(s.records) - advisorDecisionMaxRecords
		s.records = s.records[excess:]
	}

	return rows.Err()
}

// scanDecisionRow 扫描一行并返回 AdvisorDecisionRecord。
func scanDecisionRow(rows *sql.Rows) (*AdvisorDecisionRecord, error) {
	var rec AdvisorDecisionRecord
	var requestUID, promptHash, defaultPlanHash, outcome, misrouteSeverity, reason sql.NullString
	var hintJSON string
	var applied int
	var createdAt string

	err := rows.Scan(
		&rec.DecisionUID, &requestUID, &rec.AdvisorUID, &rec.AdvisorOriginTier,
		&rec.Mode, &rec.TaskClass, &promptHash, &rec.InputTokenBucket,
		&hintJSON, &defaultPlanHash, &applied,
		&outcome, &misrouteSeverity, &rec.LatencyMs, &rec.EstimatedAdvisorCost,
		&reason, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	if requestUID.Valid {
		rec.RequestUID = requestUID.String
	}
	if promptHash.Valid {
		rec.PromptHash = promptHash.String
	}
	rec.DefaultPlanHash = defaultPlanHash.String
	rec.Applied = applied != 0
	rec.Outcome = outcome.String
	if misrouteSeverity.Valid {
		rec.MisrouteSeverity = misrouteSeverity.String
	}
	if reason.Valid {
		rec.Reason = reason.String
	}
	rec.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	if err := json.Unmarshal([]byte(hintJSON), &rec.Hint); err != nil {
		return nil, fmt.Errorf("反序列化 hint 失败: %w", err)
	}

	return &rec, nil
}

// ── CRUD ──

// Record 记录一条决策。自动填充 DecisionUID / CreatedAt。
// 超出上限时淘汰最旧记录。
func (s *AdvisorDecisionStore) Record(rec *AdvisorDecisionRecord) error {
	if rec.DecisionUID == "" {
		rec.DecisionUID = GenerateDecisionUID(rec.AdvisorUID, rec.TaskClass, time.Now())
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}

	// 写入内存
	s.mu.Lock()
	s.records = append(s.records, rec)

	// 淘汰最旧
	if len(s.records) > advisorDecisionMaxRecords {
		excess := len(s.records) - advisorDecisionMaxRecords
		s.records = s.records[excess:]
	}
	s.mu.Unlock()

	// 落盘
	return s.persistRecord(rec)
}

// GetStats 计算统计数据。
func (s *AdvisorDecisionStore) GetStats() AdvisorStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := AdvisorStats{
		TotalSamples: len(s.records),
	}

	for _, rec := range s.records {
		stats.TotalLatencyMs += rec.LatencyMs

		switch rec.Outcome {
		case "matched":
			stats.MatchedCount++
		case "fallback":
			stats.FallbackCount++
		case "user_override":
			stats.UserOverrideCount++
		case "upstream_error":
			stats.UpstreamErrorCount++
		case "timeout":
			stats.TimeoutCount++
		}

		if rec.MisrouteSeverity == "critical" {
			stats.CriticalMisrouteCount++
		}
		if rec.MisrouteSeverity == "major" || rec.MisrouteSeverity == "critical" {
			stats.FalseDemotionCount++
		}
	}

	if stats.TotalSamples > 0 {
		stats.MatchRate = float64(stats.MatchedCount) / float64(stats.TotalSamples)
		stats.AvgLatencyMs = float64(stats.TotalLatencyMs) / float64(stats.TotalSamples)
		stats.CriticalMisrouteRate = float64(stats.CriticalMisrouteCount) / float64(stats.TotalSamples)
		stats.FalseDemotionRate = float64(stats.FalseDemotionCount) / float64(stats.TotalSamples)
	}

	return stats
}

// ListRecent 返回最近 N 条记录（脱敏副本）。
func (s *AdvisorDecisionStore) ListRecent(n int) []*AdvisorDecisionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.records)
	if n <= 0 || n > total {
		n = total
	}

	// 从末尾取最近 N 条，反转为时间降序
	start := total - n
	result := make([]*AdvisorDecisionRecord, n)
	for i := 0; i < n; i++ {
		cp := *s.records[start+i]
		// 脱敏：清除 promptHash
		cp.PromptHash = ""
		result[n-1-i] = &cp
	}
	return result
}

// ── 持久化辅助 ──

// persistRecord 将决策记录写入 SQLite。
func (s *AdvisorDecisionStore) persistRecord(rec *AdvisorDecisionRecord) error {
	hintJSON, err := json.Marshal(rec.Hint)
	if err != nil {
		return fmt.Errorf("[AdvisorDecisionStore-Persist] 序列化 hint 失败 uid=%s: %w", rec.DecisionUID, err)
	}

	applied := 0
	if rec.Applied {
		applied = 1
	}

	_, err = s.db.Exec(`
INSERT INTO autopilot_advisor_decisions
    (decision_uid, request_uid, advisor_uid, advisor_origin_tier,
     mode, task_class, prompt_hash, input_token_bucket,
     hint_json, default_plan_hash, applied,
     outcome, misroute_severity, latency_ms, estimated_advisor_cost,
     reason, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(decision_uid) DO UPDATE SET
    outcome            = excluded.outcome,
    misroute_severity  = excluded.misroute_severity,
    latency_ms         = excluded.latency_ms,
    applied            = excluded.applied
`,
		rec.DecisionUID,
		rec.RequestUID,
		rec.AdvisorUID,
		rec.AdvisorOriginTier,
		string(rec.Mode),
		string(rec.TaskClass),
		rec.PromptHash,
		rec.InputTokenBucket,
		string(hintJSON),
		rec.DefaultPlanHash,
		applied,
		rec.Outcome,
		rec.MisrouteSeverity,
		rec.LatencyMs,
		rec.EstimatedAdvisorCost,
		rec.Reason,
		rec.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("[AdvisorDecisionStore-Persist] 写入失败 uid=%s: %w", rec.DecisionUID, err)
	}
	return nil
}

// Close 关闭 AdvisorDecisionStore。当前无需特殊清理。
func (s *AdvisorDecisionStore) Close() error {
	return nil
}

// ── DecisionUID 生成 ──

// GenerateDecisionUID 生成稳定的决策唯一标识。
func GenerateDecisionUID(advisorUID string, taskClass TaskClass, createdAt time.Time) string {
	h := fmt.Sprintf("%s|%s|%s", advisorUID, string(taskClass), createdAt.Format(time.RFC3339Nano))
	// 简单哈希：SHA256 前 16 位
	sum := sha256Sum(h)
	return "ad_" + sum[:16]
}

// sha256Sum 计算字符串的 SHA256 十六进制摘要。
func sha256Sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
