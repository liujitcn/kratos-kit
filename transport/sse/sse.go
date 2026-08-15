package sse

// SSEStream 描述一条业务 SSE 流及其订阅参数解析规则。
type SSEStream interface {
	// ID 返回业务流的稳定标识。
	ID() string
	// Resolve 将频道号和用户号解析为实际传输流标识。
	Resolve(channelID string, userID int64) (string, error)
}
