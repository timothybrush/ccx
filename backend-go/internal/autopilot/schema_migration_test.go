package autopilot

import (
	"fmt"
	"testing"
)

func TestEnsureSchemaVersion_NewDatabaseWritesBaseline(t *testing.T) {
	db := newTestDB(t)

	var before int
	if err := db.QueryRow("PRAGMA user_version").Scan(&before); err != nil {
		t.Fatalf("读取初始 user_version 失败: %v", err)
	}
	if before != 0 {
		t.Fatalf("全新内存库初始 user_version 应为 0，got %d", before)
	}

	if err := ensureSchemaVersion(db); err != nil {
		t.Fatalf("ensureSchemaVersion 失败: %v", err)
	}

	var after int
	if err := db.QueryRow("PRAGMA user_version").Scan(&after); err != nil {
		t.Fatalf("读取迁移后 user_version 失败: %v", err)
	}
	if after != autopilotSchemaVersion {
		t.Fatalf("迁移后 user_version = %d, want %d", after, autopilotSchemaVersion)
	}
}

func TestEnsureSchemaVersion_AlreadyCurrentIsNoop(t *testing.T) {
	db := newTestDB(t)

	if err := ensureSchemaVersion(db); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	// 再次调用应该是纯粹的 no-op，不报错
	if err := ensureSchemaVersion(db); err != nil {
		t.Fatalf("重复调用应为 no-op，got error: %v", err)
	}

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("读取 user_version 失败: %v", err)
	}
	if version != autopilotSchemaVersion {
		t.Fatalf("version = %d, want %d", version, autopilotSchemaVersion)
	}
}

func TestEnsureSchemaVersion_FutureVersionFailsClosed(t *testing.T) {
	db := newTestDB(t)

	// 模拟"库版本比当前代码新"（例如降级部署）
	futureVersion := autopilotSchemaVersion + 98
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", futureVersion)); err != nil {
		t.Fatalf("设置 user_version 失败: %v", err)
	}

	err := ensureSchemaVersion(db)
	if err == nil {
		t.Fatal("库版本高于代码支持版本时，ensureSchemaVersion 应返回 error")
	}

	// 版本号不应被回退/篡改——fail-closed 时不做任何写操作
	var version int
	if scanErr := db.QueryRow("PRAGMA user_version").Scan(&version); scanErr != nil {
		t.Fatalf("读取 user_version 失败: %v", scanErr)
	}
	if version != futureVersion {
		t.Fatalf("拒绝启动时不应修改 user_version，got %d, want %d", version, futureVersion)
	}
}

func TestNewProfileStoreWithDB_PropagatesSchemaVersionError(t *testing.T) {
	db := newTestDB(t)

	futureVersion := autopilotSchemaVersion + 1
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", futureVersion)); err != nil {
		t.Fatalf("设置 user_version 失败: %v", err)
	}

	store, err := NewProfileStoreWithDB(db)
	if err == nil {
		t.Fatal("schema 版本不兼容时 NewProfileStoreWithDB 应返回 error，而不是 panic 或静默成功")
	}
	if store != nil {
		t.Fatal("失败时不应返回非 nil 的 store")
	}
}

func TestEnsureSchemaVersion_V1ToV2Migration(t *testing.T) {
	db := newTestDB(t)

	// 模拟 v1 数据库：先建表（不含 reason 列），再设版本为 1
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS autopilot_advisor_decisions (
    decision_uid        TEXT PRIMARY KEY,
    request_uid         TEXT,
    advisor_uid         TEXT    NOT NULL,
    advisor_origin_tier TEXT    NOT NULL,
    mode                TEXT    NOT NULL,
    task_class          TEXT    NOT NULL,
    prompt_hash         TEXT,
    input_token_bucket  TEXT    NOT NULL DEFAULT '',
    hint_json           TEXT    NOT NULL,
    default_plan_hash   TEXT    NOT NULL DEFAULT '',
    applied             INTEGER NOT NULL DEFAULT 0,
    outcome             TEXT    NOT NULL DEFAULT '',
    misroute_severity   TEXT    NOT NULL DEFAULT '',
    latency_ms          INTEGER NOT NULL DEFAULT 0,
    estimated_advisor_cost REAL NOT NULL DEFAULT 0,
    created_at          TEXT    NOT NULL
)`); err != nil {
		t.Fatalf("建 v1 表失败: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("设置 v1 版本失败: %v", err)
	}

	// 执行迁移
	if err := ensureSchemaVersion(db); err != nil {
		t.Fatalf("v1->v2 迁移失败: %v", err)
	}

	// 验证 reason 列已添加（插入含 reason 的记录应成功，否则 no such column 会报错）
	_, err := db.Exec(`INSERT INTO autopilot_advisor_decisions
		(decision_uid, advisor_uid, advisor_origin_tier, mode, task_class,
		 input_token_bucket, hint_json, default_plan_hash, applied,
		 outcome, misroute_severity, latency_ms, estimated_advisor_cost,
		 reason, created_at)
		VALUES ('test-v2', 'ch-1', 'first', 'shadow', 'worker',
		 '', '{}', '', 0, '', '', 0, 0, 'slo_regression', '2024-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("v1->v2 迁移后插入含 reason 的记录失败（列可能未添加）: %v", err)
	}

	// 验证版本已升级
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("读取 user_version 失败: %v", err)
	}
	if version != 2 {
		t.Fatalf("v1->v2 迁移后 user_version = %d, want 2", version)
	}
}
