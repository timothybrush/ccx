package autopilot

import "testing"

func TestChannelDomainScore_UsesQualityStabilityAndDomainStrength(t *testing.T) {
	cp := ChannelProfile{
		QualityTier:   QualityTierPremium,
		StabilityTier: StabilityTierStable,
	}
	modelProfiles := []ModelProfile{
		{
			ModelID:             "custom-model",
			TaskDomainStrengths: map[TaskDomain]float64{TaskDomainCoding: 0.9},
		},
	}
	score := ChannelDomainScore(cp, TaskDomainCoding, modelProfiles)
	// domainScore=0.9*0.5 + qualityScore(1.0)*0.3 + stabilityScore(1.0)*0.2 = 0.45+0.3+0.2=0.95
	if score < 0.94 || score > 0.96 {
		t.Errorf("expected score ~0.95, got %f", score)
	}
}

func TestChannelDomainScore_NeutralWithoutModelProfiles(t *testing.T) {
	cp := ChannelProfile{
		QualityTier:   QualityTierLow,
		StabilityTier: StabilityTierUnstable,
	}
	score := ChannelDomainScore(cp, TaskDomainCoding, nil)
	// domainScore=0.5*0.5 + qualityScore(0)*0.3 + stabilityScore(0)*0.2 = 0.25
	if score < 0.24 || score > 0.26 {
		t.Errorf("expected score ~0.25, got %f", score)
	}
}

func TestBuildRecommendationsForUser_RecommendsSignificantlyBetterRarelyUsedChannel(t *testing.T) {
	acc := NewUsagePatternAccumulator(30)
	// mask1 主要用 ch_a 做 coding（9 次），偶尔用 ch_b（1 次，占比 10% > 5% 阈值，应被排除）
	for i := 0; i < 9; i++ {
		acc.RecordUsage("mask1", TaskDomainCoding, "ch_a")
	}
	acc.RecordUsage("mask1", TaskDomainCoding, "ch_b")

	lookup := channelScoreLookup{
		scores: map[string]map[TaskDomain]float64{
			"ch_a": {TaskDomainCoding: 0.5},
			"ch_b": {TaskDomainCoding: 0.9},  // 显著更高，但使用占比超过阈值，不应推荐
			"ch_c": {TaskDomainCoding: 0.95}, // 从未使用，显著更高，应被推荐
		},
		healthy: map[string]bool{"ch_a": true, "ch_b": true, "ch_c": true},
	}

	recs := BuildRecommendationsForUser("mask1", acc, lookup, RecommendationOptions{})
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %+v", recs)
	}
	if recs[0].RecommendedChannelUID != "ch_c" {
		t.Errorf("expected recommendation for ch_c, got %s", recs[0].RecommendedChannelUID)
	}
	if recs[0].CurrentChannelUID != "ch_a" {
		t.Errorf("expected current channel ch_a, got %s", recs[0].CurrentChannelUID)
	}
}

func TestBuildRecommendationsForUser_NoRecommendationWhenScoreDeltaTooSmall(t *testing.T) {
	acc := NewUsagePatternAccumulator(30)
	acc.RecordUsage("mask1", TaskDomainCoding, "ch_a")

	lookup := channelScoreLookup{
		scores: map[string]map[TaskDomain]float64{
			"ch_a": {TaskDomainCoding: 0.5},
			"ch_c": {TaskDomainCoding: 0.55}, // 差距不足默认阈值 0.15
		},
		healthy: map[string]bool{"ch_a": true, "ch_c": true},
	}

	recs := BuildRecommendationsForUser("mask1", acc, lookup, RecommendationOptions{})
	if len(recs) != 0 {
		t.Fatalf("expected no recommendations, got %+v", recs)
	}
}

func TestBuildRecommendationsForUser_ExcludesUnhealthyChannels(t *testing.T) {
	acc := NewUsagePatternAccumulator(30)
	acc.RecordUsage("mask1", TaskDomainCoding, "ch_a")

	lookup := channelScoreLookup{
		scores: map[string]map[TaskDomain]float64{
			"ch_a": {TaskDomainCoding: 0.5},
			"ch_c": {TaskDomainCoding: 0.95},
		},
		healthy: map[string]bool{"ch_a": true, "ch_c": false},
	}

	recs := BuildRecommendationsForUser("mask1", acc, lookup, RecommendationOptions{})
	if len(recs) != 0 {
		t.Fatalf("expected no recommendations for unhealthy candidate, got %+v", recs)
	}
}

func TestBuildRecommendationsForUser_EmptyInputsReturnNil(t *testing.T) {
	acc := NewUsagePatternAccumulator(30)
	if recs := BuildRecommendationsForUser("", acc, channelScoreLookup{}, RecommendationOptions{}); recs != nil {
		t.Errorf("expected nil for empty proxyKeyMask, got %+v", recs)
	}
	if recs := BuildRecommendationsForUser("mask1", nil, channelScoreLookup{}, RecommendationOptions{}); recs != nil {
		t.Errorf("expected nil for nil accumulator, got %+v", recs)
	}
	if recs := BuildRecommendationsForUser("mask1", acc, channelScoreLookup{}, RecommendationOptions{}); recs != nil {
		t.Errorf("expected nil when no usage recorded, got %+v", recs)
	}
}
