package thinkingcache

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
	"github.com/BenedictKing/ccx/internal/utils"

	_ "modernc.org/sqlite"
)

const (
	defaultTTL        = 48 * time.Hour
	defaultMaxEntries = 512
)

type Config struct {
	DBPath string
	TTL    time.Duration
}

type cacheEntry struct {
	Thinking  string
	ExpiresAt time.Time
	UpdatedAt time.Time
}

type cacheStore struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	db      *sql.DB
	dbPath  string
	ttl     time.Duration
}

var globalStore = &cacheStore{
	entries: make(map[string]cacheEntry),
	ttl:     defaultTTL,
}

// Configure enables SQLite persistence for the process-local thinking cache.
func Configure(cfg Config) error {
	ttl := normalizeTTL(cfg.TTL)
	dbPath := strings.TrimSpace(cfg.DBPath)
	if dbPath == "" {
		globalStore.mu.Lock()
		defer globalStore.mu.Unlock()
		globalStore.ttl = ttl
		globalStore.evictExpiredLocked(time.Now())
		return nil
	}

	globalStore.mu.Lock()
	if globalStore.db != nil && globalStore.dbPath == dbPath {
		globalStore.ttl = ttl
		if err := globalStore.cleanupExpiredLocked(time.Now()); err != nil {
			globalStore.mu.Unlock()
			return err
		}
		globalStore.evictExpiredLocked(time.Now())
		globalStore.mu.Unlock()
		return nil
	}
	globalStore.mu.Unlock()

	db, err := openSQLite(dbPath)
	if err != nil {
		return err
	}
	if err := initSQLiteSchema(db); err != nil {
		db.Close()
		return err
	}

	globalStore.mu.Lock()
	oldDB := globalStore.db
	oldDBPath := globalStore.dbPath
	oldEntries := globalStore.entries
	globalStore.db = db
	globalStore.dbPath = dbPath
	globalStore.ttl = ttl
	globalStore.entries = make(map[string]cacheEntry)
	if err := globalStore.cleanupExpiredLocked(time.Now()); err != nil {
		globalStore.db = oldDB
		globalStore.dbPath = oldDBPath
		globalStore.entries = oldEntries
		globalStore.mu.Unlock()
		db.Close()
		return err
	}
	if err := globalStore.loadValidEntriesLocked(time.Now()); err != nil {
		globalStore.db = oldDB
		globalStore.dbPath = oldDBPath
		globalStore.entries = oldEntries
		globalStore.mu.Unlock()
		db.Close()
		return err
	}
	globalStore.mu.Unlock()

	if oldDB != nil {
		_ = oldDB.Close()
	}
	log.Printf("[ThinkingCache-Init] Claude thinking 缓存已初始化: %s (TTL %s)", dbPath, ttl)
	return nil
}

// Close closes the SQLite persistence handle.
func Close() error {
	globalStore.mu.Lock()
	defer globalStore.mu.Unlock()
	if globalStore.db == nil {
		return nil
	}
	err := globalStore.db.Close()
	globalStore.db = nil
	globalStore.dbPath = ""
	return err
}

// ResetForTest clears the process-local cache.
func ResetForTest() {
	globalStore.mu.Lock()
	defer globalStore.mu.Unlock()
	if globalStore.db != nil {
		_ = globalStore.db.Close()
	}
	globalStore.entries = make(map[string]cacheEntry)
	globalStore.db = nil
	globalStore.dbPath = ""
	globalStore.ttl = defaultTTL
}

// ShouldTrackClaudeThinking returns true for strict DeepSeek Claude-compatible channels.
func ShouldTrackClaudeThinking(upstream *config.UpstreamConfig, bodyBytes []byte) bool {
	return isDeepSeekClaudeTarget(upstream, bodyBytes)
}

