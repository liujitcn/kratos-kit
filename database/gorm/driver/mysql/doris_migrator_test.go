package mysql

import (
	"context"
	"strings"
	"testing"
	"time"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type dorisMigrationModel struct {
	TenantID  int64  `gorm:"primaryKey;comment:租户编号"`
	ProjectID int64  `gorm:"primaryKey"`
	Name      string `gorm:"size:64;not null;comment:名称"`
	Content   string
}

type dorisNoPrimaryKeyModel struct {
	Code string `gorm:"size:32;not null"`
	Name string `gorm:"size:64"`
}

type sqlRecorder struct {
	logger.Interface
	sql []string
}

// Trace 记录 DryRun 生成的 SQL。
func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	r.sql = append(r.sql, sql)
}

// TestDorisCreateTableSQL 验证 Doris 建表 SQL 的 Key、分桶、类型和注释语法。
func TestDorisCreateTableSQL(t *testing.T) {
	recorder := &sqlRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(dorisDialector{base: gormmysql.New(gormmysql.Config{DSN: "root@tcp(127.0.0.1:9030)/test", SkipInitializeWithVersion: true})}, &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               recorder,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Set(dorisReplicationSetting, 3).Migrator().CreateTable(&dorisMigrationModel{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recorder.sql) != 1 {
		t.Fatalf("期望生成 1 条 SQL，实际为 %d", len(recorder.sql))
	}

	sql := recorder.sql[0]
	assertSQLContains(t, sql,
		"CREATE TABLE `doris_migration_models`",
		"`tenant_id` bigint COMMENT '租户编号'",
		"`name` varchar(64) NOT NULL COMMENT '名称'",
		"`content` STRING",
		"ENGINE=OLAP UNIQUE KEY (`tenant_id`,`project_id`)",
		"DISTRIBUTED BY HASH (`tenant_id`,`project_id`) BUCKETS 8",
		`PROPERTIES ("replication_num" = "3")`,
	)
	if strings.Contains(strings.ToUpper(sql), "AUTO_INCREMENT") {
		t.Fatalf("Doris 建表 SQL 不应包含 AUTO_INCREMENT: %s", sql)
	}
}

// TestDorisCreateTableWithoutPrimaryKey 验证无主键模型使用首个字段作为 Key 和分桶字段。
func TestDorisCreateTableWithoutPrimaryKey(t *testing.T) {
	recorder := &sqlRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(dorisDialector{base: gormmysql.New(gormmysql.Config{DSN: "root@tcp(127.0.0.1:9030)/test", SkipInitializeWithVersion: true})}, &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               recorder,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Migrator().CreateTable(&dorisNoPrimaryKeyModel{})
	if err != nil {
		t.Fatal(err)
	}
	assertSQLContains(t, recorder.sql[0],
		"UNIQUE KEY (`code`)",
		"DISTRIBUTED BY HASH (`code`)",
	)
}

// assertSQLContains 断言 SQL 包含全部指定片段。
func assertSQLContains(t *testing.T, sql string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(sql, fragment) {
			t.Errorf("SQL 缺少片段 %q: %s", fragment, sql)
		}
	}
}
