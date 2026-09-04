# key/vault

本地 Vault KV v1/v2 实现由 `key.NewKey` 内部使用。Vault Token、TLS 证书等认证信息使用 Vault SDK 的
环境或工作负载身份，不写入 `key.yaml`。

```yaml
type: vault
scope: prod/order-service
root_name: secret/data/kratos/prod/root
vault:
  address: http://127.0.0.1:8200
  value_key: value
```

## 本地 Vault

开发用 Compose 位于当前目录：

```bash
docker compose up -d
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=kratos-dev-token
vault kv put secret/kratos/prod/root value="$(openssl rand -base64 32)"
```

Compose 使用 Vault dev mode，数据随容器删除而丢失，仅用于本地开发和测试。
