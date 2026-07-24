# kratos-kit OAuth

`oauth` 是第三方 OAuth 登录 SDK，只负责 OAuth 流程本身，不处理业务登录态。

## 能力边界

包含：

- 根据 `api/gen/go/config/v1.OAuth` 初始化 Provider 管理器。
- 过滤配置不完整或不支持的 Provider，并返回当前可展示的跳转授权 Provider 列表。
- 生成第三方授权地址。
- 使用授权码或客户端凭证获取 Token。
- 使用 Token 获取第三方用户信息。
- 使用 `kratos-kit/cache.Cache` 保存并消费一次性 `state`。
- 使用 `go-utils/id` 生成 `state` 与 PKCE verifier。

不包含：

- HTTP 路由与 callback Controller。
- 用户绑定表、注册、登录态、JWT、Session。
- 权限、多租户、组织关系等业务判断。

## 支持 Provider

| Provider | key |
|---|---|
| GitHub | `github` |
| Gitee | `gitee` |
| Google | `google` |
| 微信开放平台 | `wechat` |
| 微信公众号网页授权 | `wechatmp` |
| 微信小程序登录 | `wechatmini` |
| 企业微信 | `wechatwork` |
| 钉钉 | `dingtalk` |
| 飞书 | `feishu` |

微信相关 Provider 的区别：

- `wechat`：微信开放平台扫码登录，授权地址为 `open.weixin.qq.com/connect/qrconnect`。
- `wechatmp`：微信公众号网页授权，授权地址为 `open.weixin.qq.com/connect/oauth2/authorize`。
- `wechatmini`：微信小程序登录及服务端接口令牌，没有授权地址；支持通过 `jscode2session` 换取 `openid/session_key`，也支持通过客户端凭证获取 `access_token`。
- `wechatwork`：企业微信登录，授权地址为 `login.work.weixin.qq.com/wwlogin/sso/login`。

## 初始化

```go
import (
    configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
    "github.com/liujitcn/kratos-kit/oauth"
)

manager, err := oauth.NewManager(&configv1.OAuth{
    Providers: map[string]*configv1.Provider{
        string(oauth.Github): {
            ClientId:     "xxx",
            ClientSecret: "xxx",
            RedirectUri:  "http://localhost:8000/oauth/github/callback",
            Scopes:       []string{"user:email"},
        },
        string(oauth.WechatMP): {
            ClientId:     "公众号 appid",
            ClientSecret: "公众号 secret",
            RedirectUri:  "http://localhost:8000/oauth/wechatmp/callback",
            Scopes:       []string{"snsapi_userinfo"},
        },
        string(oauth.WechatMini): {
            ClientId:     "小程序 appid",
            ClientSecret: "小程序 secret",
        },
    },
})
if err != nil {
    return err
}
```

外部只需要注入 `manager`，业务代码按 Provider 名称获取实现。`NewManager` 会保留配置完整且受支持的全部 Provider；`Providers()` 遍历这些实现并调用 `AuthURL("")`，只返回授权地址非空的跳转授权 Provider。`IsSupported(name)` 使用相同规则判断当前已配置的 Provider，未配置的 Provider 返回 `false`。`wechatmini` 可以通过 `Get(oauth.WechatMini)` 获取，但因 `AuthURL` 返回空字符串而不会进入展示列表：

```go
oauthProvider, err := manager.Get(oauth.Github)
if err != nil {
    return err
}
```

## 发起授权

推荐使用 `kratos-kit/cache.Cache` 保存一次性 `state`。如果当前服务已有 Redis 缓存，直接传业务里的 `cache.Cache`；本地开发也可以用内存缓存。

```go
import (
    "github.com/liujitcn/kratos-kit/cache"
    "github.com/liujitcn/kratos-kit/oauth"
    "github.com/liujitcn/kratos-kit/oauth/provider"
)

store, cleanup, err := cache.NewCache(nil)
if err != nil {
    return err
}
defer cleanup()

state, pkce, err := oauth.NewStateWithPKCE(store, oauth.StatePayload{
    Provider:    oauth.Github,
    Scene:       "login",
    RedirectURL: "/home",
}, 0)
if err != nil {
    return err
}

authURL := oauthProvider.AuthURL(
    state,
    provider.WithPKCE(pkce),
    provider.WithParam("prompt", "consent"),
)
```

`NewStateWithPKCE` 会同时生成 `state` 和 PKCE，并把 PKCE verifier 存入缓存载荷。`ttl <= 0` 时默认有效期为 10 分钟。

## 回调处理

