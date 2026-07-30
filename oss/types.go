package oss

// Type 表示对象存储类型。
type Type string

const (
	// Local 表示本地文件存储。
	Local Type = "local"
	// Aliyun 表示阿里云 OSS。
	Aliyun Type = "aliyun"
	// Ftp 表示 FTP 存储。
	Ftp Type = "ftp"
	// Minio 表示 MinIO 对象存储。
	Minio Type = "minio"
	// S3 表示 AWS S3 及兼容对象存储。
	S3 Type = "s3"
)
