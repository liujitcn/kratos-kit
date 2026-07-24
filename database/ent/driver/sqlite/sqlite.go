package sqlite

import (
	"database/sql"

	"entgo.io/ent/dialect"
	"github.com/liujitcn/kratos-kit/database/ent/driver"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	driver.Opens["sqlite"] = open
	driver.Opens[dialect.SQLite] = open
}

// open 打开 SQLite 数据库连接并返回 Ent dialect 名称。
func open(source string) (*sql.DB, string, error) {
	db, err := sql.Open("sqlite3", source)
	if err != nil {
		return nil, "", err
	}
	return db, dialect.SQLite, nil
}
