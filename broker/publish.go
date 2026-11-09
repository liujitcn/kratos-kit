package broker

import (
	"context"
	"slices"
)

// PublishHandler defines the publishing handler
type PublishHandler func(ctx context.Context, topic string, msg *Message, opts ...PublishOption) error

// PublishMiddleware defines the publishing middleware
type PublishMiddleware func(PublishHandler) PublishHandler

// ChainPublishMiddleware 按声明顺序组合发布中间件，跳过空中间件。
func ChainPublishMiddleware(h PublishHandler, mws []PublishMiddleware) PublishHandler {
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

// WrapLegacyPublishHandler wraps a legacy PublishHandler to the new PublishHandler
func WrapLegacyPublishHandler(h func(ctx context.Context, topic string, msg any, opts ...PublishOption) error) PublishHandler {
	return func(ctx context.Context, topic string, msg *Message, opts ...PublishOption) error {
		var payload any
		if msg != nil {
			if msg.Msg != nil {
				payload = msg.Msg
			} else {
				payload = msg.Body
			}
		}
		return h(ctx, topic, payload, opts...)
	}
}
