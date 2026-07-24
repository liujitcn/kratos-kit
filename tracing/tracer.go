package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const defaultTracerName = "kratos-transport"

type Tracer struct {
	tracer trace.Tracer
	opt    *options
}

func NewTracer(kind trace.SpanKind, spanName string, opts ...Option) *Tracer {
	op := options{
		propagator: propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{}),
		kind:       kind,
		tracerName: defaultTracerName,
	}
	for _, o := range opts {
		o(&op)
	}
	if op.tracerProvider != nil {
		otel.SetTracerProvider(op.tracerProvider)
	}
	op.spanName = spanName

	switch kind {
	case trace.SpanKindProducer, trace.SpanKindConsumer:
		return &Tracer{tracer: otel.Tracer(op.tracerName), opt: &op}
	case trace.SpanKindServer, trace.SpanKindClient:
		return &Tracer{tracer: otel.Tracer(op.tracerName), opt: &op}
	default:
		panic(fmt.Sprintf("unsupported span kind: %v", kind))
	}
}

// Inject 将当前上下文中的追踪信息注入到传播载体。
func (t *Tracer) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	t.opt.propagator.Inject(ctx, carrier)
}

// Start 使用默认 span 名称启动链路追踪 span。
func (t *Tracer) Start(ctx context.Context, carrier propagation.TextMapCarrier, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.StartWithName(ctx, t.opt.spanName, carrier, attrs...)
}

// StartWithName 使用指定 span 名称启动链路追踪 span。
func (t *Tracer) StartWithName(ctx context.Context, spanName string, carrier propagation.TextMapCarrier, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if t.opt.kind == trace.SpanKindServer || t.opt.kind == trace.SpanKindConsumer {
		ctx = t.opt.propagator.Extract(ctx, carrier)
	}

	opts := []trace.SpanStartOption{
		trace.WithAttributes(attrs...),
		trace.WithSpanKind(t.opt.kind),
	}

	ctx, span := t.tracer.Start(ctx, spanName, opts...)

	if t.opt.kind == trace.SpanKindClient || t.opt.kind == trace.SpanKindProducer {
		t.Inject(ctx, carrier)
	}

	return ctx, span
}

// End 结束链路追踪 span，并在错误时标记状态。
func (t *Tracer) End(ctx context.Context, span trace.Span, err error, attrs ...attribute.KeyValue) {
	if span == nil {
		return
	}
	if !span.IsRecording() {
		return
	}

	span.SetAttributes(attrs...)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}

	span.End()
}
