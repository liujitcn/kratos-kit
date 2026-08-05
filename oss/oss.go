package oss

import (
	"context"
	"fmt"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/oss/aliyun"
	"github.com/liujitcn/kratos-kit/oss/ftp"
	"github.com/liujitcn/kratos-kit/oss/local"
	"github.com/liujitcn/kratos-kit/oss/minio"
	"github.com/liujitcn/kratos-kit/oss/s3"
)

// OSS 定义项目统一使用的对象存储能力。
type OSS interface {
	// Upload 上传本地文件。
	Upload(fileName string, filePath string, localFile string) (string, error)
	// UploadByByte 上传内存数据。
	UploadByByte(fileName string, filePath string, fileByte []byte) (string, error)
	// GetFileByte 下载对象内容。
	GetFileByte(filePath string) ([]byte, error)
	// DeleteFile 删除对象。
	DeleteFile(filePath string) error
}

// NewOSS 创建对象存储，并返回配置或客户端初始化错误。
func NewOSS(cfg *configv1.Oss) (OSS, error) {
	if cfg == nil {
		return local.NewOSS("./data"), nil
	}

	rootDirectory := cfg.RootDirectory

	switch Type(cfg.Type) {
	default:
		fallthrough
	case Local:
		return local.NewOSS(rootDirectory), nil
	case Ftp:
		if cfg.Ftp == nil {
			return nil, fmt.Errorf("oss: %s config is nil", Ftp)
		}
		return ftp.NewOSS(cfg.Ftp, rootDirectory), nil
	case Aliyun:
		if cfg.Aliyun == nil {
			return nil, fmt.Errorf("oss: %s config is nil", Aliyun)
		}
		return aliyun.NewOSS(cfg.Aliyun, rootDirectory), nil
	case Minio:
		if cfg.Minio == nil {
			return nil, fmt.Errorf("oss: %s config is nil", Minio)
		}
		return minio.NewOSS(cfg.Minio, rootDirectory), nil
	case S3:
		if cfg.S3 == nil {
			return nil, fmt.Errorf("oss: %s config is nil", S3)
		}
		storage, err := s3.NewStorage(context.Background(), &s3.Config{
			Endpoint:       cfg.S3.Endpoint,
			Region:         cfg.S3.Region,
			AccessKey:      cfg.S3.AccessKey,
			SecretKey:      cfg.S3.SecretKey,
			Token:          cfg.S3.Token,
			UseSSL:         cfg.S3.UseSsl,
			ForcePathStyle: cfg.S3.ForcePathStyle,
			Bucket:         cfg.S3.BucketName,
			RootDirectory:  rootDirectory,
		})
		if err != nil {
			return nil, err
		}
		return storage, nil
	}
}
