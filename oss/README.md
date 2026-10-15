# oss

`oss` 定义统一对象存储接口，并根据 `config.v1.Oss` 创建实现。公共操作包括上传本地文件、上传字节、读取对象和删除对象。

## 存储类型

| `type` | 实现 |
| --- | --- |
| `local` | 本地文件系统，未提供配置时默认写入 `./data`。 |
| `ftp` | FTP。 |
| `aliyun` | 阿里云 OSS。 |
| `minio` | MinIO。 |
| `s3` | AWS S3 及兼容服务，支持自定义 endpoint、Path Style 和临时 token。 |

```go
store, err := oss.NewOSS(cfg)
if err != nil {
	return err
}
path, err := store.UploadByByte("avatar.png", "users/1", data)
```

对象路径会在实现内部与根目录组合；调用方只应传入相对对象路径。S3 低层能力可使用 `oss/s3.NewStorage`，其 `PutObject` 支持可 seek 和流式请求体。
