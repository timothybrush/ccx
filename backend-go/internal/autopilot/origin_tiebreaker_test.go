// origin_tiebreaker_test.go — BreakTieByOriginTier 表驱动测试

package autopilot

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper：快速构造 ScoredCandidate
func makeScored(uid string, score float64) ScoredCandidate {
	return ScoredCandidate{ChannelUID: uid, Score: score}
}

func TestBreakTieByOriginTier_NonTiePreservesScoreOrder(t *testing.T) {
	// 核心不变量：非平局输入排序结果与纯 Score 排序完全一致
	candidates := []ScoredCandidate{
		makeScored("ch-relay", 50.0),     // second
		makeScored("ch-official", 80.0),  // first
		makeScored("ch-community", 30.0), // third
	}
	originTiers := map[string]ChannelOriginTier{
		"ch-official":  OriginTierFirst,
		"ch-relay":     OriginTierSecond,
		"ch-community": OriginTierThird,
	}

	result := BreakTieByOriginTier(candidates, originTiers)

	assert.Equal(t, "ch-official", result[0].ChannelUID, "Score 80 应排第一")
	assert.Equal(t, "ch-relay", result[1].ChannelUID, "Score 50 应排第二")
	assert.Equal(t, "ch-community", result[2].ChannelUID, "Score 30 应排第三")
}

func TestBreakTieByOriginTier_TieBreakByOriginTier(t *testing.T) {
	// 平局输入按 OriginTier 正确排序
	candidates := []ScoredCandidate{
		makeScored("ch-community", 50.0), // third  → rank 1
		makeScored("ch-relay", 50.0),     // second → rank 2
		makeScored("ch-official", 50.0),  // first  → rank 3
	}
	originTiers := map[string]ChannelOriginTier{
		"ch-official":  OriginTierFirst,
		"ch-relay":     OriginTierSecond,
		"ch-community": OriginTierThird,
	}

	result := BreakTieByOriginTier(candidates, originTiers)

	assert.Equal(t, "ch-official", result[0].ChannelUID, "first tier 应排第一")
	assert.Equal(t, "ch-relay", result[1].ChannelUID, "second tier 应排第二")
	assert.Equal(t, "ch-community", result[2].ChannelUID, "third tier 应排第三")
}

func TestBreakTieByOriginTier_MixedTieAndNonTie(t *testing.T) {
	// 混合平局+非平局输入：非平局区间不变，平局区间按 OriginTier 排序
	candidates := []ScoredCandidate{
		makeScored("ch-community", 30.0), // third → rank 1
		makeScored("ch-relay-b", 50.0),   // second → rank 2
		makeScored("ch-relay-a", 50.0),   // second → rank 2（同分同 tier，顺序稳定）
		makeScored("ch-official", 50.0),  // first → rank 3
		makeScored("ch-local", 30.0),     // local → rank 1
		makeScored("ch-high", 90.0),      // first → rank 3
	}
	originTiers := map[string]ChannelOriginTier{
		"ch-high":      OriginTierFirst,
		"ch-official":  OriginTierFirst,
		"ch-relay-a":   OriginTierSecond,
		"ch-relay-b":   OriginTierSecond,
		"ch-community": OriginTierThird,
		"ch-local":     OriginTierLocal,
	}

	result := BreakTieByOriginTier(candidates, originTiers)

	// Score 90 独占最高，排第一
	assert.Equal(t, "ch-high", result[0].ChannelUID)

	// Score 50 平局区间：first > second > second
	assert.Equal(t, "ch-official", result[1].ChannelUID)
	// 两个 second 之间分数和 tier 完全相同，稳定排序应保持原序
	// 原序中 ch-relay-b 在 ch-relay-a 之前
	assert.Equal(t, "ch-relay-b", result[2].ChannelUID)
	assert.Equal(t, "ch-relay-a", result[3].ChannelUID)

	// Score 30 平局区间：third 和 local 同 rank=1，稳定排序保持原序
	assert.Equal(t, "ch-community", result[4].ChannelUID)
	assert.Equal(t, "ch-local", result[5].ChannelUID)
}

