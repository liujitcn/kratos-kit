package redisqueue

type Message struct {
	ID     string
	Stream string
	Values map[string]interface{}
}
