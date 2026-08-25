package github

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
	githubAuthURL   = "https://github.com/login/oauth/authorize"
	githubTokenURL  = "https://github.com/login/oauth/access_token"
	githubUserURL   = "https://api.github.com/user"
	githubEmailsURL = "https://api.github.com/user/emails"
)

var githubDefaultScopes = []string{"user:email"}

// Provider 实现 GitHub OAuth 能力。
type Provider struct {
	conf *configv1.Provider
}

// New 创建 GitHub OAuth Provider。
func New(conf *configv1.Provider) *Provider {
	return &Provider{
		conf: conf,
	}
}

// Name 返回 GitHub Provider 名称。
func (p *Provider) Name() provider.Type { return provider.Github }

// AuthURL 生成 GitHub OAuth 授权地址。
func (p *Provider) AuthURL(state string, opts ...provider.Option) string {
	o := provider.ApplyOptions(opts...)
	redirectURI := p.conf.GetRedirectUri()
	if o.RedirectURI != "" {
		redirectURI = o.RedirectURI
	}
	scopes := provider.ChooseScopes(p.conf.GetScopes(), o.Scopes, githubDefaultScopes)

	params := url.Values{}
	params.Set("client_id", p.conf.GetClientId())
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("scope", strings.Join(scopes, " "))
	provider.SetPKCEAuthParams(params, o.PKCE)
	provider.MergeParams(params, o.Params)
	return provider.BuildAuthURL(githubAuthURL, params)
}

// GetToken 使用 GitHub 授权码换取 Token。
func (p *Provider) GetToken(ctx context.Context, code string, opts ...provider.Option) (*provider.Token, error) {
	o := provider.ApplyOptions(opts...)
	// GitHub 当前只支持授权码换取 Token。
	if o.GrantType != provider.GrantTypeAuthorizationCode {
		return nil, provider.NewUnsupportedGrantTypeError(o.GrantType)
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
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
	// GitHub Web Application Flow 不接收授权类型参数。
	form.Del("grant_type")

	response, err := httpx.Do(
		http.MethodPost,
		githubTokenURL,
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithFormBody(form),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("github token http status %d: %s", response.StatusCode, string(raw))
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
		TokenType:    resp.TokenType,
		Scope:        resp.Scope,
		Raw:          raw,
	}, nil
}

// GetUser 使用 GitHub Token 获取用户信息。
func (p *Provider) GetUser(ctx context.Context, token *provider.Token) (*provider.User, error) {
	if token == nil || token.AccessToken == "" {
		return nil, provider.ErrInvalidToken
	}
	var resp struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	response, err := httpx.Do(
		http.MethodGet,
		githubUserURL,
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithBearerToken(token.AccessToken),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("github user http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}

	email := resp.Email
	if email == "" {
		email = p.primaryEmail(ctx, token.AccessToken)
	}

	return &provider.User{
		Provider: p.Name(),
		OpenID:   strconv.FormatInt(resp.ID, 10),
		Username: resp.Login,
		Nickname: resp.Name,
		Email:    email,
		Avatar:   resp.AvatarURL,
		Raw:      raw,
	}, nil
}

// primaryEmail 获取 GitHub 账号的主邮箱。
func (p *Provider) primaryEmail(ctx context.Context, accessToken string) string {
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	response, err := httpx.Do(
		http.MethodGet,
		githubEmailsURL,
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithBearerToken(accessToken),
	)
	if err != nil {
		return ""
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ""
	}
	err = json.Unmarshal(response.Body, &emails)
	if err != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email
		}
	}
	return ""
}
