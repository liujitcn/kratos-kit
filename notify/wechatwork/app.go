package wechatwork

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	httpx "github.com/liujitcn/go-utils/http"
	"github.com/liujitcn/kratos-kit/cache"
	notify "github.com/liujitcn/kratos-kit/notify"
	"github.com/liujitcn/kratos-kit/notify/internal/platformhttp"
	"github.com/liujitcn/kratos-kit/notify/internal/token"
)

const defaultBaseURL = "https://qyapi.weixin.qq.com/cgi-bin"

// AppConfig 描述企业微信自建应用消息配置。
type AppConfig struct {
	// Name 是渠道配置实例名。
	Name string
	// CorpID 是企业 ID。
	CorpID string
	// AgentID 是应用 Agent ID。
	AgentID int
	// CorpSecret 是应用 Secret。
	CorpSecret string
	// BaseURL 是企业微信 API 地址；留空使用官方地址。
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

// AppSender 向企业微信应用用户发送消息。
type AppSender struct {
	name       string
	corpID     string
	agentID    int
	corpSecret string
	baseURL    string
	client     *httpx.Client
	tokens     *token.Cache
}

// NewAppSender 创建企业微信应用消息发送器。
func NewAppSender(config AppConfig) (*AppSender, error) {
	if config.Name == "" || config.CorpID == "" || config.CorpSecret == "" || config.AgentID <= 0 || config.Cache == nil {
		return nil, errors.New("notify wechatwork: name, corp_id, corp_secret, agent_id and shared cache are required")
	}
	baseURL, err := platformhttp.BaseURL(config.BaseURL, defaultBaseURL, config.AllowHTTP)
	if err != nil {
		return nil, err
	}
	tokenCache, err := token.New(config.Cache, notify.WechatWorkApp, config.Name, config.CorpID, config.CorpSecret)
	if err != nil {
		return nil, err
	}
	return &AppSender{
		name: config.Name, corpID: config.CorpID, agentID: config.AgentID,
		corpSecret: config.CorpSecret, baseURL: baseURL,
		client: platformhttp.Client(config.Transport, config.Timeout), tokens: tokenCache,
	}, nil
}

// Name 返回企业微信应用渠道实例名。
func (s *AppSender) Name() string { return s.name }

// Type 返回企业微信应用消息渠道类型。
func (s *AppSender) Type() notify.Type { return notify.WechatWorkApp }

// Send 向企业微信用户发送文本或 Markdown 消息。
func (s *AppSender) Send(ctx context.Context, message notify.Message) (*notify.Receipt, error) {
	if len(message.Recipients) == 0 || message.Content == "" {
		return nil, errors.New("notify wechatwork: recipients and content are required")
	}
	userIDs := make([]string, 0, len(message.Recipients))
	for _, recipient := range message.Recipients {
		if recipient.UserID == "" {
			return nil, errors.New("notify wechatwork: recipient user_id is required")
		}
		userIDs = append(userIDs, recipient.UserID)
	}
	tokenValue, err := s.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	msgType := "text"
	msgBody := map[string]string{"content": message.Content}
	if message.ContentType == notify.Markdown {
		msgType = "markdown"
	}
	body := map[string]any{
		"touser": strings.Join(userIDs, "|"), "msgtype": msgType, "agentid": s.agentID,
		msgType: msgBody, "safe": 0,
	}
	query := url.Values{"access_token": []string{tokenValue}}
	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		MsgID       string `json:"msgid"`
		InvalidUser string `json:"invaliduser"`
	}
	if err = platformhttp.DoJSON(ctx, s.client, http.MethodPost, s.baseURL+"/message/send", query, "", body, &result); err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("notify wechatwork: API error %d: %s", result.ErrCode, result.ErrMsg)
	}
	invalidUsers := make(map[string]struct{})
	for _, userID := range strings.Split(result.InvalidUser, "|") {
		if userID != "" {
			invalidUsers[userID] = struct{}{}
		}
	}
	acceptedRecipients := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		if _, invalid := invalidUsers[userID]; !invalid {
			acceptedRecipients = append(acceptedRecipients, userID)
		}
	}
	receipt := &notify.Receipt{MessageID: result.MsgID, AcceptedRecipients: acceptedRecipients}
	if result.InvalidUser != "" {
		return receipt, fmt.Errorf("notify wechatwork: provider rejected users: %s", result.InvalidUser)
	}
	return receipt, nil
}

// accessToken 返回当前企业微信应用实例缓存的 access token。
func (s *AppSender) accessToken(ctx context.Context) (string, error) {
	return s.tokens.Get(ctx, func(ctx context.Context) (string, time.Duration, error) {
		query := url.Values{"corpid": []string{s.corpID}, "corpsecret": []string{s.corpSecret}}
		var result struct {
			ErrCode     int    `json:"errcode"`
			ErrMsg      string `json:"errmsg"`
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
		}
		err := platformhttp.DoJSON(ctx, s.client, http.MethodGet, s.baseURL+"/gettoken", query, "", nil, &result)
		if err != nil {
			return "", 0, err
		}
		if result.ErrCode != 0 {
			return "", 0, fmt.Errorf("notify wechatwork: token API error %d: %s", result.ErrCode, result.ErrMsg)
		}
		return result.AccessToken, time.Duration(result.ExpiresIn) * time.Second, nil
	})
}
