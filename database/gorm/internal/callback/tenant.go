package callback

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/liujitcn/kratos-kit/auth"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

const (
	tenantColumnName      = "tenant_id"
	tenantStructFieldName = "TenantID"
	// DefaultTenantCode 表示拥有跨租户访问能力的默认租户编码。
	DefaultTenantCode = "0000"
)

var errTenantMismatch = errors.New("tenant id mismatch")

// addTenantWhere 为当前查询、更新和删除语句追加租户条件。
func addTenantWhere(db *gorm.DB) {
	if shouldSkipDataIsolation(db) || db == nil || db.Statement == nil || db.Error != nil {
		return
	}
	if rejectUnsafeRawStatement(db) {
		return
	}
	tables, err := tenantTables(db)
	if err != nil {
		db.AddError(err)
		return
	}
	hasMainTenant := hasTenantField(db, tables)
	hasJoinTenant := hasTenantJoin(db, tables)
	if db.Error != nil {
		return
	}
	if !hasMainTenant && !hasJoinTenant {
		return
	}
	var tenantID int64
	var hasTenantScope bool
	tenantID, hasTenantScope, err = tenantIDForStatement(db)
	if err != nil {
		addDataIsolationError(db, err)
		return
	}
	if !hasTenantScope {
		return
	}
	exprs := make([]clause.Expression, 0, 2)
	if hasMainTenant {
		exprs = append(exprs, clause.Eq{
			Column: clause.Column{Table: clause.CurrentTable, Name: tenantColumnName},
			Value:  tenantID,
		})
	}
	exprs = append(exprs, applyTenantJoinConditions(db, tenantID, tables)...)
	if len(exprs) > 0 {
		db.Statement.AddClause(clause.Where{Exprs: exprs})
	}
}

// fillTenantID 在创建租户表数据时自动填充租户编号。
func fillTenantID(db *gorm.DB) {
	if shouldSkipDataIsolation(db) || db == nil || db.Statement == nil {
		return
	}
	tables, err := tenantTables(db)
	if err != nil {
		db.AddError(err)
		return
	}
	if !hasTenantField(db, tables) {
		return
	}
	var tenantID int64
	var hasTenantScope bool
	tenantID, hasTenantScope, err = tenantIDForStatement(db)
	if err != nil {
		db.AddError(err)
		return
	}
	if !hasTenantScope {
		return
	}
	var tenantField *schema.Field
	if db.Statement.Schema != nil {
		tenantField = db.Statement.Schema.FieldsByDBName[tenantColumnName]
	}
	if setTenantMap(db, tenantField, tenantID) {
		return
	}
	if tenantField == nil {
		db.AddError(fmt.Errorf("tenant field metadata missing for table %s", db.Statement.Table))
		return
	}
	setTenantField(db, db.Statement.ReflectValue, tenantField, tenantID)
}

// tenantIDForStatement 从当前 GORM 语句上下文读取租户编号。
func tenantIDForStatement(db *gorm.DB) (int64, bool, error) {
	if db == nil || db.Statement == nil {
		return 0, false, ErrTenantContextMissing
	}
	return tenantIDFromContext(db.Statement.Context)
}

// tenantIDFromContext 从登录用户信息读取当前租户编号。
func tenantIDFromContext(ctx context.Context) (int64, bool, error) {
	if ctx == nil {
		return 0, false, nil
	}
	authInfo, err := auth.FromContext(ctx)
	if err != nil || authInfo == nil {
		return 0, false, nil
	}
	// 默认租户保留跨租户管理能力，但必须携带明确的认证身份。
	if authInfo.TenantCode == DefaultTenantCode {
		return 0, false, nil
	}
	if authInfo.TenantId <= 0 {
		return 0, false, ErrTenantContextMissing
	}
	return authInfo.TenantId, true, nil
}

// hasTenantField 判断当前模型是否包含租户字段。
func hasTenantField(db *gorm.DB, scopedTables map[string]struct{}) bool {
	if db == nil || db.Statement == nil {
		return false
	}
	if db.Statement.Schema != nil {
		_, hasTenantColumn := db.Statement.Schema.FieldsByDBName[tenantColumnName]
		if hasTenantColumn {
			return true
		}
	}
	return statementUsesRegisteredTable(db, scopedTables)
}

// tenantTables 返回所有注册模型中需要租户隔离的表名。
func tenantTables(db *gorm.DB) (map[string]struct{}, error) {
	tables, _, err := getIsolationTables(db)
	return tables, err
}

// hasTenantJoin 判断当前语句是否关联了需要租户隔离的表。
func hasTenantJoin(db *gorm.DB, scopedTables map[string]struct{}) bool {
	match := func(reference sqlTableReference) bool {
		return isScopedTableReference(reference, scopedTables, tenantColumnName)
	}
	return hasScopedJoin(db, match)
}

