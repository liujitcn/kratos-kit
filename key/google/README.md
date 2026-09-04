# key/google

Google Secret Manager 实现由 `key.NewKey` 内部使用。Google SDK 使用 Application Default Credentials 或
工作负载身份。

```yaml
type: google
scope: prod/order-service
root_name: kratos-root
google:
  project: example-project
```

`root_name` 也可以直接写成 `projects/{project}/secrets/{secret}` 完整资源名。
