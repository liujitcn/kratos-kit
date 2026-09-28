# kratos-kit 通知发送 SDK

`notify` 提供通知发送渠道的公共契约和实例管理器。公共类型位于根包 `notify`，各渠道的具体 Sender 与 Config 位于对应子包。业务管理模块负责持久化 Provider 配置，并在运行时创建和替换 Sender。

## 内置渠道

| 渠道 | 模块 | 说明 |
| --- | --- | --- |
| `email` | `notify/email` | SMTP，支持 NONE、START_TLS 和 SSL。 |
| `webhook` | `notify/webhook` | 通用 JSON Webhook，可配置请求头及 HMAC-SHA256 签名。 |
| `dingtalk` | `notify/dingtalk` | 钉钉应用工作通知与群机器人消息；由不同 Sender 实现区分。 |
| `feishu_app` | `notify/feishu` | 飞书自建应用文本及富文本消息，支持多种接收者 ID。 |
| `feishu_group` | `notify/feishu` | 飞书群机器人文本及富文本消息，支持加签。 |
| `wechat_official` | `notify/wechat` | 微信公众号模板消息。 |
| `wechat_work_app` | `notify/wechatwork` | 企业微信应用文本及 Markdown 消息。 |
| `wechat_work_group` | `notify/wechatwork` | 企业微信群机器人文本及 Markdown 消息。 |
| `sms` | `notify/sms` | 阿里云、腾讯云、华为云短信适配；同属 `notify` 模块。 |

平台应用、群机器人和通用 Webhook 的 HTTP 请求统一通过 `go-utils/http` 发送，底层 transport 可注入；Webhook 与群机器人额外限制重定向和目标 IP。短信发送使用服务商官方 SDK，由 SDK 管理鉴权签名和请求协议。

## 注册与发送

渠道配置实例名称用于区分同类渠道的不同账号或目标，例如 `smtp-default`、`ops-feishu-webhook`。应用消息 Sender 需要传入宿主统一管理的 `cache.Cache`，用于共享平台 access token；业务管理模块可以从数据库读取配置后创建 Sender，并在配置更新时整体替换快照。

```go
mailSender, err := email.New(email.Config{
	Name: "smtp-default",
	Host: "smtp.example.com",
	Port: 587,
	Username: "robot@example.com",
	Password: "secret",
	From: "系统通知 <robot@example.com>",
	TLSMode: email.TLSStartTLS,
})
if err != nil {
	return err
}

webhookSender, err := webhook.New(webhook.Config{
	Name: "ops-webhook",
	URL: "https://hooks.example.com/notify",
	Secret: "signing-secret",
})
if err != nil {
	return err
}

manager, err := notify.NewManager(mailSender, webhookSender)
if err != nil {
	return err
}

receipt, err := manager.Send(ctx, "ops-webhook", notify.Message{
	Title: "服务状态",
	Content: "服务已恢复",
	ContentType: notify.Markdown,
})
```

同一渠道类型允许注册多个实例；发送时按实例名选择目标。Webhook 默认要求 HTTPS；仅明确受控的本地或内网地址可设置 `AllowHTTP`。

## OAuth 与用户绑定

`oauth` 继续只负责授权登录、Token 和用户标识获取；通知渠道负责消息发送。Admin 可将 `base_oauth_provider` 的应用凭据交给后续平台应用消息 Sender，并从 `base_third_account` 解析接收人的第三方标识。两个模块共享第三方应用身份，但职责和凭据用途保持分开。

## 并发与配置替换

Manager 的 `Register`、`Get`、`Send`、`Names` 和 `Replace` 可并发调用。`Replace` 会先校验完整新集合，再一次性切换当前快照；校验失败时原有 Sender 保持有效。Sender 创建后应视为不可变对象，凭据更新时创建新对象并调用 `Replace`。

Webhook 默认拒绝 HTTP、私网/环回/链路本地目标和重定向；仅在目标地址可信且网络隔离明确时，才启用 `AllowHTTP` 或 `AllowPrivateNetwork`。默认 HTTP transport 不读取代理环境变量，并在实际拨号时重新解析和校验目标 IP。
