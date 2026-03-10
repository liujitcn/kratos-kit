package ftp

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jlaffaye/ftp"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

type Ftp struct {
	Endpoint      string //ftp服务地址
	UserName      string //登录ftp账号
	UserPwd       string //登录ftp密码
	RootDirectory string //ftp文件存放路径
}

func NewOSS(cfg *conf.OSS_Ftp, rootDirectory string) *Ftp {
	return &Ftp{
		Endpoint:      cfg.Endpoint,
		UserName:      cfg.UserName,
		UserPwd:       cfg.UserPwd,
		RootDirectory: rootDirectory,
	}
}

func (o *Ftp) Upload(fileName string, filePath string, localFile string) (string, error) {
	//获取ftp客户端连接
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
		err = client.Quit()
		if err != nil {
			return
		}
	}()

	// 创建savePath
	savePath := path.Join(o.RootDirectory, filePath)
	err = client.ChangeDir(savePath)
	if err != nil {
		err = client.MakeDir(savePath)
		if err != nil {
			// 由于搭建ftp的时候已经给了`pwd` 777的权限，这里忽略文件夹创建的错误
			if !strings.Contains(err.Error(), "550-Create directory operation failed") {
				return "", err
			}
		}
	}
	dstName := filepath.Base(fileName)
	dstPath := path.Join(savePath, dstName)
	//文件上传
	err = client.Stor(dstPath, file)
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	res := strings.ReplaceAll(dstPath, o.RootDirectory, "")
	return res, nil
}

func (o *Ftp) UploadByByte(fileName string, filePath string, fileByte []byte) (string, error) {
	//获取ftp客户端连接
	client, err := o.getClient()
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	//上传完毕后关闭当前的ftp连接
	defer func() {
		err = client.Quit()
		if err != nil {
			return
		}
	}()

	// 创建savePath
	savePath := path.Join(o.RootDirectory, filePath)
	err = client.ChangeDir(savePath)
	if err != nil {
		err = client.MakeDir(savePath)
		if err != nil {
			// 由于搭建ftp的时候已经给了`pwd` 777的权限，这里忽略文件夹创建的错误
			if !strings.Contains(err.Error(), "550-Create directory operation failed") {
				return "", err
			}
		}
	}
	dstName := filepath.Base(fileName)
	dstPath := path.Join(savePath, dstName)
	//文件上传
	err = client.Stor(dstPath, bytes.NewReader(fileByte))
	if err != nil {
		log.Error("Error:", err)
		return "", err
	}
	res := strings.ReplaceAll(dstPath, o.RootDirectory, "")
	return res, nil

}

func (o *Ftp) GetFileByte(filePath string) ([]byte, error) {
	//获取ftp客户端连接
	client, err := o.getClient()
	if err != nil {
		log.Error("Error:", err)
		return nil, err
	}
	var ret *ftp.Response

	//上传完毕后关闭当前的ftp连接
	defer func() {
		if ret != nil {
			err = ret.Close()
			if err != nil {
				return
			}
		}

		err = client.Quit()
		if err != nil {
			return
		}
	}()

	filePath = path.Join(o.RootDirectory, filePath)
	ret, err = client.Retr(filePath)
	if err != nil {
		return nil, err
	}
	var res []byte
	_, err = ret.Read(res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (o *Ftp) DeleteFile(filePath string) error {
	//获取ftp客户端连接
	client, err := o.getClient()
	if err != nil {
		log.Error("Error:", err)
		return err
	}
	filePath = path.Join(o.RootDirectory, filePath)
	return client.Delete(filePath)
}

func (o *Ftp) getClient() (*ftp.ServerConn, error) {
	//获取ftp客户端连接
	client, err := ftp.Dial(o.Endpoint)
	if err != nil {
		log.Error("Error:", err)
		return nil, err
	}
	//ftp登录
	err = client.Login(o.UserName, o.UserPwd)
	if err != nil {
		log.Error("Error:", err)
		return nil, err
	}

	return client, nil
}
