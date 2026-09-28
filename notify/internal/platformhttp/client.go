package platformhttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	httpx "github.com/liujitcn/go-utils/http"
	notify "github.com/liujitcn/kratos-kit/notify"
)

const maxResponseBytes = 1 << 20

type requestFailure struct {
	cause error
}

// Error 返回不含请求 URL 和查询参数的错误文本。
func (e *requestFailure) Error() string { return "notify platform: request failed" }

// Unwrap 保留底层错误的判定能力。
func (e *requestFailure) Unwrap() error { return e.cause }

// Client 创建 go-utils/http 客户端，并关闭含敏感查询参数的请求日志。
func Client(transport notify.HTTPTransport, timeout time.Duration) *httpx.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if transport == nil {
		transport = &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	} else {
		// WithTimeout 会修改 HTTP 客户端；复制以避免改动调用方共享的实例。
		client := *transport
		transport = &client
	}
	return httpx.NewClient(
		httpx.WithHTTPClient(transport),
		httpx.WithTimeout(timeout),
		httpx.WithMaxResponseBodyBytes(maxResponseBytes),
		httpx.WithRequestLog(false),
	)
}

// BaseURL 校验平台 API 地址并去除末尾斜线。
func BaseURL(value, fallback string, allowHTTP bool) (string, error) {
	if value == "" {
		value = fallback
	}
	parsedURL, err := url.ParseRequestURI(value)
	if err != nil || parsedURL.Hostname() == "" || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return "", errors.New("notify platform: invalid base URL")
	}
	if parsedURL.Scheme != "https" && !(allowHTTP && parsedURL.Scheme == "http") {
		return "", errors.New("notify platform: HTTPS is required unless explicitly overridden")
	}
	return strings.TrimRight(parsedURL.String(), "/"), nil
}

// DoJSON 使用 go-utils/http 发送 JSON 请求并校验 HTTP 状态和响应大小。
func DoJSON(ctx context.Context, client *httpx.Client, method, endpoint string, query url.Values, bearerToken string, requestValue, responseValue any) error {
	return DoJSONWithHeaders(ctx, client, method, endpoint, query, nil, bearerToken, requestValue, responseValue)
}

// DoJSONWithHeaders 发送包含附加请求头的 JSON 请求。
func DoJSONWithHeaders(ctx context.Context, client *httpx.Client, method, endpoint string, query url.Values, headers map[string]string, bearerToken string, requestValue, responseValue any) error {
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("notify platform: invalid request URL: %w", err)
	}
	options := []httpx.RequestOption{
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
	}
	for key, value := range headers {
		options = append(options, httpx.WithHeader(key, value))
	}
	for key, values := range query {
		for _, value := range values {
			options = append(options, httpx.WithQuery(key, value))
		}
	}
	if bearerToken != "" {
		options = append(options, httpx.WithBearerToken(bearerToken))
	}
	if requestValue != nil {
		options = append(options, httpx.WithJSONBody(requestValue))
	}
	var response *httpx.Response
	response, err = client.Do(method, parsedURL.String(), options...)
	if err != nil {
		return &requestFailure{cause: err}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("notify platform: HTTP status %d", response.StatusCode)
	}
	if int64(len(response.Body)) > maxResponseBytes {
		return httpx.ErrResponseBodyTooLarge
	}
	if responseValue == nil {
		return nil
	}
	if len(response.Body) == 0 {
		return errors.New("notify platform: empty JSON response")
	}
	if err = response.DecodeJSON(responseValue); err != nil {
		return fmt.Errorf("notify platform: decode response: %w", err)
	}
	return nil
}
