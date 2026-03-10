package oss

import (
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/oss/aliyun"
	"github.com/liujitcn/kratos-kit/oss/ftp"
	"github.com/liujitcn/kratos-kit/oss/local"
	"github.com/liujitcn/kratos-kit/oss/minio"
)

type OSS interface {
	Upload(fileName string, filePath string, localFile string) (string, error)
	UploadByByte(fileName string, filePath string, fileByte []byte) (string, error)
	GetFileByte(filePath string) ([]byte, error)
	DeleteFile(filePath string) error
}

// NewOSS 创建一个新的文件操作
func NewOSS(cfg *conf.OSS) OSS {
	if cfg == nil {
		return local.NewOSS("./data")
	}

	rootDirectory := cfg.RootDirectory

	switch Type(cfg.Type) {
	default:
		fallthrough
	case Local:
		return local.NewOSS(rootDirectory)
	case Ftp:
		return ftp.NewOSS(cfg.Ftp, rootDirectory)
	case Aliyun:
		return aliyun.NewOSS(cfg.Aliyun, rootDirectory)
	case Minio:
		return minio.NewOSS(cfg.Minio, rootDirectory)
	}
}
