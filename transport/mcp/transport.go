package mcp

import (
	"net/http"

	"github.com/go-kratos/kratos/v3/selector"
	kratosTransport "github.com/go-kratos/kratos/v3/transport"
)

const (
	// KindMCP 是 MCP transport 类型。
	KindMCP = "mcp"
)

var _ kratosTransport.Transporter = &Transport{}

// Transport 表示 MCP transport 元信息。
type Transport struct {
	endpoint    string
	operation   string
	reqHeader   headerCarrier
	replyHeader headerCarrier
	nodeFilters []selector.NodeFilter
}

// Kind 返回 transport 类型。
func (tr *Transport) Kind() kratosTransport.Kind {
	return KindMCP
}

// Endpoint 返回 transport 端点。
func (tr *Transport) Endpoint() string {
	return tr.endpoint
}

// Operation 返回当前操作名称。
func (tr *Transport) Operation() string {
	return tr.operation
}

// RequestHeader 返回请求头。
func (tr *Transport) RequestHeader() kratosTransport.Header {
	return tr.reqHeader
}

// ReplyHeader 返回响应头。
func (tr *Transport) ReplyHeader() kratosTransport.Header {
	return tr.replyHeader
}

// NodeFilters 返回客户端节点筛选器。
func (tr *Transport) NodeFilters() []selector.NodeFilter {
	return tr.nodeFilters
}

type headerCarrier http.Header

// Get 返回指定 key 的值。
func (hc headerCarrier) Get(key string) string {
	return http.Header(hc).Get(key)
}

// Set 设置指定 key 的值。
func (hc headerCarrier) Set(key, value string) {
	http.Header(hc).Set(key, value)
}

// Keys 返回所有 header key。
func (hc headerCarrier) Keys() []string {
	keys := make([]string, 0, len(hc))
	for key := range http.Header(hc) {
		keys = append(keys, key)
	}
	return keys
}

// Add 追加指定 key 的值。
func (hc headerCarrier) Add(key, value string) {
	http.Header(hc).Add(key, value)
}

// Values 返回指定 key 的全部值。
func (hc headerCarrier) Values(key string) []string {
	return http.Header(hc).Values(key)
}
