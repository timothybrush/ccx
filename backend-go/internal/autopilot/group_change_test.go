package autopilot

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestGroupChangeDetector(t *testing.T) *GroupChangeDetector {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewProfileStoreWithDB(db)
	if err != nil {
		t.Fatalf("NewProfileStoreWithDB 失败: %v", err)
	}
	return NewGroupChangeDetector(store)
}

func TestGroupChangeDetector_TakeSnapshot(t *testing.T) {
	d := newTestGroupChangeDetector(t)

	models := []string{"gpt-4o", "gpt-4o-mini", "claude-3-5-sonnet"}
	snap := d.TakeSnapshot("ep-001", models)

	if snap.EndpointUID != "ep-001" {
		t.Fatalf("endpointUID 不匹配: got %q", snap.EndpointUID)
	}
	if len(snap.ModelList) != 3 {
		t.Fatalf("模型列表长度不匹配: got %d", len(snap.ModelList))
	}
	// 验证已排序
	if snap.ModelList[0] >= snap.ModelList[1] {
		t.Fatalf("快照模型列表未排序: %v", snap.ModelList)
	}
	if snap.ListHash == "" {
		t.Fatal("ListHash 不应为空")
	}
}

func TestGroupChangeDetector_GetSnapshot(t *testing.T) {
	d := newTestGroupChangeDetector(t)

	// 不存在
	if snap := d.GetSnapshot("nonexistent"); snap != nil {
		t.Fatal("不存在的快照应返回 nil")
	}

	d.TakeSnapshot("ep-001", []string{"gpt-4o", "gpt-4o-mini"})
	snap := d.GetSnapshot("ep-001")
	if snap == nil {
		t.Fatal("已存储的快照不应为 nil")
	}
	if len(snap.ModelList) != 2 {
		t.Fatalf("模型列表长度不匹配: got %d", len(snap.ModelList))
	}

	// 副本安全：修改返回值不影响内部存储
	snap.ModelList[0] = "mutated"
	internal := d.GetSnapshot("ep-001")
	if internal.ModelList[0] == "mutated" {
		t.Fatal("GetSnapshot 返回值应为副本，不应受外部修改影响")
	}
}

func TestGroupChangeDetector_RemoveSnapshot(t *testing.T) {
	d := newTestGroupChangeDetector(t)

	d.TakeSnapshot("ep-001", []string{"gpt-4o"})
	d.RemoveSnapshot("ep-001")

	if snap := d.GetSnapshot("ep-001"); snap != nil {
		t.Fatal("删除后快照应为 nil")
	}
}

