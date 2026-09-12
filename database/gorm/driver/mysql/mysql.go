package mysql

import (
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// dorisDialector 将 MySQL 协议能力适配为 Doris 方言。
type dorisDialector struct {
	base gorm.Dialector
}

// Name 返回 Doris 方言名称。
func (dorisDialector) Name() string {
	return "doris"
}

// Initialize 使用 MySQL 驱动初始化 Doris 连接。
func (d dorisDialector) Initialize(db *gorm.DB) error {
	return d.base.Initialize(db)
}

// Migrator 返回 Doris 专用迁移器。
func (d dorisDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return newDorisMigrator(db, d)
}

// DataTypeOf 返回字段对应的 MySQL 协议数据类型。
func (d dorisDialector) DataTypeOf(field *schema.Field) string {
	return d.base.DataTypeOf(field)
}

// DefaultValueOf 返回字段默认值表达式。
func (d dorisDialector) DefaultValueOf(field *schema.Field) clause.Expression {
	return d.base.DefaultValueOf(field)
}

// BindVarTo 写入 MySQL 协议绑定变量。
func (d dorisDialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, value interface{}) {
	d.base.BindVarTo(writer, stmt, value)
}

// QuoteTo 使用 MySQL 协议规则引用标识符。
func (d dorisDialector) QuoteTo(writer clause.Writer, value string) {
	d.base.QuoteTo(writer, value)
}

// Explain 展开 SQL 绑定变量。
func (d dorisDialector) Explain(sql string, vars ...interface{}) string {
	return d.base.Explain(sql, vars...)
}

// openDoris 创建复用 MySQL 协议的 Doris 方言。
func openDoris(dsn string) gorm.Dialector {
	return dorisDialector{base: mysql.Open(dsn)}
}

func init() {
	driver.Opens["mysql"] = mysql.Open
	driver.Opens["doris"] = openDoris
}
