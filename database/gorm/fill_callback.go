package gorm

import (
	"context"
	"reflect"
	"slices"
	"time"

	"github.com/liujitcn/kratos-kit/auth"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var auditExcludeTables = []string{
	"base_log",
}

var auditFieldNames = []string{
	"CreatedBy",
	"UpdatedBy",
	"CreatedAt",
	"UpdatedAt",
}

func init() {
	RegisterCallbackCreate(fillCreatedFields)
	RegisterCallbackUpdate(fillUpdatedFields)
}

// fillCreatedFields 在创建时回填审计字段。
func fillCreatedFields(db *gorm.DB) {
	if db == nil || db.Statement == nil {
		return
	}
	if slices.Contains(auditExcludeTables, db.Statement.Table) {
		return
	}

	userID, hasToken := getUserIDFromContext(db.Statement.Context)
	if hasToken {
		setAuditColumn(db, "CreatedBy", userID, true)
		setAuditColumn(db, "UpdatedBy", userID, true)
	}

	now := time.Now()
	setAuditColumn(db, "CreatedAt", now, true)
	setAuditColumn(db, "UpdatedAt", now, true)
}

// fillUpdatedFields 在更新时回填审计字段。
func fillUpdatedFields(db *gorm.DB) {
	if db == nil || db.Statement == nil {
		return
	}
	if slices.Contains(auditExcludeTables, db.Statement.Table) {
		return
	}

	userID, hasToken := getUserIDFromContext(db.Statement.Context)
	if hasToken {
		setAuditColumn(db, "UpdatedBy", userID, false)
	}

	setAuditColumn(db, "UpdatedAt", time.Now(), false)
}

// getUserIDFromContext 从上下文中解析当前用户 ID 和 Token 状态。
func getUserIDFromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}

	userInfo, err := auth.FromContext(ctx)
	if err != nil || userInfo == nil {
		return 0, false
	}

	return userInfo.UserId, true
}

// setAuditColumn 根据填充策略写入当前创建或更新参数中的审计字段。
func setAuditColumn(db *gorm.DB, fieldName string, value interface{}, onlyZero bool) {
	if db == nil || db.Statement == nil || db.Error != nil {
		return
	}
	if db.Statement.Schema == nil {
		metadata, fieldExists, err := getRegisteredAuditField(db, fieldName)
		if err != nil {
			db.AddError(err)
			return
		}
		if fieldExists {
			setMapColumn(db.Statement.Dest, metadata.name, metadata.dbName, value, onlyZero)
		}
		return
	}
	field := db.Statement.Schema.LookUpField(fieldName)
	if field == nil {
		return
	}
	if setMapColumn(db.Statement.Dest, field.Name, field.DBName, value, onlyZero) {
		return
	}
	setStructColumn(db, db.Statement.ReflectValue, field, value, onlyZero)
}

// setMapColumn 按填充策略写入 map 审计字段。
func setMapColumn(dest interface{}, fieldName, columnName string, value interface{}, onlyZero bool) bool {
	switch items := dest.(type) {
	case map[string]interface{}:
		setMapItem(items, fieldName, columnName, value, onlyZero)
	case *map[string]interface{}:
		if items != nil {
			setMapItem(*items, fieldName, columnName, value, onlyZero)
		}
	case []map[string]interface{}:
		for _, item := range items {
			setMapItem(item, fieldName, columnName, value, onlyZero)
		}
	case *[]map[string]interface{}:
		if items != nil {
			for _, item := range *items {
				setMapItem(item, fieldName, columnName, value, onlyZero)
			}
		}
	default:
		return false
	}
	return true
}

// setMapItem 按填充策略处理单条 map 审计字段。
func setMapItem(item map[string]interface{}, fieldName, columnName string, value interface{}, onlyZero bool) {
	if item == nil {
		return
	}
	for _, key := range []string{columnName, fieldName} {
		current, fieldExists := item[key]
		if !fieldExists {
			continue
		}
		if !onlyZero || current == nil || reflect.ValueOf(current).IsZero() {
			item[key] = value
		}
		return
	}
	item[columnName] = value
}

// setStructColumn 按填充策略递归处理结构体或结构体集合中的审计字段。
func setStructColumn(db *gorm.DB, current reflect.Value, field *schema.Field, value interface{}, onlyZero bool) {
	if !current.IsValid() || db.Error != nil {
		return
	}
	for current.Kind() == reflect.Pointer || current.Kind() == reflect.Interface {
		if current.IsNil() {
			return
		}
		current = current.Elem()
	}
	switch current.Kind() {
	case reflect.Struct:
		_, zero := field.ValueOf(db.Statement.Context, current)
		if !onlyZero || zero {
			db.AddError(field.Set(db.Statement.Context, current, value))
		}
	case reflect.Slice, reflect.Array:
		for index := 0; index < current.Len(); index++ {
			setStructColumn(db, current.Index(index), field, value, onlyZero)
		}
	}
}
