# __PROJECT_NAME__ 后端

这是 `kratos-admin` 生成的后端模板，Go module 为 `__MODULE_PATH__`。后端服务入口、业务代码、Proto 和生成文件属于同一 module。

## 结构

```text
api/proto
internal/{biz,data,service,server,task,module}
configs
migration/assets/v0.0.1/mysql
scripts
Makefile
```

每个业务模块都可以拥有自己的 Proto、模型、Service、Server、Task、国际化、OpenAPI 和迁移资源。`internal/module` 负责汇总各层 ProviderSet。

## 配置和启动

```bash
make init
make run-only
```

配置文件位于 `configs`。运行前准备数据库、缓存和密钥服务，并通过环境变量或 Secret Manager 注入敏感凭据。

## 开发命令

```bash
make gen
make test
make build
```

新增表后把模型加入 `internal/data.Models()`；新增接口后重新生成 Go、OpenAPI、脱敏和 TypeScript 产物。迁移脚本放入 `migration/assets/v0.0.1/mysql`，空目录只保留说明文件。
