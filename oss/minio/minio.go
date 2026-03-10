package minio

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

type Minio struct {
	endpoint  string // 对端端口
	accessKey string // 访问密钥
	secretKey string // 密钥
	token     string // 令牌

	useSsl        bool   // 使用SSL
	bucketName    string // 创建的bucket存储空间的名称
	RootDirectory string // 存储空间根目录
}

func NewOSS(cfg *conf.OSS_MinIO, rootDirectory string) *Minio {
	return &Minio{
		endpoint:      cfg.Endpoint,
		accessKey:     cfg.AccessKey,
		secretKey:     cfg.SecretKey,
		token:         cfg.Token,
		useSsl:        cfg.UseSsl,
		bucketName:    cfg.BucketName,
		RootDirectory: rootDirectory,
	}
}

func (o *Minio) Upload(fileName string, filePath string, localFile string) (string, error) {
	client, err := o.getClient()
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}

	//判断localFile是否存在
	var file *os.File
	file, err = os.Open(localFile)
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	defer func() {
		err = file.Close()
		if err != nil {
			return
		}
	}()
	var fileStat fs.FileInfo
	fileStat, err = file.Stat()
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}

	// 创建savePath
	savePath := path.Join(o.RootDirectory, filePath)
	dstName := filepath.Base(fileName)
	dstPath := path.Join(savePath, dstName)

	_, err = client.PutObject(context.Background(), o.bucketName, dstPath, file, fileStat.Size(), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	res := strings.ReplaceAll(dstPath, o.RootDirectory, "")
	return res, nil
}

func (o *Minio) UploadByByte(fileName string, filePath string, fileByte []byte) (string, error) {
	client, err := o.getClient()
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	// 创建savePath
	savePath := path.Join(o.RootDirectory, filePath)
	dstName := filepath.Base(fileName)
	dstPath := path.Join(savePath, dstName)
	_, err = client.PutObject(context.Background(), o.bucketName, dstPath, bytes.NewReader(fileByte), -1, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	res := strings.ReplaceAll(dstPath, o.RootDirectory, "")
	return res, nil
}

func (o *Minio) GetFileByte(filePath string) ([]byte, error) {
	client, err := o.getClient()
	if err != nil {
		log.Error("Error:", err)
		return nil, err
	}
	objectKey := path.Join(o.RootDirectory, filePath)
	var object *minio.Object
	object, err = client.GetObject(context.Background(), o.bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		log.Error("Error:", err)
		return nil, err
	}
	defer func(object *minio.Object) {
		err = object.Close()
		if err != nil {
			log.Error("Error:", err)
			return
		}
	}(object)
	var res []byte
	res, err = io.ReadAll(object)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (o *Minio) DeleteFile(filePath string) error {
	client, err := o.getClient()
	if err != nil {
		log.Error("Error:", err)
		return err
	}
	objectKey := path.Join(o.RootDirectory, filePath)
	err = client.RemoveObject(context.Background(), o.bucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (o *Minio) getClient() (*minio.Client, error) {
	client, err := minio.New(o.endpoint,
		&minio.Options{
			Creds:  credentials.NewStaticV4(o.accessKey, o.secretKey, o.token),
			Secure: o.useSsl,
		},
	)
	if err != nil {
		log.Error("failed opening connection to *Minio", err)
		return nil, err
	}
	return client, nil
}
