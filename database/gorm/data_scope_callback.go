package gorm

import (
	"context"
	"fmt"

	"github.com/liujitcn/kratos-kit/auth"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// DataScopeUnknown 表示未声明角色数据范围，默认不追加数据权限条件以兼容历史令牌。
	DataScopeUnknown int32 = iota
	// DataScopeAll 表示角色拥有全部数据范围。
	DataScopeAll
	// DataScopeDeptAndChildren 表示角色拥有本部门及子部门数据范围。
	DataScopeDeptAndChildren
	// DataScopeSelfDept 表示角色仅拥有本部门数据范围。
	DataScopeSelfDept
	// DataScopeSelfUser 表示角色仅拥有本人数据范围。
	DataScopeSelfUser
)

const (
	dataScopeCreatedByColumnName  = "created_by"
	dataScopeDeptIDColumnName     = "id"
	dataScopeDeptPathColumnName   = "path"
	dataScopeUserTableName        = "base_user"
	dataScopeDeptTableName        = "base_dept"
	dataScopeUserIDColumnName     = "id"
	dataScopeUserDeptIDColumnName = "dept_id"
)

type dataScopeIdentity struct {
	userID     int64
	deptID     int64
	tenantID   int64
	tenantCode string
	dataScope  int32
}

func init() {
	RegisterCallbackQuery(addDataScopeWhere)
	RegisterCallbackRow(addDataScopeWhere)
	RegisterCallbackUpdateBefore("gorm:update", addDataScopeWhere)
	RegisterCallbackDelete(addDataScopeWhere)
}

// addDataScopeWhere 为当前查询、更新和删除语句追加角色数据范围条件。
func addDataScopeWhere(db *gorm.DB) {
	if shouldSkipDataIsolation(db) || db == nil || db.Statement == nil || db.Error != nil {
		return
	}
	if rejectUnsafeRawStatement(db) {
		return
	}
	scopedTables, err := dataScopeTables(db)
	if err != nil {
		db.AddError(err)
		return
	}
	hasMainScope := isDataScopeDeptTable(db) || isDataScopeUserTable(db) || hasDataScopeCreatedByField(db, scopedTables)
	hasJoinScope := hasDataScopeJoin(db, scopedTables)
	if db.Error != nil {
		return
	}
	if !hasMainScope && !hasJoinScope {
		return
	}
	var identity dataScopeIdentity
	var hasDataScope bool
	identity, hasDataScope, err = dataScopeForStatement(db)
	if err != nil {
		addDataIsolationError(db, err)
		return
	}
	if !hasDataScope {
		return
	}
	var exprs []clause.Expression
	if hasMainScope {
		exprs = append(exprs, dataScopeExpr(db, identity))
	}
	exprs = append(exprs, applyDataScopeJoinConditions(db, identity, scopedTables)...)
	if len(exprs) > 0 {
		db.Statement.AddClause(clause.Where{Exprs: exprs})
	}
}

// dataScopeForStatement 从当前 GORM 语句上下文读取角色数据范围。
func dataScopeForStatement(db *gorm.DB) (dataScopeIdentity, bool, error) {
	if db == nil || db.Statement == nil {
		return dataScopeIdentity{}, false, ErrDataScopeContextMissing
	}
	return dataScopeFromContext(db.Statement.Context)
}

// dataScopeFromContext 从登录用户信息读取当前角色数据范围。
func dataScopeFromContext(ctx context.Context) (dataScopeIdentity, bool, error) {
	if ctx == nil {
		return dataScopeIdentity{}, false, nil
	}
	authInfo, err := auth.FromContext(ctx)
	if err != nil || authInfo == nil {
		return dataScopeIdentity{}, false, nil
	}
	// 未声明数据范围或拥有全部数据时，不需要追加额外数据范围条件。
	if authInfo.DataScope == DataScopeUnknown || authInfo.DataScope == DataScopeAll {
		return dataScopeIdentity{}, false, nil
	}
	return dataScopeIdentity{
		userID:     authInfo.UserId,
		deptID:     authInfo.DeptId,
		tenantID:   authInfo.TenantId,
		tenantCode: authInfo.TenantCode,
		dataScope:  authInfo.DataScope,
	}, true, nil
}

