# __PROJECT_NAME__

这是由 `kratos-admin` 生成的项目模板。模板包含一个后端 Go module、三个前端目录、共享脚本、配置、迁移和根 Makefile；业务模块清单为 `__MODULES__`。

## 目录

```text
.
├── backend
│   ├── api/proto
│   ├── internal/{biz,data,service,server,task,module}
│   ├── configs
│   ├── migration/assets/v0.0.1/mysql
│   ├── scripts
│   └── Makefile
├── frontend
│   ├── admin
│   ├── uni-app
│   └── taro-app
├── scripts
├── Makefile
└── README.md
```

前端源码、语言注册和检查构建文件由生成阶段写入；后端业务代码和服务入口属于同一个 Go module。项目只维护自己的业务模块，不修改生成器模板之外的公共运行时。

## 启动与验证

```bash
make -C backend init
make -C frontend init
make -C backend run-only
make -C backend test
make -C frontend check
```

具体配置位于 `backend/configs`。生产环境应替换示例密钥、数据库和缓存连接信息。

## 扩展业务模块

新增模块时，在下列目录增加同名业务代码和资源：

```text
backend/api/proto/<module>
backend/internal/{biz,data,service,server,task}/<module>
backend/internal/module
backend/internal/i18n/assets
backend/internal/openapi/assets
backend/migration/assets/v0.0.1/mysql
```

完成 Proto、代码生成、模型注册、迁移和前端 RPC 后，执行 `make -C backend gen`、`make -C backend test` 和 `make -C frontend check`。
