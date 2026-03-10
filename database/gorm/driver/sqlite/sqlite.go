package sqlite

import (
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
	"gorm.io/driver/sqlite"
)

func init() {
	driver.Opens["sqlite"] = sqlite.Open
}
