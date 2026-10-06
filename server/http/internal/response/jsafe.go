package response

import (
	"strconv"

	"encoding/json"
	jsonv2 "encoding/json/v2"
)

// jsSafeIntMax 表示 JS Number 可精确表示的最大安全整数，即 2^53-1。
const jsSafeIntMax = 9007199254740991

// jsSafeMarshalers 提供 64 位整数的类型级序列化规则：
// 绝对值未超出 JS 安全整数范围时保持数字输出，超出后输出字符串，避免浏览器 JSON.parse 丢失精度。
// 仅匹配 int64/uint64 本身，float32/float64 与 time.Duration 等命名类型不受影响。
var jsSafeMarshalers = jsonv2.JoinMarshalers(
	jsonv2.MarshalFunc(func(v int64) ([]byte, error) {
		if v >= -jsSafeIntMax && v <= jsSafeIntMax {
			return []byte(strconv.FormatInt(v, 10)), nil
		}
		return []byte(strconv.Quote(strconv.FormatInt(v, 10))), nil
	}),
	jsonv2.MarshalFunc(func(v uint64) ([]byte, error) {
		if v <= jsSafeIntMax {
			return []byte(strconv.FormatUint(v, 10)), nil
		}
		return []byte(strconv.Quote(strconv.FormatUint(v, 10))), nil
	}),
)

// jsSafeUnmarshalers 提供 64 位整数的类型级反序列化规则：同一字段同时接受数字与字符串两种 JSON 形态。
var jsSafeUnmarshalers = jsonv2.JoinUnmarshalers(
	jsonv2.UnmarshalFunc(func(data []byte, v *int64) error {
		return unmarshalJSSafeInt(data, v)
	}),
	jsonv2.UnmarshalFunc(func(data []byte, v *uint64) error {
		return unmarshalJSSafeUint(data, v)
	}),
)

// MarshalJSSafe 以 JS 安全整数规则编码 JSON，输出形态与 encoding/json v1 保持一致。
func MarshalJSSafe(v any) ([]byte, error) {
	return jsonv2.Marshal(v, json.DefaultOptionsV1(), jsonv2.WithMarshalers(jsSafeMarshalers))
}

// UnmarshalJSSafe 以 JS 安全整数规则解码 JSON，64 位整数同时接受数字与字符串。
func UnmarshalJSSafe(data []byte, v any) error {
	return jsonv2.Unmarshal(data, v, json.DefaultOptionsV1(), jsonv2.WithUnmarshalers(jsSafeUnmarshalers))
}

// unmarshalJSSafeInt 将数字或字符串形态的 JSON 值解析为 int64。
func unmarshalJSSafeInt(data []byte, v *int64) error {
	if string(data) == "null" {
		return nil
	}
	text, err := unquoteJSSafeNumber(data)
	if err != nil {
		return err
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}

// unmarshalJSSafeUint 将数字或字符串形态的 JSON 值解析为 uint64。
func unmarshalJSSafeUint(data []byte, v *uint64) error {
	if string(data) == "null" {
		return nil
	}
	text, err := unquoteJSSafeNumber(data)
	if err != nil {
		return err
	}
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}

// unquoteJSSafeNumber 去掉字符串形态 JSON 值的引号，数字形态原样返回。
func unquoteJSSafeNumber(data []byte) (string, error) {
	text := string(data)
	if len(text) > 1 && text[0] == '"' && text[len(text)-1] == '"' {
		return strconv.Unquote(text)
	}
	return text, nil
}
