package redisqueue

// Message 表示 Redis Stream 中的消息。
type Message struct {
	ID     string
	Stream string
	// Values 保存消息字段。
	Values map[string]any
}
