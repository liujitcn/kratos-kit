package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	httpx "github.com/liujitcn/go-utils/http"
	"github.com/liujitcn/kratos-kit/cache"
	notify "github.com/liujitcn/kratos-kit/notify"
	"github.com/liujitcn/kratos-kit/notify/internal/platformhttp"
	"github.com/liujitcn/kratos-kit/notify/internal/token"
)

const defaultBaseURL = "https://open.feishu.cn/open-apis"

// Config 描述飞书自建应用消息配置。
type Config struct {
	// Name 是渠道配置实例名。
	Name string
	// AppID 是飞书应用 App ID。
	AppID string
	// AppSecret 是飞书应用 App Secret。
	AppSecret string
	// ReceiveIDType 是接收者标识类型，支持 open_id、user_id、union_id、email 和 chat_id。
	ReceiveIDType string
	// BaseURL 是飞书 OpenAPI 地址；留空使用官方地址。
	BaseURL string
	// AllowHTTP 仅供受控测试环境使用。
	AllowHTTP bool
	// Timeout 是 HTTP 请求超时。
	Timeout time.Duration
	// Transport 可注入底层标准库连接策略；请求仍由 go-utils/http 发送。
	Transport notify.HTTPTransport
	// Cache 是宿主统一管理的平台令牌缓存。
	Cache cache.Cache
}

// AppSender 向飞书用户或群会话发送应用消息。
type AppSender struct {
	name          string
	appID         string
	appSecret     string
	receiveIDType string
	baseURL       string
	client        *httpx.Client
	tokens        *token.Cache
}

// NewAppSender 创建飞书应用消息发送器。
func NewAppSender(config Config) (*AppSender, error) {
	if config.Name == "" || config.AppID == "" || config.AppSecret == "" || config.Cache == nil {
		return nil, errors.New("notify feishu: name, app_id, app_secret and shared cache are required")
	}
	receiveIDType := config.ReceiveIDType
	if receiveIDType == "" {
		receiveIDType = "open_id"
	}
	switch receiveIDType {
	case "open_id", "user_id", "union_id", "email", "chat_id":
	default:
		return nil, fmt.Errorf("notify feishu: unsupported receive_id_type %q", receiveIDType)
	}
	baseURL, err := platformhttp.BaseURL(config.BaseURL, defaultBaseURL, config.AllowHTTP)
	if err != nil {
		return nil, err
	}
	tokenCache, err := token.New(config.Cache, notify.FeishuApp, config.Name, config.AppID, config.AppSecret)
	if err != nil {
		return nil, err
	}
	return &AppSender{
		name: config.Name, appID: config.AppID, appSecret: config.AppSecret,
		receiveIDType: receiveIDType, baseURL: baseURL,
		client: platformhttp.Client(config.Transport, config.Timeout), tokens: tokenCache,
	}, nil
}

// Name 返回飞书应用渠道实例名。
func (s *AppSender) Name() string { return s.name }

// Type 返回飞书应用消息渠道类型。
func (s *AppSender) Type() notify.Type { return notify.FeishuApp }

// Send 将文本或 Markdown 消息逐个发送给飞书接收者。
func (s *AppSender) Send(ctx context.Context, message notify.Message) (*notify.Receipt, error) {
	if len(message.Recipients) == 0 || message.Content == "" {
		return nil, errors.New("notify feishu: recipients and content are required")
	}
	tokenValue, err := s.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	receipt := &notify.Receipt{}
	for _, recipient := range message.Recipients {
		id := s.recipientID(recipient)
		if id == "" {
			return receipt, fmt.Errorf("notify feishu: recipient is missing %s", s.receiveIDType)
		}
		content, messageType, marshalErr := encodeContent(message)
		if marshalErr != nil {
			return receipt, marshalErr
		}
		query := url.Values{"receive_id_type": []string{s.receiveIDType}}
		body := map[string]string{"receive_id": id, "msg_type": messageType, "content": content}
		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				MessageID string `json:"message_id"`
			} `json:"data"`
		}
		err = platformhttp.DoJSON(ctx, s.client, http.MethodPost, s.baseURL+"/im/v1/messages", query, tokenValue, body, &result)
		if err != nil {
			return receipt, err
		}
		if result.Code != 0 {
			return receipt, fmt.Errorf("notify feishu: API error %d: %s", result.Code, result.Msg)
		}
		receipt.MessageID = result.Data.MessageID
		receipt.AcceptedRecipients = append(receipt.AcceptedRecipients, id)
	}
	return receipt, nil
}

// accessToken 返回当前飞书应用实例缓存的 tenant access token。
func (s *AppSender) accessToken(ctx context.Context) (string, error) {
	return s.tokens.Get(ctx, func(ctx context.Context) (string, time.Duration, error) {
		var result struct {
			Code              int    `json:"code"`
			Msg               string `json:"msg"`
			TenantAccessToken string `json:"tenant_access_token"`
			Expire            int64  `json:"expire"`
		}
		err := platformhttp.DoJSON(ctx, s.client, http.MethodPost,
			s.baseURL+"/auth/v3/tenant_access_token/internal", nil, "",
			map[string]string{"app_id": s.appID, "app_secret": s.appSecret}, &result)
		if err != nil {
			return "", 0, err
		}
		if result.Code != 0 {
			return "", 0, fmt.Errorf("notify feishu: token API error %d: %s", result.Code, result.Msg)
		}
		return result.TenantAccessToken, time.Duration(result.Expire) * time.Second, nil
	})
}

// recipientID 从结构化接收人中读取所配置的飞书接收标识。
func (s *AppSender) recipientID(recipient notify.Recipient) string {
	switch s.receiveIDType {
	case "open_id":
		return recipient.OpenID
	case "union_id":
		return recipient.UnionID
	case "user_id":
		return recipient.UserID
	case "email":
		return recipient.Email
	case "chat_id":
		return recipient.ChatID
	default:
		return ""
	}
}

// encodeContent 转换飞书 text 或 post 消息正文。
func encodeContent(message notify.Message) (string, string, error) {
	if message.ContentType == notify.Markdown {
		content, err := json.Marshal(map[string]any{
			"zh_cn": map[string]any{
				"title":   message.Title,
				"content": [][]map[string]string{{{"tag": "md", "text": message.Content}}},
			},
		})
		return string(content), "post", err
	}
	text := message.Content
	if message.Title != "" {
		text = message.Title + "\n" + message.Content
	}
	content, err := json.Marshal(map[string]string{"text": text})
	return string(content), "text", err
}
