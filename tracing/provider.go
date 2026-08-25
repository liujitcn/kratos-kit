package tracing

import (
	"fmt"

	"uuid"

	"go.opentelemetry.io/otel/sdk/resource"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
	semConv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

// NewTracerProvider 创建一个链路追踪器，并返回导出器或实例 ID 初始化错误。
func NewTracerProvider(exporterName, endpoint, serviceName, instanceId, version string, sampler float64) (*traceSdk.TracerProvider, error) {
	if instanceId == "" {
		instanceId = uuid.NewV7().String()
	}
	if version == "" {
		version = "x.x.x"
	}

	opts := []traceSdk.TracerProviderOption{
		traceSdk.WithSampler(traceSdk.ParentBased(traceSdk.TraceIDRatioBased(sampler))),
		traceSdk.WithResource(resource.NewSchemaless(
			semConv.ServiceNameKey.String(serviceName),
			semConv.ServiceInstanceIDKey.String(instanceId),
			semConv.ServiceVersionKey.String(version),
		)),
	}

	if len(endpoint) > 0 {
		exp, err := NewExporter(exporterName, endpoint, true)
		if err != nil {
			return nil, fmt.Errorf("tracing: create exporter: %w", err)
		}

		opts = append(opts, traceSdk.WithBatcher(exp))
	}

	return traceSdk.NewTracerProvider(opts...), nil
}
