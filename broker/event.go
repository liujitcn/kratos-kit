package broker

import (
	"context"
	"slices"
)

// Event defines the message event interface
type Event interface {
	// Topic returns the topic of the event
	Topic() string

	// Message returns the message associated with the event
	Message() *Message
	// RawMessage returns the original/raw message
	RawMessage() any

	// Ack acknowledges the message
	Ack() error

	// Error returns any error associated with the event
	Error() error
}

// Handler defines the handler invoked by subscribers
type Handler func(ctx context.Context, evt Event) error

// SubscriberMiddleware is broker subscriber middleware.
type SubscriberMiddleware func(Handler) Handler

// ChainSubscriberMiddleware 按声明顺序组合订阅中间件，跳过空中间件。
func ChainSubscriberMiddleware(h Handler, mws []SubscriberMiddleware) Handler {
	if len(mws) == 0 {
		return h
	}
	for _, mw := range slices.Backward(mws) {
		if mw == nil {
			continue
		}
		h = mw(h)
	}
	return h
}
