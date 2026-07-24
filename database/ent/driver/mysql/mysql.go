package mysql

import (
	"database/sql"

	"entgo.io/ent/dialect"
	_ "github.com/go-sql-driver/mysql"
	"github.com/liujitcn/kratos-kit/database/ent/driver"
)

func init() {
	driver.Opens["mysql"] = open
	driver.Opens["doris"] = open
}

// open 打开 MySQL 协议兼容数据库连接并返回 Ent dialect 名称。
func open(source string) (*sql.DB, string, error) {
	db, err := sql.Open("mysql", source)
	if err != nil {
		return nil, "", err
	}
	return db, dialect.MySQL, nil
}
