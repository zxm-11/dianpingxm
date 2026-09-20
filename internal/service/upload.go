package service

import (
	"errors"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"hm-dianping/internal/config"
)

type UploadService struct {
	cfg   *config.Config
	store FileStore
}

func NewUploadService(cfg *config.Config) *UploadService {
	return &UploadService{cfg: cfg, store: &localFS{}}
}

// SetFileStore 允许测试时注入 mock
func (s *UploadService) SetFileStore(fs FileStore) { s.store = fs }

// SaveBlogImage 给探店博客插入图片
// D:\nginx_dp\nginx-1.18.0\html\hmdp\imgs\blogs\4\f\4f2a….jpg
// └──────────────── image_dir ────────────────┘└── relative ──┘
func (s *UploadService) SaveBlogImage(file *multipart.FileHeader) (string, error) {
	if file == nil || file.Filename == "" {
		return "", errors.New("文件不能为空")
	}
	ext := strings.ToLower(filepath.Ext(file.Filename)) //返回文件拓展名(.jpg/.go等)
	if ext == "" {
		return "", errors.New("文件后缀不能为空")
	}
	name := uuid.NewString() + ext
	dir1 := name[0:1]
	dir2 := name[1:2]
	relative := filepath.Join("blogs", dir1, dir2, name) //分片:不让所有图片堆在一个目录里→ blogs\4\f\4f2a8c1e-...-9b3d.jpg
	//浏览器读图(ToSlash:分隔符都转换为'/' 因为上面的join只会产生'\')
	publicPath := strings.ReplaceAll(filepath.ToSlash(filepath.Join(s.cfg.Upload.PublicPrefix, relative)), "//", "/")
	//文件系统
	fullPath := filepath.Join(s.cfg.Upload.ImageDir, relative)
	return publicPath, s.store.Save(file, fullPath)
}

func (s *UploadService) DeleteBlogImage(name string) error {
	relative := strings.TrimPrefix(name, s.cfg.Upload.PublicPrefix) //去掉公共前缀(url前缀)
	relative = strings.TrimLeft(relative, "\\/")                    //去掉开头的连续 \ 和 /，把路径强制变成相对路径
	clean := filepath.Clean(relative)                               //规范化路径(windows:/->\,linux->/)
	if clean == "." || filepath.IsAbs(clean) || strings.Contains(clean, "..") {
		return errors.New("非法文件路径")
	}
	return s.store.Remove(filepath.Join(s.cfg.Upload.ImageDir, clean))
}

/*
                     same file
                         │
          ┌──────────────┴──────────────┐
          │                             │
     Go 程序写盘                    浏览器读图
          │                             │
   image_dir + relative          public_prefix + relative
          │                             │
          ▼                             ▼
   D:\...\hmdp\imgs\blogs\4\f\x.jpg   /imgs/blogs/4/f/x.jpg
          │                             │
     文件系统                           nginx

*/
