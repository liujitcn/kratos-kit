# 日志模块

## 概述

`logger` 模块负责创建 `kratos-kit` 的统一日志实现，并为应用附加标准字段（如 `caller`、`trace_id`、`span_id`）。

## 当前行为

- `logger.NewLoggerProvider` 会为应用初始化全局日志实例，并注入标准日志字段。
- `bootstrap` 在启动阶段会调用 `log.SetLogger(ctx.logger)`，因此包级 `log.Debugf`、`log.Infof` 等输出会统一进入当前应用日志链路。
- `zap` 控制台输出保留级别颜色，并优先打印绝对路径 `caller`，便于在 IDE 或终端中直接点击跳转源码。
- `zap` 文件输出会将 `caller` 压缩为相对短路径，避免日志文件内容过长。
- 本地项目短路径会优先按当前仓库根目录对应的 `go.mod` 裁剪；若存在嵌套子模块，则仍以仓库根目录为准。

## 使用说明

启用具体日志实现时，需要通过匿名导入触发工厂注册，例如：

```go
import (
	_ "github.com/liujitcn/kratos-kit/logger/zap"
)
```

完成注册后，应用启动阶段会根据配置中的 `logger.type` 创建对应日志实现。
