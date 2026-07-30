package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// TCPChecker 通过建立 TCP 连接检查目标地址是否可达。
type TCPChecker struct {
	address string
	timeout time.Duration
}

// TCP 创建 TCP 检查器。
func TCP(address string, timeout time.Duration) *TCPChecker {
	return &TCPChecker{address: address, timeout: timeout}
}

// Check 执行 TCP 连接检查。
func (t *TCPChecker) Check(ctx context.Context) Result {
	timeout := t.timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var dialer net.Dialer
	connection, err := dialer.DialContext(checkCtx, "tcp", t.address)
	if err != nil {
		return Result{
			Status:  StatusDown,
			Message: fmt.Sprintf("tcp dial %s: %v", t.address, err),
		}
	}
	_ = connection.Close()
	return Result{Status: StatusUp}
}

// HTTPChecker 通过 HTTP GET 请求检查目标端点。
type HTTPChecker struct {
	url     string
	timeout time.Duration
}

// HTTP 创建 HTTP 检查器。
func HTTP(url string, timeout time.Duration) *HTTPChecker {
	return &HTTPChecker{url: url, timeout: timeout}
}

// Check 执行 HTTP 检查，2xx 和 3xx 响应视为可用。
func (h *HTTPChecker) Check(ctx context.Context) Result {
	timeout := h.timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(checkCtx, http.MethodGet, h.url, nil)
	if err != nil {
		return Result{Status: StatusDown, Message: fmt.Sprintf("build HTTP request: %v", err)}
	}
	request.Header.Set("User-Agent", "kratos-kit-health-checker")

	var response *http.Response
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		return Result{
			Status:  StatusDown,
			Message: fmt.Sprintf("HTTP request %s: %v", h.url, err),
		}
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusBadRequest {
		return Result{Status: StatusUp}
	}
	return Result{
		Status:  StatusDown,
		Message: fmt.Sprintf("HTTP request %s returned status %d", h.url, response.StatusCode),
	}
}

// All 表示所有子检查都必须成功的组合检查器。
type All struct {
	checkers []Checker
}

// AllCheckers 创建全部成功组合检查器。
func AllCheckers(checkers ...Checker) *All {
	return &All{checkers: checkers}
}

// Check 依次执行检查器，遇到不可用结果时立即返回。
func (a *All) Check(ctx context.Context) Result {
	for _, checker := range a.checkers {
		result := checker.Check(ctx)
		if result.Status != StatusUp {
			return result
		}
	}
	return Result{Status: StatusUp}
}

// Any 表示任意一个子检查成功即可的组合检查器。
type Any struct {
	checkers []Checker
}

// AnyCheckers 创建任意成功组合检查器。
func AnyCheckers(checkers ...Checker) *Any {
	return &Any{checkers: checkers}
}

// Check 依次执行检查器，遇到可用结果时立即返回。
func (a *Any) Check(ctx context.Context) Result {
	var lastResult Result
	for _, checker := range a.checkers {
		lastResult = checker.Check(ctx)
		if lastResult.Status == StatusUp {
			return lastResult
		}
	}
	if lastResult.Message == "" {
		lastResult.Message = "no checker passed"
	}
	lastResult.Status = StatusDown
	return lastResult
}