func TestGroupChangeDetector_Compare(t *testing.T) {
	cases := []struct {
		name    string
		old     *KeyGroupSnapshot
		new     *KeyGroupSnapshot
		wantChg bool
		wantAdd []string
		wantRem []string
	}{
		{
			name:    "两者 nil",
			old:     nil,
			new:     nil,
			wantChg: false,
		},
		{
			name:    "old nil",
			old:     nil,
			new:     &KeyGroupSnapshot{ModelList: []string{"a"}, ListHash: "h1"},
			wantChg: false,
		},
		{
			name:    "new nil",
			old:     &KeyGroupSnapshot{ModelList: []string{"a"}, ListHash: "h1"},
			new:     nil,
			wantChg: false,
		},
		{
			name:    "完全相同",
			old:     &KeyGroupSnapshot{ModelList: []string{"a", "b"}, ListHash: hashModelList([]string{"a", "b"})},
			new:     &KeyGroupSnapshot{ModelList: []string{"a", "b"}, ListHash: hashModelList([]string{"a", "b"})},
			wantChg: false,
		},
		{
			name:    "新增模型",
			old:     &KeyGroupSnapshot{ModelList: []string{"a"}, ListHash: hashModelList([]string{"a"})},
			new:     &KeyGroupSnapshot{ModelList: []string{"a", "b"}, ListHash: hashModelList([]string{"a", "b"})},
			wantChg: true,
			wantAdd: []string{"b"},
			wantRem: nil,
		},
		{
			name:    "移除模型",
			old:     &KeyGroupSnapshot{ModelList: []string{"a", "b"}, ListHash: hashModelList([]string{"a", "b"})},
			new:     &KeyGroupSnapshot{ModelList: []string{"a"}, ListHash: hashModelList([]string{"a"})},
			wantChg: true,
			wantAdd: nil,
			wantRem: []string{"b"},
		},
		{
			name:    "分组漂移（同时新增和移除）",
			old:     &KeyGroupSnapshot{ModelList: []string{"a", "b"}, ListHash: hashModelList([]string{"a", "b"})},
			new:     &KeyGroupSnapshot{ModelList: []string{"b", "c"}, ListHash: hashModelList([]string{"b", "c"})},
			wantChg: true,
			wantAdd: []string{"c"},
			wantRem: []string{"a"},
		},
		{
			name:    "列表顺序不同但模型相同",
			old:     &KeyGroupSnapshot{ModelList: []string{"a", "b"}, ListHash: hashModelList([]string{"a", "b"})},
			new:     &KeyGroupSnapshot{ModelList: []string{"b", "a"}, ListHash: hashModelList([]string{"b", "a"})},
			wantChg: false,
		},
		{
			name:    "完全替换",
			old:     &KeyGroupSnapshot{ModelList: []string{"a", "b"}, ListHash: hashModelList([]string{"a", "b"})},
			new:     &KeyGroupSnapshot{ModelList: []string{"c", "d"}, ListHash: hashModelList([]string{"c", "d"})},
			wantChg: true,
			wantAdd: []string{"c", "d"},
			wantRem: []string{"a", "b"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestGroupChangeDetector(t)
			result := d.Compare(tc.old, tc.new)
			changed := result.OldHash != "" && result.NewHash != ""
			if changed != tc.wantChg {
				t.Fatalf("changed: got %v, want %v", changed, tc.wantChg)
			}
			if tc.wantChg {
				if result.OldHash == "" || result.NewHash == "" {
					t.Fatal("变更时 OldHash/NewHash 不应为空")
				}
				if !stringSliceEqual(result.AddedModels, tc.wantAdd) {
					t.Fatalf("AddedModels: got %v, want %v", result.AddedModels, tc.wantAdd)
				}
				if !stringSliceEqual(result.RemovedModels, tc.wantRem) {
					t.Fatalf("RemovedModels: got %v, want %v", result.RemovedModels, tc.wantRem)
				}
			}
		})
	}
}

func TestGroupChangeDetector_CheckGroupChange(t *testing.T) {
	cases := []struct {
		name  string
		steps []checkStep
	}{
		{
			name: "首次调用：记录快照，无变更",
			steps: []checkStep{
				{
					models:  []string{"gpt-4o", "gpt-4o-mini"},
					wantChg: false,
				},
			},
		},
		{
			name: "连续两次相同列表：无变更",
			steps: []checkStep{
				{models: []string{"gpt-4o", "gpt-4o-mini"}, wantChg: false},
				{models: []string{"gpt-4o", "gpt-4o-mini"}, wantChg: false},
			},
		},
		{
			name: "新增模型",
			steps: []checkStep{
				{models: []string{"gpt-4o"}, wantChg: false},
				{
					models:  []string{"gpt-4o", "gpt-4o-mini"},
					wantChg: true,
					wantAdd: []string{"gpt-4o-mini"},
					wantRem: nil,
				},
			},
		},
		{
			name: "移除模型",
			steps: []checkStep{
				{models: []string{"a", "b", "c"}, wantChg: false},
				{
					models:  []string{"a", "c"},
					wantChg: true,
					wantAdd: nil,
					wantRem: []string{"b"},
				},
			},
		},
		{
			name: "分组漂移",
			steps: []checkStep{
				{models: []string{"a", "b"}, wantChg: false},
				{
					models:  []string{"b", "c"},
					wantChg: true,
					wantAdd: []string{"c"},
					wantRem: []string{"a"},
				},
			},
		},
		{
			name: "漂移后恢复：视为再次变更",
			steps: []checkStep{
				{models: []string{"a", "b"}, wantChg: false},
				{
					models:  []string{"c", "d"},
					wantChg: true,
					wantAdd: []string{"c", "d"},
					wantRem: []string{"a", "b"},
				},
				{
					models:  []string{"a", "b"},
					wantChg: true,
					wantAdd: []string{"a", "b"},
					wantRem: []string{"c", "d"},
				},
			},
		},
		{
			name: "空列表到有模型",
			steps: []checkStep{
				{models: []string{}, wantChg: false},
				{
					models:  []string{"gpt-4o"},
					wantChg: true,
					wantAdd: []string{"gpt-4o"},
					wantRem: nil,
				},
			},
		},
		{
			name: "列表顺序不影响判定",
			steps: []checkStep{
				{models: []string{"b", "a"}, wantChg: false},
				{models: []string{"a", "b"}, wantChg: false},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestGroupChangeDetector(t)
			for i, step := range tc.steps {
				changed, result := d.CheckGroupChange("ch-001", "messages", "mk-test", step.models)
				if changed != step.wantChg {
					t.Fatalf("步骤 %d: changed got %v, want %v", i, changed, step.wantChg)
				}
				if step.wantChg {
					if !stringSliceEqual(result.AddedModels, step.wantAdd) {
						t.Fatalf("步骤 %d AddedModels: got %v, want %v", i, result.AddedModels, step.wantAdd)
					}
					if !stringSliceEqual(result.RemovedModels, step.wantRem) {
						t.Fatalf("步骤 %d RemovedModels: got %v, want %v", i, result.RemovedModels, step.wantRem)
					}
				}
			}
		})
	}
}