// hasTenantScope 判断数据范围子查询是否需要补充租户条件。
func (i dataScopeIdentity) hasTenantScope() bool {
	return i.tenantID > 0 && i.tenantCode != DefaultTenantCode
}

// dataScopeExpr 根据当前模型选择数据范围过滤表达式。
func dataScopeExpr(db *gorm.DB, identity dataScopeIdentity) clause.Expression {
	// 部门表直接按照部门范围展示，不按创建人字段过滤。
	if isDataScopeDeptTable(db) {
		return dataScopeDeptExprForTable(identity, clause.CurrentTable)
	}
	return dataScopeCreatedByExprForTable(identity, clause.CurrentTable, isDataScopeUserTable(db))
}

// isDataScopeDeptTable 判断当前语句是否操作部门表。
func isDataScopeDeptTable(db *gorm.DB) bool {
	if db == nil || db.Statement == nil {
		return false
	}
	if db.Statement.Schema != nil && db.Statement.Schema.Table == dataScopeDeptTableName {
		return true
	}
	return statementUsesRegisteredTable(db, map[string]struct{}{dataScopeDeptTableName: {}})
}

// isDataScopeUserTable 判断当前语句是否操作用户表。
func isDataScopeUserTable(db *gorm.DB) bool {
	if db == nil || db.Statement == nil {
		return false
	}
	if db.Statement.Schema != nil && db.Statement.Schema.Table == dataScopeUserTableName {
		return true
	}
	return statementUsesRegisteredTable(db, map[string]struct{}{dataScopeUserTableName: {}})
}

// hasDataScopeCreatedByField 判断当前模型是否包含创建人字段。
func hasDataScopeCreatedByField(db *gorm.DB, scopedTables map[string]struct{}) bool {
	if db == nil || db.Statement == nil {
		return false
	}
	if db.Statement.Schema != nil {
		_, hasCreatedByColumn := db.Statement.Schema.FieldsByDBName[dataScopeCreatedByColumnName]
		if hasCreatedByColumn {
			return true
		}
	}
	return statementUsesRegisteredTable(db, scopedTables)
}

// dataScopeTables 返回所有注册模型中需要数据范围隔离的表名。
func dataScopeTables(db *gorm.DB) (map[string]struct{}, error) {
	_, tables, err := getIsolationTables(db)
	return tables, err
}

// hasDataScopeJoin 判断当前语句是否关联了需要数据范围隔离的表。
func hasDataScopeJoin(db *gorm.DB, scopedTables map[string]struct{}) bool {
	match := func(reference sqlTableReference) bool {
		return isDataScopeTableReference(reference, scopedTables)
	}
	return hasScopedJoin(db, match)
}

// applyDataScopeJoinConditions 将关联表数据范围条件写入 JOIN ON，并返回兜底条件。
func applyDataScopeJoinConditions(db *gorm.DB, identity dataScopeIdentity, scopedTables map[string]struct{}) []clause.Expression {
	match := func(reference sqlTableReference) bool {
		return isDataScopeTableReference(reference, scopedTables)
	}
	build := func(reference sqlTableReference) clause.Expression {
		if reference.name == dataScopeDeptTableName {
			return dataScopeDeptExprForTable(identity, reference.alias)
		}
		return dataScopeCreatedByExprForTable(identity, reference.alias, false)
	}
	return applyJoinConditions(db, match, build, "data_scope")
}

