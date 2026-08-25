package wechatmp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	httpx "github.com/liujitcn/go-utils/http"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/oauth/provider"
)

const (
	wechatMPAuthURL  = "https://open.weixin.qq.com/connect/oauth2/authorize"
	wechatMPTokenURL = "https://api.weixin.qq.com/sns/oauth2/access_token"
	wechatMPUserURL  = "https://api.weixin.qq.com/sns/userinfo"
)

var wechatMPDefaultScopes = []string{"snsapi_userinfo"}

// Provider 实现微信公众号网页授权 OAuth 能力。
type Provider struct {
	conf *configv1.Provider
}

// New 创建微信公众号网页授权 OAuth Provider。
func New(conf *configv1.Provider) *Provider {
	return &Provider{
		conf: conf,
	}
}

// Name 返回微信公众号网页授权 Provider 名称。
func (p *Provider) Name() provider.Type { return provider.WechatMP }

// AuthURL 生成微信公众号网页授权地址。
func (p *Provider) AuthURL(state string, opts ...provider.Option) string {
	o := provider.ApplyOptions(opts...)
	redirectURI := p.conf.GetRedirectUri()
	if o.RedirectURI != "" {
		redirectURI = o.RedirectURI
	}
	scopes := provider.ChooseScopes(p.conf.GetScopes(), o.Scopes, wechatMPDefaultScopes)
	params := url.Values{}
	params.Set("appid", p.conf.GetClientId())
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(scopes, ","))
	params.Set("state", state)
	provider.SetPKCEAuthParams(params, o.PKCE)
	provider.MergeParams(params, o.Params)
	return provider.BuildAuthURL(wechatMPAuthURL, params) + "#wechat_redirect"
}

// GetToken 使用微信公众号网页授权码换取 Token。
func (p *Provider) GetToken(ctx context.Context, code string, opts ...provider.Option) (*provider.Token, error) {
	o := provider.ApplyOptions(opts...)
	// 微信公众号当前只支持授权码换取 Token。
	if o.GrantType != provider.GrantTypeAuthorizationCode {
		return nil, provider.NewUnsupportedGrantTypeError(o.GrantType)
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		OpenID       string `json:"openid"`
		Scope        string `json:"scope"`
		UnionID      string `json:"unionid"`
		ErrCode      int    `json:"errcode"`
		ErrMsg       string `json:"errmsg"`
	}
	params := url.Values{}
	params.Set("appid", p.conf.GetClientId())
	params.Set("secret", p.conf.GetClientSecret())
	params.Set("code", code)
	provider.SetPKCETokenParams(params, o.PKCE)
	provider.MergeParams(params, o.Params)
	// 授权类型由 Provider 能力决定，不允许通用参数覆盖。
	params.Set("grant_type", "authorization_code")
	response, err := httpx.Do(
		http.MethodGet,
		wechatMPTokenURL+"?"+params.Encode(),
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("wechatmp token http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	if resp.ErrCode != 0 {
		return nil, &provider.ProviderAPIError{
			Provider: p.Name(),
			Code:     strconv.Itoa(resp.ErrCode),
			Message:  resp.ErrMsg,
			Raw:      raw,
		}
	}
	return &provider.Token{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
		Scope:        resp.Scope,
		OpenID:       resp.OpenID,
		UnionID:      resp.UnionID,
		Raw:          raw,
	}, nil
}

// GetUser 使用微信公众号网页授权 Token 获取用户信息。
func (p *Provider) GetUser(ctx context.Context, token *provider.Token) (*provider.User, error) {
	if token == nil || token.AccessToken == "" || token.OpenID == "" {
		return nil, provider.ErrInvalidToken
	}
	var resp struct {
		OpenID   string `json:"openid"`
		Nickname string `json:"nickname"`
		Avatar   string `json:"headimgurl"`
		UnionID  string `json:"unionid"`
		ErrCode  int    `json:"errcode"`
		ErrMsg   string `json:"errmsg"`
	}
	params := url.Values{}
	params.Set("access_token", token.AccessToken)
	params.Set("openid", token.OpenID)
	params.Set("lang", "zh_CN")
	response, err := httpx.Do(
		http.MethodGet,
		wechatMPUserURL+"?"+params.Encode(),
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("wechatmp user http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("wechatmp userinfo error %d: %s", resp.ErrCode, resp.ErrMsg)
	}
	return &provider.User{
		Provider: p.Name(),
		OpenID:   resp.OpenID,
		UnionID:  resp.UnionID,
		Nickname: resp.Nickname,
		Avatar:   resp.Avatar,
		Raw:      raw,
	}, nil
}
