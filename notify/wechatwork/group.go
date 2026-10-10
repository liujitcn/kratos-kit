package wechatwork

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	httpx "github.com/liujitcn/go-utils/http"
	notify "github.com/liujitcn/kratos-kit/notify"
	"github.com/liujitcn/kratos-kit/notify/internal/platformhttp"
)

// GroupConfig 描述企业微信群机器人配置。
type GroupConfig struct {
	// Name 是渠道配置实例名。
	Name string
	// URL 是群机器人完整 Webhook 地址，通常包含 key 查询参数。
	URL string
	// AllowHTTP 仅供受控测试环境使用。
	AllowHTTP bool
	// AllowPrivateNetwork 允许明确受控的内网机器人地址。
	AllowPrivateNetwork bool
	// Timeout 是 HTTP 请求超时。
	Timeout time.Duration
}

// GroupSender 向企业微信群机器人发送消息。
type GroupSender struct {
	name   string
	url    *url.URL
	client *httpx.Client
}

// NewGroupSender 创建企业微信群机器人发送器。
func NewGroupSender(config GroupConfig) (*GroupSender, error) {
	if config.Name == "" {
		return nil, errors.New("notify wechatwork group: name is required")
	}
	parsedURL, err := url.ParseRequestURI(config.URL)
	if err != nil || parsedURL.Hostname() == "" || parsedURL.User != nil || parsedURL.Fragment != "" {
		return nil, errors.New("notify wechatwork group: valid webhook URL is required")
	}
	if parsedURL.Scheme != "https" && (!config.AllowHTTP || parsedURL.Scheme != "http") {
		return nil, errors.New("notify wechatwork group: HTTPS is required unless AllowHTTP is explicitly enabled")
	}
	return &GroupSender{
		name: config.Name, url: parsedURL,
		client: platformhttp.SafeClient(config.Timeout, config.AllowPrivateNetwork),
	}, nil
}

// Name 返回企业微信群机器人配置实例名。
func (s *GroupSender) Name() string { return s.name }

// Type 返回企业微信群机器人渠道类型。
func (s *GroupSender) Type() notify.Type { return notify.WechatWorkGroup }

// Send 发送企业微信群文本或 Markdown 消息。
func (s *GroupSender) Send(ctx context.Context, message notify.Message) (*notify.Receipt, error) {
	if message.Content == "" {
		return nil, errors.New("notify wechatwork group: content is required")
	}
	msgType := "text"
	content := map[string]string{"content": message.Content}
	if message.ContentType == notify.Markdown {
		msgType = "markdown"
	}
	body := map[string]any{"msgtype": msgType, msgType: content}
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := platformhttp.DoJSON(ctx, s.client, http.MethodPost, s.url.String(), nil, "", body, &result); err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("notify wechatwork group: API error %d: %s", result.ErrCode, result.ErrMsg)
	}
	return &notify.Receipt{}, nil
}
