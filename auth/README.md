# auth

`auth` 把认证结果、访问令牌校验和授权决策串成一个 Kratos middleware。认证引擎负责解析请求凭证，授权引擎负责根据租户、主体、动作、资源和项目做决策，根包负责把两者接入同一条请求链。

## 认证能力

`auth/authn` 提供统一接口：

- API Key、Basic、HMAC、JWT、mTLS、OAuth2、OIDC、Session 和 Noop 认证引擎；
- 服务端 `middleware.Server` 与客户端 `middleware.Client`；
- 从请求上下文读取 `AuthClaims`，或从令牌字符串直接认证；
- `auth.NewAuthMiddleware` 按 JWT 白名单和可选认证规则选择完整认证链或透传链。

详细说明见 [authn/README.md](authn/README.md) 和 [JWT README](authn/engine/jwt/README.md)。

## 授权能力

`auth/authz` 定义 `Authorizer`、批量过滤和策略写入接口，内置 Casbin、OPA、Cerbos、Zanzibar 和 Noop 适配器，并提供 `middleware.Server`。授权声明支持单个或多个主体、租户和项目。

详细说明见 [authz/README.md](authz/README.md) 和 [Casbin README](authz/engine/casbin/README.md)。

## 统一 middleware

```go
middleware := auth.NewAuthMiddleware(authenticator, authorizer, userToken, jwtConfig)
```

普通请求依次执行认证、当前访问令牌校验和授权。白名单请求直接交给业务处理器，可选认证请求会尝试解析身份但不会拦截匿名访问。认证声明缺失、令牌过期或授权失败会返回稳定的 Kratos 错误。

## 边界

本模块不保存用户、角色或租户数据，也不提供登录接口。`data.UserToken` 只用于校验当前用户的访问令牌是否仍有效；业务系统负责令牌签发、持久化和撤销。
