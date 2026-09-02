# __PROJECT_NAME__

`__PROJECT_NAME__` 是由 `kratos-admin` 创建的前后端项目，包含独立的 Kratos
后端，以及管理端、uni-app 和 Taro 三套前端 workspace。

## 目录

```text
.
├── backend       # kratos-core 后端服务
├── frontend
│   ├── admin     # 由 @liujitcn/kratos-admin-cli 生成
│   ├── uni-app   # 由 @liujitcn/kratos-uni-app-cli 生成
│   ├── taro-app  # 由 @liujitcn/kratos-taro-app-cli 生成
│   ├── Makefile  # 三套前端的聚合命令
│   └── scripts   # 依赖重装和 npm 发布脚本
├── scripts       # 前后端快捷脚本
├── Makefile
└── README.md
```

前端源码不属于本项目模板，由三个上游 CLI 在创建过程中生成。后端使用 SQLite
文件、内存缓存和内存队列，默认不需要预先启动 MySQL、Redis 或 Consul。

## 创建

```bash
kratos-admin create __PROJECT_NAME__
```

后端 Go module 当前为 `__MODULE_PATH__`。可以在生成时通过
`--module <go-module>` 指定实际 module 路径，通过 `--frontend-module <module>`
指定前端默认业务 module。

## 开发

首次进入项目后安装依赖：

```bash
make init
```

启动后端：

```bash
make run
```

启动管理端前端开发服务：

```bash
make frontend-dev
```

启动 uni-app 或 Taro H5 开发服务：

```bash
make frontend-uni-dev
make frontend-taro-dev
```

也可以进入 `frontend/` 使用从上游迁移的完整三端 Makefile：

```bash
make -C frontend help
make -C frontend init
make -C frontend ts
make -C frontend check
make -C frontend build
```

后端默认监听 HTTP `:7001` 和 gRPC `:6001`。常用生成、测试和构建命令如下：

```bash
make generate
make test
make build
```
