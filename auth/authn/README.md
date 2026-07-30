# Kratos Authn

本模块定义统一认证契约和 Kratos 中间件；具体认证器按独立 Go 模块安装。

## 支持引擎

- `engine/apikey`：静态或回调校验的 Bearer API Key；PSK 使用同一实现，不再维护重复模块
- `engine/basicauth`：RFC 7617 Basic Auth
- `engine/hmac`：绑定方法、路径、查询参数和请求体摘要的 HMAC-SHA256 Bearer token；nonce 原子消费防止重放
- `engine/jwt`：本地 JWT 签发与校验
- `engine/mtls`：直接读取 Kratos HTTP TLS 状态或 gRPC peer 证书
- `engine/oauth2`：RFC 7662 Token Introspection
- `engine/oidc`：OIDC discovery、JWKS 刷新、issuer/audience/时效校验
- `engine/session`：可插拔会话存储，默认 TTL 为 24 小时；内存实现支持过期清理和嵌套 claims 深拷贝

请求认证与客户端身份注入接口会接收实际业务请求，Kratos 中间件负责透传。
HMAC 使用 Kratos 3.0 transport context 构造规范请求；其他 token 认证器继续兼容
原生 gRPC metadata 和 Kratos HTTP/gRPC transport context。OIDC、OAuth2 的客户端
身份必须通过对应的 `WithClientToken` 显式配置，认证器不会把 subject 当作访问令牌。
