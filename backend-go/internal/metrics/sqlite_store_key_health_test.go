package metrics

import (
	"path/filepath"
	"testing"
	"time"
)

func newKeyHealthTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := NewSQLiteStore(&SQLiteStoreConfig{
		DBPath:        filepath.Join(t.TempDir(), "metrics.db"),
		RetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() err = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func keyHealthTestRecord(channelType, channelID, keyMask, checkKind string) KeyHealthRecord {
	return KeyHealthRecord{
		ChannelType: channelType,
		ChannelID:   channelID,
		KeyMask:     keyMask,
		CheckKind:   checkKind,
		LastCheckAt: time.Now(),
		LastStatus:  "ok",
		LatencyMs:   120,
		ModelCount:  5,
	}
}

func TestUpsertKeyHealth_InsertAndQuery(t *testing.T) {
	store := newKeyHealthTestStore(t)

	rec := keyHealthTestRecord("messages", "ch-1", "sk-***a", "l1")
	if err := store.UpsertKeyHealth(rec); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}
	rec2 := keyHealthTestRecord("messages", "ch-1", "sk-***a", "l2")
	rec2.ModelCount = 3
	if err := store.UpsertKeyHealth(rec2); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}

	records, err := store.GetKeyHealthForChannel("messages", "ch-1")
	if err != nil {
		t.Fatalf("GetKeyHealthForChannel() err = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records len = %d, want 2", len(records))
	}
	// ORDER BY key_mask, check_kind：l1 在前
	if records[0].CheckKind != "l1" || records[1].CheckKind != "l2" {
		t.Fatalf("check kinds = %s,%s, want l1,l2", records[0].CheckKind, records[1].CheckKind)
	}
	if records[0].LastStatus != "ok" || records[0].LatencyMs != 120 || records[0].ModelCount != 5 {
		t.Fatalf("unexpected record: %+v", records[0])
	}
	if records[0].LastCheckAt.Unix() != rec.LastCheckAt.Unix() {
		t.Fatalf("LastCheckAt = %v, want %v", records[0].LastCheckAt, rec.LastCheckAt)
	}
}

func TestUpsertKeyHealth_OverwritesExisting(t *testing.T) {
	store := newKeyHealthTestStore(t)

	rec := keyHealthTestRecord("chat", "ch-1", "sk-***a", "l1")
	if err := store.UpsertKeyHealth(rec); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}

	// 同主键再次写入：应覆盖而非新增
	rec.LastStatus = "auth_failed"
	rec.ConsecutiveFailures = 3
	rec.LatencyMs = 0
	rec.Detail = "401 unauthorized"
	later := rec.LastCheckAt.Add(time.Minute)
	rec.LastCheckAt = later
	if err := store.UpsertKeyHealth(rec); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}

	records, err := store.GetKeyHealthForChannel("chat", "ch-1")
	if err != nil {
		t.Fatalf("GetKeyHealthForChannel() err = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	got := records[0]
	if got.LastStatus != "auth_failed" || got.ConsecutiveFailures != 3 || got.Detail != "401 unauthorized" {
		t.Fatalf("record not overwritten: %+v", got)
	}
	if got.LastCheckAt.Unix() != later.Unix() {
		t.Fatalf("LastCheckAt = %v, want %v", got.LastCheckAt, later)
	}
}

func TestGetKeyHealthForChannel_ScopedByChannel(t *testing.T) {
	store := newKeyHealthTestStore(t)

	if err := store.UpsertKeyHealth(keyHealthTestRecord("messages", "ch-1", "sk-***a", "l1")); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}
	if err := store.UpsertKeyHealth(keyHealthTestRecord("messages", "ch-2", "sk-***b", "l1")); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}
	if err := store.UpsertKeyHealth(keyHealthTestRecord("chat", "ch-1", "sk-***a", "l1")); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}

	records, err := store.GetKeyHealthForChannel("messages", "ch-1")
	if err != nil {
		t.Fatalf("GetKeyHealthForChannel() err = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	if records[0].ChannelType != "messages" || records[0].ChannelID != "ch-1" || records[0].KeyMask != "sk-***a" {
		t.Fatalf("unexpected record: %+v", records[0])
	}

	// 不存在的渠道返回空
	records, err = store.GetKeyHealthForChannel("gemini", "ch-x")
	if err != nil {
		t.Fatalf("GetKeyHealthForChannel() err = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records len = %d, want 0", len(records))
	}
}

