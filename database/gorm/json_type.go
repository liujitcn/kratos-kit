package gorm

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/liujitcn/kratos-kit/database/gorm/driver"
)

// applyDialectJSONTypes 把模型上的中性 json 类型标签按当前数据源方言落地为原生 JSON 列类型。
//
// 生成器产出的 type:json 是跨库中立拼写，各库建表前需落地为真实类型（postgres 落地 jsonb、
// 达梦等无原生 JSON 类型的数据库按映射退化）；改写发生在模型 schema 缓存上，AutoMigrate
// 后续的建表与已有列比对读取的是同一份解析结果，因此两条 DDL 路径同时生效。
func applyDialectJSONTypes(db *gorm.DB, models []interface{}) error {
	if db == nil || db.Dialector == nil || len(models) == 0 {
		return nil
	}
	jsonType := driver.JSONColumnType(db.Dialector.Name())
	for _, model := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(model); err != nil {
			return fmt.Errorf("parse model to translate json type: %w", err)
		}
		for _, field := range stmt.Schema.Fields {
			if strings.EqualFold(field.TagSettings["TYPE"], "json") {
				field.TagSettings["TYPE"] = jsonType
			}
		}
	}
	return nil
}
