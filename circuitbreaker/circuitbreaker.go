package circuitbreaker

import (
	"context"
	"fmt"
	"sync"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	kratosbreaker "github.com/go-kratos/kratos/v3/middleware/circuitbreaker"
	"github.com/go-kratos/kratos/v3/transport"
)

// State 表示熔断器状态。
type State int

const (
	// StateClosed 表示请求正常放行。
	StateClosed State = iota
	// StateOpen 表示请求被熔断器拒绝。
	StateOpen
	// StateHalfOpen 表示熔断器正在执行恢复探测。
	StateHalfOpen
)

// String 返回熔断器状态名称。
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Permit 表示一次已经放行的请求。
// Finish 必须在请求结束时调用一次，nil 表示成功或不计入熔断的业务错误。
type Permit interface {
	Finish(error)
}

// Breaker 定义带请求级完成令牌的熔断器。
type Breaker interface {
	Allow() (Permit, error)
}

// Factory 创建一个按 operation 隔离的熔断器。
type Factory func() Breaker

type breakerGroup struct {
	mu       sync.Mutex
	factory  Factory
	breakers map[string]Breaker
}

// Client 创建 Kratos 客户端熔断中间件。
func Client(factory Factory) middleware.Middleware {
	if factory == nil {
		panic("circuitbreaker: factory is nil")
	}

	group := &breakerGroup{
		factory:  factory,
		breakers: make(map[string]Breaker),
	}
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (reply any, err error) {
			info, _ := transport.FromClientContext(ctx)
			operation := ""
			if info != nil {
				operation = info.Operation()
			}

			breaker := group.get(operation)
			permit, err := breaker.Allow()
			if err != nil {
				return nil, kratosbreaker.ErrNotAllowed
			}

			defer func() {
				panicValue := recover()
				if panicValue != nil {
					panicErr, ok := panicValue.(error)
					if !ok {
						panicErr = fmt.Errorf("circuitbreaker: handler panic: %v", panicValue)
					}
					permit.Finish(panicErr)
					panic(panicValue)
				}
				if isFailure(err) {
					permit.Finish(err)
				} else {
					permit.Finish(nil)
				}
			}()
			return handler(ctx, req)
		}
	}
}

// get 返回指定 operation 的熔断器，不存在时创建。
func (g *breakerGroup) get(operation string) Breaker {
	g.mu.Lock()
	defer g.mu.Unlock()

	breaker, ok := g.breakers[operation]
	if !ok {
		breaker = g.factory()
		if breaker == nil {
			panic("circuitbreaker: factory returned nil breaker")
		}
		g.breakers[operation] = breaker
	}
	return breaker
}

// isFailure 判断错误是否应计入客户端熔断统计。
func isFailure(err error) bool {
	return err != nil &&
		(kratoserrors.IsInternalServer(err) ||
			kratoserrors.IsServiceUnavailable(err) ||
			kratoserrors.IsGatewayTimeout(err))
}
