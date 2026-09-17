package callback

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/liujitcn/kratos-kit/auth"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProjectScope 是服务端解析后的租户项目范围，[0]表示对应租户全部项目。
type ProjectScope struct {
	// System 表示当前身份拥有全部项目范围，由可信内部任务或默认租户认证主体设置。
	System  bool
	Tenants map[int64][]int64
}

// ProjectScopeLoader 从可信身份实时加载项目范围，失败时禁止访问。
type ProjectScopeLoader func(context.Context) (ProjectScope, error)

// ErrProjectScopeDenied 表示项目范围缺失、非法或写入越权。
var ErrProjectScopeDenied = errors.New("project scope denied")

// RegisterProjectIsolation 为指定数据库的业务表注册项目隔离，表与字段必须由宿主显式提供。
func RegisterProjectIsolation(db *gorm.DB, columns map[string]string, load ProjectScopeLoader) error {
	isolation := &projectIsolation{columns: make(map[string]string, len(columns)), tables: make(map[string]struct{}, len(columns)), load: load}
	for table, column := range columns {
		isolation.columns[table] = column
		isolation.tables[table] = struct{}{}
	}
	return installCallbacks(db, []callbackDefinition{
		{operation: "query", name: "kratos:project:query", before: "gorm:query", after: "kratos:tenant:query", handler: isolation.filter},
		{operation: "row", name: "kratos:project:row", before: "gorm:row", after: "kratos:tenant:row", handler: isolation.filter},
		{operation: "update", name: "kratos:project:update", before: "gorm:update", after: "kratos:tenant:update", handler: isolation.update},
		{operation: "delete", name: "kratos:project:delete", before: "gorm:delete", after: "kratos:tenant:delete", handler: isolation.filter},
		{operation: "raw", name: "kratos:project:raw", before: "gorm:raw", after: "kratos:isolation:raw", handler: isolation.raw},
		{operation: "create", name: "kratos:project:create", before: "gorm:create", after: "gorm:before_create", handler: isolation.create},
	})
}

type projectIsolation struct {
	columns map[string]string
	tables  map[string]struct{}
	load    ProjectScopeLoader
}

// filter 在主表和关联表上追加租户项目成对的范围，保留原有租户和部门条件。
func (p *projectIsolation) filter(db *gorm.DB) {
	if db.Error != nil || hasDataIsolationBypass(db) {
		return
	}
	if db.Statement.SQL.Len() > 0 {
		p.raw(db)
		return
	}
	main := statementUsesRegisteredTable(db, p.tables)
	match := func(reference sqlTableReference) bool { _, ok := p.tables[reference.name]; return ok }
	joined := hasScopedJoin(db, match)
	if db.Error != nil || (!main && !joined) {
		return
	}
	if rejectUnsafeRawStatement(db) {
		return
	}
	scope, err := p.scope(db)
	if err != nil {
		addDataIsolationError(db, err)
		return
	}
	if scope.System {
		return
	}
	expressions := make([]clause.Expression, 0)
	if main {
		expressions = append(expressions, projectExpression(clause.CurrentTable, p.columns[p.mainTable(db)], scope))
	}
	expressions = append(expressions, applyJoinConditions(db, match, func(reference sqlTableReference) clause.Expression {
		return projectExpression(reference.alias, p.columns[reference.name], scope)
	}, "project")...)
	if len(expressions) > 0 {
		db.Statement.AddClause(clause.Where{Exprs: expressions})
	}
}

// update 禁止通过普通更新迁移记录的租户或项目归属，再约束原记录范围。
func (p *projectIsolation) update(db *gorm.DB) {
	if db.Error != nil || hasDataIsolationBypass(db) {
		return
	}
	if statementUsesRegisteredTable(db, p.tables) {
		protected := []string{"tenant_id", p.columns[p.mainTable(db)]}
		switch value := db.Statement.Dest.(type) {
		case map[string]interface{}:
			for _, key := range protected {
				if _, exists := value[key]; exists {
					db.AddError(ErrProjectScopeDenied)
					return
				}
			}
		default:
			if db.Statement.Schema != nil {
				value := reflect.Indirect(reflect.ValueOf(db.Statement.Dest))
				if !value.IsValid() || value.Kind() != reflect.Struct || value.Type() != db.Statement.Schema.ModelType {
					db.AddError(ErrProjectScopeDenied)
					return
				}
				selected, restricted := db.Statement.SelectAndOmitColumns(false, true)
				for _, key := range protected {
					field := db.Statement.Schema.FieldsByDBName[key]
					if field == nil {
						continue
					}
					if include, explicit := selected[key]; explicit && !include || !explicit && restricted {
						continue
					}
					_, zero := field.ValueOf(db.Statement.Context, reflect.Indirect(reflect.ValueOf(db.Statement.Dest)))
					if !zero || selected[key] {
						db.AddError(ErrProjectScopeDenied)
						return
					}
				}
			}
		}
	}
	p.filter(db)
}

