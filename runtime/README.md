# runtime

`runtime` 保存进程内共享基础设施实例，避免启动流程和业务组件之间重复传递依赖。

## 共享内容

`Runtime` 可以保存多个具名 GORM 客户端，以及默认数据库、缓存、对象存储、锁、队列、翻译器、密钥和自定义实例。`GetGormClient(name)` 找不到指定名称时回退到默认客户端；`GetGormClients` 返回副本，避免调用方直接修改内部 map。

```go
runtime := runtime.NewRuntime()
runtime.SetCache(cacheClient)
runtime.SetInterface("feature", feature)

cacheClient := runtime.GetCache()
feature := runtime.GetInterface("feature")
```

## 全局入口

[`sdk`](../sdk/README.md) 暴露一个预创建的 `sdk.Runtime`。需要隔离测试或多套运行时的代码应直接创建 `runtime.NewRuntime()`，不要依赖全局状态。