// InjectCachedClaudeThinking prepends cached thinking blocks to assistant history
// only when the request is in Claude thinking mode and the assistant content
// fingerprint has a previous exact cache hit.
func InjectCachedClaudeThinking(bodyBytes []byte, sessionID string, upstream *config.UpstreamConfig) ([]byte, int) {
	if strings.TrimSpace(sessionID) == "" || !isDeepSeekClaudeTarget(upstream, bodyBytes) {
		return bodyBytes, 0
	}

	data, ok := decodeObject(bodyBytes)
	if !ok || !claudeThinkingRequested(data) {
		return bodyBytes, 0
	}

	messages, ok := data["messages"].([]interface{})
	if !ok {
		return bodyBytes, 0
	}

	injected := 0
	for _, rawMsg := range messages {
		msg, ok := rawMsg.(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := msg["role"].(string); role != "assistant" {
			continue
		}

		content, exists := msg["content"]
		if !exists || assistantContentHasThinking(content) {
			continue
		}

		thinking, ok := LookupClaudeThinkingForContent(sessionID, content)
		if !ok {
			continue
		}

		switch typed := content.(type) {
		case []interface{}:
			next := make([]interface{}, 0, len(typed)+1)
			next = append(next, thinkingBlock(thinking))
			next = append(next, typed...)
			msg["content"] = next
		case string:
			msg["content"] = []interface{}{
				thinkingBlock(thinking),
				map[string]interface{}{"type": "text", "text": typed},
			}
		default:
			continue
		}
		injected++
	}

	if injected == 0 {
		return bodyBytes, 0
	}

	data["messages"] = messages
	nextBytes, err := utils.MarshalJSONNoEscape(data)
	if err != nil {
		return bodyBytes, 0
	}
	return nextBytes, injected
}

func thinkingBlock(thinking string) map[string]interface{} {
	return map[string]interface{}{
		"type":     "thinking",
		"thinking": thinking,
	}
}

func claudeThinkingRequested(data map[string]interface{}) bool {
	thinking, ok := data["thinking"].(map[string]interface{})
	if !ok {
		return false
	}
	thinkingType, _ := thinking["type"].(string)
	switch strings.ToLower(strings.TrimSpace(thinkingType)) {
	case "adaptive", "enabled":
		return true
	default:
		return false
	}
}

func isDeepSeekClaudeTarget(upstream *config.UpstreamConfig, bodyBytes []byte) bool {
	if upstream == nil || upstream.ServiceType != "claude" {
		return false
	}

	parts := []string{upstream.BaseURL, upstream.GetEffectiveBaseURL(), upstream.Name, upstream.Website}
	parts = append(parts, upstream.BaseURLs...)
	if strings.Contains(strings.ToLower(strings.Join(parts, " ")), "deepseek") {
		return true
	}

	data, ok := decodeObject(bodyBytes)
	if !ok {
		return false
	}
	model, _ := data["model"].(string)
	return strings.Contains(strings.ToLower(model), "deepseek")
}

func decodeObject(bodyBytes []byte) (map[string]interface{}, bool) {
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.UseNumber()

	var data map[string]interface{}
	if err := decoder.Decode(&data); err != nil {
		return nil, false
	}
	return data, true
}

// StoreClaudeThinkingForContent stores thinking by session and assistant content fingerprint.
func StoreClaudeThinkingForContent(sessionID string, content interface{}, thinking string) bool {
	if strings.TrimSpace(sessionID) == "" || !isRealThinking(thinking) {
		return false
	}

	fingerprint := FingerprintClaudeAssistantContent(content)
	if fingerprint == "" {
		return false
	}

	globalStore.store(sessionID, fingerprint, thinking)
	return true
}

// LookupClaudeThinkingForContent returns cached thinking for the assistant content fingerprint.
func LookupClaudeThinkingForContent(sessionID string, content interface{}) (string, bool) {
	if strings.TrimSpace(sessionID) == "" {
		return "", false
	}
	fingerprint := FingerprintClaudeAssistantContent(content)
	if fingerprint == "" {
		return "", false
	}
	return globalStore.lookup(sessionID, fingerprint)
}

func (s *cacheStore) store(sessionID, fingerprint, thinking string) {
	now := time.Now()
	sessionHash := hashSessionID(sessionID)
	key := cacheKeyFromParts(sessionHash, fingerprint)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.evictExpiredLocked(now)
	if _, exists := s.entries[key]; !exists {
		for len(s.entries) >= defaultMaxEntries {
			s.evictOldestLocked()
		}
	}

	ttl := s.ttl
	if ttl <= 0 {
		ttl = defaultTTL
	}
	s.entries[key] = cacheEntry{
		Thinking:  thinking,
		ExpiresAt: now.Add(ttl),
		UpdatedAt: now,
	}
	if s.db != nil {
		if err := s.upsertLocked(key, sessionHash, fingerprint, thinking, now, now.Add(ttl)); err != nil {
			log.Printf("[ThinkingCache] 警告: 写入 SQLite 缓存失败: %v", err)
		}
	}
}

func (s *cacheStore) lookup(sessionID, fingerprint string) (string, bool) {
	now := time.Now()
	key := cacheKey(sessionID, fingerprint)

	s.mu.Lock()
	entry, ok := s.entries[key]
	if ok && !s.isExpiredLocked(entry, now) {
		thinking := entry.Thinking
		s.mu.Unlock()
		return thinking, true
	}
	if ok {
		delete(s.entries, key)
		s.mu.Unlock()
		return "", false
	}

	if s.db == nil {
		s.mu.Unlock()
		return "", false
	}
	thinking, updatedAt, ok := s.lookupSQLiteLocked(key, now)
	if !ok {
		s.mu.Unlock()
		return "", false
	}
	ttl := s.ttl
	if ttl <= 0 {
		ttl = defaultTTL
	}
	s.entries[key] = cacheEntry{
		Thinking:  thinking,
		UpdatedAt: updatedAt,
		ExpiresAt: updatedAt.Add(ttl),
	}
	s.mu.Unlock()
	return thinking, true
}

func (s *cacheStore) evictExpiredLocked(now time.Time) {
	for key, entry := range s.entries {
		if s.isExpiredLocked(entry, now) {
			delete(s.entries, key)
		}
	}
}

func (s *cacheStore) isExpiredLocked(entry cacheEntry, now time.Time) bool {
	ttl := s.ttl
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return !now.Before(entry.UpdatedAt.Add(ttl))
}

func (s *cacheStore) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range s.entries {
		if oldestKey == "" || entry.UpdatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.UpdatedAt
		}
	}
	if oldestKey != "" {
		delete(s.entries, oldestKey)
	}
}

