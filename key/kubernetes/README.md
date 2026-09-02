# key/kubernetes

Kubernetes Secret API 实现由 `key.NewKey` 内部使用，并通过 Pod 的 ServiceAccount 读取 Secret。

```yaml
type: kubernetes
scope: prod/order-service
root_name: kratos-root
kubernetes:
  namespace: prod
  value_key: root-key
```

Kubernetes Secret 没有原生版本，读取结果使用资源版本号；轮换后建议重启工作负载。也可以使用 `key/file`
读取 Secret 挂载文件。
