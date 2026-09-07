# 运行时脱敏

`redact` 提供由 `cmd/protoc-gen-go-redact` 生成代码使用的运行时脱敏能力。

## 能力边界

- `Redactor`、`Apply`：执行生成消息的脱敏方法。
- `PolicyResolver`、`ApplyWith`：使用显式传入的解析器递归处理 Proto 消息；未传入时仅通过 `WithPolicyResolver` / `PolicyResolverFromContext` 使用当前请求的解析器，不保存进程级默认实例。`WithOperation`、`WithDirection` 和 `WithScene` 可按接口、请求/响应方向及场景选择策略。
- `NewFieldPolicy`：将数据库中的规则类型和 JSON 配置转换为运行时策略。
- `Mask`、`Regex`、`Email`、`Truncate`、`Hash`、`UUID`、`IP`、`URL`、`FixedLength`：实现 Proto 规则对应的具体算法。
- `RegisterCustomRedactor`：注册命名的自定义脱敏函数。
- `ServerStreamRedactor`、`BidiStreamRedactor`、`ClientStreamRedactor`：包装 gRPC 流式响应并在发送前执行脱敏。
- `StoragePolicyResolver`、`StorageValueStore`、`RedactStorage`：通过抽象策略和存储接口执行敏感字段的加密、脱敏值保存、原文恢复和摘要查询。

本模块不负责具体数据库表、Repository 和策略配置；具体数据库适配与规则管理由业务模块实现，代码生成由 `cmd/protoc-gen-go-redact` 完成。

## 依赖注入

宿主通过构造参数注入 `PolicyResolver`，在 HTTP、gRPC（含流式）和 MCP 请求入口将其写入请求上下文。
生成的服务包装器会保留此上下文，因此无需设置全局策略或修改生成产物；流式包装器也可直接设置自身的 `Resolver` 字段。
显式 `ApplyWith(ctx, resolver, message)` 优先于请求上下文；两者都未提供解析器时执行消息默认规则。
不同应用与请求不会互相覆盖解析器，`WithPolicyResolver(ctx, nil)` 可清除继承的请求解析器。

## 安装

```bash
go get github.com/liujitcn/kratos-kit/redact@latest
```

生成后的 `*.pb.redact.go` 会自动导入本模块。
