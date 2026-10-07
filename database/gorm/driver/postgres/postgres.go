package postgres

import (
	"database/sql"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/migrator"

	"github.com/liujitcn/kratos-kit/database/gorm/driver"
)

func init() {
	driver.Opens["postgres"] = openOrderedPostgres
}

// orderedIndexDialector 在官方 PG 方言之上有序返回索引列信息。
type orderedIndexDialector struct {
	gorm.Dialector
}

// Migrator 返回带有序索引内省的迁移器包装。
func (d orderedIndexDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return orderedIndexMigrator{Migrator: d.Dialector.Migrator(db), db: db}
}

// SavePoint 创建事务保存点。gorm.Dialector 接口未声明保存点方法，包装器必须显式透传，
// 否则嵌套事务（如 CreateInBatches 超过单批上限）断言 SavePointerDialectorInterface 失败，
// 会报 unsupported driver。
func (d orderedIndexDialector) SavePoint(db *gorm.DB, name string) error {
	if savePointer, ok := d.Dialector.(gorm.SavePointerDialectorInterface); ok {
		return savePointer.SavePoint(db, name)
	}
	return gorm.ErrUnsupportedDriver
}

// RollbackTo 回滚到指定事务保存点，透传逻辑与 SavePoint 一致。
func (d orderedIndexDialector) RollbackTo(db *gorm.DB, name string) error {
	if savePointer, ok := d.Dialector.(gorm.SavePointerDialectorInterface); ok {
		return savePointer.RollbackTo(db, name)
	}
	return gorm.ErrUnsupportedDriver
}

// openOrderedPostgres 创建按索引定义序内省索引列的 PG 方言。
func openOrderedPostgres(dsn string) gorm.Dialector {
	return orderedIndexDialector{Dialector: postgres.Open(dsn)}
}

// orderedIndexMigrator 修正官方索引内省的列顺序。
//
// 官方 indexSql 用 a.attnum = ANY(i.indkey) 关联列，返回的是表列序而不是索引定义序，
// 代码生成器据此写出的索引 priority 与建表定义不符；本包装改用 unnest WITH ORDINALITY
// 按定义序输出，保证不同源库生成的索引标签一致。
type orderedIndexMigrator struct {
	gorm.Migrator
	db *gorm.DB
}

// orderedIndexRow 索引列内省结果行。
type orderedIndexRow struct {
	TableName  string `gorm:"column:table_name"`
	IndexName  string `gorm:"column:index_name"`
	NonUnique  bool   `gorm:"column:non_unique"`
	Primary    bool   `gorm:"column:primary"`
	ColumnName string `gorm:"column:column_name"`
}

// orderedIndexSQL 与官方 indexSql 语义一致，但按 indkey 数组定义序输出索引列。
const orderedIndexSQL = `
SELECT
	ct.relname AS table_name,
	ci.relname AS index_name,
	i.indisunique AS non_unique,
	i.indisprimary AS primary,
	a.attname AS column_name
FROM
	pg_index i
	LEFT JOIN pg_class ct ON ct.oid = i.indrelid
	LEFT JOIN pg_class ci ON ci.oid = i.indexrelid
	CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS u(attnum, ord)
	LEFT JOIN pg_attribute a ON a.attrelid = ct.oid AND a.attnum = u.attnum
	LEFT JOIN pg_constraint con ON con.conindid = i.indexrelid
WHERE
	con.oid IS NULL
	AND ct.relkind = 'r'
	AND ct.relname = ?
ORDER BY ci.relname, u.ord`

// GetIndexes 按索引定义序返回表索引信息。
func (m orderedIndexMigrator) GetIndexes(value interface{}) ([]gorm.Index, error) {
	indexes := make([]gorm.Index, 0)
	err := m.runWithValue(value, func(stmt *gorm.Statement) error {
		rows := make([]orderedIndexRow, 0)
		if err := m.db.Raw(orderedIndexSQL, stmt.Table).Scan(&rows).Error; err != nil {
			return err
		}
		// 查询已按 index_name + 定义序排序，顺序分组即得每个索引的定义序列清单。
		columns := make(map[string][]string)
		uniques := make(map[string]bool)
		tableNames := make(map[string]string)
		names := make([]string, 0)
		for _, row := range rows {
			if _, ok := columns[row.IndexName]; !ok {
				names = append(names, row.IndexName)
			}
			columns[row.IndexName] = append(columns[row.IndexName], row.ColumnName)
			uniques[row.IndexName] = row.NonUnique
			tableNames[row.IndexName] = row.TableName
		}
		for _, name := range names {
			indexes = append(indexes, &migrator.Index{
				TableName:       tableNames[name],
				NameValue:       name,
				ColumnList:      columns[name],
				PrimaryKeyValue: sql.NullBool{Valid: true},
				UniqueValue:     sql.NullBool{Bool: uniques[name], Valid: true},
			})
		}
		return nil
	})
	return indexes, err
}

// runWithValue 与 gorm 迁移器同名方法语义一致：字符串入参视为表名，模型入参先解析。
func (m orderedIndexMigrator) runWithValue(value interface{}, fc func(*gorm.Statement) error) error {
	stmt := &gorm.Statement{DB: m.db}
	if m.db.Statement != nil {
		stmt.Table = m.db.Statement.Table
		stmt.TableExpr = m.db.Statement.TableExpr
	}
	if table, ok := value.(string); ok {
		stmt.Table = table
	} else if err := stmt.ParseWithSpecialTableName(value, stmt.Table); err != nil {
		return err
	}
	return fc(stmt)
}
