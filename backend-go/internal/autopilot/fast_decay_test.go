package autopilot

import (
	"math"
	"sync"
	"testing"
)

// ── IsFastDecayEligible 测试 ──

func TestIsFastDecayEligible(t *testing.T) {
	tests := []struct {
		name     string
		poolTag  PoolTag
		expected bool
	}{
		{"temp pool eligible", PoolTagTemp, true},
		{"regular pool not eligible", PoolTagRegular, false},
		{"premium pool not eligible", PoolTagPremium, false},
		{"empty pool not eligible", PoolTag(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsFastDecayEligible(tt.poolTag)
			if got != tt.expected {
				t.Errorf("IsFastDecayEligible(%q) = %v, want %v", tt.poolTag, got, tt.expected)
			}
		})
	}
}

// ── 连续失败衰减测试（表驱动）──

func TestFastDecay_ConsecutiveFailure(t *testing.T) {
	tests := []struct {
		name             string
		failCount        int
		wantDecayFactor  float64 // 期望 DecayFactor = pow(0.85, failCount)
		wantConsecutiveN int
	}{
		{"1 fail", 1, 0.85, 1},
		{"2 fails", 2, 0.85 * 0.85, 2},
		{"3 fails", 3, 0.85 * 0.85 * 0.85, 3},
		{"5 fails", 5, math.Pow(0.85, 5), 5},
		{"10 fails", 10, math.Pow(0.85, 10), 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := NewFastDecayScorer()
			uid := "ep-test-fail"

			for i := 0; i < tt.failCount; i++ {
				scorer.RecordResult(uid, false)
			}

			factor, consFail := scorer.RawState(uid)
			if consFail != tt.wantConsecutiveN {
				t.Errorf("ConsecutiveFail = %d, want %d", consFail, tt.wantConsecutiveN)
			}
			if math.Abs(factor-tt.wantDecayFactor) > 1e-6 {
				t.Errorf("DecayFactor = %.6f, want %.6f", factor, tt.wantDecayFactor)
			}

			// Score = baseScore(1.0) * DecayFactor
			score := scorer.Score(uid)
			if math.Abs(score-tt.wantDecayFactor) > 1e-6 {
				t.Errorf("Score = %.6f, want %.6f", score, tt.wantDecayFactor)
			}
		})
	}
}

// ── 成功恢复测试 ──

func TestFastDecay_SuccessRecovery(t *testing.T) {
	scorer := NewFastDecayScorer()
	uid := "ep-test-recover"

	// 连续失败 3 次，DecayFactor = pow(0.85, 3) ≈ 0.614125
	for i := 0; i < 3; i++ {
		scorer.RecordResult(uid, false)
	}

	factor, _ := scorer.RawState(uid)
	expectedAfterFail := 0.85 * 0.85 * 0.85
	if math.Abs(factor-expectedAfterFail) > 1e-6 {
		t.Fatalf("after 3 fails: DecayFactor = %.6f, want %.6f", factor, expectedAfterFail)
	}

	// 成功一次，回升 +0.15
	scorer.RecordResult(uid, true)
	factor, consFail := scorer.RawState(uid)
	expectedAfterRecover := math.Min(1.0, expectedAfterFail+0.15)
	if math.Abs(factor-expectedAfterRecover) > 1e-6 {
		t.Errorf("after 1 success: DecayFactor = %.6f, want %.6f", factor, expectedAfterRecover)
	}
	if consFail != 0 {
		t.Errorf("after success: ConsecutiveFail = %d, want 0", consFail)
	}

	// 再成功几次直到恢复到 1.0
	for i := 0; i < 10; i++ {
		scorer.RecordResult(uid, true)
	}
	factor, _ = scorer.RawState(uid)
	if math.Abs(factor-1.0) > 1e-6 {
		t.Errorf("after many successes: DecayFactor = %.6f, want 1.0", factor)
	}
}

// ── 失败后成功再失败：连续失败计数器重置 ──

