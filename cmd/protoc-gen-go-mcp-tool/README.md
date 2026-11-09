# protoc-gen-go-mcp-tool

该 protoc 插件从 protobuf service 生成基于 `github.com/mark3labs/mcp-go` 的 MCP Tool 注册代码。生成工具的执行函数直接调用进程内服务端实现，输入输出 schema 通过 mcp-go 从 protobuf Go 类型推导。

## 使用

```bash
protoc --go-mcp-tool_out=. path/to/service.proto
```

生成文件后缀为 `_mcp_tool.go`，注册函数接收 `*server.MCPServer`。工具名称按 `/package.Service/Method` 转为 snake_case；递归输入使用通用 object schema，递归输出不展开 schema，执行阶段仍通过 `CallToolRequest.BindArguments` 绑定真实 protobuf 请求。

结果通过 `mcp.NewToolResultJSON` 同时返回文本和结构化数据，业务错误返回 `IsError` 工具结果。从旧 SDK 升级时，请重新运行上述生成命令，调用方可将 `transport/mcp.Server.MCPServer()` 传给注册函数。

如需在其他代码中复用名称规则，可调用 `utils.ToolNameFromRPCPath`。
