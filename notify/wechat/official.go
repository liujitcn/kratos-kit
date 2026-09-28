package wechat

import (
	"context"
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

const defaultBaseURL = "https://api.weixin.qq.com/cgi-bin"

// Config 描述微信公众号模板消息渠道。
type Config struct {
	// Name 是渠道配置实例名。
	Name string
	// AppID 是公众号 AppID。
	AppID string
	// AppSecret 是公众号 AppSecret。
	AppSecret string
	// BaseURL 是公众号 API 地址；留空使用官方地址。
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

// OfficialSender 发送微信公众号模板消息。
type OfficialSender struct {
	name      string
	appID     string
	appSecret string
	baseURL   string
	client    *httpx.Client
	tokens    *token.Cache
}

// NewOfficialSender 创建微信公众号模板消息发送器。
func NewOfficialSender(config Config) (*OfficialSender, error) {
	if config.Name == "" || config.AppID == "" || config.AppSecret == "" || config.Cache == nil {
		return nil, errors.New("notify wechat: name, app_id, app_secret and shared cache are required")
	}
	baseURL, err := platformhttp.BaseURL(config.BaseURL, defaultBaseURL, config.AllowHTTP)
	if err != nil {
		return nil, err
	}
	tokenCache, err := token.New(config.Cache, notify.WechatOfficial, config.Name, config.AppID, config.AppSecret)
	if err != nil {
		return nil, err
	}
	return &OfficialSender{
		name: config.Name, appID: config.AppID, appSecret: config.AppSecret,
		baseURL: baseURL, client: platformhttp.Client(config.Transport, config.Timeout), tokens: tokenCache,
	}, nil
}

// Name 返回微信公众号渠道实例名。
func (s *OfficialSender) Name() string { return s.name }

// Type 返回微信公众号模板消息渠道类型。
func (s *OfficialSender) Type() notify.Type { return notify.WechatOfficial }

// Send 按收件人的 OpenID 逐个发送公众号模板消息。
func (s *OfficialSender) Send(ctx context.Context, message notify.Message) (*notify.Receipt, error) {
	if len(message.Recipients) == 0 || message.Template == nil || message.Template.Code == "" {
		return nil, errors.New("notify wechat: recipients and template code are required")
	}
	tokenValue, err := s.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	data := make(map[string]map[string]string, len(message.Template.Params))
	for name, value := range message.Template.Params {
		data[name] = map[string]string{"value": value}
	}
	receipt := &notify.Receipt{}
	for _, recipient := range message.Recipients {
		if recipient.OpenID == "" {
			return receipt, errors.New("notify wechat: recipient open_id is required")
		}
		body := map[string]any{
			"touser": recipient.OpenID, "template_id": message.Template.Code, "data": data,
		}
		if message.Template.URL != "" {
			body["url"] = message.Template.URL
		}
		var result struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
			MsgID   int64  `json:"msgid"`
		}
		query := url.Values{"access_token": []string{tokenValue}}
		err = platformhttp.DoJSON(ctx, s.client, http.MethodPost,
			s.baseURL+"/message/template/send", query, "", body, &result)
		if err != nil {
			return receipt, err
		}
		if result.ErrCode != 0 {
			return receipt, fmt.Errorf("notify wechat: API error %d: %s", result.ErrCode, result.ErrMsg)
		}
		receipt.MessageID = fmt.Sprintf("%d", result.MsgID)
		receipt.AcceptedRecipients = append(receipt.AcceptedRecipients, recipient.OpenID)
	}
	return receipt, nil
}

// accessToken 返回当前公众号实例缓存的 access token。
func (s *OfficialSender) accessToken(ctx context.Context) (string, error) {
	return s.tokens.Get(ctx, func(ctx context.Context) (string, time.Duration, error) {
		query := url.Values{
			"grant_type": []string{"client_credential"},
			"appid":      []string{s.appID},
			"secret":     []string{s.appSecret},
		}
		var result struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
			ErrCode     int    `json:"errcode"`
			ErrMsg      string `json:"errmsg"`
		}
		err := platformhttp.DoJSON(ctx, s.client, http.MethodGet, s.baseURL+"/token", query, "", nil, &result)
		if err != nil {
			return "", 0, err
		}
		if result.ErrCode != 0 {
			return "", 0, fmt.Errorf("notify wechat: token API error %d: %s", result.ErrCode, result.ErrMsg)
		}
		return result.AccessToken, time.Duration(result.ExpiresIn) * time.Second, nil
	})
}