func TestFastDecay_FailSuccessFail(t *testing.T) {
	scorer := NewFastDecayScorer()
	uid := "ep-test-reset"

	// 失败 2 次
	scorer.RecordResult(uid, false)
	scorer.RecordResult(uid, false)
	_, consFail := scorer.RawState(uid)
	if consFail != 2 {
		t.Fatalf("after 2 fails: ConsecutiveFail = %d, want 2", consFail)
	}

	// 成功 1 次：连续失败归零
	scorer.RecordResult(uid, true)
	_, consFail = scorer.RawState(uid)
	if consFail != 0 {
		t.Fatalf("after success: ConsecutiveFail = %d, want 0", consFail)
	}

	// 再失败 1 次：连续失败 = 1（不是 3）
	scorer.RecordResult(uid, false)
	factor, consFail := scorer.RawState(uid)
	if consFail != 1 {
		t.Errorf("after 1 more fail: ConsecutiveFail = %d, want 1", consFail)
	}
	expectedFactor := 0.85 // 全新计数，不是从之前累计
	if math.Abs(factor-expectedFactor) > 1e-6 {
		t.Errorf("after 1 more fail: DecayFactor = %.6f, want %.6f", factor, expectedFactor)
	}
}

// ── 断流（stream break）衰减更激进 ──

func TestFastDecay_StreamBreak(t *testing.T) {
	scorer := NewFastDecayScorer()
	uid := "ep-test-stream"

	// 断流 1 次：DecayFactor = pow(0.70, 1) = 0.70
	scorer.RecordStreamBreak(uid)
	factor, consFail := scorer.RawState(uid)
	if consFail != 1 {
		t.Errorf("ConsecutiveFail = %d, want 1", consFail)
	}
	expectedFactor := 0.70
	if math.Abs(factor-expectedFactor) > 1e-6 {
		t.Errorf("DecayFactor = %.6f, want %.6f", factor, expectedFactor)
	}

	// 断流 2 次：DecayFactor = pow(0.70, 2) = 0.49
	scorer.RecordStreamBreak(uid)
	factor, consFail = scorer.RawState(uid)
	if consFail != 2 {
		t.Errorf("ConsecutiveFail = %d, want 2", consFail)
	}
	expectedFactor = 0.70 * 0.70
	if math.Abs(factor-expectedFactor) > 1e-6 {
		t.Errorf("DecayFactor = %.6f, want %.6f", factor, expectedFactor)
	}
}

// ── 未记录的 endpoint 返回 1.0 ──

func TestFastDecay_UnknownEndpoint(t *testing.T) {
	scorer := NewFastDecayScorer()

	score := scorer.Score("nonexistent-uid")
	if score != 1.0 {
		t.Errorf("Score for unknown endpoint = %.6f, want 1.0", score)
	}

	factor, consFail := scorer.RawState("nonexistent-uid")
	if factor != 1.0 || consFail != 0 {
		t.Errorf("RawState for unknown endpoint = (%.6f, %d), want (1.0, 0)", factor, consFail)
	}
}

// ── Reset 功能 ──

func TestFastDecay_Reset(t *testing.T) {
	scorer := NewFastDecayScorer()
	uid := "ep-test-reset"

	// 多次失败
	for i := 0; i < 5; i++ {
		scorer.RecordResult(uid, false)
	}
	if scorer.Score(uid) >= 1.0 {
		t.Fatal("expected score < 1.0 after 5 failures")
	}

	// Reset
	scorer.Reset(uid)
	if scorer.Score(uid) != 1.0 {
		t.Errorf("Score after reset = %.6f, want 1.0", scorer.Score(uid))
	}
	if scorer.TrackedCount() != 0 {
		t.Errorf("TrackedCount after reset = %d, want 0", scorer.TrackedCount())
	}
}

// ── 多 endpoint 独立性 ──

func TestFastDecay_MultipleEndpoints(t *testing.T) {
	scorer := NewFastDecayScorer()

	// ep-a 失败 3 次
	for i := 0; i < 3; i++ {
		scorer.RecordResult("ep-a", false)
	}
	// ep-b 无操作
	// ep-c 成功 1 次
	scorer.RecordResult("ep-c", true)

	if scorer.TrackedCount() != 2 {
		t.Errorf("TrackedCount = %d, want 2 (ep-a and ep-c)", scorer.TrackedCount())
	}

	scoreA := scorer.Score("ep-a")
	scoreB := scorer.Score("ep-b")
	scoreC := scorer.Score("ep-c")

	if math.Abs(scoreA-0.85*0.85*0.85) > 1e-6 {
		t.Errorf("ep-a Score = %.6f, want %.6f", scoreA, 0.85*0.85*0.85)
	}
	if scoreB != 1.0 {
		t.Errorf("ep-b Score = %.6f, want 1.0 (untracked)", scoreB)
	}
	if scoreC != 1.0 {
		t.Errorf("ep-c Score = %.6f, want 1.0 (success, recovery capped)", scoreC)
	}
}

// ── 并发安全测试 ──