func TestGroupChangeDetector_CheckGroupChange_Independent(t *testing.T) {
	// 不同 endpoint 互不干扰
	d := newTestGroupChangeDetector(t)

	d.CheckGroupChange("ch-001", "messages", "mk-001", []string{"a", "b"})
	d.CheckGroupChange("ch-002", "messages", "mk-002", []string{"x", "y"})

	// ch-001 变更，ch-002 不变
	changed1, _ := d.CheckGroupChange("ch-001", "messages", "mk-001", []string{"a", "c"})
	changed2, _ := d.CheckGroupChange("ch-002", "messages", "mk-002", []string{"x", "y"})

	if !changed1 {
		t.Fatal("ch-001 应检测到变更")
	}
	if changed2 {
		t.Fatal("ch-002 不应检测到变更")
	}
}

func TestHashModelList_Deterministic(t *testing.T) {
	models := []string{"gpt-4o", "claude-3", "gemini-pro"}
	h1 := hashModelList(models)
	h2 := hashModelList(models)
	if h1 != h2 {
		t.Fatalf("相同输入应产生相同哈希: %s != %s", h1, h2)
	}
}

func TestDiffModelLists(t *testing.T) {
	cases := []struct {
		name    string
		old     []string
		new     []string
		added   []string
		removed []string
	}{
		{
			name: "无变化",
			old:  []string{"a", "b"},
			new:  []string{"a", "b"},
		},
		{
			name:    "仅新增",
			old:     []string{"a"},
			new:     []string{"a", "b"},
			added:   []string{"b"},
			removed: nil,
		},
		{
			name:    "仅移除",
			old:     []string{"a", "b"},
			new:     []string{"a"},
			added:   nil,
			removed: []string{"b"},
		},
		{
			name:    "同时新增和移除",
			old:     []string{"a", "b"},
			new:     []string{"b", "c"},
			added:   []string{"c"},
			removed: []string{"a"},
		},
		{
			name:    "两者都为空",
			old:     nil,
			new:     nil,
			added:   nil,
			removed: nil,
		},
		{
			name:    "从空到有",
			old:     nil,
			new:     []string{"a"},
			added:   []string{"a"},
			removed: nil,
		},
		{
			name:    "从有到空",
			old:     []string{"a"},
			new:     nil,
			added:   nil,
			removed: []string{"a"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			added, removed := diffModelLists(tc.old, tc.new)
			if !stringSliceEqual(added, tc.added) {
				t.Fatalf("added: got %v, want %v", added, tc.added)
			}
			if !stringSliceEqual(removed, tc.removed) {
				t.Fatalf("removed: got %v, want %v", removed, tc.removed)
			}
		})
	}
}

// ── 测试辅助 ──

// checkStep CheckGroupChange 的单步测试输入。
type checkStep struct {
	models  []string
	wantChg bool
	wantAdd []string
	wantRem []string
}

// stringSliceEqual 比较两个字符串切片是否相等（nil 与空切片视为相等）。
func stringSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
