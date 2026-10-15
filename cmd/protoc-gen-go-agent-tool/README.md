# protoc-gen-go-agent-tool

该 protoc 插件从 protobuf service 生成 Eino Agent Tool。生成代码通过服务端接口直接调用进程内方法，不创建额外的 gRPC client 或网络请求。

## 使用

```bash
protoc --go-agent-tool_out=. path/to/service.proto
```

生成文件后缀为 `_agent_tool.go`。每个 RPC 方法生成工具名称、输入 schema 和执行函数；服务端实现作为构造参数传入。生成器会处理 deprecated service/method 标记，并对递归消息 schema 使用 `any` 作为安全降级类型。

工具名称按标准 RPC 路径转换，例如 `/orders.v1.OrderService/GetOrder` 对应 `orders_v1_order_service_get_order`。
