package mysql

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// tidbDialector 将 MySQL 协议能力适配为 TiDB 方言。
type tidbDialector struct {
	base gorm.Dialector
}

// Name 返回 MySQL 方言名：TiDB 完全兼容 MySQL 协议，gorm 层（迁移器、gorm/gen 的类型推断、
// 按方言分支的 SQL 构造）统一按 MySQL 语义处理；tidb 仅作为配置驱动身份，由上层按 Client.Driver() 区分。
func (tidbDialector) Name() string {
	return "mysql"
}

// Initialize 使用 MySQL 驱动初始化 TiDB 连接。
func (d tidbDialector) Initialize(db *gorm.DB) error {
	return d.base.Initialize(db)
}

// Migrator 透传 MySQL 迁移器，TiDB 兼容 MySQL 的表结构自省与 DDL 行为。
func (d tidbDialector) Migrator(db *gorm.DB) gorm.Migrator {
	migrator, ok := d.base.(interface {
		Migrator(*gorm.DB) gorm.Migrator
	})
	if !ok {
		return nil
	}
	return migrator.Migrator(db)
}

// SavePoint 在事务内创建保存点，嵌套事务依赖该能力。
func (d tidbDialector) SavePoint(tx *gorm.DB, name string) error {
	savePointer, ok := d.base.(gorm.SavePointerDialectorInterface)
	if !ok {
		return gorm.ErrUnsupportedDriver
	}
	return savePointer.SavePoint(tx, name)
}

// RollbackTo 回滚事务到指定保存点。
func (d tidbDialector) RollbackTo(tx *gorm.DB, name string) error {
	savePointer, ok := d.base.(gorm.SavePointerDialectorInterface)
	if !ok {
		return gorm.ErrUnsupportedDriver
	}
	return savePointer.RollbackTo(tx, name)
}

// DataTypeOf 返回字段对应的 MySQL 协议数据类型。
func (d tidbDialector) DataTypeOf(field *schema.Field) string {
	return d.base.DataTypeOf(field)
}

// DefaultValueOf 返回字段默认值表达式。
func (d tidbDialector) DefaultValueOf(field *schema.Field) clause.Expression {
	return d.base.DefaultValueOf(field)
}

// BindVarTo 写入 MySQL 协议绑定变量。
func (d tidbDialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, value interface{}) {
	d.base.BindVarTo(writer, stmt, value)
}

// QuoteTo 使用 MySQL 协议规则引用标识符。
func (d tidbDialector) QuoteTo(writer clause.Writer, value string) {
	d.base.QuoteTo(writer, value)
}

// Explain 展开 SQL 绑定变量。
func (d tidbDialector) Explain(sql string, vars ...interface{}) string {
	return d.base.Explain(sql, vars...)
}

// openTiDB 创建复用 MySQL 协议的 TiDB 方言。
func openTiDB(dsn string) gorm.Dialector {
	return tidbDialector{base: mysql.Open(dsn)}
}
