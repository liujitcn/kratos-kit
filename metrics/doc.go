// Package metrics 定义与监控后端无关的指标记录接口。
//
// Provider 可将相同的业务指标发送到 Prometheus、OpenTelemetry 或
// Datadog。Counter、Histogram 和 Gauge 都接收 context、指标名称、
// 数值和字符串标签。
//
// 示例：
//
//	provider, _ := prometheus.New(prometheus.WithNamespace("myapp"))
//	provider.Counter(ctx, "requests_total", 1, map[string]string{"method": "GET"})
//	provider.Histogram(ctx, "request_duration_seconds", 0.042, nil)
//	provider.Gauge(ctx, "queue_depth", 42, map[string]string{"queue": "email"})
package metrics
