package driver

import "testing"

// TestNormalizeColumnType 验证源库方言类型到中立方言的翻译结果。
func TestNormalizeColumnType(t *testing.T) {
	cases := []struct {
		name     string
		rawType  string
		kind     FieldKind
		expected string
		keep     bool
	}{
		{name: "MySQL时间精度", rawType: "datetime(3)", kind: FieldKindTime, expected: "", keep: false},
		{name: "PG带时区时间", rawType: "timestamp with time zone", kind: FieldKindTime, expected: "", keep: false},
		{name: "PG无时区时间", rawType: "timestamp without time zone", kind: FieldKindTime, expected: "", keep: false},
		{name: "timestamptz", rawType: "timestamptz", kind: FieldKindTime, expected: "", keep: false},
		{name: "date保留", rawType: "date", kind: FieldKindTime, expected: "date", keep: true},
		{name: "布尔tinyint", rawType: "tinyint(1)", kind: FieldKindBool, expected: "", keep: false},
		{name: "PG布尔", rawType: "boolean", kind: FieldKindBool, expected: "", keep: false},
		{name: "数值tinyint降smallint", rawType: "tinyint", kind: FieldKindInt, expected: "smallint", keep: true},
		{name: "integer归一int", rawType: "integer", kind: FieldKindInt, expected: "int", keep: true},
		{name: "去掉unsigned", rawType: "bigint unsigned", kind: FieldKindInt, expected: "bigint", keep: true},
		{name: "int显示宽度", rawType: "int(11)", kind: FieldKindInt, expected: "int", keep: true},
		{name: "smallint保留", rawType: "smallint", kind: FieldKindInt, expected: "smallint", keep: true},
		{name: "character varying", rawType: "character varying(100)", kind: FieldKindString, expected: "varchar(100)", keep: true},
		{name: "PG的char归一char", rawType: "character(64)", kind: FieldKindString, expected: "char(64)", keep: true},
		{name: "varchar保留", rawType: "varchar(64)", kind: FieldKindString, expected: "varchar(64)", keep: true},
		{name: "mediumtext归一text", rawType: "mediumtext", kind: FieldKindString, expected: "text", keep: true},
		{name: "text保留", rawType: "text", kind: FieldKindString, expected: "text", keep: true},
		{name: "json保留中性类型", rawType: "json", kind: FieldKindString, expected: "json", keep: true},
		{name: "jsonb保留中性类型", rawType: "jsonb", kind: FieldKindString, expected: "json", keep: true},
		{name: "blob字节去掉标签", rawType: "blob", kind: FieldKindBytes, expected: "", keep: false},
		{name: "binary字节去掉标签", rawType: "binary(16)", kind: FieldKindBytes, expected: "", keep: false},
		{name: "字符串二进制列", rawType: "varbinary(1023)", kind: FieldKindString, expected: "varchar(1023)", keep: true},
		{name: "浮点交给驱动", rawType: "double", kind: FieldKindFloat, expected: "", keep: false},
		{name: "decimal保留精度", rawType: "decimal(10,2)", kind: FieldKindFloat, expected: "decimal(10,2)", keep: true},
		{name: "未知类型透传", rawType: "uuid", kind: FieldKindOther, expected: "uuid", keep: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value, _, keep := NormalizeColumnType(testCase.rawType, testCase.kind)
			if keep != testCase.keep {
				t.Fatalf("keep 期望 %v 实际 %v（%s）", testCase.keep, keep, value)
			}
			if keep && value != testCase.expected {
				t.Fatalf("类型期望 %s 实际 %s", testCase.expected, value)
			}
		})
	}
}

// TestJSONColumnType 验证中性 json 列按方言落地的原生类型。
func TestJSONColumnType(t *testing.T) {
	cases := []struct {
		name     string
		dialect  string
		expected string
	}{
		{name: "postgres落地jsonb", dialect: "postgres", expected: "jsonb"},
		{name: "方言大小写不敏感", dialect: "Postgres", expected: "jsonb"},
		{name: "oracle落地clob", dialect: "oracle", expected: "clob"},
		{name: "sqlserver落地nvarchar", dialect: "sqlserver", expected: "nvarchar(max)"},
		{name: "sqlite落地text", dialect: "sqlite", expected: "text"},
		{name: "mysql原生json", dialect: "mysql", expected: "json"},
		{name: "doris原生json", dialect: "doris", expected: "json"},
		{name: "未知方言默认json", dialect: "unknown", expected: "json"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if value := JSONColumnType(testCase.dialect); value != testCase.expected {
				t.Fatalf("类型期望 %s 实际 %s", testCase.expected, value)
			}
		})
	}
}

// TestNormalizeColumnTypeBinarySize 验证带长度的二进制列把长度迁入 size。
func TestNormalizeColumnTypeBinarySize(t *testing.T) {
	cases := []struct {
		name    string
		rawType string
		size    int
	}{
		{name: "binary带长度", rawType: "binary(32)", size: 32},
		{name: "varbinary带长度", rawType: "varbinary(1023)", size: 1023},
		{name: "blob无长度", rawType: "blob", size: 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, size, keep := NormalizeColumnType(testCase.rawType, FieldKindBytes)
			if keep {
				t.Fatal("字节类型应移除 type 标签")
			}
			if size != testCase.size {
				t.Fatalf("size 期望 %d 实际 %d", testCase.size, size)
			}
		})
	}
}
