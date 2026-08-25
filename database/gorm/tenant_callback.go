package gorm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

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

func init() {
	RegisterCallbackQuery(addTenantWhere)
	RegisterCallbackRow(addTenantWhere)
	RegisterCallbackCreate(fillTenantID)
	RegisterCallbackUpdateBefore("gorm:update", addTenantWhere)
	RegisterCallbackDelete(addTenantWhere)
}

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

// hasScopedJoin 判断当前语句是否存在匹配指定规则的关联表。
func hasScopedJoin(db *gorm.DB, match func(sqlTableReference) bool) bool {
	hasMatch := false
	if fromClause, clauseExists := db.Statement.Clauses["FROM"]; clauseExists {
		if from, isFromClause := fromClause.Expression.(clause.From); isFromClause {
			for _, join := range from.Joins {
				if join.Expression != nil {
					addDataIsolationError(db, ErrRawDataIsolationUnsupported)
					continue
				}
				reference := newSQLTableReference(join.Table.Name, join.Table.Alias, nil)
				if match(reference) {
					hasMatch = true
					if join.Type == clause.LeftJoin && len(join.Using) > 0 {
						addDataIsolationError(db, ErrRawDataIsolationUnsupported)
					}
				}
			}
		}
	}
	for _, join := range db.Statement.Joins {
		associationReferences := associationJoinReferences(db.Statement.Schema, join.Name, join.Alias)
		if len(associationReferences) > 0 {
			for _, reference := range associationReferences {
				if match(reference) {
					hasMatch = true
				}
			}
			continue
		}
		segments := rawJoinSegments(db, join.Name)
		if len(segments) == 0 {
			addDataIsolationError(db, ErrRawDataIsolationUnsupported)
			continue
		}
		for _, segment := range segments {
			if segment.reference.name == "" {
				addDataIsolationError(db, ErrRawDataIsolationUnsupported)
				continue
			}
			if match(segment.reference) {
				hasMatch = true
				if rawJoinRequiresUnsupportedOuterFallback(segment) {
					addDataIsolationError(db, ErrRawDataIsolationUnsupported)
				}
			}
		}
	}
	return hasMatch
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

// applyJoinConditions 将关联表隔离条件优先写入 JOIN ON，并返回需要写入 WHERE 的兜底条件。
func applyJoinConditions(db *gorm.DB, match func(sqlTableReference) bool, build func(sqlTableReference) clause.Expression, placeholderPrefix string) []clause.Expression {
	expandNestedAssociationJoins(db)
	seen := make(map[string]struct{})
	joinedAssociations := make(map[string]struct{})
	var fallback []clause.Expression
	if fromClause, clauseExists := db.Statement.Clauses["FROM"]; clauseExists {
		if from, isFromClause := fromClause.Expression.(clause.From); isFromClause {
			for index := range from.Joins {
				join := &from.Joins[index]
				reference := newSQLTableReference(join.Table.Name, join.Table.Alias, nil)
				if !match(reference) {
					continue
				}
				expr := build(reference)
				if join.Type == clause.RightJoin || join.Type == clause.CrossJoin || join.Expression != nil || len(join.Using) > 0 {
					fallback = appendUniqueJoinExpression(fallback, seen, reference.alias, expr)
					continue
				}
				join.ON.Exprs = append(join.ON.Exprs, expr)
				seen[reference.alias] = struct{}{}
			}
			fromClause.Expression = from
			db.Statement.Clauses["FROM"] = fromClause
		}
	}
	for index := range db.Statement.Joins {
		join := &db.Statement.Joins[index]
		associationReferences := associationJoinReferences(db.Statement.Schema, join.Name, join.Alias)
		if len(associationReferences) > 0 {
			parts := strings.Split(join.Name, ".")
			for referenceIndex, reference := range associationReferences {
				associationPath := strings.Join(parts[:referenceIndex+1], ".")
				if _, alreadyJoined := joinedAssociations[associationPath]; alreadyJoined {
					continue
				}
				joinedAssociations[associationPath] = struct{}{}
				if !match(reference) {
					continue
				}
				expr := build(reference)
				if referenceIndex == len(associationReferences)-1 && join.JoinType != clause.RightJoin && join.JoinType != clause.CrossJoin {
					if join.On == nil {
						join.On = &clause.Where{}
					}
					join.On.Exprs = append(join.On.Exprs, expr)
					seen[reference.alias] = struct{}{}
					continue
				}
				fallback = appendUniqueJoinExpression(fallback, seen, reference.alias, expr)
			}
			continue
		}
		var injections []rawJoinInjection
		for segmentIndex, segment := range rawJoinSegments(db, join.Name) {
			if !match(segment.reference) {
				continue
			}
			expr := build(segment.reference)
			if !segment.supportsOn {
				fallback = appendUniqueJoinExpression(fallback, seen, segment.reference.alias, expr)
				continue
			}
			placeholder := fmt.Sprintf("__kratos_%s_isolation_%d_%d", placeholderPrefix, index, segmentIndex)
			injections = append(injections, rawJoinInjection{
				conditionStart: segment.conditionStart,
				conditionEnd:   segment.conditionEnd,
				placeholder:    placeholder,
			})
			join.Conds = append(join.Conds, sql.Named(placeholder, expr))
			seen[segment.reference.alias] = struct{}{}
		}
		if len(injections) > 0 {
			join.Name = appendRawJoinOnConditions(join.Name, injections)
		}
	}
	return fallback
}

// expandNestedAssociationJoins 将嵌套关联展开为逐级 JOIN，便于为每一级写入独立 ON 条件。
func expandNestedAssociationJoins(db *gorm.DB) {
	if db == nil || db.Statement == nil || db.Statement.Schema == nil || len(db.Statement.Joins) == 0 {
		return
	}
	original := append(db.Statement.Joins[:0:0], db.Statement.Joins...)
	expanded := db.Statement.Joins[:0]
	seen := make(map[string]struct{}, len(original))
	for _, join := range original {
		references := associationJoinReferences(db.Statement.Schema, join.Name, join.Alias)
		if len(references) <= 1 {
			if _, alreadySeen := seen[join.Name]; !alreadySeen {
				expanded = append(expanded, join)
				seen[join.Name] = struct{}{}
			}
			continue
		}
		parts := strings.Split(join.Name, ".")
		for index := range references {
			path := strings.Join(parts[:index+1], ".")
			if _, alreadySeen := seen[path]; alreadySeen {
				continue
			}
			item := join
			item.Name = path
			if join.On != nil {
				on := *join.On
				on.Exprs = slices.Clone(join.On.Exprs)
				item.On = &on
			}
			if index < len(references)-1 {
				item.Alias = ""
			}
			expanded = append(expanded, item)
			seen[path] = struct{}{}
		}
	}
	db.Statement.Joins = expanded
}

// appendUniqueJoinExpression 按表别名去重追加关联条件。
func appendUniqueJoinExpression(exprs []clause.Expression, seen map[string]struct{}, alias string, expr clause.Expression) []clause.Expression {
	if _, alreadySeen := seen[alias]; alreadySeen {
		return exprs
	}
	seen[alias] = struct{}{}
	return append(exprs, expr)
}

// associationJoinReferences 解析模型关联 JOIN 中的实际表和别名。
func associationJoinReferences(modelSchema *schema.Schema, joinName, finalAlias string) []sqlTableReference {
	if modelSchema == nil {
		return nil
	}
	parts := strings.Split(joinName, ".")
	currentSchema := modelSchema
	alias := ""
	references := make([]sqlTableReference, 0, len(parts))
	for index, part := range parts {
		relation, relationExists := currentSchema.Relationships.Relations[part]
		if !relationExists {
			return nil
		}
		if alias == "" {
			alias = relation.Name
		} else {
			alias += "__" + relation.Name
		}
		if index == len(parts)-1 && finalAlias != "" {
			alias = finalAlias
		}
		references = append(references, newSQLTableReference(relation.FieldSchema.Table, alias, relation.FieldSchema))
		currentSchema = relation.FieldSchema
	}
	return references
}

type sqlTableReference struct {
	name        string
	alias       string
	modelSchema *schema.Schema
}

type rawSQLToken struct {
	value string
	start int
	end   int
}

type rawJoinSegment struct {
	reference      sqlTableReference
	conditionStart int
	conditionEnd   int
	joinType       string
	supportsOn     bool
}

type rawJoinInjection struct {
	conditionStart int
	conditionEnd   int
	placeholder    string
}

// newSQLTableReference 创建规范化的表引用。
func newSQLTableReference(name, alias string, modelSchema *schema.Schema) sqlTableReference {
	name = normalizeSQLIdentifier(name)
	alias = normalizeSQLIdentifier(alias)
	if alias == "" {
		alias = name
	}
	return sqlTableReference{name: name, alias: alias, modelSchema: modelSchema}
}

// isScopedTableReference 判断关联表是否声明指定隔离字段或已注册为受保护表。
func isScopedTableReference(reference sqlTableReference, scopedTables map[string]struct{}, fieldName string) bool {
	if reference.modelSchema != nil {
		if _, hasField := reference.modelSchema.FieldsByDBName[fieldName]; hasField {
			return true
		}
	}
	_, isScoped := scopedTables[reference.name]
	return isScoped
}

// rawJoinReferences 从原生 JOIN 片段中解析表名和别名，并返回片段是否可安全识别。
func rawJoinReferences(db *gorm.DB, query string) ([]sqlTableReference, bool) {
	segments := rawJoinSegments(db, query)
	if len(segments) == 0 {
		return nil, false
	}
	references := make([]sqlTableReference, 0, len(segments))
	for _, segment := range segments {
		if segment.reference.name == "" {
			return nil, false
		}
		references = append(references, segment.reference)
	}
	return references, true
}

// rawJoinSegments 解析原生 SQL 中每个顶层 JOIN 的表引用和 ON 条件范围。
func rawJoinSegments(db *gorm.DB, query string) []rawJoinSegment {
	tokens := rawSQLTokens(query)
	var joinIndexes []int
	for index, token := range tokens {
		if rawSQLKeyword(token.value) == "JOIN" {
			joinIndexes = append(joinIndexes, index)
		}
	}
	segments := make([]rawJoinSegment, 0, len(joinIndexes))
	for position, joinIndex := range joinIndexes {
		segmentEnd := len(query)
		if position+1 < len(joinIndexes) {
			segmentEnd = rawJoinStart(tokens, joinIndexes[position+1])
		}
		tableIndex := joinIndex + 1
		for tableIndex < len(tokens) && tokens[tableIndex].start < segmentEnd {
			keyword := rawSQLKeyword(tokens[tableIndex].value)
			if keyword != "LATERAL" && keyword != "ONLY" {
				break
			}
			tableIndex++
		}
		if tableIndex >= len(tokens) || tokens[tableIndex].start >= segmentEnd {
			continue
		}

		name := normalizeRawSQLIdentifier(db, tokens[tableIndex].value)
		if strings.HasPrefix(strings.TrimSpace(tokens[tableIndex].value), "(") {
			name = ""
		}
		alias := name
		aliasIndex := tableIndex + 1
		if aliasIndex < len(tokens) && tokens[aliasIndex].start < segmentEnd && rawSQLKeyword(tokens[aliasIndex].value) == "AS" {
			aliasIndex++
		}
		if aliasIndex < len(tokens) && tokens[aliasIndex].start < segmentEnd && !isJoinConditionKeyword(tokens[aliasIndex].value) {
			alias = normalizeRawSQLIdentifier(db, tokens[aliasIndex].value)
		}

		onIndex := -1
		for index := tableIndex + 1; index < len(tokens) && tokens[index].start < segmentEnd; index++ {
			if rawSQLKeyword(tokens[index].value) == "ON" {
				onIndex = index
				break
			}
		}
		segment := rawJoinSegment{
			reference: newSQLTableReference(name, alias, nil),
			joinType:  rawJoinType(tokens, joinIndex),
		}
		if onIndex >= 0 && rawJoinAllowsOn(tokens, joinIndex) {
			segment.conditionStart = skipRawSQLSpace(query, tokens[onIndex].end, segmentEnd)
			segment.conditionEnd = segmentEnd
			segment.supportsOn = segment.conditionStart < segment.conditionEnd
		}
		segments = append(segments, segment)
	}
	return segments
}

// rawSQLTokens 返回忽略嵌套括号、字符串和注释内容后的顶层 SQL token。
func rawSQLTokens(query string) []rawSQLToken {
	var tokens []rawSQLToken
	tokenStart := -1
	depth := 0
	var quote byte
	flush := func(end int) {
		if tokenStart >= 0 {
			tokens = append(tokens, rawSQLToken{value: query[tokenStart:end], start: tokenStart, end: end})
			tokenStart = -1
		}
	}
	for index := 0; index < len(query); index++ {
		current := query[index]
		if quote != 0 {
			if current == quote {
				if index+1 < len(query) && query[index+1] == quote && quote != ']' {
					index++
					continue
				}
				quote = 0
			}
			continue
		}
		if depth == 0 && current == '-' && index+1 < len(query) && query[index+1] == '-' {
			flush(index)
			for index < len(query) && query[index] != '\n' {
				index++
			}
			continue
		}
		if depth == 0 && current == '/' && index+1 < len(query) && query[index+1] == '*' {
			flush(index)
			index += 2
			for index+1 < len(query) && (query[index] != '*' || query[index+1] != '/') {
				index++
			}
			index++
			continue
		}
		switch current {
		case '\'', '"', '`':
			if tokenStart < 0 {
				tokenStart = index
			}
			quote = current
		case '[':
			if tokenStart < 0 {
				tokenStart = index
			}
			quote = ']'
		case '(':
			if tokenStart < 0 {
				tokenStart = index
			}
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ' ', '\t', '\r', '\n', ',':
			if depth == 0 {
				flush(index)
			}
		default:
			if tokenStart < 0 {
				tokenStart = index
			}
		}
	}
	flush(len(query))
	return tokens
}

// rawJoinStart 返回 JOIN 片段包含关联类型关键字的起始位置。
func rawJoinStart(tokens []rawSQLToken, joinIndex int) int {
	start := tokens[joinIndex].start
	index := joinIndex - 1
	if index >= 0 && rawSQLKeyword(tokens[index].value) == "OUTER" {
		start = tokens[index].start
		index--
	}
	if index >= 0 {
		switch rawSQLKeyword(tokens[index].value) {
		case "LEFT", "RIGHT", "FULL", "INNER", "CROSS":
			start = tokens[index].start
			index--
		}
	}
	if index >= 0 && rawSQLKeyword(tokens[index].value) == "NATURAL" {
		start = tokens[index].start
	}
	return start
}

// rawJoinAllowsOn 判断当前 JOIN 的隔离条件是否可以安全写入 ON。
func rawJoinAllowsOn(tokens []rawSQLToken, joinIndex int) bool {
	switch rawJoinType(tokens, joinIndex) {
	case "RIGHT", "FULL", "CROSS":
		return false
	default:
		return true
	}
}

// rawJoinType 返回当前原生 JOIN 的关联类型，省略类型时按 INNER 处理。
func rawJoinType(tokens []rawSQLToken, joinIndex int) string {
	index := joinIndex - 1
	if index >= 0 && rawSQLKeyword(tokens[index].value) == "OUTER" {
		index--
	}
	if index < 0 {
		return "INNER"
	}
	switch joinType := rawSQLKeyword(tokens[index].value); joinType {
	case "LEFT", "RIGHT", "FULL", "INNER", "CROSS":
		return joinType
	default:
		return "INNER"
	}
}

// rawJoinRequiresUnsupportedOuterFallback 判断隔离条件是否会破坏外连接语义。
func rawJoinRequiresUnsupportedOuterFallback(segment rawJoinSegment) bool {
	return !segment.supportsOn && (segment.joinType == "LEFT" || segment.joinType == "FULL")
}

// appendRawJoinOnConditions 为原生 JOIN 的每个 ON 条件追加隔离表达式。
func appendRawJoinOnConditions(query string, injections []rawJoinInjection) string {
	for index := len(injections) - 1; index >= 0; index-- {
		injection := injections[index]
		conditionEnd := injection.conditionEnd
		for conditionEnd > injection.conditionStart && isRawSQLSpace(query[conditionEnd-1]) {
			conditionEnd--
		}
		replacement := "(" + query[injection.conditionStart:conditionEnd] + ") AND @" + injection.placeholder + query[conditionEnd:injection.conditionEnd]
		query = query[:injection.conditionStart] + replacement + query[injection.conditionEnd:]
	}
	return query
}

// skipRawSQLSpace 跳过指定 SQL 范围开头的空白字符。
func skipRawSQLSpace(query string, start, end int) int {
	for start < end && isRawSQLSpace(query[start]) {
		start++
	}
	return start
}

// isRawSQLSpace 判断字符是否为 SQL 空白字符。
func isRawSQLSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

// rawSQLKeyword 返回用于比较的 SQL 关键字。
func rawSQLKeyword(value string) string {
	return strings.ToUpper(strings.Trim(value, "`\"[]"))
}

// isJoinConditionKeyword 判断原生 JOIN 表名后是否已经进入关联条件或索引提示。
func isJoinConditionKeyword(value string) bool {
	switch rawSQLKeyword(value) {
	case "ON", "USING", "JOIN", "LEFT", "RIGHT", "FULL", "INNER", "CROSS", "NATURAL", "USE", "FORCE", "IGNORE", "INDEX", "KEY", "PARTITION":
		return true
	default:
		return false
	}
}

// normalizeSQLIdentifier 规范化用于匹配注册表的简单 SQL 标识符。
func normalizeSQLIdentifier(value string) string {
	value = strings.Trim(value, "`\"[](),")
	parts := strings.Split(value, ".")
	return strings.Trim(parts[len(parts)-1], "`\"[](),")
}

// normalizeRawSQLIdentifier 按数据库规则规范化原生 SQL 中未引用的标识符。
func normalizeRawSQLIdentifier(db *gorm.DB, value string) string {
	name := normalizeSQLIdentifier(value)
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" || isQuotedSQLIdentifier(value) {
		return name
	}
	return strings.ToLower(name)
}

// isQuotedSQLIdentifier 判断原生 SQL 标识符的最后一段是否被显式引用。
func isQuotedSQLIdentifier(value string) bool {
	value = strings.Trim(strings.TrimSpace(value), "(),")
	parts := strings.Split(value, ".")
	last := strings.TrimSpace(parts[len(parts)-1])
	if len(last) < 2 {
		return false
	}
	return last[0] == '`' && last[len(last)-1] == '`' ||
		last[0] == '"' && last[len(last)-1] == '"' ||
		last[0] == '[' && last[len(last)-1] == ']'
}

// statementUsesRegisteredTable 判断当前无 Schema 语句是否直接操作已注册表。
func statementUsesRegisteredTable(db *gorm.DB, registeredTables map[string]struct{}) bool {
	if db == nil || db.Statement == nil {
		return false
	}
	if _, isRegistered := registeredTables[db.Statement.Table]; isRegistered {
		return true
	}
	if db.Statement.TableExpr == nil {
		return false
	}
	fields := strings.Fields(db.Statement.TableExpr.SQL)
	if len(fields) == 0 {
		return false
	}
	_, isRegistered := registeredTables[normalizeRawSQLIdentifier(db, fields[0])]
	return isRegistered
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
