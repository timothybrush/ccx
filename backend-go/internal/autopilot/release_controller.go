package autopilot

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
)

// ── ReleaseController（设计 §5）──
//
// ReleaseController 集中处理灰度发布的状态迁移、分桶、readiness 和安全覆盖。
// SmartRouter 只消费其结果，不自行判断发布状态。

// ReleaseController 管理 Autopilot 的受控发布生命周期。
type ReleaseController struct {
	mu sync.RWMutex

	// 当前发布快照
	snapshot RoutingReleaseSnapshot

	// 安全覆盖：当自动降级触发时，设置进程内强制模式
	safetyOverride       RoutingMode
	safetyOverrideActive bool

	// 上次晋升时间
	lastPromotionAt time.Time

	// 配置引用
	configManager *config.ConfigManager
	traceStore    *TraceStore
	now           func() time.Time
}

// NewReleaseController 创建 ReleaseController。
func NewReleaseController(cm *config.ConfigManager, ts *TraceStore) *ReleaseController {
	rc := &ReleaseController{
		configManager: cm,
		traceStore:    ts,
		now:           time.Now,
	}
	rc.refreshSnapshot()
	return rc
}

// RefreshSnapshot 从配置重新加载发布快照（热重载回调）。
func (rc *ReleaseController) RefreshSnapshot() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.refreshSnapshot()
}

func (rc *ReleaseController) refreshSnapshot() {
	cfg := rc.configManager.GetAutopilotRouting()
	rc.snapshot = RoutingReleaseSnapshot{
		ReleaseID:         cfg.ReleaseID,
		PolicyFingerprint: "", // 由调用方在决策时计算
		TargetMode:        RoutingMode(cfg.RoutingMode),
		EffectiveMode:     RoutingMode(cfg.EffectiveRoutingMode()),
		RolloutPercent:    cfg.RolloutPercent,
		RolloutSeed:       cfg.RolloutSeed,
		ControlPercent:    cfg.ControlPercent,
		CreatedAt:         rc.now(),
	}
}

// ── 状态迁移 ──

// AllowedTransition 检查状态迁移是否合法。
// 管理员只能逐级晋升，不能从 off 直接跳到 auto/active。
// 允许随时降级到较低状态。
func (rc *ReleaseController) AllowedTransition(from, to string) error {
	fromMode := RoutingMode(from)
	toMode := RoutingMode(to)

	// 降级总是允许
	if modeRank(toMode) < modeRank(fromMode) {
		return nil
	}
	// 同级不变
	if fromMode == toMode {
		return nil
	}
	// 只允许升一级
	if modeRank(toMode) != modeRank(fromMode)+1 {
		return fmt.Errorf("不允许从 %s 直接跳到 %s，只能逐级晋升", fromMode, toMode)
	}
	return nil
}

// modeRank 返回模式的顺序号（越大越激进）。
func modeRank(mode RoutingMode) int {
	switch mode {
	case RoutingModeOff:
		return 0
	case RoutingModeShadow:
		return 1
	case RoutingModeAssist:
		return 2
	case RoutingModeAuto:
		return 3
	case RoutingModeActive:
		return 4
	default:
		return 0
	}
}

// ── 分桶 ──

// ComputeCohort 根据 session ID 和请求关联 ID 计算分桶。
// 先应用保护选择与硬约束，再决定 treatment/control/bypass。
func (rc *ReleaseController) ComputeCohort(sessionID, correlationID string, isProtected bool) Cohort {
	if isProtected {
		return CohortBypass
	}

	rc.mu.RLock()
	snapshot := rc.snapshot
	rc.mu.RUnlock()

	// 确定分桶输入：优先 session ID
	id := sessionID
	if id == "" {
		id = correlationID
	}

	bucket := StableBucket(id, snapshot.RolloutSeed)
	return InBucket(bucket, snapshot.RolloutPercent, snapshot.ControlPercent)
}

// ── 快照 ──

// CurrentSnapshot 返回当前发布快照的不可变副本。
func (rc *ReleaseController) CurrentSnapshot() RoutingReleaseSnapshot {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.snapshot
}

// ── 安全覆盖 ──

