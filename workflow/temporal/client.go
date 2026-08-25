package temporal

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

const (
	defaultHostPort  = "localhost:7233"
	defaultNamespace = "default"

	tracerMessageSystemKey = "temporal"
	tracerName             = "github.com/liujitcn/kratos-kit/workflow/temporal"
	spanNameProducer       = "temporal-producer"
	spanNameConsumer       = "temporal-consumer"
)

// WorkflowClient 提供 Temporal 工作流操作的高层封装。
type WorkflowClient struct {
	client  client.Client
	options ClientOptions

	tracer trace.Tracer
}

// NewClient 创建并连接 Temporal 客户端。
func NewClient(opts ...func(*ClientOptions)) (*WorkflowClient, error) {
	options := ClientOptions{
		HostPort:  defaultHostPort,
		Namespace: defaultNamespace,
	}
	for _, o := range opts {
		o(&options)
	}

	c, err := client.NewClient(client.Options{
		HostPort:  options.HostPort,
		Namespace: options.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to temporal server at %s: %w", options.HostPort, err)
	}

	log.Info(fmt.Sprintf("connected to temporal server at %s (namespace: %s)", options.HostPort, options.Namespace))

	return &WorkflowClient{
		client:  c,
		options: options,
	}, nil
}

// WithTracing 为客户端启用 OpenTelemetry 链路追踪。
func (wc *WorkflowClient) WithTracing() {
	wc.tracer = otel.Tracer(tracerName)
}

// Close 关闭底层 Temporal 客户端连接。
func (wc *WorkflowClient) Close() error {
	if wc.client != nil {
		wc.client.Close()
	}
	log.Info("disconnected from temporal server")
	return nil
}

// TemporalClient 返回底层 Temporal SDK 客户端，供高级场景使用。
func (wc *WorkflowClient) TemporalClient() client.Client {
	return wc.client
}

// Execute 异步启动一次工作流执行。
// 返回本次执行的 run ID。
func (wc *WorkflowClient) Execute(ctx context.Context, args any, opts ExecuteOptions) (string, error) {
	if wc.client == nil {
		return "", fmt.Errorf("temporal client is not connected")
	}

	workflowFn := opts.WorkflowFn
	if workflowFn == nil {
		workflowFn = BrokerMessageWorkflow
	}

	swo := toStartWorkflowOptions(opts)

	ctx, span := wc.startProducerSpan(ctx, opts.TaskQueue)

	we, err := wc.client.ExecuteWorkflow(ctx, swo, workflowFn, args)

	wc.finishProducerSpan(span, err)

	if err != nil {
		return "", fmt.Errorf("execute workflow error: %w", err)
	}

	return we.GetRunID(), nil
}

// ExecuteSync 启动工作流并等待完成，返回工作流结果。
func (wc *WorkflowClient) ExecuteSync(ctx context.Context, args any, opts ExecuteOptions) ([]byte, error) {
	if wc.client == nil {
		return nil, fmt.Errorf("temporal client is not connected")
	}

	workflowFn := opts.WorkflowFn
	if workflowFn == nil {
		workflowFn = BrokerMessageWorkflow
	}

	swo := toStartWorkflowOptions(opts)

	ctx, span := wc.startProducerSpan(ctx, opts.TaskQueue)

	we, err := wc.client.ExecuteWorkflow(ctx, swo, workflowFn, args)

	if err != nil {
		wc.finishProducerSpan(span, err)
		return nil, fmt.Errorf("execute workflow error: %w", err)
	}

	var result []byte
	if err = we.Get(ctx, &result); err != nil {
		wc.finishProducerSpan(span, err)
		return nil, fmt.Errorf("get workflow result error: %w", err)
	}

	wc.finishProducerSpan(span, nil)

	return result, nil
}

// Signal 向正在运行的工作流发送信号。
func (wc *WorkflowClient) Signal(ctx context.Context, workflowID, runID, signalName string, arg any) error {
	if wc.client == nil {
		return fmt.Errorf("temporal client is not connected")
	}
	return wc.client.SignalWorkflow(ctx, workflowID, runID, signalName, arg)
}

// Query 查询正在运行的工作流状态。
func (wc *WorkflowClient) Query(ctx context.Context, workflowID, runID, queryType string, arg any) (any, error) {
	if wc.client == nil {
		return nil, fmt.Errorf("temporal client is not connected")
	}
	result, err := wc.client.QueryWorkflow(ctx, workflowID, runID, queryType, arg)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Cancel 请求取消正在运行的工作流。
func (wc *WorkflowClient) Cancel(ctx context.Context, workflowID, runID string) error {
	if wc.client == nil {
		return fmt.Errorf("temporal client is not connected")
	}
	return wc.client.CancelWorkflow(ctx, workflowID, runID)
}

// Describe 获取工作流执行描述信息。
func (wc *WorkflowClient) Describe(ctx context.Context, workflowID, runID string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	if wc.client == nil {
		return nil, fmt.Errorf("temporal client is not connected")
	}

	result, err := wc.client.DescribeWorkflowExecution(ctx, workflowID, runID)
	if err != nil {
		return nil, fmt.Errorf("describe workflow error: %w", err)
	}
	return result, nil
}

////////////////////////////////////////////////////////////////////////////////
/// OpenTelemetry 链路追踪辅助方法
////////////////////////////////////////////////////////////////////////////////

func (wc *WorkflowClient) startProducerSpan(ctx context.Context, topic string) (context.Context, trace.Span) {
	if wc.tracer == nil {
		return ctx, nil
	}

	return wc.tracer.Start(ctx, spanNameProducer,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String(tracerMessageSystemKey),
			semconv.MessagingDestinationKindTopic,
			semconv.MessagingDestinationKey.String(topic),
		),
	)
}

func (wc *WorkflowClient) finishProducerSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func (wc *WorkflowClient) startConsumerSpan(ctx context.Context, topic string) (context.Context, trace.Span) {
	if wc.tracer == nil {
		return ctx, nil
	}

	return wc.tracer.Start(ctx, spanNameConsumer,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String(tracerMessageSystemKey),
			semconv.MessagingDestinationKindTopic,
			semconv.MessagingDestinationKey.String(topic),
			semconv.MessagingOperationReceive,
		),
	)
}

func (wc *WorkflowClient) finishConsumerSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
