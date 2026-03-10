package sqlserver

import (
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
	"gorm.io/driver/sqlserver"
)

func init() {
	driver.Opens["sqlserver"] = sqlserver.Open
}
