package driver

import "database/sql"

// OpenFunc 定义 Ent SQL 驱动打开函数。
type OpenFunc func(source string) (*sql.DB, string, error)

// Opens 保存通过 init 注册的 Ent SQL 驱动打开函数。
var Opens = map[string]OpenFunc{}
