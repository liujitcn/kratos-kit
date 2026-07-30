package metrics

import "context"

// Metrics 定义与监控后端无关的指标记录接口。
type Metrics interface {
	// Counter 增加一个单调递增计数器。
	Counter(ctx context.Context, name string, value int64, labels map[string]string)
	// Histogram 记录一次分布观测值。
	Histogram(ctx context.Context, name string, value float64, labels map[string]string)
	// Gauge 设置一个瞬时值。
	Gauge(ctx context.Context, name string, value float64, labels map[string]string)
}

// Closer 定义持有后台资源的指标提供者关闭接口。
type Closer interface {
	Close() error
}
