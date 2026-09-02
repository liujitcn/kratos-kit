# key/aws

AWS Secrets Manager 实现由 `key.NewKey` 内部使用。AWS SDK 使用默认 credential chain，生产环境建议
使用 IAM Role 或工作负载身份。

```yaml
type: aws
scope: prod/order-service
root_name: prod/kratos/root
root_version: version-id
aws:
  region: cn-north-1
  version_stage: AWSCURRENT
```