// isDataScopeTableReference 判断关联表是否需要数据范围隔离。
func isDataScopeTableReference(reference sqlTableReference, scopedTables map[string]struct{}) bool {
	if reference.name == dataScopeDeptTableName {
		return true
	}
	if reference.name == dataScopeUserTableName {
		return true
	}
	return isScopedTableReference(reference, scopedTables, dataScopeCreatedByColumnName)
}

// dataScopeDeptExprForTable 构建指定表别名的部门数据范围条件。
func dataScopeDeptExprForTable(identity dataScopeIdentity, table string) clause.Expression {
	if identity.deptID <= 0 {
		return denyDataScopeExpr(table, dataScopeDeptIDColumnName)
	}
	switch identity.dataScope {
	case DataScopeSelfUser, DataScopeSelfDept:
		// 本人数据在部门表上退化为本人所在部门。
		return clause.Eq{
			Column: dataScopeColumn(table, dataScopeDeptIDColumnName),
			Value:  identity.deptID,
		}
	case DataScopeDeptAndChildren:
		return clause.Or(
			clause.Eq{
				Column: dataScopeColumn(table, dataScopeDeptIDColumnName),
				Value:  identity.deptID,
			},
			clause.Like{
				Column: dataScopeColumn(table, dataScopeDeptPathColumnName),
				Value:  dataScopeDeptPathLikeValue(identity.deptID),
			},
		)
	default:
		// 未知的限制型数据范围按无权限处理，避免误放大访问范围。
		return denyDataScopeExpr(table, dataScopeDeptIDColumnName)
	}
}

// dataScopeCreatedByExprForTable 构建指定表别名的创建人数据范围条件。
func dataScopeCreatedByExprForTable(identity dataScopeIdentity, table string, useDerivedCreator bool) clause.Expression {
	switch identity.dataScope {
	case DataScopeSelfUser:
		if identity.userID <= 0 {
			return denyDataScopeExpr(table, dataScopeCreatedByColumnName)
		}
		return clause.Eq{
			Column: dataScopeColumn(table, dataScopeCreatedByColumnName),
			Value:  identity.userID,
		}
	case DataScopeSelfDept:
		if identity.deptID <= 0 {
			return denyDataScopeExpr(table, dataScopeCreatedByColumnName)
		}
		return createdByInDeptExpr(identity, table, useDerivedCreator)
	case DataScopeDeptAndChildren:
		if identity.deptID <= 0 {
			return denyDataScopeExpr(table, dataScopeCreatedByColumnName)
		}
		return createdByInDeptAndChildrenExpr(identity, table, useDerivedCreator)
	default:
		// 未知的限制型数据范围按无权限处理，避免误放大访问范围。
		return denyDataScopeExpr(table, dataScopeCreatedByColumnName)
	}
}

// createdByInDeptExpr 使用 EXISTS 构建创建人属于本人部门的半连接条件。
func createdByInDeptExpr(identity dataScopeIdentity, table string, useDerivedCreator bool) clause.Expression {
	if useDerivedCreator {
		return createdByInDeptDerivedExpr(identity, table)
	}
	sql := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM %s u WHERE u.%s = ? AND u.%s = ?",
		dataScopeUserTableName,
		dataScopeUserIDColumnName,
		dataScopeUserDeptIDColumnName,
	)
	vars := []interface{}{dataScopeColumn(table, dataScopeCreatedByColumnName), identity.deptID}
	if identity.hasTenantScope() {
		sql += fmt.Sprintf(" AND u.%s = ?", tenantColumnName)
		vars = append(vars, identity.tenantID)
	}
	sql += ")"
	return clause.Expr{SQL: sql, Vars: vars}
}

