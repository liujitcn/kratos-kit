# sdk

`sdk` 提供仓库级共享运行时入口：

```go
import "github.com/liujitcn/kratos-kit/sdk"

sdk.Runtime.SetCache(cacheClient)
cacheClient := sdk.Runtime.GetCache()
```

`sdk.Runtime` 的具体能力由 [`runtime.Runtime`](../runtime/README.md) 定义，包含数据库、缓存、队列、对象存储、锁、翻译器、密钥和自定义实例。应用启动阶段负责写入，业务组件通过 getter 读取。

全局运行时适合单进程应用；测试或需要多套依赖的场景应创建独立 `runtime.Application`。
