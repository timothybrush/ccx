package autopilot

import (
	"log"
	"math"
	"sync"
	"time"
)

// FastDecay — 白嫖池快速衰减 shadow 评分（设计 §6.5）
//
// Phase 1 shadow/read-only 阶段：此模块不接入调度链路，
// 仅由健康中心用于展示 endpoint 的实时衰减分数。
// 调度器不会读取此处的分数，不会影响真实请求路由。

// ── 衰减参数 ──

const (
	// defaultDecayBase 是普通失败的衰减基数。
	// 连续失败 N 次后 DecayFactor = pow(0.85, N)。
	defaultDecayBase = 0.85

	// streamBreakDecayBase 是断流（stream break）的衰减基数，比普通失败更激进。
	streamBreakDecayBase = 0.70

	// successRecoveryStep 是每次成功后 DecayFactor 的回升步长。
	successRecoveryStep = 0.15

	// defaultBaseScore 是 endpoint 未指定基础分时的默认值。
	defaultBaseScore = 1.0

	// maxDecayScore 是衰减分数的上限。
	maxDecayScore = 1.0
)

// ── 内部状态 ──

// decayState 记录单个 endpoint 的实时衰减状态。
type decayState struct {
	DecayFactor     float64   // 当前衰减系数，1.0 = 无衰减
	ConsecutiveFail int       // 连续失败次数
	LastUpdate      time.Time // 最后更新时间
}

// ── FastDecayScorer ──

// FastDecayScorer 管理多个 endpoint 的快速衰减评分。
// 并发安全，适合在请求级调用。
//
// 仅对白嫖池（PoolTagTemp）和临时渠道生效；其他池返回 1.0。
type FastDecayScorer struct {
	mu        sync.RWMutex
	states    map[string]*decayState // key = endpointUID
	baseScore float64                // endpoint 基础分（当前统一使用默认值 1.0）
}

// NewFastDecayScorer 创建一个 FastDecayScorer 实例。
func NewFastDecayScorer() *FastDecayScorer {
	return &FastDecayScorer{
		states:    make(map[string]*decayState),
		baseScore: defaultBaseScore,
	}
}

// ── 池标签判断 ──

// IsFastDecayEligible 判断给定的 PoolTag 是否属于快速衰减的作用范围。
// 设计 §6.5 触发条件：costTier=free|cheap 或 poolTag=temp 的渠道自动启用 FastDecay。
// Phase 1 仅检查 PoolTag，CostTier 的判断由上游调度器在调用前过滤。
func IsFastDecayEligible(poolTag PoolTag) bool {
	return poolTag == PoolTagTemp
}

// ── 记录结果 ──

// RecordResult 记录一次请求结果并更新衰减状态。
// 如果 endpointUID 不在已跟踪列表中，会自动初始化。
// success=true 表示请求成功，success=false 表示请求失败。
func (s *FastDecayScorer) RecordResult(endpointUID string, success bool) {
	if success {
		s.onSuccess(endpointUID)
	} else {
		s.onFailure(endpointUID)
	}
}

// RecordStreamBreak 记录一次断流事件。
// 断流比普通失败更严重，使用更激进的衰减基数（0.70 vs 0.85）。
func (s *FastDecayScorer) RecordStreamBreak(endpointUID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.getOrCreateState(endpointUID)
	st.ConsecutiveFail++
	st.DecayFactor = math.Pow(streamBreakDecayBase, float64(st.ConsecutiveFail))
	st.LastUpdate = time.Now()

	log.Printf("[FastDecay-StreamBreak] endpoint=%s consecutiveFail=%d decayFactor=%.4f",
		endpointUID, st.ConsecutiveFail, st.DecayFactor)
}

// ── 查询分数 ──

// Score 返回指定 endpoint 的当前有效衰减分数。
// 分数 = baseScore * DecayFactor，范围 [0, 1]。
// 如果 endpoint 未被记录过，返回 1.0（无衰减）。
func (s *FastDecayScorer) Score(endpointUID string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st, exists := s.states[endpointUID]
	if !exists {
		return maxDecayScore
	}
	return s.baseScore * st.DecayFactor
}

// RawState 返回指定 endpoint 的原始衰减因子和连续失败次数（供 UI 展示）。
// 如果 endpoint 未被记录过，返回 (1.0, 0)。
func (s *FastDecayScorer) RawState(endpointUID string) (decayFactor float64, consecutiveFail int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st, exists := s.states[endpointUID]
	if !exists {
		return 1.0, 0
	}
	return st.DecayFactor, st.ConsecutiveFail
}

// Reset 重置指定 endpoint 的衰减状态。
func (s *FastDecayScorer) Reset(endpointUID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.states, endpointUID)
	log.Printf("[FastDecay-Reset] endpoint=%s", endpointUID)
}

// TrackedCount 返回当前被跟踪的 endpoint 数量（供监控）。
func (s *FastDecayScorer) TrackedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.states)
}

// ── 内部方法 ──

// onSuccess 处理请求成功：DecayFactor 快速回升。
func (s *FastDecayScorer) onSuccess(endpointUID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.getOrCreateState(endpointUID)
	st.DecayFactor = math.Min(maxDecayScore, st.DecayFactor+successRecoveryStep)
	st.ConsecutiveFail = 0
	st.LastUpdate = time.Now()
}

// onFailure 处理请求失败：指数衰减。
// 连续失败 N 次后 DecayFactor = pow(0.85, N)。
// 1次失败: 0.85, 2次: 0.72, 3次: 0.61, 5次: 0.44, 10次: 0.20。
func (s *FastDecayScorer) onFailure(endpointUID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.getOrCreateState(endpointUID)
	st.ConsecutiveFail++
	st.DecayFactor = math.Pow(defaultDecayBase, float64(st.ConsecutiveFail))
	st.LastUpdate = time.Now()

	log.Printf("[FastDecay-Failure] endpoint=%s consecutiveFail=%d decayFactor=%.4f",
		endpointUID, st.ConsecutiveFail, st.DecayFactor)
}

// getOrCreateState 获取或初始化 endpoint 的衰减状态。
// 调用者必须持有写锁。
func (s *FastDecayScorer) getOrCreateState(endpointUID string) *decayState {
	st, exists := s.states[endpointUID]
	if !exists {
		st = &decayState{
			DecayFactor: 1.0,
			LastUpdate:  time.Now(),
		}
		s.states[endpointUID] = st
	}
	return st
}
