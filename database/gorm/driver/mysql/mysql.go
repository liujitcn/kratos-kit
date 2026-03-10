package mysql

import (
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
	"gorm.io/driver/mysql"
)

func init() {
	driver.Opens["mysql"] = mysql.Open
}
