package feishu

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

// GroupConfig 描述飞书群自定义机器人配置。
type GroupConfig struct {
	// Name 是渠道配置实例名。
	Name string
	// URL 是群机器人 Webhook 地址。
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

// GroupSender 向飞书群机器人发送消息。
type GroupSender struct {
	name   string
	url    *url.URL
	secret string
	client *httpx.Client
}

// NewGroupSender 创建飞书群机器人发送器。
func NewGroupSender(config GroupConfig) (*GroupSender, error) {
	if config.Name == "" {
		return nil, errors.New("notify feishu group: name is required")
	}
	parsedURL, err := url.ParseRequestURI(config.URL)
	if err != nil || parsedURL.Hostname() == "" || parsedURL.User != nil || parsedURL.Fragment != "" {
		return nil, errors.New("notify feishu group: valid webhook URL is required")
	}
	if parsedURL.Scheme != "https" && (!config.AllowHTTP || parsedURL.Scheme != "http") {
		return nil, errors.New("notify feishu group: HTTPS is required unless AllowHTTP is explicitly enabled")
	}
	return &GroupSender{
		name: config.Name, url: parsedURL, secret: config.Secret,
		client: platformhttp.SafeClient(config.Timeout, config.AllowPrivateNetwork),
	}, nil
}

// Name 返回飞书群机器人配置实例名。
func (s *GroupSender) Name() string { return s.name }

// Type 返回飞书群机器人渠道类型。
func (s *GroupSender) Type() notify.Type { return notify.FeishuGroup }

// Send 发送飞书群文本或富文本消息。
func (s *GroupSender) Send(ctx context.Context, message notify.Message) (*notify.Receipt, error) {
	if message.Content == "" {
		return nil, errors.New("notify feishu group: content is required")
	}
	body, err := encodeRobotMessage(message)
	if err != nil {
		return nil, err
	}
	endpoint := *s.url
	if s.secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, []byte(timestamp+"\n"+s.secret))
		_, _ = mac.Write(nil)
		query := endpoint.Query()
		query.Set("timestamp", timestamp)
		query.Set("sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		endpoint.RawQuery = query.Encode()
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err = platformhttp.DoJSON(ctx, s.client, http.MethodPost, endpoint.String(), nil, "", body, &result); err != nil {
		return nil, err
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("notify feishu group: API error %d: %s", result.Code, result.Msg)
	}
	return &notify.Receipt{}, nil
}

// encodeRobotMessage 构建飞书群机器人接受的消息载荷。
func encodeRobotMessage(message notify.Message) (map[string]any, error) {
	if message.ContentType == notify.Markdown {
		return map[string]any{
			"msg_type": "post",
			"content": map[string]any{
				"post": map[string]any{
					"zh_cn": map[string]any{
						"title":   message.Title,
						"content": [][]map[string]string{{{"tag": "md", "text": message.Content}}},
					},
				},
			},
		}, nil
	}
	return map[string]any{"msg_type": "text", "content": map[string]string{"text": message.Content}}, nil
}
