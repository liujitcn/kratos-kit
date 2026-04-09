package logger

import (
	"context"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	// DefaultFullCaller 返回绝对文件路径和行号，交由具体 logger 再做统一格式化。
	DefaultFullCaller = FullCaller(4)
)

// FullCaller 返回指定深度的绝对文件路径和行号。
func FullCaller(depth int) log.Valuer {
	return func(context.Context) any {
		_, file, line, _ := runtime.Caller(depth)
		return filepath.ToSlash(file) + ":" + strconv.Itoa(line)
	}
}
