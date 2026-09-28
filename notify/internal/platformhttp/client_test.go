package platformhttp

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDoJSONRedactsQueryFromRequestErrors(t *testing.T) {
	transportError := errors.New("connection refused")
	client := Client(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, transportError
	})}, time.Second)
	var result map[string]any
	err := DoJSON(context.Background(), client, http.MethodGet, "https://example.test/token",
		url.Values{"access_token": {"sensitive-token"}}, "", nil, &result)
	if err == nil || strings.Contains(err.Error(), "sensitive-token") {
		t.Fatalf("expected a redacted request error, got %v", err)
	}
	if !errors.Is(err, transportError) {
		t.Fatalf("expected wrapped transport error, got %v", err)
	}
}

func TestDoJSONRejectsEmptyExpectedResponse(t *testing.T) {
	client := Client(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: http.NoBody}, nil
	})}, time.Second)
	var result map[string]any
	if err := DoJSON(context.Background(), client, http.MethodPost, "https://example.test/send", nil, "", nil, &result); err == nil {
		t.Fatal("expected an error for an empty JSON response")
	}
}

func TestClientDoesNotMutateInjectedHTTPClient(t *testing.T) {
	injectedClient := &http.Client{Timeout: 30 * time.Second}
	_ = Client(injectedClient, time.Second)
	if injectedClient.Timeout != 30*time.Second {
		t.Fatalf("injected client timeout was modified: %s", injectedClient.Timeout)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip 使用函数实现执行测试请求。
func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