func cacheKey(sessionID, fingerprint string) string {
	return cacheKeyFromParts(hashSessionID(sessionID), fingerprint)
}

func hashSessionID(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:])
}

func cacheKeyFromParts(sessionHash, fingerprint string) string {
	return sessionHash + ":" + fingerprint
}

func isRealThinking(thinking string) bool {
	return strings.TrimSpace(thinking) != ""
}

func normalizeTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultTTL
	}
	minTTL := time.Hour
	maxTTL := 30 * 24 * time.Hour
	if ttl < minTTL {
		return minTTL
	}
	if ttl > maxTTL {
		return maxTTL
	}
	return ttl
}

func openSQLite(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, fmt.Errorf("创建 thinking cache 数据库目录失败: %w", err)
	}

	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 thinking cache 数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	return db, nil
}

func initSQLiteSchema(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS claude_thinking_cache (
			cache_key TEXT PRIMARY KEY,
			session_hash TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			thinking TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_claude_thinking_cache_session
			ON claude_thinking_cache(session_hash);

		CREATE INDEX IF NOT EXISTS idx_claude_thinking_cache_expires_at
			ON claude_thinking_cache(expires_at);
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("初始化 thinking cache schema 失败: %w", err)
	}
	return nil
}

func (s *cacheStore) upsertLocked(key, sessionHash, fingerprint, thinking string, updatedAt, expiresAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO claude_thinking_cache
			(cache_key, session_hash, fingerprint, thinking, expires_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET
			thinking = excluded.thinking,
			expires_at = excluded.expires_at,
			updated_at = excluded.updated_at
	`, key, sessionHash, fingerprint, thinking, expiresAt.Unix(), updatedAt.Unix())
	return err
}

func (s *cacheStore) lookupSQLiteLocked(key string, now time.Time) (string, time.Time, bool) {
	ttl := s.ttl
	if ttl <= 0 {
		ttl = defaultTTL
	}
	cutoff := now.Add(-ttl).Unix()

	var thinking string
	var updatedAtUnix int64
	err := s.db.QueryRow(`
		SELECT thinking, updated_at
		FROM claude_thinking_cache
		WHERE cache_key = ? AND updated_at >= ?
	`, key, cutoff).Scan(&thinking, &updatedAtUnix)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", time.Time{}, false
		}
		log.Printf("[ThinkingCache] 警告: 查询 SQLite 缓存失败: %v", err)
		return "", time.Time{}, false
	}
	return thinking, time.Unix(updatedAtUnix, 0), true
}

func (s *cacheStore) cleanupExpiredLocked(now time.Time) error {
	if s.db == nil {
		return nil
	}
	ttl := s.ttl
	if ttl <= 0 {
		ttl = defaultTTL
	}
	cutoff := now.Add(-ttl).Unix()
	_, err := s.db.Exec("DELETE FROM claude_thinking_cache WHERE updated_at < ?", cutoff)
	if err != nil {
		return fmt.Errorf("清理过期 thinking cache 失败: %w", err)
	}
	return nil
}

func (s *cacheStore) loadValidEntriesLocked(now time.Time) error {
	if s.db == nil {
		return nil
	}
	ttl := s.ttl
	if ttl <= 0 {
		ttl = defaultTTL
	}
	cutoff := now.Add(-ttl).Unix()
	rows, err := s.db.Query(`
		SELECT cache_key, thinking, updated_at
		FROM claude_thinking_cache
		WHERE updated_at >= ?
		ORDER BY updated_at DESC
		LIMIT ?
	`, cutoff, defaultMaxEntries)
	if err != nil {
		return fmt.Errorf("加载 thinking cache 失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var thinking string
		var updatedAtUnix int64
		if err := rows.Scan(&key, &thinking, &updatedAtUnix); err != nil {
			return err
		}
		updatedAt := time.Unix(updatedAtUnix, 0)
		s.entries[key] = cacheEntry{
			Thinking:  thinking,
			UpdatedAt: updatedAt,
			ExpiresAt: updatedAt.Add(ttl),
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

// FingerprintClaudeAssistantContent fingerprints assistant content after removing thinking blocks.
func FingerprintClaudeAssistantContent(content interface{}) string {
	normalized := normalizeAssistantContent(content)
	if len(normalized) == 0 {
		return ""
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func normalizeAssistantContent(content interface{}) []interface{} {
	switch typed := content.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return []interface{}{map[string]interface{}{"type": "text", "text": typed}}
	case []interface{}:
		normalized := make([]interface{}, 0, len(typed))
		for _, rawBlock := range typed {
			block, ok := normalizeAssistantBlock(rawBlock)
			if ok {
				normalized = append(normalized, block)
			}
		}
		return normalized
	default:
		return nil
	}
}

func normalizeAssistantBlock(rawBlock interface{}) (interface{}, bool) {
	block, ok := rawBlock.(map[string]interface{})
	if !ok {
		return nil, false
	}

	blockType, _ := block["type"].(string)
	blockType = strings.TrimSpace(blockType)
	switch blockType {
	case "", "thinking", "redacted_thinking":
		return nil, false
	case "text":
		text, _ := block["text"].(string)
		if text == "" {
			return nil, false
		}
		return map[string]interface{}{"type": "text", "text": text}, true
	case "tool_use", "server_tool_use":
		normalized := map[string]interface{}{"type": blockType}
		if id, _ := block["id"].(string); id != "" {
			normalized["id"] = id
		}
		if name, _ := block["name"].(string); name != "" {
			normalized["name"] = name
		}
		if input, exists := block["input"]; exists {
			normalized["input"] = normalizeJSONValue(input)
		}
		return normalized, true
	default:
		normalized := make(map[string]interface{}, len(block))
		keys := make([]string, 0, len(block))
		for key := range block {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if shouldSkipFingerprintField(key) {
				continue
			}
			normalized[key] = normalizeJSONValue(block[key])
		}
		if len(normalized) == 0 {
			return nil, false
		}
		return normalized, true
	}
}

func shouldSkipFingerprintField(key string) bool {
	switch key {
	case "thinking", "signature", "cache_control":
		return true
	default:
		return false
	}
}

func normalizeJSONValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			normalized[key] = normalizeJSONValue(value)
		}
		return normalized
	case []interface{}:
		normalized := make([]interface{}, 0, len(typed))
		for _, value := range typed {
			normalized = append(normalized, normalizeJSONValue(value))
		}
		return normalized
	default:
		return typed
	}
}

func assistantContentHasThinking(content interface{}) bool {
	blocks, ok := content.([]interface{})
	if !ok {
		return false
	}
	for _, rawBlock := range blocks {
		block, ok := rawBlock.(map[string]interface{})
		if !ok {
			continue
		}
		blockType, _ := block["type"].(string)
		if blockType != "thinking" && blockType != "redacted_thinking" {
			continue
		}
		thinking, _ := block["thinking"].(string)
		if isRealThinking(thinking) {
			return true
		}
	}
	return false
}
