# key

`key` 按 `configv1.Key` 创建密钥实例，并按用途派生 32 字节业务密钥。核心包只提供一个接口和一个初始化入口：

```go
type Key interface {
	Derive(context.Context, string) ([]byte, error)
}

func NewKey(ctx context.Context, cfg *configv1.Key) (Key, error)
```

业务代码不需要初始化或调用 Vault、AWS、Google、Azure、Kubernetes 的客户端。`NewKey` 内部根据
`cfg.Type` 选择实现：`file`、`vault`、`aws`、`google`、`azure`、`kubernetes`；空值或未知值使用 `file`。

## 配置

密钥配置独立放在 `key.yaml` 或 `key.<env>.yaml`，文件内容直接对应 `configv1.Key`，不嵌套在 `Bootstrap` 中：

```yaml
type: vault
scope: prod/order-service
root_name: secret/data/kratos/prod/root
root_version: "3"
vault:
  address: http://127.0.0.1:8200
  value_key: value
```

Provider 的认证信息不写入该文件，使用各 SDK 的工作负载身份或环境认证。

## 启动流程

`bootstrap.RunApp` 默认执行以下流程：

```text
读取 key.yaml / key.<env>.yaml
        ↓
复用 sdk.Runtime.GetKey() 中已有的 Key 实例
        ↓（没有时调用 key.NewKey）
Derive("config") 获取配置解密密钥
        ↓
使用 ENC[config:payload] 解密配置
        ↓
初始化业务应用
```

没有 key 配置时默认使用 `${conf}/root.key`，默认配置目录下即 `configs/root.key`。根密钥必须预先生成，
系统不会自动生成或覆盖：

```bash
umask 077
openssl rand -base64 32 > configs/root.key
```

也可以在启动前自行创建 `Key` 实例并保存到运行时：

```go
value, err := key.NewKey(ctx, keyConfig)
if err != nil {
	return err
}
sdk.Runtime.SetKey(value)
```

之后继续调用原有的 `bootstrap.RunApp` 即可。

## 派生规则

派生算法为 HKDF-SHA256，派生上下文包含：

```text
scope + purpose + root_version
```

相同根密钥、范围、用途和根版本会生成相同结果；不同服务或用途必须使用不同的 `scope`/`purpose`。
根密钥轮换时增加新版本，旧版本应保留到历史 Token 和密文不再需要为止。

## Provider

Provider 子包由 `key.NewKey` 内部调用，业务只需要依赖核心模块：

```bash
go get github.com/liujitcn/kratos-kit/key@latest
```

- `key/file`：本地文件，适合开发环境和挂载的 Kubernetes Secret。
- `key/vault`：HashiCorp Vault KV v1/v2；本地 Compose 位于 `key/vault/docker-compose.yml`。
- `key/aws`：AWS Secrets Manager，使用 AWS SDK 默认 credential chain。
- `key/google`：Google Secret Manager，使用 Application Default Credentials。
- `key/azure`：Azure Key Vault Secrets，使用 Managed Identity 或 DefaultAzureCredential。
- `key/kubernetes`：Kubernetes Secret API，使用 Pod ServiceAccount。
