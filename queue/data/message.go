package data

// Message 表示队列消息及其重试状态。
type Message struct {
	ID string
	// Values 保存消息字段。
	Values     map[string]any
	ErrorCount int
}

type ConsumerFunc func(Message) error
