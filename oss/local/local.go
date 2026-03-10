package local

import (
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/log"
)

type Local struct {
	RootDirectory string
	perm          os.FileMode
}

func NewOSS(rootDirectory string) *Local {
	return &Local{
		RootDirectory: rootDirectory,
		perm:          0777,
	}
}

func (o *Local) Upload(fileName string, filePath string, localFile string) (string, error) {
	_, err := os.Stat(o.RootDirectory)
	if err != nil {
		if !os.IsExist(err) {
			err = os.MkdirAll(o.RootDirectory, o.perm)
			if err != nil {
				return "", err
			}
		}
	}

	var file, dstFile *os.File
	defer func() {
		err = file.Close()
		if err != nil {
			return
		}
		err = dstFile.Close()
		if err != nil {
			return
		}
	}()

	//判断localFile是否存在
	file, err = os.Open(localFile)
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}

	// 创建savePath
	savePath := path.Join(o.RootDirectory, filePath)
	_, err = os.Stat(savePath)
	if err != nil {
		if !os.IsExist(err) {
			err = os.MkdirAll(savePath, o.perm)
			if err != nil {
				return "", err
			}
		}
	}

	dstName := filepath.Base(fileName)
	dstPath := path.Join(savePath, dstName)
	dstFile, err = os.Create(dstPath)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(dstFile, file)
	if err != nil {
		return "", err
	}
	res := path.Join(filePath, dstName)
	return res, nil
}

func (o *Local) UploadByByte(fileName string, filePath string, fileByte []byte) (string, error) {
	_, err := os.Stat(o.RootDirectory)
	if err != nil {
		if !os.IsExist(err) {
			err = os.MkdirAll(o.RootDirectory, o.perm)
			if err != nil {
				return "", err
			}
		}
	}

	var dstFile *os.File
	defer func() {
		err = dstFile.Close()
		if err != nil {
			return
		}
	}()

	// 创建savePath
	savePath := path.Join(o.RootDirectory, filePath)
	_, err = os.Stat(savePath)
	if err != nil {
		if !os.IsExist(err) {
			err = os.MkdirAll(savePath, o.perm)
			if err != nil {
				return "", err
			}
		}
	}

	dstName := filepath.Base(fileName)
	dstPath := path.Join(savePath, dstName)
	err = os.WriteFile(dstPath, fileByte, o.perm)
	if err != nil {
		return "", err
	}
	res := path.Join(filePath, dstName)
	return res, nil
}

func (o *Local) GetFileByte(filePath string) ([]byte, error) {
	filePath = path.Join(o.RootDirectory, filePath)
	return os.ReadFile(filePath)
}

func (o *Local) DeleteFile(filePath string) error {
	filePath = path.Join(o.RootDirectory, filePath)
	return os.Remove(filePath)
}
