package health

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

// Status 表示组件的健康状态。
type Status int

const (
	// StatusUnknown 表示尚未检查或检查结果不确定。
	StatusUnknown Status = iota
	// StatusUp 表示组件可用。
	StatusUp
	// StatusDown 表示组件不可用。
	StatusDown
)

// String 返回健康状态的字符串表示。
func (s Status) String() string {
	switch s {
	case StatusUp:
		return "up"
	case StatusDown:
		return "down"
	default:
		return "unknown"
	}
}

// Result 表示单次检查或聚合检查的结果。
type Result struct {
	// Status 是检查结果状态。
	Status Status `json:"status"`
	// Message 是可选的检查消息。
	Message string `json:"message,omitempty"`
	// Details 是可选的结构化检查详情。
	Details map[string]any `json:"details,omitempty"`
}

// Checker 定义一个支持 context 取消的健康检查。
type Checker interface {
	Check(ctx context.Context) Result
}

// PingFunc 将返回 error 的函数适配为 Checker。
type PingFunc func(ctx context.Context) error

// Check 执行 PingFunc。
func (f PingFunc) Check(ctx context.Context) Result {
	if f == nil {
		return Result{Status: StatusUnknown, Message: "checker is nil"}
	}
	err := f(ctx)
	if err != nil {
		return Result{Status: StatusDown, Message: err.Error()}
	}
	return Result{Status: StatusUp}
}

// Health 管理多个命名检查器，并发聚合检查结果。
type Health struct {
	mu       sync.RWMutex
	checkers map[string]Checker
	timeout  time.Duration
}

// Option 配置 Health。
type Option func(*Health)

// WithTimeout 设置一次聚合检查允许占用的最长时间。
func WithTimeout(timeout time.Duration) Option {
	return func(health *Health) {
		if timeout > 0 {
			health.timeout = timeout
		}
	}
}

// New 创建 Health，默认检查超时为五秒。
func New(options ...Option) *Health {
	health := &Health{
		checkers: make(map[string]Checker),
		timeout:  5 * time.Second,
	}
	for _, option := range options {
		option(health)
	}
	return health
}

// Register 注册或替换一个命名检查器。
func (h *Health) Register(name string, checker Checker) {
	if name == "" {
		panic("health: checker name cannot be empty")
	}
	if checker == nil {
		panic("health: checker cannot be nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// Deregister 删除一个命名检查器。
func (h *Health) Deregister(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.checkers, name)
}

// Check 并发执行所有检查器，并返回聚合结果。
func (h *Health) Check(ctx context.Context) Result {
	h.mu.RLock()
	names := make([]string, 0, len(h.checkers))
	checkers := make(map[string]Checker, len(h.checkers))
	for name, checker := range h.checkers {
		names = append(names, name)
		checkers[name] = checker
	}
	timeout := h.timeout
	h.mu.RUnlock()

	if len(names) == 0 {
		return Result{Status: StatusUp, Message: "no checkers registered"}
	}
	slices.Sort(names)

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type namedResult struct {
		name   string
		result Result
	}
	results := make(chan namedResult, len(names))
	pending := make(map[string]struct{}, len(names))
	for _, name := range names {
		pending[name] = struct{}{}
		checker := checkers[name]
		go func() {
			results <- namedResult{name: name, result: checker.Check(checkCtx)}
		}()
	}

	overall := StatusUp
	details := make(map[string]any, len(names))
	for len(pending) > 0 {
		select {
		case item := <-results:
			if _, ok := pending[item.name]; !ok {
				continue
			}
			delete(pending, item.name)
			details[item.name] = resultDetails(item.result)
			if item.result.Status == StatusDown {
				overall = StatusDown
			} else if item.result.Status == StatusUnknown && overall == StatusUp {
				overall = StatusUnknown
			}
		case <-checkCtx.Done():
			for _, name := range names {
				if _, ok := pending[name]; !ok {
					continue
				}
				details[name] = map[string]any{
					"status":  StatusDown.String(),
					"message": fmt.Sprintf("checker %q: %v", name, checkCtx.Err()),
				}
			}
			return Result{Status: StatusDown, Details: details}
		}
	}
	return Result{Status: overall, Details: details}
}

// Names 按名称排序返回所有已注册检查器。
func (h *Health) Names() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	names := make([]string, 0, len(h.checkers))
	for name := range h.checkers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// resultDetails 将检查结果转换为聚合详情。
func resultDetails(result Result) map[string]any {
	details := make(map[string]any, len(result.Details)+2)
	details["status"] = result.Status.String()
	if result.Message != "" {
		details["message"] = result.Message
	}
	for key, value := range result.Details {
		details[key] = value
	}
	return details
}
