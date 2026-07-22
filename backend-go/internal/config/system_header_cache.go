package config

import (
	"fmt"
	"sync"
	"time"
)

// SystemHeaderFilterEntry 记录单个渠道-key-模型的最优过滤层级
type SystemHeaderFilterEntry struct {
	Level        int       `json:"level"`         // 0-3
	DetectedAt   time.Time `json:"detected_at"`   // 探测时间
	SuccessCount int       `json:"success_count"`  // 成功次数
	FailureCount int       `json:"failure_count"`  // 失败次数
	LastError    string    `json:"last_error"`     // 最近错误
}

// SystemHeaderFilterCache 管理 system header 过滤层级的缓存
type SystemHeaderFilterCache struct {
	cache map[string]*SystemHeaderFilterEntry
	mu    sync.RWMutex
}

// NewSystemHeaderFilterCache 创建新的缓存实例
func NewSystemHeaderFilterCache() *SystemHeaderFilterCache {
	return &SystemHeaderFilterCache{
		cache: make(map[string]*SystemHeaderFilterEntry),
	}
}

// GenerateCacheKey 生成缓存键
func GenerateCacheKey(channelUID, keyHash, model string) string {
	return fmt.Sprintf("%s:%s:%s", channelUID, keyHash, model)
}

// Get 获取缓存条目
func (c *SystemHeaderFilterCache) Get(channelUID, keyHash, model string) *SystemHeaderFilterEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := GenerateCacheKey(channelUID, keyHash, model)
	entry, ok := c.cache[key]
	if !ok {
		return nil
	}

	// 检查是否过期（24小时）
	if time.Since(entry.DetectedAt) > 24*time.Hour {
		return nil
	}

	return entry
}

// Set 设置缓存条目
func (c *SystemHeaderFilterCache) Set(channelUID, keyHash, model string, level int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := GenerateCacheKey(channelUID, keyHash, model)
	entry, ok := c.cache[key]
	if !ok {
		entry = &SystemHeaderFilterEntry{}
		c.cache[key] = entry
	}

	entry.Level = level
	entry.DetectedAt = time.Now()
	entry.SuccessCount++
	entry.LastError = ""
}

// RecordFailure 记录失败
func (c *SystemHeaderFilterCache) RecordFailure(channelUID, keyHash, model string, err string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := GenerateCacheKey(channelUID, keyHash, model)
	entry, ok := c.cache[key]
	if !ok {
		entry = &SystemHeaderFilterEntry{}
		c.cache[key] = entry
	}

	entry.FailureCount++
	entry.LastError = err
}

// Clear 清除所有缓存
func (c *SystemHeaderFilterCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*SystemHeaderFilterEntry)
}

// Size 返回缓存大小
func (c *SystemHeaderFilterCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache)
}
