package zap

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var moduleRootCache sync.Map

// normalizeCallerPath 统一 caller 文件路径格式，本地项目优先输出“项目名/相对路径”。
func normalizeCallerPath(file string) string {
	file = filepath.ToSlash(file)
	if file == "" {
		return file
	}
	if formattedPath, ok := formatModuleFile(file); ok {
		return formattedPath
	}
	return trimModuleCachePath(file)
}

// formatModuleFile 将本地项目绝对路径格式化为“项目名/相对路径”。
func formatModuleFile(file string) (string, bool) {
	if file == "" || file[0] != '/' {
		return file, file != ""
	}
	if strings.Contains(file, ".git@") {
		return "", false
	}

	var moduleRoot, found = findProjectRoot(filepath.Dir(file))
	if !found {
		return "", false
	}

	var relativePath, err = filepath.Rel(moduleRoot, file)
	if err != nil {
		return "", false
	}

	relativePath = filepath.ToSlash(relativePath)
	if strings.HasPrefix(relativePath, "../") {
		return "", false
	}

	var moduleName = moduleDisplayName(moduleRoot)
	if moduleName == "" {
		return "", false
	}

	return moduleName + "/" + relativePath, true
}

// trimModuleCachePath 压缩模块缓存路径，保留依赖模块名和相对路径。
func trimModuleCachePath(file string) string {
	if file == "" || file[0] != '/' {
		return file
	}

	var idx0 = strings.LastIndexByte(file, '/')
	if idx0 == -1 {
		return file
	}
	var idx1 = strings.LastIndexByte(file[:idx0], '/')
	if idx1 == -1 {
		return file
	}

	var idx2 = strings.LastIndex(file, ".git@")
	if idx2 == -1 {
		return file[idx0+1:]
	}
	var idx3 = strings.LastIndexByte(file[:idx2], '/')
	if idx3 == -1 {
		return file[idx0+1:]
	}

	var builder strings.Builder
	builder.Grow(idx2 - idx3)
	builder.WriteByte('[')

	var upperNext = false
	for i := idx3 + 1; i < idx2; i++ {
		c := file[i]
		if c == '!' {
			upperNext = true
			continue
		}
		if upperNext {
			if c >= 'a' && c <= 'z' {
				c = c + 'A' - 'a'
			}
			upperNext = false
		}
		builder.WriteByte(c)
	}

	builder.WriteByte(']')
	var prefix = builder.String()
	if idx3 == idx1 {
		return prefix + file[idx0+1:]
	}

	return prefix + file[idx1+1:]
}

// findProjectRoot 从目录向上查找当前仓库范围内最外层的 go.mod 所在目录。
func findProjectRoot(dir string) (string, bool) {
	dir = filepath.Clean(dir)
	if value, ok := moduleRootCache.Load(dir); ok {
		root, _ := value.(string)
		if root == "" {
			return "", false
		}
		return root, true
	}

	var visited []string
	var current = dir
	var foundRoot string
	for {
		visited = append(visited, current)
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			foundRoot = current
		}
		// 到达仓库根后停止继续向上，避免子模块误用自身 go.mod。
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			break
		}

		var parent = filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	if foundRoot != "" {
		for _, item := range visited {
			moduleRootCache.Store(item, foundRoot)
		}
		return foundRoot, true
	}

	for _, item := range visited {
		moduleRootCache.Store(item, "")
	}
	return "", false
}

// moduleDisplayName 返回模块显示名，本地项目优先目录名，依赖模块会去掉版本后缀。
func moduleDisplayName(moduleRoot string) string {
	var base = filepath.Base(moduleRoot)
	if idx := strings.IndexByte(base, '@'); idx >= 0 {
		base = base[:idx]
	}

	if isMajorVersionDir(base) {
		var parent = filepath.Base(filepath.Dir(moduleRoot))
		if idx := strings.IndexByte(parent, '@'); idx >= 0 {
			parent = parent[:idx]
		}
		if parent != "" && parent != "." && parent != string(filepath.Separator) {
			base = parent
		}
	}

	base = strings.TrimSuffix(base, ".git")
	return base
}

// isMajorVersionDir 判断目录名是否为 Go 模块常见的主版本目录，如 v2、v3。
func isMajorVersionDir(name string) bool {
	if len(name) < 2 || name[0] != 'v' {
		return false
	}
	for i := 1; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return true
}
