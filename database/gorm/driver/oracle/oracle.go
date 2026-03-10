package oracle

import (
	oracle "github.com/godoes/gorm-oracle"
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
)

func init() {
	driver.Opens["oracle"] = oracle.Open
}
