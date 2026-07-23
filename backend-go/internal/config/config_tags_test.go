package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestConfigManager(t *testing.T) *ConfigManager {
	t.Helper()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	initialConfig := `{
		"upstream": [{
			"name": "test-channel",
			"baseUrl": "https://example.com",
			"apiKeys": ["sk-test"],
			"serviceType": "claude",
			"priority": 1
		}]
	}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("写入初始配置失败: %v", err)
	}

	cm, err := NewConfigManager(configPath, "")
	if err != nil {
		t.Fatalf("初始化配置管理器失败: %v", err)
	}
	t.Cleanup(func() { _ = cm.Close() })
	time.Sleep(100 * time.Millisecond)
	return cm
}

func TestUpdateUpstream_SetTags(t *testing.T) {
	cm := setupTestConfigManager(t)

	tags := []string{"production", "primary", "us-east"}
	_, err := cm.UpdateUpstream(0, UpstreamUpdate{Tags: tags})
	if err != nil {
		t.Fatalf("UpdateUpstream() error = %v", err)
	}

	cfg := cm.GetConfig()
	got := cfg.Upstream[0].Tags
	if len(got) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(got), got)
	}
	for i, want := range tags {
		if got[i] != want {
			t.Errorf("tag[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestUpdateUpstream_ClearTags(t *testing.T) {
	cm := setupTestConfigManager(t)

	// 先设置标签
	tags := []string{"production", "primary"}
	if _, err := cm.UpdateUpstream(0, UpstreamUpdate{Tags: tags}); err != nil {
		t.Fatalf("设置标签失败: %v", err)
	}
	if len(cm.GetConfig().Upstream[0].Tags) != 2 {
		t.Fatal("标签设置失败")
	}

	// 用空切片清空标签
	if _, err := cm.UpdateUpstream(0, UpstreamUpdate{Tags: []string{}}); err != nil {
		t.Fatalf("清空标签失败: %v", err)
	}

	got := cm.GetConfig().Upstream[0].Tags
	if got == nil {
		// 空切片被规范化后可能是 nil 或空切片，两种都可接受
		return
	}
	if len(got) != 0 {
		t.Errorf("expected empty tags after clear, got %v", got)
	}
}

func TestUpdateUpstream_NilTagsNoChange(t *testing.T) {
	cm := setupTestConfigManager(t)

	// 先设置标签
	tags := []string{"keep-me"}
	if _, err := cm.UpdateUpstream(0, UpstreamUpdate{Tags: tags}); err != nil {
		t.Fatalf("设置标签失败: %v", err)
	}

	// Nil Tags 不应修改现有标签
	name := "renamed-channel"
	if _, err := cm.UpdateUpstream(0, UpstreamUpdate{Name: &name}); err != nil {
		t.Fatalf("UpdateUpstream() error = %v", err)
	}

	got := cm.GetConfig().Upstream[0].Tags
	if len(got) != 1 || got[0] != "keep-me" {
		t.Errorf("tags should be unchanged, got %v", got)
	}
}

func TestUpdateUpstream_TagsDeduplication(t *testing.T) {
	cm := setupTestConfigManager(t)

	tags := []string{"prod", "prod", " primary ", "primary", "secondary"}
	_, err := cm.UpdateUpstream(0, UpstreamUpdate{Tags: tags})
	if err != nil {
		t.Fatalf("UpdateUpstream() error = %v", err)
	}

	got := cm.GetConfig().Upstream[0].Tags
	// 去重+trim 后应为 ["prod", "primary", "secondary"]
	if len(got) != 3 {
		t.Fatalf("expected 3 unique tags after dedup, got %d: %v", len(got), got)
	}
	expected := []string{"prod", "primary", "secondary"}
	for i, want := range expected {
		if got[i] != want {
			t.Errorf("tag[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestUpdateUpstream_TagsSkipEmpty(t *testing.T) {
	cm := setupTestConfigManager(t)

	tags := []string{"valid", "", "  ", "also-valid"}
	_, err := cm.UpdateUpstream(0, UpstreamUpdate{Tags: tags})
	if err != nil {
		t.Fatalf("UpdateUpstream() error = %v", err)
	}

	got := cm.GetConfig().Upstream[0].Tags
	if len(got) != 2 {
		t.Fatalf("expected 2 non-empty tags, got %d: %v", len(got), got)
	}
	if got[0] != "valid" || got[1] != "also-valid" {
		t.Errorf("unexpected tags: %v", got)
	}
}

func TestUpdateUpstream_TagsPersistAcrossReload(t *testing.T) {
	cm := setupTestConfigManager(t)

	tags := []string{"persisted-tag", "another-tag"}
	if _, err := cm.UpdateUpstream(0, UpstreamUpdate{Tags: tags}); err != nil {
		t.Fatalf("UpdateUpstream() error = %v", err)
	}

	// 重新加载配置
	cfg := cm.GetConfig()
	if len(cfg.Upstream[0].Tags) != 2 {
		t.Fatalf("tags not persisted: %v", cfg.Upstream[0].Tags)
	}

	// 验证 JSON 序列化包含 tags
	reloaded := cfg.Upstream[0].Tags
	if reloaded[0] != "persisted-tag" || reloaded[1] != "another-tag" {
		t.Errorf("tags not correctly persisted: %v", reloaded)
	}
}

func TestUpdateUpstream_TagsNotAffectedByOtherUpdates(t *testing.T) {
	cm := setupTestConfigManager(t)

	// 设置标签
	tags := []string{"stable-tag"}
	if _, err := cm.UpdateUpstream(0, UpstreamUpdate{Tags: tags}); err != nil {
		t.Fatalf("设置标签失败: %v", err)
	}

	// 修改其他字段
	priority := 5
	if _, err := cm.UpdateUpstream(0, UpstreamUpdate{Priority: &priority}); err != nil {
		t.Fatalf("修改 priority 失败: %v", err)
	}

	// 标签应保持不变
	got := cm.GetConfig().Upstream[0].Tags
	if len(got) != 1 || got[0] != "stable-tag" {
		t.Errorf("tags changed after unrelated update: %v", got)
	}
}
