package http

import (
	"github.com/go-kratos/kratos/v3/encoding"
	kratosJSON "github.com/go-kratos/kratos/v3/encoding/json"
	"github.com/liujitcn/kratos-kit/server/http/internal/response"
)

func init() {
	// 覆盖 Kratos 默认 JSON 编解码器，统一走 JS 安全整数规则：
	// 编码时 64 位整数超出 2^53-1 输出字符串，解码时同一字段同时接受数字与字符串，
	// 保证浏览器拿到的大整数可以原样回传。其余行为与 encoding/json v1 保持一致。
	encoding.RegisterCodec(jsSafeJSONCodec{})
}

// jsSafeJSONCodec 基于 encoding/json/v2 的 JSON 编解码器，兜住 Kratos JSON 请求解码与非 protobuf 响应编码。
type jsSafeJSONCodec struct{}

// Marshal 编码 JSON，64 位整数超出 JS 安全整数范围时输出字符串。
func (jsSafeJSONCodec) Marshal(v any) ([]byte, error) {
	return response.MarshalJSSafe(v)
}

// Unmarshal 解码 JSON，64 位整数同时接受数字与字符串两种形态。
func (jsSafeJSONCodec) Unmarshal(data []byte, v any) error {
	return response.UnmarshalJSSafe(data, v)
}

// Name 返回编解码器名称，固定为 json 以替换 Kratos 默认实现。
func (jsSafeJSONCodec) Name() string {
	return kratosJSON.Name
}