func TestBreakTieByOriginTier_MissingChannelUID(t *testing.T) {
	// originTiers map 中缺失某 ChannelUID 时按 unknown(rank 0) 处理，不 panic
	candidates := []ScoredCandidate{
		makeScored("ch-missing", 50.0),      // 不在 map → unknown → rank 0
		makeScored("ch-official", 50.0),     // first → rank 3
		makeScored("ch-also-missing", 50.0), // 不在 map → unknown → rank 0
	}
	originTiers := map[string]ChannelOriginTier{
		"ch-official": OriginTierFirst,
	}

	result := BreakTieByOriginTier(candidates, originTiers)

	assert.Equal(t, "ch-official", result[0].ChannelUID, "first 应排第一")
	// 两个 missing 同为 unknown rank 0，稳定排序保持原序
	assert.Equal(t, "ch-missing", result[1].ChannelUID)
	assert.Equal(t, "ch-also-missing", result[2].ChannelUID)
}

func TestBreakTieByOriginTier_LocalEqualsThird(t *testing.T) {
	// local 和 third 同 rank=1，平局时不应区分
	candidates := []ScoredCandidate{
		makeScored("ch-local", 50.0),     // local → rank 1
		makeScored("ch-community", 50.0), // third → rank 1
	}
	originTiers := map[string]ChannelOriginTier{
		"ch-local":     OriginTierLocal,
		"ch-community": OriginTierThird,
	}

	result := BreakTieByOriginTier(candidates, originTiers)

	// 同 rank 时稳定排序保持原序
	assert.Equal(t, "ch-local", result[0].ChannelUID)
	assert.Equal(t, "ch-community", result[1].ChannelUID)
}

func TestBreakTieByOriginTier_SingleAndEmpty(t *testing.T) {
	tests := []struct {
		name       string
		candidates []ScoredCandidate
	}{
		{"空列表", []ScoredCandidate{}},
		{"单元素", []ScoredCandidate{makeScored("ch-a", 42.0)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BreakTieByOriginTier(tt.candidates, nil)
			assert.Equal(t, len(tt.candidates), len(result))
		})
	}
}

func TestBreakTieByOriginTier_NearEqualScoresNotTreatedAsTie(t *testing.T) {
	// 分数微小差异（非严格相等）不应被 tie-breaker 干扰
	candidates := []ScoredCandidate{
		makeScored("ch-community", 50.0),  // third, 略低
		makeScored("ch-official", 50.001), // first, 略高
	}
	originTiers := map[string]ChannelOriginTier{
		"ch-official":  OriginTierFirst,
		"ch-community": OriginTierThird,
	}

	result := BreakTieByOriginTier(candidates, originTiers)

	// Score 不同，origin tier 不应改变顺序
	assert.Equal(t, "ch-official", result[0].ChannelUID)
	assert.Equal(t, "ch-community", result[1].ChannelUID)
}

func TestOriginTierRank_AllTiers(t *testing.T) {
	tests := []struct {
		tier ChannelOriginTier
		want int
	}{
		{OriginTierFirst, 3},
		{OriginTierSecond, 2},
		{OriginTierThird, 1},
		{OriginTierLocal, 1},
		{OriginTierUnknown, 0},
		{ChannelOriginTier("bogus"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			assert.Equal(t, tt.want, originTierRank(tt.tier))
		})
	}
}

