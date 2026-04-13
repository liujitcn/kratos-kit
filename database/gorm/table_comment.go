package gorm

import (
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TableCommenter 定义数据库表注释提供接口。
type TableCommenter interface {
	// TableComment 返回数据库表注释。
	TableComment() string
}

// applyRegisteredTableComments 为已注册模型补充表注释。
func applyRegisteredTableComments(db *gorm.DB, models ...interface{}) error {
	var err error
	for _, model := range models {
		err = applyTableComment(db, model)
		if err != nil {
			return err
		}
	}
	return nil
}

// applyTableComment 为单个模型补充表注释。
func applyTableComment(db *gorm.DB, model interface{}) error {
	var (
		comment string
		ok      bool
		table   interface{}
		err     error
	)

	if db == nil || model == nil {
		return nil
	}

	comment, ok = resolveTableComment(model)
	if !ok || comment == "" {
		return nil
	}

	table, err = resolveModelTable(db, model)
	if err != nil {
		return err
	}

	err = execTableCommentSQL(db, table, comment)
	if err != nil {
		return err
	}
	return nil
}

// resolveTableComment 解析模型声明的表注释。
func resolveTableComment(model interface{}) (string, bool) {
	commenter, ok := model.(TableCommenter)
	if ok {
		return commenter.TableComment(), true
	}

	modelValue := reflect.ValueOf(model)
	if !modelValue.IsValid() {
		return "", false
	}

	// 兼容调用方注册结构体值、而注释方法定义在指针接收者上的场景。
	if modelValue.Kind() != reflect.Pointer {
		modelPtrValue := reflect.New(modelValue.Type())
		modelPtrValue.Elem().Set(modelValue)
		commenter, ok = modelPtrValue.Interface().(TableCommenter)
		if ok {
			return commenter.TableComment(), true
		}
	}

	return "", false
}

// resolveModelTable 解析模型最终映射的表名表达式。
func resolveModelTable(db *gorm.DB, model interface{}) (interface{}, error) {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return nil, err
	}

	// schema.table 这种场景会由 TableExpr 保留完整限定名。
	if stmt.TableExpr != nil {
		return *stmt.TableExpr, nil
	}
	return clause.Table{Name: stmt.Table}, nil
}

// execTableCommentSQL 执行表注释更新。
func execTableCommentSQL(db *gorm.DB, table interface{}, comment string) error {
	sql, err := buildTableCommentSQL(db, table, comment)
	if err != nil {
		return err
	}

	return db.Exec(sql).Error
}

// buildTableCommentSQL 使用 GORM 官方 Statement 能力构造表注释 SQL。
func buildTableCommentSQL(db *gorm.DB, table interface{}, comment string) (string, error) {
	// 使用 GORM 的引用和参数展开能力，避免手写标识符与字符串转义。
	tx := db.Session(&gorm.Session{NewDB: true, DryRun: true})
	stmt := &gorm.Statement{DB: tx}

	//noinspection SqlNoDataSourceInspection
	_, err := stmt.WriteString("ALTER TABLE ")
	if err != nil {
		return "", err
	}

	stmt.WriteQuoted(table)

	_, err = stmt.WriteString(" COMMENT = ")
	if err != nil {
		return "", err
	}

	stmt.AddVar(&stmt.SQL, comment)

	if stmt.DB.Error != nil {
		return "", stmt.DB.Error
	}

	return stmt.DB.Dialector.Explain(stmt.SQL.String(), stmt.Vars...), nil
}
