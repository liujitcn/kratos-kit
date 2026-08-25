package logger

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var callerModuleRootCache sync.Map

type callerModuleInfo struct {
	root string
	path string
}

// FormatConsoleCaller 格式化控制台 caller，保留可点击的绝对源码路径。
func FormatConsoleCaller(caller string) string {
	var file string
	var line int
	var ok bool
	file, line, ok = splitCaller(ParseCaller(caller))
	if !ok {
		return ""
	}
	return file + ":" + strconv.Itoa(line)
}

// FormatFileCaller 格式化文件或远程日志 caller，输出“Go module/相对路径:行号”。
func FormatFileCaller(caller string) string {
	var file string
	var line int
	var ok bool
	file, line, ok = splitCaller(ParseCaller(caller))
	if !ok {
		return ""
	}
	return normalizeCallerFile(file) + ":" + strconv.Itoa(line)
}

// normalizeCallerFile 将 caller 文件路径压缩为 Go module 路径。
func normalizeCallerFile(file string) string {
	file = filepath.ToSlash(file)
	if file == "" {
		return file
	}
	var formattedPath string
	var ok bool
	formattedPath, ok = formatModuleFile(file)
	if ok {
		return formattedPath
	}
	return trimModuleCachePath(file)
}

// formatModuleFile 根据源码附近的 go.mod 生成 module 路径。
func formatModuleFile(file string) (string, bool) {
	if file == "" || file[0] != '/' {
		return file, file != ""
	}

	var module callerModuleInfo
	var found bool
	module, found = findCallerModuleRoot(filepath.Dir(file))
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
	if module.path == "" {
		return "", false
	}
	return strings.TrimSuffix(module.path, "/") + "/" + relativePath, true
}

// trimModuleCachePath 在源码目录不可访问时按 Go module cache 路径回退格式化。
func trimModuleCachePath(file string) string {
	var marker = "/pkg/mod/"
	var index = strings.Index(file, marker)
	if index < 0 {
		return file
	}

	var parts = strings.Split(file[index+len(marker):], "/")
	for i := range parts {
		parts[i] = decodeModuleCacheSegment(parts[i])
		var versionIndex = strings.IndexByte(parts[i], '@')
		if versionIndex >= 0 {
			parts[i] = parts[i][:versionIndex]
		}
	}
	return strings.Join(parts, "/")
}

// decodeModuleCacheSegment 还原 Go module cache 对大写字符的转义。
func decodeModuleCacheSegment(segment string) string {
	if !strings.Contains(segment, "!") {
		return segment
	}

	var builder strings.Builder
	builder.Grow(len(segment))
	var upperNext bool
	for i := 0; i < len(segment); i++ {
		var char = segment[i]
		if char == '!' {
			upperNext = true
			continue
		}
		if upperNext && char >= 'a' && char <= 'z' {
			char = char + 'A' - 'a'
		}
		upperNext = false
		builder.WriteByte(char)
	}
	return builder.String()
}

// findCallerModuleRoot 从源码目录向上查找最近的 go.mod。
func findCallerModuleRoot(dir string) (callerModuleInfo, bool) {
	dir = filepath.Clean(dir)
	var cached any
	var ok bool
	cached, ok = callerModuleRootCache.Load(dir)
	if ok {
		var module, valid = cached.(callerModuleInfo)
		if !valid || module.root == "" {
			return callerModuleInfo{}, false
		}
		return module, true
	}

	var visited []string
	var current = dir
	var foundModule callerModuleInfo
	for {
		visited = append(visited, current)
		var goModPath = filepath.Join(current, "go.mod")
		var statErr error
		_, statErr = os.Stat(goModPath)
		if statErr == nil {
			foundModule = callerModuleInfo{
				root: current,
				path: readCallerModulePath(goModPath),
			}
			break
		}

		var parent = filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	for _, visitedDir := range visited {
		callerModuleRootCache.Store(visitedDir, foundModule)
	}
	if foundModule.root == "" {
		return callerModuleInfo{}, false
	}
	return foundModule, true
}

// readCallerModulePath 从 go.mod 读取 module 声明。
func readCallerModulePath(goModPath string) string {
	var data, err = os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		var fields = strings.Fields(line)
		if len(fields) < 2 {
			return ""
		}
		return strings.Trim(fields[1], "`\"")
	}
	return ""
}
