package wechatwork

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
	wechatWorkAuthURL   = "https://login.work.weixin.qq.com/wwlogin/sso/login"
	wechatWorkTokenURL  = "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	wechatWorkUserIDURL = "https://qyapi.weixin.qq.com/cgi-bin/auth/getuserinfo"
	wechatWorkUserURL   = "https://qyapi.weixin.qq.com/cgi-bin/user/get"
)

// Provider 实现企业微信 OAuth 能力。
type Provider struct {
	conf *configv1.Provider
}

// New 创建企业微信 OAuth Provider。
func New(conf *configv1.Provider) *Provider {
	return &Provider{
		conf: conf,
	}
}

// Name 返回企业微信 Provider 名称。
func (p *Provider) Name() provider.Type { return provider.WechatWork }

// AuthURL 生成企业微信 OAuth 授权地址。
func (p *Provider) AuthURL(state string, opts ...provider.Option) string {
	o := provider.ApplyOptions(opts...)
	redirectURI := p.conf.GetRedirectUri()
	if o.RedirectURI != "" {
		redirectURI = o.RedirectURI
	}
	params := url.Values{}
	// 默认使用企业内部应用登录，外部可通过 provider.WithParam 覆盖同名参数。
	params.Set("login_type", "CorpApp")
	params.Set("appid", p.conf.GetClientId())
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)
	provider.SetPKCEAuthParams(params, o.PKCE)
	provider.MergeParams(params, o.Params)
	return provider.BuildAuthURL(wechatWorkAuthURL, params)
}

// GetToken 使用企业微信授权码换取 Token。
func (p *Provider) GetToken(ctx context.Context, code string, opts ...provider.Option) (*provider.Token, error) {
	o := provider.ApplyOptions(opts...)
	// 企业微信当前只支持授权码换取 Token。
	if o.GrantType != provider.GrantTypeAuthorizationCode {
		return nil, provider.NewUnsupportedGrantTypeError(o.GrantType)
	}
	accessToken, raw1, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var userID string
	var openID string
	var unionID string
	var raw2 []byte
	params := url.Values{}
	provider.SetPKCETokenParams(params, o.PKCE)
	provider.MergeParams(params, o.Params)
	// 企业微信用户标识接口不接收授权类型参数。
	params.Del("grant_type")
	userID, openID, unionID, raw2, err = p.userID(ctx, accessToken, code, params)
	if err != nil {
		return nil, err
	}
	raw := append(raw1, raw2...)
	return &provider.Token{
		AccessToken:  accessToken,
		RefreshToken: openID,
		OpenID:       userID,
		UnionID:      unionID,
		Raw:          raw,
	}, nil
}

// GetUser 使用企业微信 Token 获取用户信息。
func (p *Provider) GetUser(ctx context.Context, token *provider.Token) (*provider.User, error) {
	if token == nil || token.AccessToken == "" || token.OpenID == "" {
		return nil, provider.ErrInvalidToken
	}
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		UserID  string `json:"userid"`
		Name    string `json:"name"`
		Alias   string `json:"alias"`
		Avatar  string `json:"avatar"`
		Email   string `json:"email"`
	}
	params := url.Values{}
	params.Set("access_token", token.AccessToken)
	params.Set("userid", token.OpenID)
	response, err := httpx.Do(
		http.MethodGet,
		wechatWorkUserURL+"?"+params.Encode(),
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("wechatwork user http status %d: %s", response.StatusCode, string(raw))
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
	return &provider.User{
		Provider: p.Name(),
		OpenID:   resp.UserID,
		UnionID:  token.UnionID,
		Username: resp.Alias,
		Nickname: resp.Name,
		Email:    resp.Email,
		Avatar:   resp.Avatar,
		Raw:      raw,
	}, nil
}

// accessToken 获取企业微信服务端访问令牌。
func (p *Provider) accessToken(ctx context.Context) (string, []byte, error) {
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Token   string `json:"access_token"`
	}
	params := url.Values{}
	params.Set("corpid", p.conf.GetClientId())
	params.Set("corpsecret", p.conf.GetClientSecret())
	response, err := httpx.Do(
		http.MethodGet,
		wechatWorkTokenURL+"?"+params.Encode(),
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return "", nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", raw, fmt.Errorf("wechatwork token http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return "", raw, err
	}
	if resp.ErrCode != 0 {
		return "", raw, &provider.ProviderAPIError{
			Provider: p.Name(),
			Code:     strconv.Itoa(resp.ErrCode),
			Message:  resp.ErrMsg,
			Raw:      raw,
		}
	}
	return resp.Token, raw, nil
}

// userID 根据授权码获取企业微信用户标识。
func (p *Provider) userID(ctx context.Context, accessToken, code string, extraParams url.Values) (string, string, string, []byte, error) {
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		UserID  string `json:"userid"`
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
	}
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("code", code)
	provider.MergeParams(params, extraParams)
	response, err := httpx.Do(
		http.MethodGet,
		wechatWorkUserIDURL+"?"+params.Encode(),
		httpx.WithContext(ctx),
		httpx.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return "", "", "", nil, err
	}
	raw := response.Body
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", "", "", raw, fmt.Errorf("wechatwork user id http status %d: %s", response.StatusCode, string(raw))
	}
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		return "", "", "", raw, err
	}
	if resp.ErrCode != 0 {
		return "", "", "", raw, &provider.ProviderAPIError{
			Provider: p.Name(),
			Code:     strconv.Itoa(resp.ErrCode),
			Message:  resp.ErrMsg,
			Raw:      raw,
		}
	}
	if resp.UserID == "" {
		return "", resp.OpenID, resp.UnionID, raw, fmt.Errorf("wechatwork user id is empty")
	}
	return resp.UserID, resp.OpenID, resp.UnionID, raw, nil
}
