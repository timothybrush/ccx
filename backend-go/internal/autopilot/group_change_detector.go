package autopilot

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ── 分组变更检测（设计 §3.9）──
//
// Phase 1 shadow 模式：仅记录快照与变更日志，不触发自动重探测。

// KeyGroupSnapshot 模型列表快照，用于检测 key 分组漂移。
type KeyGroupSnapshot struct {
	EndpointUID string    `json:"endpointUid"`
	ModelList   []string  `json:"modelList"`
	ListHash    string    `json:"listHash"`
	SnapshotAt  time.Time `json:"snapshotAt"`
}

// GroupChangeResult 分组变更检测结果。
type GroupChangeResult struct {
	OldHash       string    `json:"oldHash"`
	NewHash       string    `json:"newHash"`
	AddedModels   []string  `json:"addedModels"`
	RemovedModels []string  `json:"removedModels"`
	ChangedAt     time.Time `json:"changedAt"`
}

// GroupChangeDetector 检测 endpoint 的模型列表变化（分组漂移）。
//
// Phase 1：纯 shadow，结果仅记录日志，不影响调度。
type GroupChangeDetector struct {
	store     *ProfileStore
	mu        sync.RWMutex
	snapshots map[string]*KeyGroupSnapshot // key = endpointUID
}

// NewGroupChangeDetector 创建 GroupChangeDetector。
func NewGroupChangeDetector(store *ProfileStore) *GroupChangeDetector {
	return &GroupChangeDetector{
		store:     store,
		snapshots: make(map[string]*KeyGroupSnapshot),
	}
}

// TakeSnapshot 为 endpoint 生成模型列表快照并存储。
func (d *GroupChangeDetector) TakeSnapshot(endpointUID string, models []string) *KeyGroupSnapshot {
	sorted := sortedCopy(models)
	snap := &KeyGroupSnapshot{
		EndpointUID: endpointUID,
		ModelList:   sorted,
		ListHash:    hashModelList(sorted),
		SnapshotAt:  time.Now(),
	}

	d.mu.Lock()
	d.snapshots[endpointUID] = snap
	d.mu.Unlock()

	return snap
}

// Compare 对比新旧两个快照，输出变更列表。
// 两个快照都为 nil 时视为无变化；仅 new 为 nil 时也视为无变化。
func (d *GroupChangeDetector) Compare(old, new *KeyGroupSnapshot) GroupChangeResult {
	if old == nil || new == nil {
		return GroupChangeResult{}
	}
	if old.ListHash == new.ListHash {
		return GroupChangeResult{}
	}

	added, removed := diffModelLists(old.ModelList, new.ModelList)
	return GroupChangeResult{
		OldHash:       old.ListHash,
		NewHash:       new.ListHash,
		AddedModels:   added,
		RemovedModels: removed,
		ChangedAt:     time.Now(),
	}
}

// CheckGroupChange 对比本次探测的模型列表与上次快照，返回是否发生变化。
//
// 首次调用会记录快照并返回 false。后续调用：
//   - 模型列表无变化 → 返回 false，不更新快照
//   - 模型列表有变化 → 返回 true + 变更详情，同时更新快照
//
// Phase 1：仅记录日志，不触发自动重探测。
func (d *GroupChangeDetector) CheckGroupChange(
	channelUID string,
	channelKind string,
	metricsKey string,
	currentModels []string,
) (bool, GroupChangeResult) {
	endpointUID := buildGroupSnapshotKey(channelUID, metricsKey)

	d.mu.RLock()
	lastSnapshot := d.snapshots[endpointUID]
	d.mu.RUnlock()

	// 首次：记录快照
	if lastSnapshot == nil {
		d.TakeSnapshot(endpointUID, currentModels)
		return false, GroupChangeResult{}
	}

	currentHash := hashModelList(currentModels)
	if currentHash == lastSnapshot.ListHash {
		return false, GroupChangeResult{}
	}

	// 分组变更：计算差异
	added, removed := diffModelLists(lastSnapshot.ModelList, currentModels)
	now := time.Now()

	// 更新快照
	d.TakeSnapshot(endpointUID, currentModels)

	return true, GroupChangeResult{
		OldHash:       lastSnapshot.ListHash,
		NewHash:       currentHash,
		AddedModels:   added,
		RemovedModels: removed,
		ChangedAt:     now,
	}
}

// GetSnapshot 获取 endpoint 的当前快照，不存在返回 nil。
func (d *GroupChangeDetector) GetSnapshot(endpointUID string) *KeyGroupSnapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	snap := d.snapshots[endpointUID]
	if snap == nil {
		return nil
	}
	cp := *snap
	cp.ModelList = make([]string, len(snap.ModelList))
	copy(cp.ModelList, snap.ModelList)
	return &cp
}

// RemoveSnapshot 删除 endpoint 的快照。
func (d *GroupChangeDetector) RemoveSnapshot(endpointUID string) {
	d.mu.Lock()
	delete(d.snapshots, endpointUID)
	d.mu.Unlock()
}

// ── 内部辅助 ──

// hashModelList 计算模型列表的 SHA-256 哈希（排序后计算，保证顺序无关）。
func hashModelList(models []string) string {
	sorted := sortedCopy(models)
	h := sha256.New()
	for _, m := range sorted {
		_, _ = fmt.Fprintf(h, "%s\n", m)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// sortedCopy 返回模型列表的排序副本，不修改原始切片。
func sortedCopy(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	cp := make([]string, len(models))
	copy(cp, models)
	sort.Strings(cp)
	return cp
}

// diffModelLists 对比两个已排序模型列表，返回新增和移除的模型。
func diffModelLists(oldModels, newModels []string) (added, removed []string) {
	oldSet := make(map[string]struct{}, len(oldModels))
	for _, m := range oldModels {
		oldSet[m] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newModels))
	for _, m := range newModels {
		newSet[m] = struct{}{}
	}

	for _, m := range newModels {
		if _, ok := oldSet[m]; !ok {
			added = append(added, m)
		}
	}
	for _, m := range oldModels {
		if _, ok := newSet[m]; !ok {
			removed = append(removed, m)
		}
	}
	return
}

// buildGroupSnapshotKey 构建快照的内存索引键。
func buildGroupSnapshotKey(channelUID, metricsKey string) string {
	return channelUID + "::" + metricsKey
}
