package callback

import (
	"strings"

	"gorm.io/gorm"
)

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
