package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	httpx "github.com/liujitcn/go-utils/http"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/oauth/provider"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleUserURL  = "https://openidconnect.googleapis.com/v1/userinfo"
)

var googleDefaultScopes = []string{"openid", "profile", "email"}

// Provider 实现 Google OAuth 能力。
type Provider struct {
	conf *configv1.Provider
}

// New 创建 Google OAuth Provider。
func New(conf *configv1.Provider) *Provider {
	return &Provider{
		conf: conf,
	}
}

// Name 返回 Google Provider 名称。
func (p *Provider) Name() provider.Type { return provider.Google }

// AuthURL 生成 Google OAuth 授权地址。
func (p *Provider) AuthURL(state string, opts ...provider.Option) string {
	o := provider.ApplyOptions(opts...)
	redirectURI := p.conf.GetRedirectUri()
	if o.RedirectURI != "" {
		redirectURI = o.RedirectURI
	}
	scopes := provider.ChooseScopes(p.conf.GetScopes(), o.Scopes, googleDefaultScopes)
	params := url.Values{}
	params.Set("client_id", p.conf.GetClientId())
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("scope", strings.Join(scopes, " "))
	params.Set("access_type", "offline")
	provider.SetPKCEAuthParams(params, o.PKCE)
	provider.MergeParams(params, o.Params)
	return provider.BuildAuthURL(googleAuthURL, params)
}

// GetToken 使用 Google 授权码换取 Token。
func (p *Provider) GetToken(ctx context.Context, code string, opts ...provider.Option) (*provider.Token, error) {
	o := provider.ApplyOptions(opts...)
	// Google 当前只支持授权码换取 Token。
	if o.GrantType != provider.GrantTypeAuthorizationCode {
		return nil, provider.NewUnsupportedGrantTypeError(o.GrantType)
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	form := url.Values{}
	form.Set("client_id", p.conf.GetClientId())
	form.Set("client_secret", p.conf.GetClientSecret())
	form.Set("code", code)
	form.Set("redirect_uri", p.conf.GetRedirectUri())
	provider.SetPKCETokenParams(form, o.PKCE)
	provider.MergeParams(form, o.Params)
	// 授权类型由 Provider 能力决定，不允许通用参数覆盖。
	form.Set("grant_type", "authorization_code")
	response, err := httpx.Do(
		http.MethodPost,
		googleTokenURL,
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithFormBody(form),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("google token http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &provider.ProviderAPIError{
			Provider: p.Name(),
			Code:     resp.Error,
			Message:  resp.ErrorDesc,
			Raw:      raw,
		}
	}
	return &provider.Token{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		IDToken:      resp.IDToken,
		TokenType:    resp.TokenType,
		ExpiresIn:    resp.ExpiresIn,
		Scope:        resp.Scope,
		Raw:          raw,
	}, nil
}

// GetUser 使用 Google Token 获取用户信息。
func (p *Provider) GetUser(ctx context.Context, token *provider.Token) (*provider.User, error) {
	if token == nil || token.AccessToken == "" {
		return nil, provider.ErrInvalidToken
	}
	var resp struct {
		Sub           string `json:"sub"`
		Name          string `json:"name"`
		Email         string `json:"email"`
		Picture       string `json:"picture"`
		PreferredName string `json:"preferred_username"`
	}
	response, err := httpx.Do(
		http.MethodGet,
		googleUserURL,
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithBearerToken(token.AccessToken),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("google user http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	return &provider.User{
		Provider: p.Name(),
		OpenID:   resp.Sub,
		Username: resp.PreferredName,
		Nickname: resp.Name,
		Email:    resp.Email,
		Avatar:   resp.Picture,
		Raw:      raw,
	}, nil
}
