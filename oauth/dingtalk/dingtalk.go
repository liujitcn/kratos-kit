package dingtalk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	httpx "github.com/liujitcn/go-utils/http"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/oauth/provider"
)

const (
	dingTalkAuthURL  = "https://login.dingtalk.com/oauth2/auth"
	dingTalkTokenURL = "https://api.dingtalk.com/v1.0/oauth2/userAccessToken"
	dingTalkUserURL  = "https://api.dingtalk.com/v1.0/contact/users/me"
)

var dingTalkDefaultScopes = []string{"openid"}

// Provider 实现钉钉 OAuth 能力。
type Provider struct {
	conf *configv1.Provider
}

// New 创建钉钉 OAuth Provider。
func New(conf *configv1.Provider) *Provider {
	return &Provider{
		conf: conf,
	}
}

// Name 返回钉钉 Provider 名称。
func (p *Provider) Name() provider.Type { return provider.DingTalk }

// AuthURL 生成钉钉 OAuth 授权地址。
func (p *Provider) AuthURL(state string, opts ...provider.Option) string {
	o := provider.ApplyOptions(opts...)
	redirectURI := p.conf.GetRedirectUri()
	if o.RedirectURI != "" {
		redirectURI = o.RedirectURI
	}
	scopes := provider.ChooseScopes(p.conf.GetScopes(), o.Scopes, dingTalkDefaultScopes)
	params := url.Values{}
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("client_id", p.conf.GetClientId())
	params.Set("scope", provider.JoinScopes(scopes, " "))
	params.Set("state", state)
	params.Set("prompt", "consent")
	provider.SetPKCEAuthParams(params, o.PKCE)
	provider.MergeParams(params, o.Params)
	return provider.BuildAuthURL(dingTalkAuthURL, params)
}

// GetToken 使用钉钉授权码换取 Token。
func (p *Provider) GetToken(ctx context.Context, code string, opts ...provider.Option) (*provider.Token, error) {
	o := provider.ApplyOptions(opts...)
	// 钉钉当前只支持授权码换取 Token。
	if o.GrantType != provider.GrantTypeAuthorizationCode {
		return nil, provider.NewUnsupportedGrantTypeError(o.GrantType)
	}
	var resp struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpireIn     int64  `json:"expireIn"`
		CorpID       string `json:"corpId"`
	}
	req := map[string]string{
		"clientId":     p.conf.GetClientId(),
		"clientSecret": p.conf.GetClientSecret(),
		"code":         code,
	}
	if o.PKCE != nil && o.PKCE.Verifier != "" {
		req["codeVerifier"] = o.PKCE.Verifier
	}
	for k, vs := range o.Params {
		if len(vs) > 0 {
			req[k] = vs[len(vs)-1]
		}
	}
	// 授权类型由 Provider 能力决定，不允许通用参数覆盖。
	req["grantType"] = "authorization_code"
	response, err := httpx.Do(
		http.MethodPost,
		dingTalkTokenURL,
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithJSONBody(req),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("dingtalk token http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	return &provider.Token{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpireIn,
		UnionID:      resp.CorpID,
		Raw:          raw,
	}, nil
}

// GetUser 使用钉钉 Token 获取用户信息。
func (p *Provider) GetUser(ctx context.Context, token *provider.Token) (*provider.User, error) {
	if token == nil || token.AccessToken == "" {
		return nil, provider.ErrInvalidToken
	}
	var resp struct {
		OpenID  string `json:"openId"`
		UnionID string `json:"unionId"`
		Nick    string `json:"nick"`
		Avatar  string `json:"avatarUrl"`
		Mobile  string `json:"mobile"`
		Email   string `json:"email"`
	}
	response, err := httpx.Do(
		http.MethodGet,
		dingTalkUserURL,
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithHeader("x-acs-dingtalk-access-token", token.AccessToken),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("dingtalk user http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	if resp.OpenID == "" && resp.UnionID == "" {
		return nil, fmt.Errorf("dingtalk userinfo missing openId and unionId")
	}
	return &provider.User{
		Provider: p.Name(),
		OpenID:   resp.OpenID,
		UnionID:  resp.UnionID,
		Nickname: resp.Nick,
		Email:    resp.Email,
		Avatar:   resp.Avatar,
		Raw:      raw,
	}, nil
}
