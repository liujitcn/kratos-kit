# protoc-gen-go-mcp-tool

该 protoc 插件从 protobuf service 生成 MCP Tool 注册代码。生成工具的执行函数直接调用进程内服务端实现，输入输出 schema 根据 protobuf 描述生成。

## 使用

```bash
protoc --go-mcp-tool_out=. path/to/service.proto
```

生成文件后缀为 `_mcp_tool.go`。工具名称按 `/package.Service/Method` 转为 snake_case；递归消息无法安全展开 JSON Schema 时会降级为 `any`，执行阶段仍转换为真实 protobuf 请求。

如需在其他代码中复用名称规则，可调用 `utils.ToolNameFromRPCPath`。
