# 运行时脱敏

`redact` 提供由 `cmd/protoc-gen-go-redact` 生成代码使用的运行时脱敏能力。

## 能力边界

- `Redactor`、`Apply`：执行生成消息的脱敏方法。
- `PolicyResolver`、`ApplyWith`：按运行时上下文解析字段策略并递归处理 Proto 消息；`WithScene` 可为同一字段选择不同场景策略。
- `NewFieldPolicy`：将数据库中的规则类型和 JSON 配置转换为运行时策略。
- `Mask`、`Regex`、`Email`、`Truncate`、`Hash`、`UUID`、`IP`、`URL`、`FixedLength`：实现 Proto 规则对应的具体算法。
- `RegisterCustomRedactor`：注册命名的自定义脱敏函数。
- `ServerStreamRedactor`、`BidiStreamRedactor`、`ClientStreamRedactor`：包装 gRPC 流式响应并在发送前执行脱敏。

本模块不解析 Proto、不生成代码，也不负责数据库存储和策略配置；代码生成由 `cmd/protoc-gen-go-redact` 完成。

## 安装

```bash
go get github.com/liujitcn/kratos-kit/redact@latest
```

生成后的 `*.pb.redact.go` 会自动导入本模块。