// createdByInDeptDerivedExpr 使用派生创建人集合规避更新用户表时的 MySQL 1093。
func createdByInDeptDerivedExpr(identity dataScopeIdentity, table string) clause.Expression {
	sql := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM (SELECT %s FROM %s WHERE %s = ?",
		dataScopeUserIDColumnName,
		dataScopeUserTableName,
		dataScopeUserDeptIDColumnName,
	)
	vars := []interface{}{identity.deptID}
	if identity.hasTenantScope() {
		sql += fmt.Sprintf(" AND %s = ?", tenantColumnName)
		vars = append(vars, identity.tenantID)
	}
	sql += fmt.Sprintf(") scoped_creator WHERE scoped_creator.%s = ?)", dataScopeUserIDColumnName)
	vars = append(vars, dataScopeColumn(table, dataScopeCreatedByColumnName))
	return clause.Expr{SQL: sql, Vars: vars}
}

// createdByInDeptAndChildrenExpr 使用 EXISTS 和部门关联构建本部门及子部门半连接条件。
func createdByInDeptAndChildrenExpr(identity dataScopeIdentity, table string, useDerivedCreator bool) clause.Expression {
	if useDerivedCreator {
		return createdByInDeptAndChildrenDerivedExpr(identity, table)
	}
	sql := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM %s u JOIN %s d ON u.%s = d.%s WHERE u.%s = ? AND (d.%s = ? OR d.%s LIKE ?)",
		dataScopeUserTableName,
		dataScopeDeptTableName,
		dataScopeUserDeptIDColumnName,
		dataScopeDeptIDColumnName,
		dataScopeUserIDColumnName,
		dataScopeDeptIDColumnName,
		dataScopeDeptPathColumnName,
	)
	vars := []interface{}{
		dataScopeColumn(table, dataScopeCreatedByColumnName),
		identity.deptID,
		dataScopeDeptPathLikeValue(identity.deptID),
	}
	if identity.hasTenantScope() {
		sql += fmt.Sprintf(" AND u.%s = ? AND d.%s = ?", tenantColumnName, tenantColumnName)
		vars = append(vars, identity.tenantID, identity.tenantID)
	}
	sql += ")"
	return clause.Expr{SQL: sql, Vars: vars}
}

// createdByInDeptAndChildrenDerivedExpr 使用派生创建人集合构建用户表的部门范围条件。
func createdByInDeptAndChildrenDerivedExpr(identity dataScopeIdentity, table string) clause.Expression {
	sql := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM (SELECT u.%s FROM %s u JOIN %s d ON u.%s = d.%s WHERE (d.%s = ? OR d.%s LIKE ?)",
		dataScopeUserIDColumnName,
		dataScopeUserTableName,
		dataScopeDeptTableName,
		dataScopeUserDeptIDColumnName,
		dataScopeDeptIDColumnName,
		dataScopeDeptIDColumnName,
		dataScopeDeptPathColumnName,
	)
	vars := []interface{}{identity.deptID, dataScopeDeptPathLikeValue(identity.deptID)}
	if identity.hasTenantScope() {
		sql += fmt.Sprintf(" AND u.%s = ? AND d.%s = ?", tenantColumnName, tenantColumnName)
		vars = append(vars, identity.tenantID, identity.tenantID)
	}
	sql += fmt.Sprintf(") scoped_creator WHERE scoped_creator.%s = ?)", dataScopeUserIDColumnName)
	vars = append(vars, dataScopeColumn(table, dataScopeCreatedByColumnName))
	return clause.Expr{SQL: sql, Vars: vars}
}

// denyDataScopeExpr 构建恒不命中的指定表字段条件。
func denyDataScopeExpr(table, columnName string) clause.Expression {
	return clause.Eq{
		Column: dataScopeColumn(table, columnName),
		Value:  int64(-1),
	}
}

// dataScopeColumn 构建指定表字段引用。
func dataScopeColumn(table, name string) clause.Column {
	return clause.Column{Table: table, Name: name}
}

// dataScopeDeptPathLikeValue 构建基于部门路径的子部门匹配值。
func dataScopeDeptPathLikeValue(deptID int64) string {
	return fmt.Sprintf("%%/%d/%%", deptID)
}