// SetSafetyOverride 设置进程内安全覆盖模式（自动降级时调用）。
// 覆盖在下次人工晋升前保持有效。
func (rc *ReleaseController) SetSafetyOverride(mode RoutingMode) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.safetyOverride = mode
	rc.safetyOverrideActive = true
	log.Printf("[ReleaseController-Safety] 安全覆盖已激活: %s", mode)
}

// ClearSafetyOverride 清除安全覆盖（人工晋升时调用）。
func (rc *ReleaseController) ClearSafetyOverride() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.safetyOverrideActive = false
	log.Printf("[ReleaseController-Safety] 安全覆盖已清除")
}

// EffectiveMode 返回考虑安全覆盖后的实际生效模式。
func (rc *ReleaseController) EffectiveMode() RoutingMode {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if rc.safetyOverrideActive {
		return rc.safetyOverride
	}
	return rc.snapshot.EffectiveMode
}

// ── 晋升比例阶梯 ──

// AllowedRolloutSteps 是允许的灰度比例递增序列。
var AllowedRolloutSteps = []int{1, 5, 25, 50, 100}

// NextRolloutStep 返回当前比例的下一个允许阶梯。
// 如果已是最大阶梯，返回当前值。
func NextRolloutStep(current int) int {
	for _, step := range AllowedRolloutSteps {
		if step > current {
			return step
		}
	}
	return current
}

// ── 降级 ──

// EvaluateAndApplyRegression 检查 auto/active 是否应降级，并在必要时应用安全覆盖。
// 返回降级报告（nil 表示无需降级）。
func (rc *ReleaseController) EvaluateAndApplyRegression() *AutoRegressionReport {
	if rc.traceStore == nil {
		return nil
	}

	report, err := rc.traceStore.EvaluateAutoRegression(rc.now())
	if err != nil || !report.ShouldRollback {
		return nil
	}

	// 检查降级严重度
	fallback := RoutingModeShadow
	for _, reason := range report.Reasons {
		if reason == "failopen_rate_regression" {
			// fail-open 突增 → 直接 shadow
			fallback = RoutingModeShadow
			break
		}
	}
	// 普通回归 → assist
	if fallback == RoutingModeShadow {
		for _, reason := range report.Reasons {
			if reason == "success_rate_regression" || reason == "fallback_rate_regression" || reason == "latency_regression" {
				fallback = RoutingModeAssist
				break
			}
		}
		// 如果没有任何已知原因，默认 assist
		if fallback == RoutingModeShadow {
			fallback = RoutingModeAssist
		}
	}

	rc.SetSafetyOverride(fallback)
	rc.persistSafetyEvent(fallback, report)

	return &report
}

func (rc *ReleaseController) persistSafetyEvent(mode RoutingMode, report AutoRegressionReport) {
	if rc.traceStore == nil {
		return
	}
	event := AutoSafetyEvent{
		FromMode:  string(rc.snapshot.EffectiveMode),
		ToMode:    string(mode),
		Reasons:   report.Reasons,
		Observed:  report.Observed,
		Baseline:  report.Baseline,
		CreatedAt: rc.now(),
	}
	if err := rc.traceStore.RecordAutoSafetyEvent(event); err != nil {
		log.Printf("[ReleaseController-Safety] 安全事件落盘失败: %v", err)
	}
}

// ── 发布 ID 生成 ──

// GenerateReleaseID 生成不可预测的发布批次 ID。
func GenerateReleaseID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("rel_%d", time.Now().UnixNano())
	}
	return "rel_" + hex.EncodeToString(buf[:4])
}

// GenerateRolloutSeed 生成不可预测的分桶种子。
func GenerateRolloutSeed() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("seed_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

// ── RoutingReleaseSnapshot 辅助方法 ──

// SnapshotRolloutSeed 返回分桶种子（安全访问）。
func SnapshotRolloutSeed(s RoutingReleaseSnapshot) string {
	return s.RolloutSeed
}

// SnapshotControlPercent 返回对照组百分比（安全访问）。
func SnapshotControlPercent(s RoutingReleaseSnapshot) int {
	return s.ControlPercent
}
