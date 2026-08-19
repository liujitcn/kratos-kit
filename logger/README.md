# 日志模块

## 概述

`logger` 模块负责创建 `kratos-kit` 的统一日志实现，并为应用附加标准字段（如 `caller`、`trace_id`、`span_id`）。当前模块基于 Kratos v3 与标准库 `log/slog`。

## 当前行为

- `logger.NewLoggerProvider` 会为应用初始化全局日志实例，并注入标准日志字段。
- `bootstrap` 在启动阶段会调用 `log.SetDefault(ctx.logger)`，因此通过 Kratos v3 `log` 包输出的结构化日志会统一进入当前应用日志链路。
- `zap`、`logrus`、`zerolog` 的文本格式统一为 `时间 LEVEL caller 消息 结构化字段`。
- 控制台输出保留绝对路径 `caller`，便于在 IDE 或终端中直接点击源码。
- 文件输出根据 caller 源码附近最近的 `go.mod` 生成 `module/相对路径:行号`，例如 `github.com/liujitcn/kratos-core/data/base_log.go:25`。
- GORM/SQL 日志优先使用业务层传入的 caller；同时兼容从旧日志消息首段推断 caller，SQL 正文保持不变。
- `fluent`、`aliyun`、`tencent` 按 `level`、`msg`、`caller`、业务字段上传结构化日志，caller 使用 module 路径。
- 空 `trace_id`、`span_id` 以及重复的 `service.*` 标准字段会被过滤。

## 使用说明

启用具体日志实现时，需要通过匿名导入触发工厂注册，例如：

```go
import (
	_ "github.com/liujitcn/kratos-kit/logger/zap"
)
```

完成注册后，应用启动阶段会根据配置中的 `logger.type` 创建对应日志实现。

## Logrus 配置

`filepath` 为空时仅输出控制台；配置目录后写入 `info.log`，并可通过 `enable_console` 同时输出控制台。文件输出始终关闭颜色并清理 ANSI 控制码。

```yaml
logger:
  type: logrus
  logrus:
    level: debug
    formatter: text
    timestamp_format: "2006-01-02 15:04:05.000"
    filepath: ./data/logs
    max_size: 100
    max_age: 30
    max_backups: 5
    enable_console: true
```

## Zerolog 配置

`writer` 默认为 `stdout`，还支持 `stderr`、`console`、`file`、`lumberjack`。文件类 writer 将 `filepath` 视为目录并写入 `info.log`。

```yaml
logger:
  type: zerolog
  zerolog:
    level: debug
    writer: lumberjack
    filepath: ./data/logs
    max_size: 100
    max_age: 30
    max_backups: 5
    enable_console: true
```