func TestFastDecay_ConcurrentSafety(t *testing.T) {
	scorer := NewFastDecayScorer()
	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // 3 类操作

	// 并发 RecordResult success
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				scorer.RecordResult("ep-concurrent", true)
			}
		}()
	}

	// 并发 RecordResult failure
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				scorer.RecordResult("ep-concurrent", false)
			}
		}()
	}

	// 并发 Score 查询
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				score := scorer.Score("ep-concurrent")
				if score < 0 || score > 1.0 {
					t.Errorf("Score out of range [0,1]: %.6f", score)
				}
			}
		}()
	}

	wg.Wait()

	// 最终分数在 [0, 1] 范围内即可
	score := scorer.Score("ep-concurrent")
	if score < 0 || score > 1.0 {
		t.Errorf("Final Score = %.6f, out of range [0, 1]", score)
	}
}

// ── Scores 批量查询 ──

func TestFastDecay_Scores_Batch(t *testing.T) {
	scorer := NewFastDecayScorer()

	// ep-a 失败 3 次
	for i := 0; i < 3; i++ {
		scorer.RecordResult("ep-a", false)
	}
	// ep-b 无操作（未被记录）
	// ep-c 失败 1 次
	scorer.RecordResult("ep-c", false)

	uids := []string{"ep-a", "ep-b", "ep-c", "ep-unknown"}
	scores := scorer.Scores(uids)

	if scores == nil {
		t.Fatal("Scores returned nil for non-empty uids")
	}
	if len(scores) != 4 {
		t.Fatalf("Scores len = %d, want 4", len(scores))
	}

	// ep-a: 3 fails → pow(0.85, 3)
	expectedA := 0.85 * 0.85 * 0.85
	if math.Abs(scores["ep-a"]-expectedA) > 1e-6 {
		t.Errorf("ep-a Score = %.6f, want %.6f", scores["ep-a"], expectedA)
	}

	// ep-b: 未记录 → 1.0
	if scores["ep-b"] != 1.0 {
		t.Errorf("ep-b Score = %.6f, want 1.0", scores["ep-b"])
	}

	// ep-c: 1 fail → 0.85
	expectedC := 0.85
	if math.Abs(scores["ep-c"]-expectedC) > 1e-6 {
		t.Errorf("ep-c Score = %.6f, want %.6f", scores["ep-c"], expectedC)
	}

	// ep-unknown: 未记录 → 1.0
	if scores["ep-unknown"] != 1.0 {
		t.Errorf("ep-unknown Score = %.6f, want 1.0", scores["ep-unknown"])
	}
}

func TestFastDecay_Scores_EmptyUIDs(t *testing.T) {
	scorer := NewFastDecayScorer()
	scorer.RecordResult("ep-a", false)

	scores := scorer.Scores(nil)
	if scores != nil {
		t.Errorf("Scores(nil) = %v, want nil", scores)
	}

	scores = scorer.Scores([]string{})
	if scores != nil {
		t.Errorf("Scores([]) = %v, want nil", scores)
	}
}

func TestFastDecay_Scores_ConcurrentSafety(t *testing.T) {
	scorer := NewFastDecayScorer()
	const goroutines = 20
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				scorer.RecordResult("ep-concurrent", false)
			}
		}()
	}

	// 并发 Scores 查询
	wg2 := sync.WaitGroup{}
	wg2.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg2.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				scores := scorer.Scores([]string{"ep-concurrent", "ep-unknown"})
				if scores == nil {
					t.Error("Scores returned nil")
				}
				for _, v := range scores {
					if v < 0 || v > 1.0 {
						t.Errorf("Score out of range [0,1]: %.6f", v)
					}
				}
			}
		}()
	}

	wg.Wait()
	wg2.Wait()
}

// ── 非白嫖池恒为 1.0（通过 IsFastDecayEligible 判断）──

func TestIsFastDecayEligible_OnlyTemp(t *testing.T) {
	// 所有非 temp 池标签都应返回 false
	nonTempPools := []PoolTag{PoolTagRegular, PoolTagPremium, PoolTag(""), PoolTag("unknown")}
	for _, pool := range nonTempPools {
		if IsFastDecayEligible(pool) {
			t.Errorf("IsFastDecayEligible(%q) = true, want false", pool)
		}
	}

	// 只有 temp 池返回 true
	if !IsFastDecayEligible(PoolTagTemp) {
		t.Error("IsFastDecayEligible(PoolTagTemp) = false, want true")
	}
}
