package zap

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var moduleRootCache sync.Map

type moduleInfo struct {
	root string
	path string
}

// normalizeCallerPath 统一 caller 文件路径格式，优先输出“go.mod module/相对路径”。
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

// formatModuleFile 将绝对路径格式化为“go.mod module/相对路径”。
func formatModuleFile(file string) (string, bool) {
	if file == "" || file[0] != '/' {
		return file, file != ""
	}
	if strings.Contains(file, ".git@") {
		return "", false
	}

	var module, found = findModuleRoot(filepath.Dir(file))
	if !found {
		return "", false
	}

	var relativePath, err = filepath.Rel(module.root, file)
	if err != nil {
		return "", false
	}

	relativePath = filepath.ToSlash(relativePath)
	if strings.HasPrefix(relativePath, "../") {
		return "", false
	}

	var moduleName = module.path
	if moduleName == "" {
		moduleName = moduleDisplayName(module.root)
	}
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

// findModuleRoot 从目录向上查找最近的 go.mod 所在目录与 module 名称。
func findModuleRoot(dir string) (moduleInfo, bool) {
	dir = filepath.Clean(dir)
	if value, ok := moduleRootCache.Load(dir); ok {
		module, _ := value.(moduleInfo)
		if module.root == "" {
			return moduleInfo{}, false
		}
		return module, true
	}

	var visited []string
	var current = dir
	var foundModule moduleInfo
	for {
		visited = append(visited, current)
		var goModPath = filepath.Join(current, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			foundModule = moduleInfo{
				root: current,
				path: readModulePath(goModPath),
			}
			break
		}

		var parent = filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	if foundModule.root != "" {
		for _, item := range visited {
			moduleRootCache.Store(item, foundModule)
		}
		return foundModule, true
	}

	for _, item := range visited {
		moduleRootCache.Store(item, moduleInfo{})
	}
	return moduleInfo{}, false
}

// readModulePath 从 go.mod 中读取 module 声明。
func readModulePath(goModPath string) string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return ""
		}
		return strings.Trim(fields[1], "`\"")
	}
	return ""
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
