# config 包说明

## 概述

`config` 包用于统一加载 Kratos 配置，支持：

- 本地文件配置源
- 远程配置源（通过工厂注册机制按类型创建）
- 运行时注册自定义配置结构并统一 `Scan`

核心入口：

- `LoadBootstrapConfig(configPath, env string, keyValue key.Key) error`
- `SecretCipher.EncryptMarkedConfig(data []byte, format string) ([]byte, error)`
- `NewProvider(cfg *configv1.Config) (config.Source, error)`（远程配置工厂入口）

## 加载流程

`LoadBootstrapConfig(configPath, env, keyValue)` 是配置包唯一的完整加载入口，执行逻辑如下：

1. 优先使用调用方传入的 `keyValue`；传入 `nil` 时从 `sdk.Runtime.GetKey()` 获取。
2. 按 `purpose=config` 派生配置密钥，并自动解密 `ENC[...]` 字段。
3. 按 `env` 加载基础配置和环境覆盖配置。
4. 若存在 `config.yaml`，读取远程配置源参数并与本地源合并加载。
5. 扫描并填充已注册配置对象（默认包含 `Bootstrap` 及其常见子配置）。

调用该入口前，业务或 `bootstrap.RunApp` 可初始化 `key.Key` 并传入；也可以传入 `nil`，由配置包复用
`sdk.Runtime` 中已设置的实例。配置包不会自行生成或轮换根密钥。

说明：远程源配置文件名固定为 `config.yaml`。

应用通过 `bootstrap.RunApp` 启动时，会在上述完整配置加载前单独读取 `key.yaml` 和
`key.<env>.yaml`，准备配置解密密钥，再使用同一套配置源加载业务配置。`key` 文件不参与远程配置，
也不能使用 `ENC[...]` 加密；它只描述 Provider 和根密钥引用。未配置 key 文件时默认使用 `${configPath}/root.key`。

## 环境配置

`LoadBootstrapConfig` 支持在同一目录中按运行环境加载覆盖文件。基础文件始终加载，当前环境文件随后加载并覆盖同名字段，其他环境文件不会进入配置源。

```text
configs/
├── data.yaml
├── data.dev.yaml
├── data.prod.yaml
├── server.yaml
└── server.prod.yaml
```

传入 `env=dev` 时加载顺序为：

```text
data.yaml -> server.yaml -> data.dev.yaml
```

`data.prod.yaml` 和 `server.prod.yaml` 会被忽略。某个环境覆盖文件不存在时直接保留对应基础配置，因此环境文件只需声明差异字段。环境名称仅允许小写字母、数字、短横线和下划线；文件名中除扩展名外的最后一个句点用于分隔环境名称。

使用环境配置时，本地源按“基础配置 -> 当前环境配置”的顺序合并；启用远程配置源后，远程配置仍在本地源之后加载并拥有最高优先级。`config.<env>.yaml` 可以覆盖 `config.yaml` 中的远程配置源参数。

bootstrap 会在调用配置入口前读取独立的 `key.yaml` 和 `key.<env>.yaml`，初始化或复用 `sdk.Runtime` 中的
`key.Key`。`key` 配置只描述 Provider 和根密钥引用，不能使用 `ENC[...]` 加密。

## 敏感字段加密

配置文件只需要加密敏感字段时，可使用 `SecretCipher` 将明文转换为
`ENC[payload]` 格式；内部认证标识固定为 `kratos-kit:config`，不会写入密文，普通字段不需要修改。
需要加密的明文先写成 `ENC(明文)` 标记。
`ENC(...)` 可以占据整个值，也可以嵌入字符串中；嵌入时只替换标记部分。

```go
key := []byte("01234567890123456789012345678901") // 应从环境变量、KMS 或 Vault 获取
cipher, err := config.NewSecretCipher(key)
if err != nil {
	return err
}
```

配置文件中只标记需要加密的值：

```yaml
data:
  redis:
    password: ENC(redis-password)
  database:
    source: "root:ENC(db-password)@tcp(127.0.0.1:3306)/app"
```

批量生成密文时，`EncryptMarkedConfig` 只替换显式标记的 `ENC(明文)`，不会根据字段名猜测：

```go
encryptedConfig, err := cipher.EncryptMarkedConfig(configBytes, "yaml")
if err != nil {
	return err
}
```

处理后的配置会变成：

```yaml
data:
  redis:
    password: ENC[...]
  database:
    source: "root:ENC[...]@tcp(127.0.0.1:3306)/app"
```

`Decrypt` 会解密值中一个或多个 `ENC[...]` 标记，普通明文和未标记部分会原样返回，便于配置加载器递归处理配置树。
加密密钥不能与配置文件放在同一位置；密钥轮换时应保留旧密钥用于解密历史配置。

应用启动时配置入口固定使用内部认证标识 `kratos-kit:config`，离线加密流程应使用相同的派生规则；该标识不会写入配置密文。

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

启动配置还支持 `bootstrap.translator`，由 `github.com/liujitcn/kratos-kit/translator`
按 `translator.type` 创建机器翻译 Provider。内置 `google`、`baidu`、`alibaba` 和
`volc`，通用超时使用 `translator.timeout`；自定义 Provider 可通过模块注册接口扩展。

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
	return config.LoadBootstrapConfig("configs", "dev", nil)
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

- 远程配置源参数固定从 `filepath.Join(configPath, "config.yaml")` 读取，因此 `configPath` 需与该约定匹配。
- `polaris` 实现当前在 `init()` 中注册到 `TypeNacos`，这会导致与 `nacos` 同时引入时发生重复注册冲突；建议修复后再同时使用。
