# config 包说明

## 概述

`config` 包用于统一加载 Kratos 配置，支持：

- 本地文件配置源
- 远程配置源（通过工厂注册机制按类型创建）
- 运行时注册自定义配置结构并统一 `Scan`

核心入口：

- `LoadBootstrapConfig(configPath string) error`
- `NewConfigProvider(configPath string) (config.Config, error)`
- `NewProvider(cfg *conf.Config) (config.Source, error)`

## 加载流程

`LoadBootstrapConfig` 的执行逻辑如下：

1. 先调用 `NewConfigProvider(configPath)` 创建 `config.Config`。
2. 始终加载本地配置源（`configPath`）。
3. 若 `configPath/config.yaml` 存在，则先读取其中的 `conf.Config`，再创建远程配置源并与本地源合并加载。
4. 扫描并填充已注册配置对象（默认包含 `Bootstrap` 及其常见子配置）。

说明：远程源配置文件名固定为 `config.yaml`。

## 工厂机制

支持通过类型注册远程配置源工厂：

- `RegisterFactory(name Type, f Factory) error`
- `MustRegisterFactory(name Type, f Factory)`
- `GetFactory(name Type) (Factory, bool)`
- `ListFactories() []string`

`NewProvider` 会读取 `cfg.type`，按类型从工厂表创建远程 `config.Source`。

内置类型常量：

- `file`
- `apollo`
- `consul`
- `etcd`
- `kubernetes`
- `nacos`
- `polaris`

## 远程配置文件示例（config.yaml）

下面示例展示如何在本地配置目录下启用远程配置源：

```yaml
# 文件位置：{configPath}/config.yaml
config:
  type: etcd
  etcd:
    endpoints:
      - "127.0.0.1:2379"
    timeout: 5s
    key: "kratos.bootstrap"
```

`type` 决定使用哪个远程实现，且只会创建一种远程源。

## 各远程源参数说明

### Etcd

对应字段：`config.etcd`。

- `endpoints`: etcd 地址列表
- `timeout`: 连接超时时间（`google.protobuf.Duration`）
- `key`: 配置 key（内部会将 `.` 转为 `/`）

### Consul

对应字段：`config.consul`。

- `scheme`: 协议（如 `http` / `https`）
- `address`: Consul 地址
- `key`: 配置 key（内部会将 `.` 转为 `/`）

### Nacos

对应字段：`config.nacos`。

- `address`, `port`: 服务地址
- `username`, `password`: 鉴权
- `namespace_id`, `group`, `data_id`: 命名空间与配置标识
- `timeout_ms`, `beat_interval`, `update_thread_num`: 客户端行为参数
- `log_level`, `cache_dir`, `log_dir`: 日志与缓存
- `not_load_cache_at_start`, `update_cache_when_empty`: 缓存策略

默认值：

- `group` 为空时使用 `DEFAULT_GROUP`
- `data_id` 为空时使用 `bootstrap.yaml`

### Apollo

对应字段：`config.apollo`。

- `endpoint`: Apollo 服务地址
- `app_id`: 应用 ID
- `cluster`: 集群
- `namespace`: 命名空间
- `secret`: 密钥

### Kubernetes

对应字段：`config.kubernetes`。

- `namespace`: 必填
- `label_selector`: ConfigMap 标签筛选
- `field_selector`: ConfigMap 字段筛选
- `kube_config`: 集群外访问配置文件
- `master`: API Server 地址（通常与 `kube_config` 搭配）

### Polaris

对应字段：`config.polaris`。

- `namespace`
- `file_group`
- `file_name`

## 使用示例

```go
package example

import (
	"github.com/liujitcn/kratos-kit/config"

	_ "github.com/liujitcn/kratos-kit/config/etcd" // 按需引入具体实现
)

func loadConfig() error {
	// configPath 通常是配置目录（目录内可包含 bootstrap 文件和可选的 config.yaml）
	return config.LoadBootstrapConfig("configs")
}
```

注册自定义配置结构：

```go
package example

import (
	"github.com/liujitcn/kratos-kit/config"
	"google.golang.org/protobuf/proto"
)

func registerCustom(c proto.Message) {
	config.RegisterConfig(c)
}
```

## 已知限制

- `LoadRemoteConfigSourceConfigs` 会查找 `filepath.Join(configPath, "config.yaml")`，因此 `configPath` 需与该约定匹配。
- `polaris` 实现当前在 `init()` 中注册到 `TypeNacos`，这会导致与 `nacos` 同时引入时发生重复注册冲突；建议修复后再同时使用。
