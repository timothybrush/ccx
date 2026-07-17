package autopilot

import (
	"database/sql"
	"fmt"
	"log"
)

// ── Autopilot SQLite schema 版本化（设计 §12.2 P1.5）──
//
// 7 个 autopilot store（ProfileStore/ProfileChangelogStore/LocalRuntimeStore/
// SubscriptionStore/TraceStore/ManualIntentStore/AdvisorDecisionStore）全部
// 复用同一个 *sql.DB 连接（由 ProfileStore 打开，其余 store 通过 store.DB() 接力）。
// 因此 schema 版本是这个 DB 文件级别的单一版本，不是每张表各自一个版本。
//
// 参考 internal/metrics/sqlite_store.go 的 PRAGMA user_version 迁移模式。

// autopilotSchemaVersion 当前代码期望的 schema 版本。
// v1 = 现有 7 张表的建表语句本身（首次引入版本化时的基线，无需 ALTER）。
// 后续新增/变更表结构时，在 ensureSchemaVersion 里追加 "if version < N { ... }" 迁移块，
// 并将本常量递增。
const autopilotSchemaVersion = 5

// ensureSchemaVersion 在任何 CREATE TABLE 之前执行一次版本检查/迁移。
// 必须在 ProfileStore 打开 DB 后、调用 initProfileStoreSchema 之前调用——
// 它是唯一的 DB 打开入口，其余 6 个 store 复用同一连接，不需要重复调用。
//
//   - version == 0（全新库）：直接写 PRAGMA user_version = autopilotSchemaVersion，
//     交给后续各 initXxxSchema 的 CREATE TABLE IF NOT EXISTS 建表。
//   - version == autopilotSchemaVersion：无需操作。
//   - 0 < version < autopilotSchemaVersion：未来加 v2+ 时，在此插入门控迁移块。
//   - version > autopilotSchemaVersion：库版本比当前代码新（例如降级部署），
//     返回 error，不做任何写操作——调用方（NewProfileStore）会把这个 error
//     原样传播到 main.go 的现有 fail-open 分支（禁用 Autopilot，保留现有调度）。
func ensureSchemaVersion(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("[Autopilot-SchemaMigration] db 为 nil")
	}

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("[Autopilot-SchemaMigration] 读取 PRAGMA user_version 失败: %w", err)
	}

	if version > autopilotSchemaVersion {
		return fmt.Errorf(
			"[Autopilot-SchemaMigration] 数据库 schema 版本 (%d) 高于当前代码支持的版本 (%d)，"+
				"可能是降级部署或数据库损坏，拒绝启动 Autopilot 以避免数据不一致",
			version, autopilotSchemaVersion,
		)
	}

	if version == autopilotSchemaVersion {
		// 幂等自愈：即便 user_version 已是当前版本，仍校验关键列是否真实存在。
		// 历史上曾出现「user_version=2 但 advisor_decisions 缺 reason 列」的漂移库
		//（开发期版本常量与建表/迁移未同步导致），此处兜底补列，避免 loadAll 查询失败
		// 阻断整个 Autopilot 初始化。列已存在时为空操作。
		return ensureCurrentSchemaColumns(db)
	}

	// v1 -> v2: advisor_decisions 新增 reason 列（SLO regression 自动回滚记录触发原因）
	// 仅对已有 v1 schema 的数据库执行 ALTER TABLE；全新库（version=0）的 CREATE TABLE 已包含新列。
	if version > 0 && version < 2 {
		migrations := []string{
			"ALTER TABLE autopilot_advisor_decisions ADD COLUMN reason TEXT NOT NULL DEFAULT ''",
			"PRAGMA user_version = 2",
		}
		for _, stmt := range migrations {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("[Autopilot-SchemaMigration] v1->v2 迁移失败: %w", err)
			}
		}
		log.Printf("[Autopilot-SchemaMigration] schema 升级: v1 -> v2")
		version = 2
	}

	// v2 -> v3: endpoint 画像增加账号身份，用于 account -> channel -> key -> baseURL 聚合查询。
	if version > 0 && version < 3 {
		if exists, err := tableExists(db, "autopilot_endpoint_profiles"); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] 检查 endpoint profile 表失败: %w", err)
		} else if exists {
			has, err := columnExists(db, "autopilot_endpoint_profiles", "account_uid")
			if err != nil {
				return fmt.Errorf("[Autopilot-SchemaMigration] 检查 account_uid 失败: %w", err)
			}
			if !has {
				_, err = db.Exec("ALTER TABLE autopilot_endpoint_profiles ADD COLUMN account_uid TEXT NOT NULL DEFAULT ''")
			}
			if err != nil {
				return fmt.Errorf("[Autopilot-SchemaMigration] v2->v3 迁移失败: %w", err)
			}
		}
		if _, err := db.Exec("PRAGMA user_version = 3"); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] 写入 v3 版本失败: %w", err)
		}
		log.Printf("[Autopilot-SchemaMigration] schema 升级: v2 -> v3")
		version = 3
	}

	// v3 -> v4: endpoint 画像增加稳定凭证身份。
	if version > 0 && version < 4 {
		if exists, err := tableExists(db, "autopilot_endpoint_profiles"); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] 检查 endpoint profile 表失败: %w", err)
		} else if exists {
			has, err := columnExists(db, "autopilot_endpoint_profiles", "credential_uid")
			if err != nil {
				return fmt.Errorf("[Autopilot-SchemaMigration] 检查 credential_uid 失败: %w", err)
			}
			if !has {
				_, err = db.Exec("ALTER TABLE autopilot_endpoint_profiles ADD COLUMN credential_uid TEXT NOT NULL DEFAULT ''")
			}
			if err != nil {
				return fmt.Errorf("[Autopilot-SchemaMigration] v3->v4 迁移失败: %w", err)
			}
		}
		if _, err := db.Exec("PRAGMA user_version = 4"); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] 写入 v4 版本失败: %w", err)
		}
		log.Printf("[Autopilot-SchemaMigration] schema 升级: v3 -> v4")
		version = 4
	}

	// v4 -> v5: 路由 trace 增加请求终态，供无偏窗口统计与 auto 上线闸门使用。
	if version > 0 && version < 5 {
		if err := ensureRoutingTraceOutcomeColumns(db); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] v4->v5 迁移失败: %w", err)
		}
		if err := initRoutingSafetySchema(db); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] v4->v5 建立路由安全表失败: %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 5"); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] 写入 v5 版本失败: %w", err)
		}
		log.Printf("[Autopilot-SchemaMigration] schema 升级: v4 -> v5")
		version = 5
	}

	// version == 0：全新库，直接写入当前基线版本；version 属于 (0, autopilotSchemaVersion) 但
	// 未命中任何迁移块的情况理论上不应出现（说明版本常量与迁移块不同步），同样兜底写回当前版本，
	// 不阻塞启动——迁移块本身的正确性由后续新增迁移时的测试覆盖。
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", autopilotSchemaVersion)); err != nil {
		return fmt.Errorf("[Autopilot-SchemaMigration] 写入 PRAGMA user_version 失败: %w", err)
	}

	if version != 0 {
		log.Printf("[Autopilot-SchemaMigration] schema 版本已从 %d 更新至 %d", version, autopilotSchemaVersion)
	}

	// 全新库/正常升级路径也走一次列自愈，确保 reason 列一定存在（幂等）。
	return ensureCurrentSchemaColumns(db)
}

