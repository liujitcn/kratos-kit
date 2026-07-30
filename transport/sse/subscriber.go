package sse

import (
	"net/http"
	"net/url"
	"strings"
)

// Subscriber 保存单个 SSE 订阅连接及其请求快照。
type Subscriber struct {
	quit       chan *Subscriber
	connection chan *Event
	removed    chan struct{}
	registered chan struct{}
	streamDone <-chan struct{}
	eventId    string
	URL        *url.URL    // URL 是订阅请求 URL 的快照。
	Header     http.Header // Header 是订阅请求头的快照。
}

// close 从所属事件流移除订阅者，事件流已关闭时直接返回。
func (s *Subscriber) close() {
	select {
	case s.quit <- s:
	case <-s.streamDone:
		return
	}
	if s.removed != nil {
		select {
		case <-s.removed:
		case <-s.streamDone:
		}
	}
}

// HeaderValue 返回订阅请求的指定请求头。
func (s *Subscriber) HeaderValue(key string) string {
	if s == nil || s.Header == nil {
		return ""
	}
	return s.Header.Get(key)
}

// Authorization 返回订阅请求的 Authorization 头。
func (s *Subscriber) Authorization() string {
	return s.HeaderValue("Authorization")
}

// BearerToken 返回订阅请求中的 Bearer 令牌并兼容裸令牌。
func (s *Subscriber) BearerToken() string {
	return extractBearerToken(s.Authorization())
}

// Token 依次从自定义请求头、Authorization 和查询参数中提取令牌。
func (s *Subscriber) Token(headerKey string) string {
	if headerKey != "" {
		if token := strings.TrimSpace(s.HeaderValue(headerKey)); token != "" {
			return token
		}
	}
	if token := s.BearerToken(); token != "" {
		return token
	}
	if s == nil || s.URL == nil {
		return ""
	}
	return strings.TrimSpace(s.URL.Query().Get("token"))
}
