package local

import (
	"io"
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v3/log"
)

// Local 提供基于本地文件系统的对象存储实现。
type Local struct {
	// RootDirectory 是对象文件的根目录。
	RootDirectory string
	perm          os.FileMode
}

// NewOSS 创建本地文件系统对象存储。
func NewOSS(rootDirectory string) *Local {
	return &Local{
		RootDirectory: rootDirectory,
		perm:          0777,
	}
}

// Upload 将本地文件上传到对象存储目录。
func (o *Local) Upload(fileName string, filePath string, localFile string) (result string, err error) {
	if err = os.MkdirAll(o.RootDirectory, o.perm); err != nil {
		return "", err
	}

	var file *os.File
	file, err = os.Open(localFile)
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	savePath := filepath.Join(o.RootDirectory, filePath)
	if err = os.MkdirAll(savePath, o.perm); err != nil {
		return "", err
	}

	dstName := filepath.Base(fileName)
	dstPath := filepath.Join(savePath, dstName)
	var dstFile *os.File
	dstFile, err = os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if _, err = io.Copy(dstFile, file); err != nil {
		return "", err
	}
	return filepath.Join(filePath, dstName), nil
}

// UploadByByte 将字节数据写入对象存储目录。
func (o *Local) UploadByByte(fileName string, filePath string, fileByte []byte) (string, error) {
	var err error
	if err = os.MkdirAll(o.RootDirectory, o.perm); err != nil {
		return "", err
	}

	savePath := filepath.Join(o.RootDirectory, filePath)
	if err = os.MkdirAll(savePath, o.perm); err != nil {
		return "", err
	}

	dstName := filepath.Base(fileName)
	dstPath := filepath.Join(savePath, dstName)
	if err = os.WriteFile(dstPath, fileByte, o.perm); err != nil {
		return "", err
	}
	return filepath.Join(filePath, dstName), nil
}

// GetFileByte 读取本地对象文件。
func (o *Local) GetFileByte(filePath string) ([]byte, error) {
	filePath = filepath.Join(o.RootDirectory, filePath)
	return os.ReadFile(filePath)
}

// DeleteFile 删除本地对象文件。
func (o *Local) DeleteFile(filePath string) error {
	filePath = filepath.Join(o.RootDirectory, filePath)
	return os.Remove(filePath)
}
