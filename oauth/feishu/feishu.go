package feishu

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
	feishuAuthURL  = "https://open.feishu.cn/open-apis/authen/v1/authorize"
	feishuTokenURL = "https://open.feishu.cn/open-apis/authen/v2/oauth/token"
	feishuUserURL  = "https://open.feishu.cn/open-apis/authen/v1/user_info"
)

var feishuDefaultScopes []string

// Provider 实现飞书 OAuth 能力。
type Provider struct {
	conf *configv1.Provider
}

// New 创建飞书 OAuth Provider。
func New(conf *configv1.Provider) *Provider {
	return &Provider{
		conf: conf,
	}
}

// Name 返回飞书 Provider 名称。
func (p *Provider) Name() provider.Type { return provider.Feishu }

// AuthURL 生成飞书 OAuth 授权地址。
func (p *Provider) AuthURL(state string, opts ...provider.Option) string {
	o := provider.ApplyOptions(opts...)
	redirectURI := p.conf.GetRedirectUri()
	if o.RedirectURI != "" {
		redirectURI = o.RedirectURI
	}
	scopes := provider.ChooseScopes(p.conf.GetScopes(), o.Scopes, feishuDefaultScopes)
	params := url.Values{}
	params.Set("app_id", p.conf.GetClientId())
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)
	if len(scopes) > 0 {
		params.Set("scope", strings.Join(scopes, " "))
	}
	provider.SetPKCEAuthParams(params, o.PKCE)
	provider.MergeParams(params, o.Params)
	return provider.BuildAuthURL(feishuAuthURL, params)
}

// GetToken 使用飞书授权码换取 Token。
func (p *Provider) GetToken(ctx context.Context, code string, opts ...provider.Option) (*provider.Token, error) {
	o := provider.ApplyOptions(opts...)
	// 飞书当前只支持授权码换取 Token。
	if o.GrantType != provider.GrantTypeAuthorizationCode {
		return nil, provider.NewUnsupportedGrantTypeError(o.GrantType)
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			TokenType    string `json:"token_type"`
			ExpiresIn    int64  `json:"expires_in"`
			Scope        string `json:"scope"`
		} `json:"data"`
	}
	req := map[string]string{
		"client_id":     p.conf.GetClientId(),
		"client_secret": p.conf.GetClientSecret(),
		"code":          code,
		"redirect_uri":  p.conf.GetRedirectUri(),
	}
	if o.PKCE != nil && o.PKCE.Verifier != "" {
		req["code_verifier"] = o.PKCE.Verifier
	}
	for k, vs := range o.Params {
		if len(vs) > 0 {
			req[k] = vs[len(vs)-1]
		}
	}
	// 授权类型由 Provider 能力决定，不允许通用参数覆盖。
	req["grant_type"] = "authorization_code"
	response, err := httpx.Do(
		http.MethodPost,
		feishuTokenURL,
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithJSONBody(req),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("feishu token http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, &provider.ProviderAPIError{
			Provider: p.Name(),
			Code:     fmt.Sprint(resp.Code),
			Message:  resp.Msg,
			Raw:      raw,
		}
	}
	return &provider.Token{
		AccessToken:  resp.Data.AccessToken,
		RefreshToken: resp.Data.RefreshToken,
		TokenType:    resp.Data.TokenType,
		ExpiresIn:    resp.Data.ExpiresIn,
		Scope:        resp.Data.Scope,
		Raw:          raw,
	}, nil
}

// GetUser 使用飞书 Token 获取用户信息。
func (p *Provider) GetUser(ctx context.Context, token *provider.Token) (*provider.User, error) {
	if token == nil || token.AccessToken == "" {
		return nil, provider.ErrInvalidToken
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			OpenID  string `json:"open_id"`
			UnionID string `json:"union_id"`
			Name    string `json:"name"`
			EnName  string `json:"en_name"`
			Avatar  string `json:"avatar_url"`
			Email   string `json:"email"`
		} `json:"data"`
	}
	response, err := httpx.Do(
		http.MethodGet,
		feishuUserURL,
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
		httpx.WithBearerToken(token.AccessToken),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("feishu user http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, &provider.ProviderAPIError{
			Provider: p.Name(),
			Code:     fmt.Sprint(resp.Code),
			Message:  resp.Msg,
			Raw:      raw,
		}
	}
	return &provider.User{
		Provider: p.Name(),
		OpenID:   resp.Data.OpenID,
		UnionID:  resp.Data.UnionID,
		Nickname: resp.Data.Name,
		Email:    resp.Data.Email,
		Avatar:   resp.Data.Avatar,
		Raw:      raw,
	}, nil
}
