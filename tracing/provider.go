package tracing

import (
	"fmt"

	"uuid"

	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

// NewTracerProvider 创建一个链路追踪器，并返回导出器或实例 ID 初始化错误。
func NewTracerProvider(exporterName, endpoint, serviceName, instanceId, version string, sampler float64) (*trace.TracerProvider, error) {
	if instanceId == "" {
		instanceId = uuid.NewV7().String()
	}
	if version == "" {
		version = "x.x.x"
	}

	opts := []trace.TracerProviderOption{
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(sampler))),
		trace.WithResource(resource.NewSchemaless(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceInstanceIDKey.String(instanceId),
			semconv.ServiceVersionKey.String(version),
		)),
	}

	if len(endpoint) > 0 {
		exp, err := NewExporter(exporterName, endpoint, true)
		if err != nil {
			return nil, fmt.Errorf("tracing: create exporter: %w", err)
		}

		opts = append(opts, trace.WithBatcher(exp))
	}

	return trace.NewTracerProvider(opts...), nil
}
