package driver

import (
	"strconv"
	"strings"
)

// FieldKind 字段 Go 语义类别，屏蔽生成器 Go 类型与 GORM schema 数据类型之间的表示差异。
type FieldKind int

const (
	// FieldKindString 字符串类型。
	FieldKindString FieldKind = iota
	// FieldKindBool 布尔类型。
	FieldKindBool
	// FieldKindTime 时间类型。
	FieldKindTime
	// FieldKindBytes 字节切片类型。
	FieldKindBytes
	// FieldKindFloat 浮点类型。
	FieldKindFloat
	// FieldKindInt 整数类型。
	FieldKindInt
	// FieldKindOther 其他类型，类型拼写原样透传。
	FieldKindOther
)

// NormalizeColumnType 把源库类型拼写翻译为多数据库通用的中立方言。
//
// keep=false 表示该类型方言敏感（时间、布尔、字节、浮点），应去掉 type 标签交由 GORM 按目标驱动推导；
// size>0 表示源类型携带长度（仅带长度的二进制列），需要同时写入 size 标签，保证 MySQL 生成可索引的
// varbinary(N) 而不是无长度 blob。生成器与运行时迁移兜底共用本映射，保证两端行为一致。
func NormalizeColumnType(rawType string, kind FieldKind) (neutral string, size int, keep bool) {
	value := strings.ToLower(strings.TrimSpace(rawType))
	// PostgreSQL、达梦等不支持无符号整数，统一去掉 unsigned 修饰。
	value = strings.TrimSuffix(value, " unsigned")
	length := ""
	if index := strings.Index(value, "("); index >= 0 {
		length = value[index:]
		value = strings.TrimSpace(value[:index])
	}
	switch value {
	case "datetime", "timestamp", "timestamptz", "timestamp with time zone", "timestamp without time zone":
		// 时间精度与时区语义各库差异大，交给驱动按 Go 类型推导。
		if kind == FieldKindTime {
			return "", 0, false
		}
		return value + length, 0, true
	case "date", "time":
		// 各数据库都支持 date/time 原生类型，保留原拼写。
		return value + length, 0, true
	case "tinyint":
		// tinyint(1) 承载布尔语义；数值语义在 PostgreSQL、达梦没有对应类型，统一为 smallint。
		if kind == FieldKindBool {
			return "", 0, false
		}
		return "smallint", 0, true
	case "boolean", "bool":
		if kind == FieldKindBool {
			return "", 0, false
		}
		return value, 0, true
	case "integer":
		return "int", 0, true
	case "int":
		// 去掉 MySQL 显示宽度，如 int(11)。
		return "int", 0, true
	case "smallint", "bigint":
		return value, 0, true
	case "character varying", "character":
		return "varchar" + length, 0, true
	case "varchar", "char":
		return value + length, 0, true
	case "tinytext", "mediumtext", "longtext":
		return "text", 0, true
	case "text", "clob":
		return value, 0, true
	case "json":
		// 达梦等国产数据库没有 JSON 类型，统一用文本承载 JSON 字符串。
		return "text", 0, true
	case "enum", "set":
		return "text", 0, true
	case "float", "double":
		// MySQL double/float 在 PostgreSQL 没有同名类型，交给驱动推导。
		if kind == FieldKindFloat {
			return "", 0, false
		}
		return value + length, 0, true
	case "decimal", "numeric", "real":
		// 带精度的定点类型各库通用，保留原拼写。
		return value + length, 0, true
	case "blob", "tinyblob", "mediumblob", "longblob", "bytea":
		if kind == FieldKindBytes {
			return "", 0, false
		}
		return value, 0, true
	case "varbinary", "binary":
		if kind == FieldKindBytes {
			// 二进制长度迁入 size 标签：MySQL 据此生成 varbinary(N)，索引不要求键长度。
			if length == "" {
				return "", 0, false
			}
			parsed, err := strconv.Atoi(strings.Trim(length, "()"))
			if err != nil {
				return "", 0, false
			}
			return "", parsed, false
		}
		// 字符串承载的二进制列按原长度归一为 varchar，避免无标签 string 退化成默认长度。
		return "varchar" + length, 0, true
	default:
		return value + length, 0, true
	}
}
