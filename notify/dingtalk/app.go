package dingtalk

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

const defaultBaseURL = "https://oapi.dingtalk.com"

// Config 描述钉钉企业内部应用消息渠道。
type Config struct {
	// Name 是渠道配置实例名。
	Name string
	// AppKey 是钉钉应用标识。
	AppKey string
	// AppSecret 是钉钉应用密钥。
	AppSecret string
	// AgentID 是钉钉应用 Agent ID。
	AgentID int64
	// BaseURL 是钉钉 OpenAPI 地址；留空使用官方地址。
	BaseURL string
	// AllowHTTP 仅供受控测试环境使用。
	AllowHTTP bool
	// Timeout 是 HTTP 请求超时。
	Timeout time.Duration
	// Transport 可注入底层标准库连接策略；请求仍由 go-utils/http 发送。
	Transport notify.HTTPTransport
	// Cache 是由宿主统一管理的缓存实例，用于应用令牌缓存。
	Cache cache.Cache
}

// AppSender 向钉钉企业内部应用用户发送工作通知。
type AppSender struct {
	name      string
	appKey    string
	appSecret string
	agentID   int64
	baseURL   string
	client    *httpx.Client
	tokens    *token.Cache
}

type apiResult struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// NewAppSender 创建钉钉应用消息发送器。
func NewAppSender(config Config) (*AppSender, error) {
	if config.Name == "" || config.AppKey == "" || config.AppSecret == "" || config.AgentID <= 0 || config.Cache == nil {
		return nil, errors.New("notify dingtalk: name, app credentials, agent ID and shared cache are required")
	}
	baseURL, err := platformhttp.BaseURL(config.BaseURL, defaultBaseURL, config.AllowHTTP)
	if err != nil {
		return nil, err
	}
	tokenCache, err := token.New(config.Cache, notify.DingTalk, config.Name, config.AppKey, config.AppSecret)
	if err != nil {
		return nil, err
	}
	return &AppSender{
		name: config.Name, appKey: config.AppKey, appSecret: config.AppSecret,
		agentID: config.AgentID, baseURL: baseURL,
		client: platformhttp.Client(config.Transport, config.Timeout), tokens: tokenCache,
	}, nil
}

// Name 返回钉钉应用渠道实例名。
func (s *AppSender) Name() string { return s.name }

// Type 返回钉钉应用消息渠道类型。
func (s *AppSender) Type() notify.Type { return notify.DingTalk }

// Send 将文本工作通知发送给指定钉钉用户。
func (s *AppSender) Send(ctx context.Context, message notify.Message) (*notify.Receipt, error) {
	if len(message.Recipients) == 0 || message.Content == "" {
		return nil, errors.New("notify dingtalk: recipients and content are required")
	}
	accessToken, err := s.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	userIDs := make([]string, 0, len(message.Recipients))
	for _, recipient := range message.Recipients {
		userID := recipient.UserID
		if userID == "" && recipient.UnionID != "" {
			userID, err = s.userID(ctx, accessToken, recipient.UnionID)
			if err != nil {
				return nil, err
			}
		}
		if userID == "" {
			return nil, errors.New("notify dingtalk: recipient requires user_id or union_id")
		}
		userIDs = append(userIDs, userID)
	}
	var result struct {
		apiResult
		TaskID    int64  `json:"task_id"`
		RequestID string `json:"request_id"`
	}
	endpoint := s.baseURL + "/topapi/message/corpconversation/asyncsend_v2"
	query := url.Values{"access_token": []string{accessToken}}
	body := map[string]any{
		"agent_id":    s.agentID,
		"userid_list": strings.Join(userIDs, ","),
		"msg":         map[string]any{"msgtype": "text", "text": map[string]string{"content": message.Content}},
	}
	if err = platformhttp.DoJSON(ctx, s.client, http.MethodPost, endpoint, query, "", body, &result); err != nil {
		return nil, err
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("notify dingtalk: API error %d: %s", result.ErrCode, result.ErrMsg)
	}
	messageID := result.RequestID
	if messageID == "" && result.TaskID != 0 {
		messageID = fmt.Sprint(result.TaskID)
	}
	return &notify.Receipt{MessageID: messageID, AcceptedRecipients: userIDs}, nil
}

// accessToken 返回当前应用实例缓存的 access token。
func (s *AppSender) accessToken(ctx context.Context) (string, error) {
	return s.tokens.Get(ctx, func(ctx context.Context) (string, time.Duration, error) {
		query := url.Values{"appkey": []string{s.appKey}, "appsecret": []string{s.appSecret}}
		var result struct {
			apiResult
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
		}
		if err := platformhttp.DoJSON(ctx, s.client, http.MethodGet, s.baseURL+"/gettoken", query, "", nil, &result); err != nil {
			return "", 0, err
		}
		if result.ErrCode != 0 {
			return "", 0, fmt.Errorf("notify dingtalk: token API error %d: %s", result.ErrCode, result.ErrMsg)
		}
		return result.AccessToken, time.Duration(result.ExpiresIn) * time.Second, nil
	})
}

// userID 通过 UnionID 查询钉钉应用可用的企业 UserID。
func (s *AppSender) userID(ctx context.Context, accessToken, unionID string) (string, error) {
	query := url.Values{"access_token": []string{accessToken}}
	var result struct {
		apiResult
		Result struct {
			UserID string `json:"userid"`
		} `json:"result"`
	}
	err := platformhttp.DoJSON(ctx, s.client, http.MethodPost,
		s.baseURL+"/topapi/user/getbyunionid", query, "",
		map[string]string{"unionid": unionID}, &result)
	if err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("notify dingtalk: user lookup error %d: %s", result.ErrCode, result.ErrMsg)
	}
	if result.Result.UserID == "" {
		return "", errors.New("notify dingtalk: user lookup returned an empty user_id")
	}
	return result.Result.UserID, nil
}
