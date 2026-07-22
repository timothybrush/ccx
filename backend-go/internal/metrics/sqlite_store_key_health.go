package metrics

import (
	"time"
)

// KeyHealthRecord 渠道保活验证结果（key_health 表的一行）
type KeyHealthRecord struct {
	ChannelType         string    // 渠道类型：messages/chat/responses/gemini/images/vectors
	ChannelID           string    // 渠道 ID
	KeyMask             string    // 脱敏的 API Key
	CheckKind           string    // 验证级别："l1" 或 "l2"
	LastCheckAt         time.Time // 上次验证时间
	LastStatus          string    // 验证结果："ok"、"auth_failed" 或 "error"
	ConsecutiveFailures int64     // 连续失败次数
	LatencyMs           int64     // 验证延迟（毫秒）
	ModelCount          int       // 验证时探测到的模型数
	Detail              string    // 失败原因摘要（成功时可为空）
}

// UpsertKeyHealth 写入或覆盖指定 (渠道, key, 验证级别) 的健康验证结果。
// 直接写库（与 UpsertCircuitState 一致），写入量低，无需走内存缓冲。
func (s *SQLiteStore) UpsertKeyHealth(rec KeyHealthRecord) error {
	_, err := s.db.Exec(`
		INSERT INTO key_health (
			channel_type, channel_id, key_mask, check_kind,
			last_check_at, last_status, consecutive_failures,
			latency_ms, model_count, detail
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(channel_type, channel_id, key_mask, check_kind) DO UPDATE SET
			last_check_at = excluded.last_check_at,
			last_status = excluded.last_status,
			consecutive_failures = excluded.consecutive_failures,
			latency_ms = excluded.latency_ms,
			model_count = excluded.model_count,
			detail = excluded.detail
	`, rec.ChannelType, rec.ChannelID, rec.KeyMask, rec.CheckKind,
		rec.LastCheckAt.Unix(), rec.LastStatus, rec.ConsecutiveFailures,
		rec.LatencyMs, rec.ModelCount, rec.Detail)
	return err
}

// GetKeyHealthForChannel 查询指定渠道下所有 key 的健康验证结果（管理 API 展示用）。
func (s *SQLiteStore) GetKeyHealthForChannel(channelType, channelID string) ([]KeyHealthRecord, error) {
	return s.queryKeyHealth(`
		SELECT channel_type, channel_id, key_mask, check_kind,
		       last_check_at, last_status, consecutive_failures,
		       latency_ms, model_count, detail
		FROM key_health
		WHERE channel_type = ? AND channel_id = ?
		ORDER BY key_mask, check_kind
	`, channelType, channelID)
}

// GetAllKeyHealth 查询全量健康验证结果（调度器重启后恢复上次验证时间用）。
func (s *SQLiteStore) GetAllKeyHealth() ([]KeyHealthRecord, error) {
	return s.queryKeyHealth(`
		SELECT channel_type, channel_id, key_mask, check_kind,
		       last_check_at, last_status, consecutive_failures,
		       latency_ms, model_count, detail
		FROM key_health
		ORDER BY channel_type, channel_id, key_mask, check_kind
	`)
}

// DeleteKeyHealthForChannel 删除指定渠道的全部健康验证结果（渠道删除时清理）。
func (s *SQLiteStore) DeleteKeyHealthForChannel(channelType, channelID string) error {
	_, err := s.db.Exec(
		"DELETE FROM key_health WHERE channel_type = ? AND channel_id = ?",
		channelType, channelID,
	)
	return err
}

// DeleteKeyHealthForKey 删除指定渠道下某个 key 的健康验证结果（key 被移除/拉黑时清理）。
func (s *SQLiteStore) DeleteKeyHealthForKey(channelType, channelID, keyMask string) error {
	_, err := s.db.Exec(
		"DELETE FROM key_health WHERE channel_type = ? AND channel_id = ? AND key_mask = ?",
		channelType, channelID, keyMask,
	)
	return err
}

// queryKeyHealth 执行 key_health 查询并扫描结果集
func (s *SQLiteStore) queryKeyHealth(query string, args ...interface{}) ([]KeyHealthRecord, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []KeyHealthRecord
	for rows.Next() {
		var rec KeyHealthRecord
		var lastCheckAt int64
		if err := rows.Scan(
			&rec.ChannelType, &rec.ChannelID, &rec.KeyMask, &rec.CheckKind,
			&lastCheckAt, &rec.LastStatus, &rec.ConsecutiveFailures,
			&rec.LatencyMs, &rec.ModelCount, &rec.Detail,
		); err != nil {
			return nil, err
		}
		rec.LastCheckAt = time.Unix(lastCheckAt, 0)
		records = append(records, rec)
	}
	return records, rows.Err()
}
