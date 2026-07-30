package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/liujitcn/kratos-kit/tracing"
)

var _ propagation.TextMapCarrier = messageCarrier{}

type messageCarrier struct {
	message *nats.Msg
}

// Get 读取 NATS 消息 header。
func (c messageCarrier) Get(key string) string {
	if c.message == nil || c.message.Header == nil {
		return ""
	}
	return c.message.Header.Get(key)
}

// Set 写入 NATS 消息 header。
func (c messageCarrier) Set(key, value string) {
	if c.message == nil {
		return
	}
	if c.message.Header == nil {
		c.message.Header = make(nats.Header)
	}
	c.message.Header.Set(key, value)
}

// Keys 返回 NATS 消息全部 header 名称。
func (c messageCarrier) Keys() []string {
	if c.message == nil {
		return nil
	}
	keys := make([]string, 0, len(c.message.Header))
	for key := range c.message.Header {
		keys = append(keys, key)
	}
	return keys
}

// startProducerSpan 启动消息生产 span 并注入传播信息。
func startProducerSpan(ctx context.Context, tracer *tracing.Tracer, system string, message *nats.Msg) (context.Context, trace.Span) {
	if tracer == nil {
		return ctx, nil
	}
	return tracer.Start(
		ctx,
		messageCarrier{message: message},
		attribute.String("messaging.system", system),
		attribute.String("messaging.destination.name", message.Subject),
		attribute.String("messaging.operation.type", "publish"),
	)
}

// startConsumerSpan 提取传播信息并启动消息消费 span。
func startConsumerSpan(ctx context.Context, tracer *tracing.Tracer, system string, message *nats.Msg) (context.Context, trace.Span) {
	if tracer == nil {
		return ctx, nil
	}
	return tracer.Start(
		ctx,
		messageCarrier{message: message},
		attribute.String("messaging.system", system),
		attribute.String("messaging.destination.name", message.Subject),
		attribute.String("messaging.operation.type", "receive"),
	)
}

// finishSpan 结束消息 span 并记录错误。
func finishSpan(ctx context.Context, tracer *tracing.Tracer, span trace.Span, err error) {
	if tracer != nil {
		tracer.End(ctx, span, err)
	}
}
