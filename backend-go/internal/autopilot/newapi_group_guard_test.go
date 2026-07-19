package autopilot

import "testing"

func TestResolveNewApiProvisionGroups_AllEligible(t *testing.T) {
	limit := 1.0
	resolved, err := resolveNewApiProvisionGroups(map[string]float64{
		"default": 1,
		"cheap":   0.5,
		"premium": 2,
	}, "", true, &limit)
	if err != nil {
		t.Fatalf("resolveNewApiProvisionGroups 返回错误: %v", err)
	}
	if len(resolved) != 2 || resolved[0].Name != "cheap" || resolved[0].Ratio != 0.5 || resolved[1].Name != "default" || resolved[1].MaxMultiplier != 1 {
		t.Fatalf("自动选择的分组不匹配: %+v", resolved)
	}
}

func TestResolveNewApiProvisionGroups_PreservesLegacyDefaultGroup(t *testing.T) {
	limit := 1.0
	resolved, err := resolveNewApiProvisionGroups(map[string]float64{
		"default": 1,
		"cheap":   0.5,
	}, "", false, &limit)
	if err != nil {
		t.Fatalf("resolveNewApiProvisionGroups 返回错误: %v", err)
	}
	if len(resolved) != 1 || resolved[0].Name != "default" || resolved[0].Ratio != 1 {
		t.Fatalf("旧调用的 default 分组语义丢失: %+v", resolved)
	}
}

func TestResolveNewApiProvisionGroupsRejectsUnsafeOrUnknownGroups(t *testing.T) {
	limit := 1.0
	groups := map[string]float64{"default": 1, "premium": 2}
	for _, group := range []string{"premium", "missing"} {
		if _, err := resolveNewApiProvisionGroups(groups, group, false, &limit); err == nil {
			t.Fatalf("分组 %q 应被拒绝", group)
		}
	}
}

func TestResolveNewApiProvisionGroupsRejectsAmbiguousMode(t *testing.T) {
	limit := 1.0
	if _, err := resolveNewApiProvisionGroups(map[string]float64{"default": 1}, "default", true, &limit); err == nil {
		t.Fatal("显式分组和自动全部模式不能同时存在")
	}
}

func TestDefaultNewApiProvisionKeyNameForGroup(t *testing.T) {
	if got := defaultNewApiProvisionKeyNameForGroup("Premium Group"); got != "ccx-autopilot-premium-group" {
		t.Fatalf("分组 Key 名称 = %q", got)
	}
}
