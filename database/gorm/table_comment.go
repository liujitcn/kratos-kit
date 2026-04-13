package gorm

import (
	"reflect"

	"github.com/go-kratos/kratos/v2/log"
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

// execTableCommentSQL 根据方言执行表注释更新。
func execTableCommentSQL(db *gorm.DB, table interface{}, comment string) error {
	switch db.Dialector.Name() {
	case "mysql":
		return db.Exec("ALTER TABLE ? COMMENT = ?", table, comment).Error
	case "postgres":
		return db.Exec("COMMENT ON TABLE ? IS ?", table, gorm.Expr(db.Dialector.Explain("$1", comment))).Error
	default:
		// 其他方言暂未统一支持，记录日志后跳过，避免影响迁移主流程。
		log.Warnf("gorm AutoMigrate 当前方言未实现表注释回填，driver=%s", db.Dialector.Name())
		return nil
	}
}
