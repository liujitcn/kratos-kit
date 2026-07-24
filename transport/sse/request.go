package sse

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// StreamIDResolver 从 HTTP 请求中解析 stream ID。
type StreamIDResolver func(*http.Request) (string, error)

// ResolveStreamIDFromQuery 返回基于 query 参数的 stream 解析器。
func ResolveStreamIDFromQuery(key string) StreamIDResolver {
	return func(r *http.Request) (string, error) {
		if r == nil || r.URL == nil {
			return "", fmt.Errorf("request url is nil")
		}
		return r.URL.Query().Get(key), nil
	}
}

// ResolveStreamIDFromPathPrefix 返回基于路径前缀的 stream 解析器。
func ResolveStreamIDFromPathPrefix(prefix string) StreamIDResolver {
	return func(r *http.Request) (string, error) {
		if r == nil || r.URL == nil {
			return "", fmt.Errorf("request url is nil")
		}
		if prefix == "" {
			return strings.Trim(r.URL.Path, "/"), nil
		}
		return strings.TrimPrefix(r.URL.Path, prefix), nil
	}
}

// ResolveStreamIDFromPathVar 返回基于路径变量的 stream 解析器。
func ResolveStreamIDFromPathVar(name string) StreamIDResolver {
	return func(r *http.Request) (string, error) {
		if r == nil {
			return "", fmt.Errorf("request is nil")
		}
		return mux.Vars(r)[name], nil
	}
}
