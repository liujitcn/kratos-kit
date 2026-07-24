package bigquery

import (
	"github.com/liujitcn/kratos-kit/database/gorm/driver"
	"gorm.io/driver/bigquery"
)

func init() {
	driver.Opens["bigquery"] = bigquery.Open
}