// create 校验每一条新增记录的目标项目，避免拥有列表权限却向任意项目写入。
func (p *projectIsolation) create(db *gorm.DB) {
	if db.Error != nil || hasDataIsolationBypass(db) || !statementUsesRegisteredTable(db, p.tables) {
		return
	}
	scope, err := p.scope(db)
	if err != nil {
		db.AddError(err)
		return
	}
	if scope.System {
		return
	}
	column := p.columns[p.mainTable(db)]
	var check func(reflect.Value) bool
	check = func(value reflect.Value) bool {
		for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
			if value.IsNil() {
				return false
			}
			value = value.Elem()
		}
		if !value.IsValid() {
			return false
		}
		if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
			for i := 0; i < value.Len(); i++ {
				if !check(value.Index(i)) {
					return false
				}
			}
			return true
		}
		var tenantValue, projectValue interface{}
		if value.Kind() == reflect.Map && value.Type().Key().Kind() == reflect.String {
			tenant := value.MapIndex(reflect.ValueOf("tenant_id"))
			project := value.MapIndex(reflect.ValueOf(column))
			if !tenant.IsValid() || !project.IsValid() {
				return false
			}
			tenantValue, projectValue = tenant.Interface(), project.Interface()
		} else if value.Kind() == reflect.Struct && db.Statement.Schema != nil {
			tenantField := db.Statement.Schema.FieldsByDBName["tenant_id"]
			projectField := db.Statement.Schema.FieldsByDBName[column]
			if tenantField == nil || projectField == nil {
				return false
			}
			tenantValue, _ = tenantField.ValueOf(db.Statement.Context, value)
			projectValue, _ = projectField.ValueOf(db.Statement.Context, value)
		} else {
			return false
		}
		tenantID, _, tenantOK := tenantIDFromValue(tenantValue)
		projectID, _, projectOK := tenantIDFromValue(projectValue)
		ids := scope.Tenants[tenantID]
		return tenantOK && projectOK && tenantID > 0 && projectID > 0 && (slices.Equal(ids, []int64{0}) || slices.Contains(ids, projectID))
	}
	if !check(reflect.ValueOf(db.Statement.Dest)) {
		db.AddError(ErrProjectScopeDenied)
	}
}

// raw 拒绝普通请求通过原生 SQL 绕过项目过滤，内部任务必须显式声明身份。
func (p *projectIsolation) raw(db *gorm.DB) {
	if db.Error != nil || hasDataIsolationBypass(db) {
		return
	}
	scope, err := p.scope(db)
	if err != nil {
		addDataIsolationError(db, err)
		return
	}
	if !scope.System {
		addDataIsolationError(db, ErrRawDataIsolationUnsupported)
	}
}

// scope 校验加载器结果，任何非法来源均不能被当作全部权限。
func (p *projectIsolation) scope(db *gorm.DB) (ProjectScope, error) {
	if db.Statement.Context == nil {
		return ProjectScope{}, ErrProjectScopeDenied
	}
	if p.load == nil {
		return projectScopeFromAuth(db.Statement.Context)
	}
	scope, err := p.load(db.Statement.Context)
	if err != nil {
		return ProjectScope{}, err
	}
	for tenantID, ids := range scope.Tenants {
		if tenantID <= 0 {
			return ProjectScope{}, ErrProjectScopeDenied
		}
		for _, id := range ids {
			if id < 0 || id == 0 && len(ids) != 1 {
				return ProjectScope{}, ErrProjectScopeDenied
			}
		}
	}
	return scope, nil
}

// mainTable 取得带别名查询的实际表名。
func (p *projectIsolation) mainTable(db *gorm.DB) string {
	if _, ok := p.tables[db.Statement.Table]; ok {
		return db.Statement.Table
	}
	if db.Statement.TableExpr != nil {
		parts := strings.Fields(db.Statement.TableExpr.SQL)
		if len(parts) > 0 {
			return normalizeRawSQLIdentifier(db, parts[0])
		}
	}
	return ""
}

// projectScopeFromAuth 从认证载荷读取按租户分组的项目范围，缺少范围时保持拒绝访问。
func projectScopeFromAuth(ctx context.Context) (ProjectScope, error) {
	authInfo, err := auth.FromContext(ctx)
	if err != nil || authInfo == nil {
		return ProjectScope{}, ErrProjectScopeDenied
	}
	if authInfo.TenantCode == DefaultTenantCode {
		return ProjectScope{System: true}, nil
	}
	scope := ProjectScope{Tenants: make(map[int64][]int64, len(authInfo.TenantProjects))}
	for _, item := range authInfo.TenantProjects {
		if existing, ok := scope.Tenants[item.TenantId]; ok {
			if slices.Equal(existing, []int64{0}) || slices.Equal(item.ProjectId, []int64{0}) {
				scope.Tenants[item.TenantId] = []int64{0}
				continue
			}
			scope.Tenants[item.TenantId] = append(existing, item.ProjectId...)
			continue
		}
		scope.Tenants[item.TenantId] = slices.Clone(item.ProjectId)
	}
	return scope, nil
}

// projectExpression 按租户分组构建项目范围，空集合使用不可能存在的项目ID。
func projectExpression(table, column string, scope ProjectScope) clause.Expression {
	expressions := make([]clause.Expression, 0, len(scope.Tenants))
	for tenantID, ids := range scope.Tenants {
		tenant := clause.Eq{Column: clause.Column{Table: table, Name: "tenant_id"}, Value: tenantID}
		if slices.Equal(ids, []int64{0}) {
			expressions = append(expressions, tenant)
			continue
		}
		if len(ids) == 0 {
			continue
		}
		values := make([]interface{}, len(ids))
		for index, id := range ids {
			values[index] = id
		}
		expressions = append(expressions, clause.And(tenant, clause.IN{Column: clause.Column{Table: table, Name: column}, Values: values}))
	}
	if len(expressions) == 0 {
		return clause.Eq{Column: clause.Column{Table: table, Name: column}, Value: int64(0)}
	}
	// 外层 AND 避免 GORM 将单分支 OR 与调用方已有 WHERE 拼成放宽范围的 OR。
	return clause.And(clause.Or(expressions...))
}
