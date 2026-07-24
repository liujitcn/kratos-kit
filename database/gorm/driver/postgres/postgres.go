package postgres

import (
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
	"gorm.io/driver/postgres"
)

func init() {
	driver.Opens["postgres"] = postgres.Open
}
