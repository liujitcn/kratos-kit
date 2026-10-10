package dingtalk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	httpx "github.com/liujitcn/go-utils/http"
	notify "github.com/liujitcn/kratos-kit/notify"
	"github.com/liujitcn/kratos-kit/notify/internal/platformhttp"
)

// GroupConfig 描述钉钉群机器人配置。
type GroupConfig struct {
	// Name 是渠道配置实例名。
	Name string
	// URL 是自定义机器人 Webhook 地址。
	URL string
	// Secret 是机器人加签密钥；未启用加签时留空。
	Secret string
	// AllowHTTP 仅供受控测试环境使用。
	AllowHTTP bool
	// AllowPrivateNetwork 允许明确受控的内网机器人地址。
	AllowPrivateNetwork bool
	// Timeout 是 HTTP 请求超时。
	Timeout time.Duration
}

// GroupSender 向钉钉群机器人发送文本或 Markdown 消息。
type GroupSender struct {
	name   string
	url    *url.URL
	secret string
	client *httpx.Client
}

// NewGroupSender 创建钉钉群机器人发送器。
func NewGroupSender(config GroupConfig) (*GroupSender, error) {
	if config.Name == "" {
		return nil, errors.New("notify dingtalk group: name is required")
	}
	parsedURL, err := url.ParseRequestURI(config.URL)
	if err != nil || parsedURL.Hostname() == "" || parsedURL.User != nil || parsedURL.Fragment != "" {
		return nil, errors.New("notify dingtalk group: valid webhook URL is required")
	}
	if parsedURL.Scheme != "https" && (!config.AllowHTTP || parsedURL.Scheme != "http") {
		return nil, errors.New("notify dingtalk group: HTTPS is required unless AllowHTTP is explicitly enabled")
	}
	return &GroupSender{
		name: config.Name, url: parsedURL, secret: config.Secret,
		client: platformhttp.SafeClient(config.Timeout, config.AllowPrivateNetwork),
	}, nil
}

// Name 返回钉钉群机器人配置实例名。
func (s *GroupSender) Name() string { return s.name }

// Type 返回钉钉群机器人渠道类型。
func (s *GroupSender) Type() notify.Type { return notify.DingTalk }

// Send 发送钉钉群文本或 Markdown 消息。
func (s *GroupSender) Send(ctx context.Context, message notify.Message) (*notify.Receipt, error) {
	if message.Content == "" {
		return nil, errors.New("notify dingtalk group: content is required")
	}
	endpoint := *s.url
	if s.secret != "" {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		mac := hmac.New(sha256.New, []byte(s.secret))
		_, _ = mac.Write([]byte(timestamp + "\n" + s.secret))
		query := endpoint.Query()
		query.Set("timestamp", timestamp)
		query.Set("sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		endpoint.RawQuery = query.Encode()
	}
	payload := map[string]any{"msgtype": "text", "text": map[string]string{"content": message.Content}}
	if message.ContentType == notify.Markdown {
		payload = map[string]any{
			"msgtype":  "markdown",
			"markdown": map[string]string{"title": message.Title, "text": message.Content},
		}
	}
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := platformhttp.DoJSON(ctx, s.client, http.MethodPost, endpoint.String(), nil, "", payload, &result); err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("notify dingtalk group: API error %d: %s", result.ErrCode, result.ErrMsg)
	}
	return &notify.Receipt{}, nil
}
