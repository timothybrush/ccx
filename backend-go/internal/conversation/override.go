package conversation

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type ChannelEntry struct {
	ChannelIndex int    `json:"channelIndex"`
	ChannelName  string `json:"channelName"`
}

type ChannelSequenceOverride struct {
	ConversationID   string         `json:"conversationId"`
	Kind             string         `json:"kind"`
	UserID           string         `json:"userID"`
	Sequence         []ChannelEntry `json:"sequence"`
	SubagentSequence []ChannelEntry `json:"subagentSequence,omitempty"` // subagent 角色专用序列（为空时 fallback 到 Sequence）
	SetAt            time.Time      `json:"setAt"`
	ExpiresAt        time.Time      `json:"expiresAt"`
	IsPerpetual      bool           `json:"isPerpetual,omitempty"` // 永不过期（手动恢复前不会自动过期）
	ttlDuration      time.Duration  `json:"-"`                     // 原始有效期（续期时使用，不序列化）
}

// clone 返回 override 的深拷贝（用于返回快照，避免并发数据竞争）。
func (o *ChannelSequenceOverride) clone() *ChannelSequenceOverride {
	c := *o
	if o.Sequence != nil {
		c.Sequence = make([]ChannelEntry, len(o.Sequence))
		copy(c.Sequence, o.Sequence)
	}
	return &c
}

type OverrideManager struct {
	mu        sync.RWMutex
	overrides map[string]*ChannelSequenceOverride // conversationID → override
	userIndex map[string]string                   // kind:userID → conversationID
	ttl       time.Duration
	stopCh    chan struct{}
}

func NewOverrideManager(ttl time.Duration) *OverrideManager {
	om := &OverrideManager{
		overrides: make(map[string]*ChannelSequenceOverride),
		userIndex: make(map[string]string),
		ttl:       ttl,
		stopCh:    make(chan struct{}),
	}
	go om.cleanupLoop()
	return om
}

// SetOverride 设置会话级渠道序列覆盖。
// overrideDuration: 0=使用系统默认 TTL；<0（如 -1）=永不过期；>0=自定义时长。
func (om *OverrideManager) SetOverride(conversationID, kind, userID string, sequence []ChannelEntry, overrideDuration time.Duration) error {
	if len(sequence) == 0 {
		return fmt.Errorf("sequence cannot be empty")
	}
	if conversationID == "" || kind == "" || userID == "" {
		return fmt.Errorf("conversationID, kind, and userID are required")
	}

	om.mu.Lock()
	defer om.mu.Unlock()

	now := time.Now()
	override := &ChannelSequenceOverride{
		ConversationID: conversationID,
		Kind:           kind,
		UserID:         userID,
		Sequence:       sequence,
		SetAt:          now,
	}

	switch {
	case overrideDuration < 0:
		override.IsPerpetual = true
	case overrideDuration > 0:
		override.ExpiresAt = now.Add(overrideDuration)
		override.ttlDuration = overrideDuration
	default:
		override.ExpiresAt = now.Add(om.ttl)
		override.ttlDuration = om.ttl
	}

	om.overrides[conversationID] = override
	compositeKey := kind + ":" + userID
	om.userIndex[compositeKey] = conversationID

	if override.IsPerpetual {
		log.Printf("[OverrideManager-Set] 设置覆盖: conv=%s, kind=%s, 序列长度=%d, 永不过期",
			conversationID, kind, len(sequence))
	} else {
		log.Printf("[OverrideManager-Set] 设置覆盖: conv=%s, kind=%s, 序列长度=%d, 过期=%s",
			conversationID, kind, len(sequence), override.ExpiresAt.Format("15:04:05"))
	}

	return nil
}

func (om *OverrideManager) GetOverride(conversationID string) (*ChannelSequenceOverride, bool) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	override, ok := om.overrides[conversationID]
	if !ok {
		return nil, false
	}
	if !override.IsPerpetual && time.Now().After(override.ExpiresAt) {
		return nil, false
	}
	return override.clone(), true
}

func (om *OverrideManager) GetOverrideForUser(kind, userID string) ([]ChannelEntry, bool) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	compositeKey := kind + ":" + userID
	convID, ok := om.userIndex[compositeKey]
	if !ok {
		return nil, false
	}

	override, ok := om.overrides[convID]
	if !ok {
		return nil, false
	}
	if !override.IsPerpetual && time.Now().After(override.ExpiresAt) {
		return nil, false
	}
	return override.Sequence, true
}

// GetOverrideForUserWithRole 角色感知的 override 查找。
// agentRole == "subagent" 且存在 SubagentSequence 时返回 subagent 序列；否则 fallback 到主序列。
func (om *OverrideManager) GetOverrideForUserWithRole(kind, userID, agentRole string) ([]ChannelEntry, bool) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	compositeKey := kind + ":" + userID
	convID, ok := om.userIndex[compositeKey]
	if !ok {
		return nil, false
	}

	override, ok := om.overrides[convID]
	if !ok {
		return nil, false
	}
	if !override.IsPerpetual && time.Now().After(override.ExpiresAt) {
		return nil, false
	}

	if agentRole == "subagent" && len(override.SubagentSequence) > 0 {
		return override.SubagentSequence, true
	}
	return override.Sequence, true
}

