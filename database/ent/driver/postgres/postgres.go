package postgres

import (
	"database/sql"

	"entgo.io/ent/dialect"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/liujitcn/kratos-kit/database/ent/driver"
)

func init() {
	driver.Opens["postgres"] = open
	driver.Opens["postgresql"] = open
}

// open 打开 PostgreSQL 数据库连接并返回 Ent dialect 名称。
func open(source string) (*sql.DB, string, error) {
	db, err := sql.Open("pgx", source)
	if err != nil {
		return nil, "", err
	}
	return db, dialect.Postgres, nil
}
