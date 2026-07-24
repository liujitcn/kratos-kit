package wechatmini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	httpx "github.com/liujitcn/go-utils/http"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/oauth/provider"
)

const (
	wechatMiniTokenURL       = "https://api.weixin.qq.com/sns/jscode2session"
	wechatMiniAccessTokenURL = "https://api.weixin.qq.com/cgi-bin/token"
)

// Provider 实现微信小程序登录能力。
type Provider struct {
	conf *configv1.Provider
}

// New 创建微信小程序登录 Provider。
func New(conf *configv1.Provider) *Provider {
	return &Provider{conf: conf}
}

// Name 返回微信小程序 Provider 名称。
func (p *Provider) Name() provider.Type { return provider.WechatMini }

// AuthURL 返回空字符串；微信小程序登录由小程序端 wx.login 获取 code。
func (p *Provider) AuthURL(state string, opts ...provider.Option) string {
	return ""
}

// GetToken 根据 OAuth 授权类型换取小程序登录凭证或服务端接口访问令牌。
func (p *Provider) GetToken(ctx context.Context, code string, opts ...provider.Option) (*provider.Token, error) {
	o := provider.ApplyOptions(opts...)
	// OAuth 授权类型决定调用小程序登录凭证或服务端接口令牌端点。
	switch o.GrantType {
	case provider.GrantTypeAuthorizationCode:
		return p.getAuthorizationCodeToken(ctx, code, o)
	case provider.GrantTypeClientCredentials:
		return p.getClientCredentialsToken(ctx, o)
	default:
		return nil, provider.NewUnsupportedGrantTypeError(o.GrantType)
	}
}

// GetUser 根据小程序登录 Token 返回 openid 与 unionid。
func (p *Provider) GetUser(ctx context.Context, token *provider.Token) (*provider.User, error) {
	if token == nil || token.AccessToken == "" || token.OpenID == "" {
		return nil, provider.ErrInvalidToken
	}
	return &provider.User{
		Provider: p.Name(),
		OpenID:   token.OpenID,
		UnionID:  token.UnionID,
		Raw:      token.Raw,
	}, nil
}

// getAuthorizationCodeToken 使用小程序登录 code 换取 session_key 与 openid。
func (p *Provider) getAuthorizationCodeToken(ctx context.Context, code string, o provider.Options) (*provider.Token, error) {
	var resp struct {
		OpenID     string `json:"openid"`
		SessionKey string `json:"session_key"`
		UnionID    string `json:"unionid"`
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
	}
	params := url.Values{}
	params.Set("appid", p.conf.GetClientId())
	params.Set("secret", p.conf.GetClientSecret())
	params.Set("js_code", code)
	provider.MergeParams(params, o.Params)
	params.Set("grant_type", string(o.GrantType))

	statusCode, raw, err := doTokenRequest(ctx, wechatMiniTokenURL, params)
	if err != nil {
		return nil, err
	}
	// 微信接口返回非成功 HTTP 状态时，保留响应内容用于排查。
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("wechatmini token http status %d: %s", statusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	// 微信业务错误统一转换为 Provider 错误。
	if resp.ErrCode != 0 {
		return nil, &provider.ProviderAPIError{
			Provider: p.Name(),
			Code:     strconv.Itoa(resp.ErrCode),
			Message:  resp.ErrMsg,
			Raw:      raw,
		}
	}
	// 登录响应缺少用户标识或会话密钥时，不生成不完整 Token。
	if resp.OpenID == "" || resp.SessionKey == "" {
		return nil, provider.ErrInvalidToken
	}
	return &provider.Token{
		AccessToken: resp.SessionKey,
		TokenType:   "session_key",
		OpenID:      resp.OpenID,
		UnionID:     resp.UnionID,
		Raw:         raw,
	}, nil
}

// getClientCredentialsToken 使用小程序凭证获取服务端接口访问令牌。
func (p *Provider) getClientCredentialsToken(ctx context.Context, o provider.Options) (*provider.Token, error) {
	var resp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	params := url.Values{}
	params.Set("appid", p.conf.GetClientId())
	params.Set("secret", p.conf.GetClientSecret())
	provider.MergeParams(params, o.Params)
	params.Set("grant_type", "client_credential")

	statusCode, raw, err := doTokenRequest(ctx, wechatMiniAccessTokenURL, params)
	if err != nil {
		return nil, err
	}
	// 微信接口返回非成功 HTTP 状态时，保留响应内容用于排查。
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("wechatmini access token http status %d: %s", statusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	// 微信业务错误统一转换为 Provider 错误。
	if resp.ErrCode != 0 {
		return nil, &provider.ProviderAPIError{
			Provider: p.Name(),
			Code:     strconv.Itoa(resp.ErrCode),
			Message:  resp.ErrMsg,
			Raw:      raw,
		}
	}
	// 服务端令牌或有效期无效时，避免业务侧缓存不可用凭证。
	if resp.AccessToken == "" || resp.ExpiresIn <= 0 {
		return nil, provider.ErrInvalidToken
	}
	return &provider.Token{
		AccessToken: resp.AccessToken,
		TokenType:   "access_token",
		ExpiresIn:   resp.ExpiresIn,
		Raw:         raw,
	}, nil
}

// doTokenRequest 使用全局 httpx 客户端请求微信小程序 Token 端点。
func doTokenRequest(ctx context.Context, endpoint string, params url.Values) (int, []byte, error) {
	response, err := httpx.Do(
		http.MethodGet,
		endpoint+"?"+params.Encode(),
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode, response.Body, nil
}