callback 收到第三方返回的 `code` 与 `state` 后，先校验并消费 `state`，再用同一个 PKCE verifier 换取 Token。

```go
payload, err := oauth.VerifyState(store, state)
if err != nil {
    return err
}

oauthProvider, err := manager.Get(payload.Provider)
if err != nil {
    return err
}

token, err := oauthProvider.GetToken(ctx, code, provider.WithPKCE(payload.PKCE))
if err != nil {
    return err
}

user, err := oauthProvider.GetUser(ctx, token)
if err != nil {
    return err
}
```

`VerifyState` 读取成功后会删除缓存，保证同一个 `state` 只能使用一次。

## 不使用 PKCE

如果接入平台或业务场景不需要 PKCE，可以只生成并保存普通 `state`。

```go
state, err := oauth.NewState(store, oauth.StatePayload{
    Provider: oauth.DingTalk,
    Scene:    "bind",
}, 0)
if err != nil {
    return err
}

authURL := oauthProvider.AuthURL(state)
```

回调时直接使用授权码换取 Token：

```go
payload, err := oauth.VerifyState(store, state)
if err != nil {
    return err
}

oauthProvider, err := manager.Get(payload.Provider)
if err != nil {
    return err
}

token, err := oauthProvider.GetToken(ctx, code)
if err != nil {
    return err
}
```

## 微信小程序登录

微信小程序登录没有 OAuth 跳转授权地址，不会出现在 `Manager.Providers()` 返回的跳转授权展示列表中，也不需要调用 `AuthURL`。小程序端先调用 `wx.login()` 获取 `code`，服务端可以通过 `manager.Get(oauth.WechatMini)` 获取小程序 Provider 后换取 token：

```go
oauthProvider, err := manager.Get(oauth.WechatMini)
if err != nil {
    return err
}

loginToken, err := oauthProvider.GetToken(ctx, code)
if err != nil {
    return err
}

user, err := oauthProvider.GetUser(ctx, loginToken)
if err != nil {
    return err
}

accessToken, err := oauthProvider.GetToken(
    ctx,
    "",
    provider.WithGrantType(provider.GrantTypeClientCredentials),
)
if err != nil {
    return err
}
```

不传 `WithGrantType` 时默认使用 `provider.GrantTypeAuthorizationCode`。此时 `wechatmini` 会把微信返回的 `session_key` 放入 `loginToken.AccessToken`，并设置 `loginToken.TokenType = "session_key"`；`openid` 与 `unionid` 分别放入 `loginToken.OpenID`、`loginToken.UnionID`。`GetUser` 不请求微信用户资料，只返回 `openid/unionid`。

指定 `provider.GrantTypeClientCredentials` 时，`code` 参数不参与请求，返回的 `access_token` 放入 `accessToken.AccessToken`，有效期放入 `accessToken.ExpiresIn`，并设置 `accessToken.TokenType = "access_token"`。

微信小程序 Provider 统一通过 `httpx.Do` 发送请求，因此会继承 `httpx.Init` 或 `httpx.SetDefaultClient` 配置的代理、TLS、默认请求头与超时。当前 `httpx` 会记录最终请求 URL，微信接口的 `secret` 与登录 code 位于查询参数中，使用时需要限制相关日志的访问范围。

## 可选参数

通用可选参数放在 `oauth/provider` 包中：

- `provider.WithScopes(...)`：覆盖配置中的 scope。
- `provider.WithRedirectURI(uri)`：覆盖配置中的回调地址。
- `provider.WithParam(key, value)`：追加或覆盖授权地址、换 Token 请求中的扩展参数；`grant_type` 由 `WithGrantType` 控制，不能通过通用参数覆盖。
- `provider.WithPKCE(pkce)`：为授权地址设置 `code_challenge`，为换 Token 请求设置 `code_verifier`。
- `provider.WithGrantType(grantType)`：指定 `GetToken` 使用的 OAuth 授权类型，默认值为 `provider.GrantTypeAuthorizationCode`；当前微信小程序还支持 `provider.GrantTypeClientCredentials`。

## 错误

- `oauth.ErrInvalidState`：state 不存在、过期、重复使用或载荷解析失败。
- `oauth.ErrInvalidToken`：Token 为空或缺少访问令牌。
- `oauth.ErrUnsupportedGrantType`：Provider 不支持指定的 OAuth 授权类型。
- `oauth.ProviderNotFoundError`：配置或调用的 Provider 名称不支持。
- `oauth.ProviderAPIError`：第三方 Provider 返回的业务错误。

## 验证

```bash
cd oauth
go test ./...
go vet ./...
```