func TestBreakTieByOriginTier_FullRankOrder(t *testing.T) {
	// 验证完整 rank 序关系：first > second > {third, local} > unknown
	candidates := []ScoredCandidate{
		makeScored("ch-unknown", 50.0),
		makeScored("ch-local", 50.0),
		makeScored("ch-third", 50.0),
		makeScored("ch-second", 50.0),
		makeScored("ch-first", 50.0),
	}
	originTiers := map[string]ChannelOriginTier{
		"ch-first":   OriginTierFirst,
		"ch-second":  OriginTierSecond,
		"ch-third":   OriginTierThird,
		"ch-local":   OriginTierLocal,
		"ch-unknown": OriginTierUnknown,
	}

	result := BreakTieByOriginTier(candidates, originTiers)

	assert.Equal(t, "ch-first", result[0].ChannelUID)
	assert.Equal(t, "ch-second", result[1].ChannelUID)
	// third 和 local 同 rank=1，稳定排序保持原序：local 在 third 之前（输入顺序）
	assert.Equal(t, "ch-local", result[2].ChannelUID)
	assert.Equal(t, "ch-third", result[3].ChannelUID)
	assert.Equal(t, "ch-unknown", result[4].ChannelUID)
}

func TestBreakTieByOriginTier_ExactEqualityWithFloatSpecialValues(t *testing.T) {
	// 验证 NaN / Inf 不误触发 tie-breaker（极端场景防御）
	candidates := []ScoredCandidate{
		makeScored("ch-a", math.Inf(1)),
		makeScored("ch-b", 50.0),
	}
	originTiers := map[string]ChannelOriginTier{
		"ch-a": OriginTierThird,
		"ch-b": OriginTierFirst,
	}

	result := BreakTieByOriginTier(candidates, originTiers)

	// Inf(1) > 50.0，Score 不同，origin tier 不影响
	assert.Equal(t, "ch-a", result[0].ChannelUID)
	assert.Equal(t, "ch-b", result[1].ChannelUID)
}

// TestBreakTieByOriginTier_PropertyNonTieMatchesScoreSort
// 属性测试：随机分数（严格不相等）下，BreakTieByOriginTier 排序结果
// 与纯 Score 排序完全一致。每轮生成 5-10 个候选、随机 OriginTier，
// 确保平局逻辑不会干扰非平局场景。
func TestBreakTieByOriginTier_PropertyNonTieMatchesScoreSort(t *testing.T) {
	tiers := []ChannelOriginTier{
		OriginTierFirst, OriginTierSecond, OriginTierThird,
		OriginTierLocal, OriginTierUnknown,
	}

	rng := rand.New(rand.NewSource(42)) // 固定种子，保证可复现

	for round := 0; round < 50; round++ {
		n := 5 + rng.Intn(6) // 5-10 个候选
		candidates := make([]ScoredCandidate, n)
		originTiers := make(map[string]ChannelOriginTier, n)

		// 生成严格不相等的分数（用递增值 + 微扰避免浮点碰撞）
		scores := make([]float64, n)
		for i := range scores {
			scores[i] = float64(i+1)*100 + rng.Float64()*0.5
		}
		// 随机打乱顺序（保证测试不是输入已排序的特例）
		rng.Shuffle(n, func(i, j int) {
			scores[i], scores[j] = scores[j], scores[i]
		})

		for i := 0; i < n; i++ {
			uid := fmt.Sprintf("ch-%d-%d", round, i)
			candidates[i] = ScoredCandidate{ChannelUID: uid, Score: scores[i]}
			originTiers[uid] = tiers[rng.Intn(len(tiers))]
		}

		// 纯 Score 降序排序（参照基线）
		scoreSorted := make([]ScoredCandidate, n)
		copy(scoreSorted, candidates)
		sort.SliceStable(scoreSorted, func(i, j int) bool {
			return scoreSorted[i].Score > scoreSorted[j].Score
		})

		// BreakTieByOriginTier 排序
		tieBroken := make([]ScoredCandidate, n)
		copy(tieBroken, candidates)
		BreakTieByOriginTier(tieBroken, originTiers)

		// 两者必须完全一致（逐元素 UID 比较）
		for i := 0; i < n; i++ {
			assert.Equal(t, scoreSorted[i].ChannelUID, tieBroken[i].ChannelUID,
				"round=%d rank=%d 非平局时排序结果应与纯 Score 排序一致", round, i)
		}
	}
}