func TestGetAllKeyHealth(t *testing.T) {
	store := newKeyHealthTestStore(t)

	channelTypes := []string{"messages", "chat", "responses", "gemini", "images", "vectors"}
	for _, ct := range channelTypes {
		if err := store.UpsertKeyHealth(keyHealthTestRecord(ct, "ch-1", "sk-***a", "l1")); err != nil {
			t.Fatalf("UpsertKeyHealth(%s) err = %v", ct, err)
		}
	}

	records, err := store.GetAllKeyHealth()
	if err != nil {
		t.Fatalf("GetAllKeyHealth() err = %v", err)
	}
	if len(records) != len(channelTypes) {
		t.Fatalf("records len = %d, want %d", len(records), len(channelTypes))
	}
	seen := make(map[string]bool)
	for _, rec := range records {
		seen[rec.ChannelType] = true
	}
	for _, ct := range channelTypes {
		if !seen[ct] {
			t.Fatalf("missing channel_type %s in GetAllKeyHealth result", ct)
		}
	}
}

func TestDeleteKeyHealthForChannel(t *testing.T) {
	store := newKeyHealthTestStore(t)

	if err := store.UpsertKeyHealth(keyHealthTestRecord("messages", "ch-1", "sk-***a", "l1")); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}
	if err := store.UpsertKeyHealth(keyHealthTestRecord("messages", "ch-1", "sk-***b", "l1")); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}
	if err := store.UpsertKeyHealth(keyHealthTestRecord("messages", "ch-2", "sk-***c", "l1")); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}

	if err := store.DeleteKeyHealthForChannel("messages", "ch-1"); err != nil {
		t.Fatalf("DeleteKeyHealthForChannel() err = %v", err)
	}

	records, err := store.GetKeyHealthForChannel("messages", "ch-1")
	if err != nil {
		t.Fatalf("GetKeyHealthForChannel() err = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("ch-1 records len = %d, want 0", len(records))
	}

	// 其他渠道不受影响
	all, err := store.GetAllKeyHealth()
	if err != nil {
		t.Fatalf("GetAllKeyHealth() err = %v", err)
	}
	if len(all) != 1 || all[0].ChannelID != "ch-2" {
		t.Fatalf("remaining records = %+v, want only ch-2", all)
	}
}

func TestDeleteKeyHealthForKey(t *testing.T) {
	store := newKeyHealthTestStore(t)

	if err := store.UpsertKeyHealth(keyHealthTestRecord("messages", "ch-1", "sk-***a", "l1")); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}
	if err := store.UpsertKeyHealth(keyHealthTestRecord("messages", "ch-1", "sk-***a", "l2")); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}
	if err := store.UpsertKeyHealth(keyHealthTestRecord("messages", "ch-1", "sk-***b", "l1")); err != nil {
		t.Fatalf("UpsertKeyHealth() err = %v", err)
	}

	if err := store.DeleteKeyHealthForKey("messages", "ch-1", "sk-***a"); err != nil {
		t.Fatalf("DeleteKeyHealthForKey() err = %v", err)
	}

	records, err := store.GetKeyHealthForChannel("messages", "ch-1")
	if err != nil {
		t.Fatalf("GetKeyHealthForChannel() err = %v", err)
	}
	if len(records) != 1 || records[0].KeyMask != "sk-***b" {
		t.Fatalf("remaining records = %+v, want only sk-***b", records)
	}
}
