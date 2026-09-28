package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	httpx "github.com/liujitcn/go-utils/http"
	notify "github.com/liujitcn/kratos-kit/notify"
	"github.com/liujitcn/kratos-kit/notify/internal/platformhttp"
)

const defaultSignatureHeader = "X-Notification-Signature"

// Config 描述一个通用 Webhook 渠道实例。
type Config struct {
	// Name 是渠道配置实例名，在一个 Manager 中必须唯一。
	Name string
	// URL 是接收端地址；默认只允许 HTTPS。
	URL string
	// Secret 用于对请求体计算 HMAC-SHA256 签名；留空时不签名。
	Secret string
	// SignatureHeader 是签名头名称；有 Secret 且未指定时使用默认值。
	SignatureHeader string
	// Headers 是附加请求头。
	Headers map[string]string
	// Timeout 是请求超时；非正数时使用 10 秒。
	Timeout time.Duration
	// AllowHTTP 仅用于明确受控的内网或本地接收端。
	AllowHTTP bool
	// AllowPrivateNetwork 允许解析到私网地址；默认拒绝私网、环回、链路本地和保留地址。
	AllowPrivateNetwork bool
}

// Sender 使用 JSON 和可选 HMAC-SHA256 签名调用通用 Webhook。
type Sender struct {
	name            string
	url             string
	secret          string
	signatureHeader string
	headers         map[string]string
	client          *httpx.Client
}

type payload struct {
	Title       string             `json:"title,omitempty"`
	Content     string             `json:"content"`
	ContentType notify.ContentType `json:"content_type,omitempty"`
	Metadata    map[string]string  `json:"metadata,omitempty"`
}

// New 创建并校验 Webhook 发送器。
func New(config Config) (*Sender, error) {
	if config.Name == "" {
		return nil, errors.New("notify webhook: name is required")
	}
	parsedURL, err := url.ParseRequestURI(config.URL)
	if err != nil || parsedURL.Hostname() == "" || parsedURL.User != nil || parsedURL.Fragment != "" {
		return nil, errors.New("notify webhook: valid URL without user info or fragment is required")
	}
	if parsedURL.Scheme != "https" && !(config.AllowHTTP && parsedURL.Scheme == "http") {
		return nil, errors.New("notify webhook: HTTPS is required unless AllowHTTP is explicitly enabled")
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	for key, value := range config.Headers {
		canonicalKey := textproto.CanonicalMIMEHeaderKey(key)
		if canonicalKey == "" || isRestrictedHeader(canonicalKey) || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("notify webhook: invalid header %q", key)
		}
	}
	headers := make(map[string]string, len(config.Headers))
	for key, value := range config.Headers {
		canonicalKey := textproto.CanonicalMIMEHeaderKey(key)
		if _, exists := headers[canonicalKey]; exists {
			return nil, fmt.Errorf("notify webhook: duplicate header %q", canonicalKey)
		}
		headers[canonicalKey] = value
	}
	signatureHeader := config.SignatureHeader
	if signatureHeader == "" {
		signatureHeader = defaultSignatureHeader
	}
	canonicalSignatureHeader := textproto.CanonicalMIMEHeaderKey(signatureHeader)
	if canonicalSignatureHeader == "" || isRestrictedHeader(canonicalSignatureHeader) || canonicalSignatureHeader == "Content-Type" {
		return nil, fmt.Errorf("notify webhook: invalid signature header %q", signatureHeader)
	}
	client := platformhttp.SafeClient(timeout, config.AllowPrivateNetwork)
	return &Sender{
		name: config.Name, url: parsedURL.String(), secret: config.Secret,
		signatureHeader: signatureHeader, headers: headers, client: client,
	}, nil
}

// Name 返回 Webhook 渠道配置实例名。
func (s *Sender) Name() string { return s.name }

// Type 返回通用 Webhook 渠道类型。
func (s *Sender) Type() notify.Type { return notify.Webhook }

// Send 使用 JSON 请求体发送通知。
func (s *Sender) Send(ctx context.Context, message notify.Message) (*notify.Receipt, error) {
	body, err := json.Marshal(payload{
		Title: message.Title, Content: message.Content, ContentType: message.ContentType,
		Metadata: message.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("notify webhook: encode payload: %w", err)
	}
	options := []httpx.RequestOption{
		httpx.WithContext(ctx),
		httpx.WithHeader("Content-Type", "application/json"),
		httpx.WithBodyBytes(body),
	}
	for key, value := range s.headers {
		options = append(options, httpx.WithHeader(key, value))
	}
	if s.secret != "" {
		signature := hmac.New(sha256.New, []byte(s.secret))
		_, _ = signature.Write(body)
		options = append(options, httpx.WithHeader(s.signatureHeader, "sha256="+hex.EncodeToString(signature.Sum(nil))))
	}
	response, err := s.client.Do(http.MethodPost, s.url, options...)
	if err != nil {
		return nil, fmt.Errorf("notify webhook: send request: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("notify webhook: HTTP status %d", response.StatusCode)
	}
	return &notify.Receipt{MessageID: response.Header.Get("X-Message-ID")}, nil
}

// isRestrictedHeader 判断是否为禁止覆盖的传输层请求头。
func isRestrictedHeader(name string) bool {
	switch name {
	case "Connection", "Content-Length", "Host", "Proxy-Authorization", "Transfer-Encoding":
		return true
	default:
		return false
	}
}
