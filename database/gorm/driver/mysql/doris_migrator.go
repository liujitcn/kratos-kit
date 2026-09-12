package mysql

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
)

const (
	dorisReplicationSetting = "gorm:doris_replication_num"
	dorisDefaultBuckets     = 8
)

// dorisMigrator 实现 Doris 可安全自动执行的迁移能力。
type dorisMigrator struct {
	migrator.Migrator
	Dialector gorm.Dialector
}

// newDorisMigrator 创建 Doris 专用迁移器。
func newDorisMigrator(db *gorm.DB, dialector gorm.Dialector) gorm.Migrator {
	return dorisMigrator{
		Migrator:  migrator.Migrator{Config: migrator.Config{DB: db, Dialector: dialector}},
		Dialector: dialector,
	}
}

// MigrateColumn 跳过已有字段的自动变更，避免生成 Doris 不兼容的 MySQL DDL。
func (dorisMigrator) MigrateColumn(interface{}, *schema.Field, gorm.ColumnType) error {
	return nil
}

// AlterColumn 跳过已有字段的自动变更，复杂字段调整应通过版本化 SQL 完成。
func (dorisMigrator) AlterColumn(interface{}, string) error {
	return nil
}

// FullDataTypeOf 返回 Doris 兼容的完整字段类型。
func (m dorisMigrator) FullDataTypeOf(field *schema.Field) clause.Expr {
	expr := m.Migrator.FullDataTypeOf(field)
	expr.SQL = strings.ReplaceAll(expr.SQL, "longtext", "STRING")
	expr.SQL = strings.ReplaceAll(expr.SQL, "LONGTEXT", "STRING")
	expr.SQL = strings.ReplaceAll(expr.SQL, " AUTO_INCREMENT", "")
	expr.SQL = strings.ReplaceAll(expr.SQL, " auto_increment", "")
	if value, ok := field.TagSettings["COMMENT"]; ok {
		expr.SQL += " COMMENT " + m.Dialector.Explain("?", value)
	}
	return expr
}

// CreateTable 使用 Doris OLAP 表语法创建模型表。
func (m dorisMigrator) CreateTable(values ...interface{}) error {
	var err error
	for _, value := range m.ReorderModels(values, false) {
		err = m.createTable(value)
		if err != nil {
			return err
		}
	}
	return nil
}

// createTable 创建单个 Doris 模型表。
func (m dorisMigrator) createTable(value interface{}) error {
	return m.RunWithValue(value, func(stmt *gorm.Statement) error {
		if stmt.Schema == nil {
			return errors.New("failed to get schema")
		}

		createSQL := "CREATE TABLE ? ("
		args := []interface{}{m.CurrentTable(stmt)}
		for _, dbName := range stmt.Schema.DBNames {
			field := stmt.Schema.FieldsByDBName[dbName]
			if field.IgnoreMigration {
				continue
			}
			createSQL += "? ?,"
			args = append(args, clause.Column{Name: dbName}, m.FullDataTypeOf(field))
		}
		createSQL = strings.TrimSuffix(createSQL, ",") + ") ENGINE=OLAP"

		keyFields := stmt.Schema.PrimaryFields
		if len(keyFields) == 0 {
			keyFields = firstMigratableFields(stmt.Schema)
		}
		if len(keyFields) == 0 {
			return errors.New("Doris 迁移模型至少需要一个可迁移字段")
		}

		createSQL += " UNIQUE KEY ("
		for index, field := range keyFields {
			if index > 0 {
				createSQL += ","
			}
			createSQL += "?"
			args = append(args, clause.Column{Name: field.DBName})
		}
		createSQL += ") DISTRIBUTED BY HASH ("
		for index, field := range keyFields {
			if index > 0 {
				createSQL += ","
			}
			createSQL += "?"
			args = append(args, clause.Column{Name: field.DBName})
		}
		createSQL += fmt.Sprintf(") BUCKETS %d", dorisDefaultBuckets)

		replication := interface{}(1)
		if value, ok := m.DB.Get(dorisReplicationSetting); ok {
			replication = value
		}
		createSQL += fmt.Sprintf(` PROPERTIES ("replication_num" = "%v")`, replication)
		if tableOption, ok := m.DB.Get("gorm:table_options"); ok {
			createSQL += " " + strings.TrimSpace(fmt.Sprint(tableOption))
		}
		return m.DB.Exec(createSQL, args...).Error
	})
}

// firstMigratableFields 返回无主键模型用于 Doris Key 和分桶的首个字段。
func firstMigratableFields(modelSchema *schema.Schema) []*schema.Field {
	for _, dbName := range modelSchema.DBNames {
		field := modelSchema.FieldsByDBName[dbName]
		if !field.IgnoreMigration {
			return []*schema.Field{field}
		}
	}
	return nil
}
