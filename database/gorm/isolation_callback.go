package gorm

import (
	"errors"

	"github.com/liujitcn/kratos-kit/auth"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	skipDataIsolationSettingKey = "kratos-kit:skip-data-isolation"
	deniedSingleRowSettingKey   = "kratos-kit:denied-single-row"
	deniedSingleRowSQL          = "SELECT 1 WHERE 1 = 0"
	deniedOracleSingleRowSQL    = "SELECT 1 FROM dual WHERE 1 = 0"
)

var (
	// ErrTenantContextMissing 表示租户表操作缺少有效的租户身份。
	ErrTenantContextMissing = errors.New("tenant context missing")
	// ErrDataScopeContextMissing 表示数据权限表操作缺少有效的用户身份。
	ErrDataScopeContextMissing = errors.New("data scope context missing")
	// ErrRawDataIsolationUnsupported 表示原生 SQL 无法由回调安全追加数据隔离条件。
	ErrRawDataIsolationUnsupported = errors.New("raw SQL requires explicit data isolation bypass")
)

func init() {
	RegisterCallbackRaw(rejectRawDataIsolation)
}

// SkipDataIsolation 显式跳过租户和角色数据范围隔离，仅供可信的系统任务使用。
func SkipDataIsolation(db *gormdb.DB) *gormdb.DB {
	if db == nil {
		return nil
	}
	return db.Set(skipDataIsolationSettingKey, true)
}

// shouldSkipDataIsolation 判断当前语句是否需要跳过数据隔离。
func shouldSkipDataIsolation(db *gormdb.DB) bool {
	if db == nil {
		return false
	}
	value, settingExists := db.Get(skipDataIsolationSettingKey)
	if settingExists {
		var skip bool
		var isBool bool
		skip, isBool = value.(bool)
		if isBool && skip {
			return true
		}
	}
	if db.Statement == nil {
		return false
	}
	if db.Statement.Context == nil {
		return true
	}
	authInfo, err := auth.FromContext(db.Statement.Context)
	return err != nil || authInfo == nil
}

// rejectUnsafeRawStatement 拒绝进入查询回调的原生 SQL。
func rejectUnsafeRawStatement(db *gormdb.DB) bool {
	if db == nil || db.Statement == nil || db.Statement.SQL.Len() == 0 {
		return false
	}
	if denyRawSingleRowQuery(db) {
		return true
	}
	db.AddError(ErrRawDataIsolationUnsupported)
	return true
}

// rejectRawDataIsolation 拒绝未显式跳过数据隔离的原生 SQL。
func rejectRawDataIsolation(db *gormdb.DB) {
	if shouldSkipDataIsolation(db) || db == nil || db.Error != nil {
		return
	}
	db.AddError(ErrRawDataIsolationUnsupported)
}

// addDataIsolationError 为支持错误返回的调用保留原错误，Row 查询则改为恒不命中。
func addDataIsolationError(db *gormdb.DB, err error) {
	if db == nil || err == nil || denySingleRowQuery(db) {
		return
	}
	db.AddError(err)
}

// denySingleRowQuery 为 Row 查询追加恒不命中的条件，避免 GORM 因回调错误返回 nil。
func denySingleRowQuery(db *gormdb.DB) bool {
	if !isSingleRowQuery(db) {
		return false
	}
	if _, denied := db.Statement.Settings.Load(deniedSingleRowSettingKey); denied {
		return true
	}
	db.Statement.AddClause(clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "1 = 0"}}})
	db.Statement.Settings.Store(deniedSingleRowSettingKey, true)
	return true
}

// denyRawSingleRowQuery 将无法安全改写的 Raw Row 查询替换为跨方言恒不命中的 SQL。
func denyRawSingleRowQuery(db *gormdb.DB) bool {
	if !isSingleRowQuery(db) {
		return false
	}
	if _, denied := db.Statement.Settings.Load(deniedSingleRowSettingKey); denied {
		return true
	}
	db.Statement.SQL.Reset()
	if db.Dialector != nil && db.Dialector.Name() == "oracle" {
		db.Statement.SQL.WriteString(deniedOracleSingleRowSQL)
	} else {
		db.Statement.SQL.WriteString(deniedSingleRowSQL)
	}
	db.Statement.Vars = nil
	db.Statement.Settings.Store(deniedSingleRowSettingKey, true)
	return true
}

// isSingleRowQuery 判断当前语句是否由 GORM Row API 发起。
func isSingleRowQuery(db *gormdb.DB) bool {
	if db == nil || db.Statement == nil {
		return false
	}
	value, settingExists := db.Get("rows")
	if !settingExists {
		return false
	}
	var isRows bool
	var isBool bool
	isRows, isBool = value.(bool)
	return isBool && !isRows
}
