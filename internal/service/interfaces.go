package service

import (
	"mime/multipart"
	"os"
	"path/filepath"
)

// FileStore 文件存储接口（替代 var saveMultipartFile hack）
type FileStore interface {
	Save(file *multipart.FileHeader, target string) error
	Remove(path string) error
}

// localFS 默认本地文件系统实现
type localFS struct{}

// 本地文件系统保存文件
func (l *localFS) Save(file *multipart.FileHeader, target string) error {
	//MkdirAll递归创建父目录(如果目录存在则什么都不做)   filepath.Dir(target): 取出 target 的父目录
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	src, err := file.Open() //Open() 返回一个 multipart.File（本质是个 io.ReadCloser），可以从里面读取上传的文件内容
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(target) //创建文件
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = dst.ReadFrom(src) //把源文件内容拷贝到目标文件
	return err
}

func (l *localFS) Remove(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IDGenerator 全局唯一 ID 生成器接口
type IDGenerator interface {
	NextID(prefix string) (uint64, error)
}
