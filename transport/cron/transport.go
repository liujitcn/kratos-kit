package cron

import (
	"github.com/go-kratos/kratos/v3/selector"
	kratosTransport "github.com/go-kratos/kratos/v3/transport"
)

const (
	KindCron = "cron"
)

var _ kratosTransport.Transporter = &Transport{}

// Transport 表示 Cron 传输上下文。
type Transport struct {
	endpoint    string
	operation   string
	reqHeader   headerCarrier
	replyHeader headerCarrier
	nodeFilters []selector.NodeFilter
}

// Kind 返回传输类型。
func (tr *Transport) Kind() kratosTransport.Kind {
	return KindCron
}

// Endpoint 返回传输端点。
func (tr *Transport) Endpoint() string {
	return tr.endpoint
}

// Operation 返回当前操作名称。
func (tr *Transport) Operation() string {
	return tr.operation
}

// RequestHeader 返回请求头载体。
func (tr *Transport) RequestHeader() kratosTransport.Header {
	return tr.reqHeader
}

// ReplyHeader 返回响应头载体。
func (tr *Transport) ReplyHeader() kratosTransport.Header {
	return tr.replyHeader
}

// NodeFilters 返回客户端节点筛选器。
func (tr *Transport) NodeFilters() []selector.NodeFilter {
	return tr.nodeFilters
}

type headerCarrier struct{}

// Get 返回指定键对应的值。
func (hc headerCarrier) Get(_ string) string {
	return ""
}

// Set 写入指定键值对。
func (hc headerCarrier) Set(_ string, _ string) {

}

// Keys 返回当前载体中的所有键。
func (hc headerCarrier) Keys() []string {
	return nil
}

// Add 追加指定键的值。
func (hc headerCarrier) Add(_ string, _ string) {

}

// Values 返回指定键对应的全部值。
func (hc headerCarrier) Values(_ string) []string {
	return nil
}
