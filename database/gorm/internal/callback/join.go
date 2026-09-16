package callback

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type sqlTableReference struct {
	name        string
	alias       string
	modelSchema *schema.Schema
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
