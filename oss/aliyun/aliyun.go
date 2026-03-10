package aliyun

import (
	"bytes"
	"path"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

type Aliyun struct {
	Endpoint        string //OSS服务器的访问地址，这个地址一般分为好几种，最好理解的就是它可以分为内网和外网，我们在选择时候一般选择外网
	AccessKeyId     string //accessKeyId对应的值，一般是做访问权限用的
	AccessKeySecret string //加密的，不做解释，一般是考虑考虑安全问题
	BucketName      string //创建的bucket存储空间的名称
	RootDirectory   string // 存储空间根目录
}

func NewOSS(cfg *conf.OSS_Aliyun, rootDirectory string) *Aliyun {
	return &Aliyun{
		Endpoint:        cfg.Endpoint,
		AccessKeyId:     cfg.AccessKeyId,
		AccessKeySecret: cfg.AccessKeySecret,
		BucketName:      cfg.BucketName,
		RootDirectory:   rootDirectory,
	}
}

func (o *Aliyun) Upload(fileName string, filePath string, localFile string) (string, error) {
	bucket, err := o.getBucket()
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	// 创建savePath
	savePath := path.Join(o.RootDirectory, filePath)
	dstName := filepath.Base(fileName)
	dstPath := path.Join(savePath, dstName)
	// 设置分片大小为100 KB，指定分片上传并发数为3，并开启断点续传上传。
	// 其中<yourObjectName>与objectKey是同一概念，表示断点续传上传文件到OSS时需要指定包含文件后缀在内的完整路径，例如abc/efg/123.jpg。
	// "LocalFile"为filePath，100*1024为partSize。
	err = bucket.UploadFile(dstPath, localFile, 100*1024, oss.Routines(3), oss.Checkpoint(true, ""))
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	res := strings.ReplaceAll(dstPath, o.RootDirectory, "")
	return res, nil
}

func (o *Aliyun) UploadByByte(fileName string, filePath string, fileByte []byte) (string, error) {
	bucket, err := o.getBucket()
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	// 创建savePath
	savePath := path.Join(o.RootDirectory, filePath)
	dstName := filepath.Base(fileName)
	dstPath := path.Join(savePath, dstName)
	//上传byte数组
	err = bucket.PutObject(dstPath, bytes.NewReader(fileByte))
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	res := strings.ReplaceAll(dstPath, o.RootDirectory, "")
	return res, nil
}

func (o *Aliyun) GetFileByte(filePath string) ([]byte, error) {
	bucket, err := o.getBucket()
	if err != nil {
		log.Error("Error:", err)
		return nil, err
	}
	objectKey := path.Join(o.RootDirectory, filePath)
	req := oss.GetObjectRequest{ObjectKey: objectKey}
	var result *oss.GetObjectResult
	result, err = bucket.DoGetObject(&req, nil)
	if err != nil {
		log.Error("Error", err)
		return nil, err
	}
	defer func(Response *oss.Response) {
		err = Response.Close()
		if err != nil {
			return
		}
	}(result.Response)
	var res []byte
	_, err = result.Response.Read(res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (o *Aliyun) DeleteFile(filePath string) error {
	bucket, err := o.getBucket()
	if err != nil {
		log.Error("Error:", err)
		return err
	}
	objectKey := path.Join(o.RootDirectory, filePath)
	err = bucket.DeleteObject(objectKey)
	if err != nil {
		log.Error("Error", err)
		return err
	}
	return nil
}

func (o *Aliyun) getBucket() (*oss.Bucket, error) {
	//获取oss客户端连接
	client, err := oss.New(o.Endpoint, o.AccessKeyId, o.AccessKeySecret)
	if err != nil {
		log.Error("Error:", err)
		return nil, err
	}
	// 获取bucket
	var bucket *oss.Bucket
	bucket, err = client.Bucket(o.BucketName)
	if err != nil {
		return nil, err
	}

	return bucket, nil
}
