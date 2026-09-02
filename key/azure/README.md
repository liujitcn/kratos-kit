# key/azure

Azure Key Vault Secrets 实现由 `key.NewKey` 内部使用。Azure SDK 使用 Managed Identity 或
DefaultAzureCredential。

```yaml
type: azure
scope: prod/order-service
root_name: kratos-root
root_version: version-id
azure:
  vault_url: https://example.vault.azure.net
```
