package data

type Message struct {
	ID         string
	Values     map[string]interface{}
	ErrorCount int
}

type ConsumerFunc func(Message) error