// applyTenantJoinConditions 将租户条件写入 JOIN ON，并返回无法写入 ON 的兜底条件。
func applyTenantJoinConditions(db *gorm.DB, tenantID int64, scopedTables map[string]struct{}) []clause.Expression {
	match := func(reference sqlTableReference) bool {
		return isScopedTableReference(reference, scopedTables, tenantColumnName)
	}
	build := func(reference sqlTableReference) clause.Expression {
		return clause.Eq{Column: clause.Column{Table: reference.alias, Name: tenantColumnName}, Value: tenantID}
	}
	return applyJoinConditions(db, match, build, "tenant")
}

// setTenantMap 将租户编号写入 map 创建参数。
func setTenantMap(db *gorm.DB, tenantField *schema.Field, tenantID int64) bool {
	switch dest := db.Statement.Dest.(type) {
	case map[string]interface{}:
		setTenantMapItem(db, dest, tenantField, tenantID)
	case *map[string]interface{}:
		if dest != nil {
			setTenantMapItem(db, *dest, tenantField, tenantID)
		}
	case []map[string]interface{}:
		for _, item := range dest {
			setTenantMapItem(db, item, tenantField, tenantID)
		}
	case *[]map[string]interface{}:
		if dest != nil {
			for _, item := range *dest {
				setTenantMapItem(db, item, tenantField, tenantID)
			}
		}
	default:
		return false
	}
	return true
}

// setTenantMapItem 在单条 map 数据上填充或校验租户编号。
func setTenantMapItem(db *gorm.DB, item map[string]interface{}, tenantField *schema.Field, tenantID int64) {
	if item == nil || db.Error != nil {
		return
	}
	key, value, valueExists := tenantMapValue(item, tenantField)
	if !valueExists {
		item[tenantColumnName] = tenantID
		return
	}

	currentTenantID, zero, valid := tenantIDFromValue(value)
	if !valid {
		fieldName := tenantStructFieldName
		if tenantField != nil {
			fieldName = tenantField.Name
		}
		db.AddError(fmt.Errorf("tenant field %s has unsupported value %#v", fieldName, value))
		return
	}
	if zero {
		item[key] = tenantID
		return
	}
	if currentTenantID != tenantID {
		db.AddError(fmt.Errorf("%w: current tenant %d, record tenant %d", errTenantMismatch, tenantID, currentTenantID))
	}
}

// tenantMapValue 从 map 中读取租户字段值。
func tenantMapValue(item map[string]interface{}, tenantField *schema.Field) (string, interface{}, bool) {
	keys := []string{tenantColumnName, tenantStructFieldName}
	if tenantField != nil {
		keys = []string{tenantField.DBName, tenantField.Name}
	}
	for _, key := range keys {
		if value, valueExists := item[key]; valueExists {
			return key, value, true
		}
	}
	return "", nil, false
}

// setTenantField 将租户编号写入结构体或结构体集合。
func setTenantField(db *gorm.DB, value reflect.Value, tenantField *schema.Field, tenantID int64) {
	if !value.IsValid() || db.Error != nil {
		return
	}
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Struct:
		currentValue, zero := tenantField.ValueOf(db.Statement.Context, value)
		if zero {
			db.AddError(tenantField.Set(db.Statement.Context, value, tenantID))
			return
		}
		currentTenantID, _, valid := tenantIDFromValue(currentValue)
		if !valid {
			db.AddError(fmt.Errorf("tenant field %s has unsupported value %#v", tenantField.Name, currentValue))
			return
		}
		if currentTenantID != tenantID {
			db.AddError(fmt.Errorf("%w: current tenant %d, record tenant %d", errTenantMismatch, tenantID, currentTenantID))
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			setTenantField(db, value.Index(i), tenantField, tenantID)
		}
	}
}

// tenantIDFromValue 将字段值解析成租户编号，并返回该值是否为空。
func tenantIDFromValue(value interface{}) (int64, bool, bool) {
	if value == nil {
		return 0, true, true
	}
	reflectValue := reflect.ValueOf(value)
	for reflectValue.Kind() == reflect.Pointer || reflectValue.Kind() == reflect.Interface {
		if reflectValue.IsNil() {
			return 0, true, true
		}
		reflectValue = reflectValue.Elem()
	}
	switch reflectValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		signedTenantID := reflectValue.Int()
		return signedTenantID, signedTenantID == 0, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		const maxInt64 = uint64(1<<63 - 1)
		unsignedTenantID := reflectValue.Uint()
		if unsignedTenantID > maxInt64 {
			return 0, false, false
		}
		return int64(unsignedTenantID), unsignedTenantID == 0, true
	case reflect.String:
		if reflectValue.String() == "" {
			return 0, true, true
		}
		stringTenantID, err := strconv.ParseInt(reflectValue.String(), 10, 64)
		if err != nil {
			return 0, false, false
		}
		return stringTenantID, stringTenantID == 0, true
	default:
		return 0, false, false
	}
}