// SetSubagentOverride 在已有 override 上设置 subagent 专用序列（复用 TTL/过期时间）。
// 若该会话尚无主 override，则一并创建（subagent 序列同时作为主序列，保证可用）。
func (om *OverrideManager) SetSubagentOverride(conversationID, kind, userID string, subagentSequence []ChannelEntry) error {
	if len(subagentSequence) == 0 {
		return fmt.Errorf("subagent sequence cannot be empty")
	}
	if conversationID == "" || kind == "" || userID == "" {
		return fmt.Errorf("conversationID, kind, and userID are required")
	}

	om.mu.Lock()
	defer om.mu.Unlock()

	now := time.Now()
	override, exists := om.overrides[conversationID]
	if !exists {
		override = &ChannelSequenceOverride{
			ConversationID: conversationID,
			Kind:           kind,
			UserID:         userID,
			Sequence:       subagentSequence, // 兜底主序列，避免 subagent 未命中时空覆盖
			SetAt:          now,
			ExpiresAt:      now.Add(om.ttl),
			ttlDuration:    om.ttl,
		}
		om.overrides[conversationID] = override
		compositeKey := kind + ":" + userID
		om.userIndex[compositeKey] = conversationID
	}
	override.SubagentSequence = subagentSequence
	override.SetAt = now

	log.Printf("[OverrideManager-SetSubagent] 设置 subagent 覆盖: conv=%s, kind=%s, 序列长度=%d",
		conversationID, kind, len(subagentSequence))
	return nil
}

func (om *OverrideManager) RemoveOverride(conversationID string) bool {
	om.mu.Lock()
	defer om.mu.Unlock()

	override, ok := om.overrides[conversationID]
	if !ok {
		return false
	}

	compositeKey := override.Kind + ":" + override.UserID
	delete(om.userIndex, compositeKey)
	delete(om.overrides, conversationID)

	log.Printf("[OverrideManager-Remove] 移除覆盖: conv=%s", conversationID)
	return true
}

func (om *OverrideManager) RemoveOverrideByUser(kind, userID string) bool {
	om.mu.Lock()
	defer om.mu.Unlock()

	compositeKey := kind + ":" + userID
	convID, ok := om.userIndex[compositeKey]
	if !ok {
		return false
	}

	delete(om.userIndex, compositeKey)
	delete(om.overrides, convID)

	log.Printf("[OverrideManager-Remove] 渠道熔断自动清除覆盖: conv=%s (user: %s)", convID, userID)
	return true
}

func (om *OverrideManager) GetAllOverrides() map[string]*ChannelSequenceOverride {
	om.mu.RLock()
	defer om.mu.RUnlock()

	now := time.Now()
	result := make(map[string]*ChannelSequenceOverride, len(om.overrides))
	for id, override := range om.overrides {
		if override.IsPerpetual || now.Before(override.ExpiresAt) {
			result[id] = override.clone()
		}
	}
	return result
}

// RefreshTTL 续期指定会话的 override TTL（永不过期的 override 不受影响）。
// 使用该 override 原始设置的有效期续期，而非系统默认值。
func (om *OverrideManager) RefreshTTL(conversationID string) bool {
	om.mu.Lock()
	defer om.mu.Unlock()

	override, ok := om.overrides[conversationID]
	if !ok || override.IsPerpetual {
		return false
	}
	override.ExpiresAt = time.Now().Add(override.ttlDuration)
	return true
}

// RefreshOverrideForUser 按 kind:userID 续期 override TTL（供调度器 idle 续期）。
// 使用该 override 原始设置的有效期续期，而非系统默认值。
func (om *OverrideManager) RefreshOverrideForUser(kind, userID string) bool {
	om.mu.Lock()
	defer om.mu.Unlock()

	compositeKey := kind + ":" + userID
	convID, ok := om.userIndex[compositeKey]
	if !ok {
		return false
	}
	override, ok := om.overrides[convID]
	if !ok || override.IsPerpetual {
		return false
	}
	override.ExpiresAt = time.Now().Add(override.ttlDuration)
	return true
}

// SetDefaultTTL 动态更新系统默认 TTL。
func (om *OverrideManager) SetDefaultTTL(ttl time.Duration) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.ttl = ttl
}

// PurgeOrphans 清理不属于任何活跃会话的孤儿 override。
// 当 ConversationTracker 过期清理会话后，调用此方法同步移除对应的 override。
func (om *OverrideManager) PurgeOrphans(activeConversationIDs map[string]bool) {
	om.mu.Lock()
	defer om.mu.Unlock()

	var removed int
	for id, override := range om.overrides {
		if !activeConversationIDs[id] {
			compositeKey := override.Kind + ":" + override.UserID
			delete(om.userIndex, compositeKey)
			delete(om.overrides, id)
			removed++
		}
	}
	if removed > 0 {
		log.Printf("[OverrideManager-PurgeOrphans] 清理 %d 个孤儿覆盖, 剩余 %d", removed, len(om.overrides))
	}
}

func (om *OverrideManager) Stop() {
	close(om.stopCh)
}

func (om *OverrideManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-om.stopCh:
			return
		case <-ticker.C:
			om.cleanup()
		}
	}
}

func (om *OverrideManager) cleanup() {
	om.mu.Lock()
	defer om.mu.Unlock()

	now := time.Now()
	var removed int

	for id, override := range om.overrides {
		if !override.IsPerpetual && now.After(override.ExpiresAt) {
			compositeKey := override.Kind + ":" + override.UserID
			delete(om.userIndex, compositeKey)
			delete(om.overrides, id)
			removed++
		}
	}

	if removed > 0 {
		log.Printf("[OverrideManager-Cleanup] 清理 %d 个过期覆盖, 剩余 %d", removed, len(om.overrides))
	}
}