func ensureCurrentSchemaColumns(db *sql.DB) error {
	if err := ensureAdvisorDecisionColumns(db); err != nil {
		return err
	}
	if err := ensureEndpointProfileColumns(db); err != nil {
		return err
	}
	return ensureRoutingTraceOutcomeColumns(db)
}

func ensureRoutingTraceOutcomeColumns(db *sql.DB) error {
	const table = "autopilot_routing_traces"
	exists, err := tableExists(db, table)
	if err != nil || !exists {
		return err
	}
	wantColumns := map[string]string{
		"outcome_recorded":      "outcome_recorded INTEGER NOT NULL DEFAULT 0",
		"outcome":               "outcome TEXT NOT NULL DEFAULT ''",
		"success":               "success INTEGER NOT NULL DEFAULT 0",
		"channel_fallback":      "channel_fallback INTEGER NOT NULL DEFAULT 0",
		"status_code":           "status_code INTEGER NOT NULL DEFAULT 0",
		"request_duration_ms":   "request_duration_ms INTEGER NOT NULL DEFAULT 0",
		"first_byte_latency_ms": "first_byte_latency_ms INTEGER NOT NULL DEFAULT 0",
		"completed_at":          "completed_at TEXT NOT NULL DEFAULT ''",
	}
	for column, definition := range wantColumns {
		has, err := columnExists(db, table, column)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, definition)); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] 补列 %s.%s 失败: %w", table, column, err)
		}
		log.Printf("[Autopilot-SchemaMigration] 自愈补列: %s.%s", table, column)
	}
	return nil
}

func ensureEndpointProfileColumns(db *sql.DB) error {
	const table = "autopilot_endpoint_profiles"
	exists, err := tableExists(db, table)
	if err != nil || !exists {
		return err
	}
	wantColumns := map[string]string{
		"account_uid":    "account_uid TEXT NOT NULL DEFAULT ''",
		"credential_uid": "credential_uid TEXT NOT NULL DEFAULT ''",
	}
	for column, definition := range wantColumns {
		has, err := columnExists(db, table, column)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, definition)); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] 补列 %s.%s 失败: %w", table, column, err)
		}
		log.Printf("[Autopilot-SchemaMigration] 自愈补列: %s.%s", table, column)
	}
	return nil
}

// ensureAdvisorDecisionColumns 校验 advisor_decisions 表的关键列是否存在，缺失则补齐。
// 用于兜底修复历史 schema 漂移库（user_version 与实际表结构不一致）。
// 表尚不存在时（全新库，CREATE TABLE 还没执行）跳过，交给建表语句。
func ensureAdvisorDecisionColumns(db *sql.DB) error {
	const table = "autopilot_advisor_decisions"

	exists, err := tableExists(db, table)
	if err != nil {
		return fmt.Errorf("[Autopilot-SchemaMigration] 检查表 %s 是否存在失败: %w", table, err)
	}
	if !exists {
		return nil // 全新库，建表语句会包含所有列
	}

	// 期望列 -> 建表定义（与 advisor_decision_store.go 的建表语句保持一致）。
	// 目前仅 reason 曾发生漂移；后续如有新增列同样在此登记，实现幂等自愈。
	wantColumns := map[string]string{
		"reason": "reason TEXT NOT NULL DEFAULT ''",
	}
	for col, def := range wantColumns {
		has, err := columnExists(db, table, col)
		if err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] 检查列 %s.%s 失败: %w", table, col, err)
		}
		if has {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, def)); err != nil {
			return fmt.Errorf("[Autopilot-SchemaMigration] 补列 %s.%s 失败: %w", table, col, err)
		}
		log.Printf("[Autopilot-SchemaMigration] 自愈补列: %s.%s", table, col)
	}
	return nil
}

// tableExists 判断表是否存在。
func tableExists(db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// columnExists 通过 PRAGMA table_info 判断列是否存在。
func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
