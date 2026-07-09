package autopilot

import (
	"testing"
	"time"
)

func TestUsagePatternAccumulator_RecordAndDistribution(t *testing.T) {
	acc := NewUsagePatternAccumulator(30)

	acc.RecordUsage("mask1", TaskDomainCoding, "ch_a")
	acc.RecordUsage("mask1", TaskDomainCoding, "ch_a")
	acc.RecordUsage("mask1", TaskDomainCoding, "ch_b")
	acc.RecordUsage("mask1", TaskDomainWriting, "ch_a")

	domainStats := acc.DomainDistribution("mask1", 7)
	if len(domainStats) != 2 {
		t.Fatalf("expected 2 domains, got %d: %+v", len(domainStats), domainStats)
	}
	if domainStats[0].Domain != TaskDomainCoding || domainStats[0].Count != 3 {
		t.Errorf("expected top domain coding with count 3, got %+v", domainStats[0])
	}
	if domainStats[1].Domain != TaskDomainWriting || domainStats[1].Count != 1 {
		t.Errorf("expected second domain writing with count 1, got %+v", domainStats[1])
	}

	channelStats := acc.ChannelDistribution("mask1", TaskDomainCoding, 7)
	if len(channelStats) != 2 {
		t.Fatalf("expected 2 channels, got %d: %+v", len(channelStats), channelStats)
	}
	if channelStats[0].ChannelUID != "ch_a" || channelStats[0].Count != 2 {
		t.Errorf("expected top channel ch_a with count 2, got %+v", channelStats[0])
	}
}

func TestUsagePatternAccumulator_EmptyProxyKeyMaskOrChannelUIDIgnored(t *testing.T) {
	acc := NewUsagePatternAccumulator(30)
	acc.RecordUsage("", TaskDomainCoding, "ch_a")
	acc.RecordUsage("mask1", TaskDomainCoding, "")

	if stats := acc.DomainDistribution("mask1", 7); len(stats) != 0 {
		t.Errorf("expected no data recorded, got %+v", stats)
	}
	if masks := acc.AllProxyKeyMasks(); len(masks) != 0 {
		t.Errorf("expected no proxy key masks, got %+v", masks)
	}
}

func TestUsagePatternAccumulator_DefaultsToGeneralDomain(t *testing.T) {
	acc := NewUsagePatternAccumulator(30)
	acc.RecordUsage("mask1", "", "ch_a")

	stats := acc.DomainDistribution("mask1", 7)
	if len(stats) != 1 || stats[0].Domain != TaskDomainGeneral {
		t.Fatalf("expected general domain fallback, got %+v", stats)
	}
}

func TestUsagePatternAccumulator_WindowExcludesOldData(t *testing.T) {
	acc := NewUsagePatternAccumulator(30)
	fixedNow := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	acc.nowFunc = func() time.Time { return fixedNow.AddDate(0, 0, -10) }
	acc.RecordUsage("mask1", TaskDomainCoding, "ch_a")

	acc.nowFunc = func() time.Time { return fixedNow }
	acc.RecordUsage("mask1", TaskDomainCoding, "ch_b")

	// windowDays=7：10 天前的记录应被排除在窗口外
	stats := acc.DomainDistribution("mask1", 7)
	if len(stats) != 1 || stats[0].Count != 1 {
		t.Fatalf("expected only recent record within 7-day window, got %+v", stats)
	}

	// windowDays=30：两条记录都应包含
	statsWide := acc.DomainDistribution("mask1", 30)
	if len(statsWide) != 1 || statsWide[0].Count != 2 {
		t.Fatalf("expected both records within 30-day window, got %+v", statsWide)
	}
}

func TestUsagePatternAccumulator_AllProxyKeyMasks(t *testing.T) {
	acc := NewUsagePatternAccumulator(30)
	acc.RecordUsage("mask1", TaskDomainCoding, "ch_a")
	acc.RecordUsage("mask2", TaskDomainWriting, "ch_b")

	masks := acc.AllProxyKeyMasks()
	if len(masks) != 2 {
		t.Fatalf("expected 2 masks, got %+v", masks)
	}
}
