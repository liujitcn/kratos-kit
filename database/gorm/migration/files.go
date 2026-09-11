package migration

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
)

// FileReference 保存迁移文件相对路径及执行时内容的 SHA-256。
type FileReference struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// NewFileReference 根据文件内容生成可验证的迁移文件引用。
func NewFileReference(name string, content []byte) FileReference {
	return FileReference{Path: name, SHA256: fmt.Sprintf("%x", sha256.Sum256(content))}
}

// EncodeFileReferences 读取文件并将路径和校验值编码为数据库记录。
func EncodeFileReferences(files fs.FS, names []string) (string, error) {
	references := make([]FileReference, 0, len(names))
	var err error
	for _, name := range names {
		var content []byte
		content, err = fs.ReadFile(files, name)
		if err != nil {
			return "", fmt.Errorf("读取迁移文件 %s: %w", name, err)
		}
		references = append(references, NewFileReference(name, content))
	}
	var encoded []byte
	encoded, err = json.Marshal(references)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
