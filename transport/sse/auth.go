package sse

import (
	"errors"
	"net/http"
	"strings"
)

var (
	// ErrUnauthorized 表示 SSE 请求未通过身份认证。
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden 表示 SSE 请求已认证但无权订阅。
	ErrForbidden = errors.New("forbidden")
)

// TokenExtractor 从 HTTP 请求中提取认证令牌。
type TokenExtractor func(r *http.Request) string

// AuthorizeFunc 校验 SSE 请求及其认证令牌。
type AuthorizeFunc func(r *http.Request, token string) error

// DefaultTokenExtractor 依次从 Authorization、X-Token 和查询参数中提取令牌。
func DefaultTokenExtractor(r *http.Request) string {
	if r == nil {
		return ""
	}
	if token := extractBearerToken(r.Header.Get("Authorization")); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.Header.Get("X-Token")); token != "" {
		return token
	}
	if r.URL == nil {
		return ""
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// isForbidden 判断授权错误是否应返回 HTTP 403。
func isForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// extractBearerToken 解析 Authorization 头中的 Bearer 令牌并兼容裸令牌。
func extractBearerToken(authorization string) string {
	authorization = strings.TrimSpace(authorization)
	if len(authorization) >= 7 && strings.EqualFold(authorization[:7], "Bearer ") {
		return strings.TrimSpace(authorization[7:])
	}
	return authorization
}
