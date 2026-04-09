package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

var gormSourceDir string
var loggerSourceDir string
var moduleRootCache sync.Map

var skipCallerFragments = []string{
	"/gorm-kit/repo/",
}

func init() {
	var pc = reflect.ValueOf(utils.FileWithLineNum).Pointer()
	var fn = runtime.FuncForPC(pc)
	if fn != nil {
		file, _ := fn.FileLine(pc)
		gormSourceDir = sourceDir(file)
	}

	_, file, _, ok := runtime.Caller(0)
	if ok {
		loggerSourceDir = filepath.ToSlash(filepath.Dir(file)) + "/"
	}
}

// Colors
const (
	Reset       = "\033[0m"
	Red         = "\033[31m"
	Green       = "\033[32m"
	Yellow      = "\033[33m"
	Blue        = "\033[34m"
	Magenta     = "\033[35m"
	Cyan        = "\033[36m"
	White       = "\033[37m"
	BlueBold    = "\033[34;1m"
	MagentaBold = "\033[35;1m"
	RedBold     = "\033[31;1m"
	YellowBold  = "\033[33;1m"
)

type gormLogger struct {
	logger.Config
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
}

// LogMode 设置 GORM 日志级别。
func (l *gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info 输出 GORM 普通信息日志。
func (l *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		writeLog(log.LevelInfo, fileWithLineNum(), l.infoStr+msg, data...)
	}
}

// Warn 输出 GORM 警告日志。
func (l *gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		writeLog(log.LevelWarn, fileWithLineNum(), l.warnStr+msg, data...)
	}
}

// Error 输出 GORM 错误日志。
func (l *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		writeLog(log.LevelError, fileWithLineNum(), l.errStr+msg, data...)
	}
}

// Trace 输出 SQL Trace 日志，并尽量保持与旧版 NcapLog 一致的样式。
func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel > logger.Silent {
		elapsed := time.Since(begin)
		switch {
		case err != nil && l.LogLevel >= logger.Error:
			var caller = fileWithLineNum()
			sql, rows := fc()
			if rows == -1 {
				writeLog(log.LevelError, caller, l.traceErrStr, err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			} else {
				writeLog(log.LevelError, caller, l.traceErrStr, err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			}
		case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
			var caller = fileWithLineNum()
			sql, rows := fc()
			slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
			if rows == -1 {
				writeLog(log.LevelWarn, caller, l.traceWarnStr, slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			} else {
				writeLog(log.LevelWarn, caller, l.traceWarnStr, slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			}
		case l.LogLevel == logger.Info:
			var caller = fileWithLineNum()
			sql, rows := fc()
			if rows == -1 {
				writeLog(log.LevelDebug, caller, l.traceStr, float64(elapsed.Nanoseconds())/1e6, "-", sql)
			} else {
				writeLog(log.LevelDebug, caller, l.traceStr, float64(elapsed.Nanoseconds())/1e6, rows, sql)
			}
		}
	}
}

// writeLog 将 caller 作为结构化字段传入，避免在消息体中重复打印同一调用位置。
func writeLog(level log.Level, caller string, format string, args ...interface{}) {
	log.Log(level,
		"msg", fmt.Sprintf(format, args...),
		"caller", caller,
	)
}

// sourceDir 统一生成目录前缀，兼容不同平台路径格式。
func sourceDir(file string) string {
	var dir = filepath.Dir(file)
	dir = filepath.Dir(dir)

	var parent = filepath.Dir(dir)
	if filepath.Base(parent) != "gorm.io" {
		parent = dir
	}

	return filepath.ToSlash(parent) + "/"
}

// fileWithLineNum 返回真正触发 SQL 的业务代码位置。
func fileWithLineNum() string {
	return findCaller(runtime.Caller)
}

// findCaller 逐层向上查找调用栈，跳过 gorm 内部和当前日志包自身。
func findCaller(caller func(skip int) (uintptr, string, int, bool)) string {
	// 仓储、生成代码和 GORM 回调链可能较深，这里适当放大扫描深度，避免过早停在公共封装层。
	for i := 2; i < 50; i++ {
		pc, file, line, ok := caller(i)
		if !ok {
			continue
		}

		file = filepath.ToSlash(file)
		if shouldSkipCallerFrame(file, callerFunctionName(pc)) {
			continue
		}

		return file + ":" + strconv.Itoa(line)
	}

	return ""
}

// shouldSkipCallerFrame 判断某个栈帧是否应该在 SQL caller 查找时跳过。
func shouldSkipCallerFrame(file string, function string) bool {
	if shouldSkipCallerFile(file) {
		return true
	}
	if function == "" {
		return false
	}

	var normalizedFunction = normalizeFunctionPath(function)
	for _, fragment := range skipCallerFragments {
		if matchesCallerFragment(normalizedFunction, fragment) {
			return true
		}
	}

	return false
}

// shouldSkipCallerFile 判断某个栈帧是否应该在 SQL caller 查找时跳过。
func shouldSkipCallerFile(file string) bool {
	if file == "" {
		return true
	}
	if strings.HasSuffix(file, ".gen.go") {
		return true
	}
	if strings.HasPrefix(file, loggerSourceDir) {
		return true
	}
	if strings.HasPrefix(file, gormSourceDir) && !strings.HasSuffix(file, "_test.go") {
		return true
	}
	for _, fragment := range skipCallerFragments {
		if matchesCallerFragment(file, fragment) {
			return true
		}
	}

	return false
}

// callerFunctionName 根据程序计数器获取函数名，供 trimpath 场景下辅助判断跳过规则。
func callerFunctionName(pc uintptr) string {
	if pc == 0 {
		return ""
	}

	var fn = runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}

	return fn.Name()
}

// matchesCallerFragment 同时兼容绝对路径和 trimpath 后的相对路径片段匹配。
func matchesCallerFragment(file string, fragment string) bool {
	if fragment == "" {
		return false
	}
	if strings.Contains(file, fragment) {
		return true
	}

	var trimmedFragment = strings.TrimPrefix(fragment, "/")
	if trimmedFragment == fragment {
		return false
	}

	return strings.Contains(file, trimmedFragment)
}

// normalizeFunctionPath 将函数名统一转换为便于片段匹配的路径形式。
func normalizeFunctionPath(function string) string {
	if function == "" {
		return function
	}

	function = strings.ReplaceAll(function, "\\", "/")
	function = strings.ReplaceAll(function, ".", "/")
	return function
}

// trimmedPath 压缩 GORM 调用点路径，保持与旧版 NcapLog 一致的显示风格。
func trimmedPath(filePath string) string {
	if formattedPath, ok := formatModulePath(filePath); ok {
		return formattedPath
	}

	idx0 := strings.LastIndexByte(filePath, '/')
	if idx0 == -1 {
		return filePath
	}
	idx1 := strings.LastIndexByte(filePath[:idx0], '/')
	if idx1 == -1 {
		return filePath
	}

	idx2 := strings.LastIndex(filePath, ".git@")
	if idx2 == -1 {
		return filePath[idx0+1:]
	}

	idx3 := strings.LastIndexByte(filePath[:idx2], '/')
	if idx3 == -1 {
		return filePath[idx0+1:]
	}

	var bd strings.Builder
	bd.Grow(idx2 - idx3)
	bd.WriteByte('[')

	var cg = false
	for i := idx3 + 1; i < idx2; i++ {
		c := filePath[i]
		if c == '!' {
			cg = true
			continue
		}
		if cg {
			if c >= 'a' && c <= 'z' {
				c = c + 'A' - 'a'
			}
			cg = false
		}
		bd.WriteByte(c)
	}

	bd.WriteByte(']')
	var prefix = bd.String()
	if idx3 == idx1 {
		return prefix + filePath[idx0+1:]
	}

	return prefix + filePath[idx1+1:]
}

// formatModulePath 将本地项目绝对路径格式化为“项目目录名/相对路径:行号”。
func formatModulePath(filePath string) (string, bool) {
	var pathWithoutLine, line, ok = splitFileAndLine(filePath)
	if !ok {
		return "", false
	}
	if pathWithoutLine == "" || pathWithoutLine[0] != '/' {
		return "", false
	}
	if strings.Contains(pathWithoutLine, "/pkg/mod/") || strings.Contains(pathWithoutLine, ".git@") {
		return "", false
	}

	var moduleRoot, found = findModuleRoot(filepath.Dir(pathWithoutLine))
	if !found {
		return "", false
	}

	var relativePath, err = filepath.Rel(moduleRoot, pathWithoutLine)
	if err != nil {
		return "", false
	}

	relativePath = filepath.ToSlash(relativePath)
	if strings.HasPrefix(relativePath, "../") {
		return "", false
	}

	return filepath.Base(moduleRoot) + "/" + relativePath + ":" + line, true
}

// splitFileAndLine 将 "文件路径:行号" 拆分为路径和行号。
func splitFileAndLine(filePath string) (string, string, bool) {
	var idx = strings.LastIndexByte(filePath, ':')
	if idx <= 0 || idx >= len(filePath)-1 {
		return "", "", false
	}

	return filePath[:idx], filePath[idx+1:], true
}

// findModuleRoot 从当前目录向上查找最近的 go.mod 所在目录。
func findModuleRoot(dir string) (string, bool) {
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
	for {
		visited = append(visited, current)
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			for _, item := range visited {
				moduleRootCache.Store(item, current)
			}
			return current, true
		}

		var parent = filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	for _, item := range visited {
		moduleRootCache.Store(item, "")
	}
	return "", false
}

type traceRecorder struct {
	logger.Interface
	BeginAt      time.Time
	SQL          string
	RowsAffected int64
	Err          error
}

// New 创建一条新的 GORM Trace 记录器。
func (l *traceRecorder) New() *traceRecorder {
	return &traceRecorder{Interface: l.Interface, BeginAt: time.Now()}
}

// Trace 记录 GORM Trace 明细。
func (l *traceRecorder) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.BeginAt = begin
	l.SQL, l.RowsAffected = fc()
	l.Err = err
}

// New 创建 GORM 日志器。
func New(config logger.Config) logger.Interface {
	var (
		infoStr      = "[info] " + "%s"
		warnStr      = "[warn] " + "%s"
		errStr       = "[error] " + "%s"
		traceStr     = "[%.3fms] [rows:%v] %s"
		traceWarnStr = "%s [%.3fms] [rows:%v] %s"
		traceErrStr  = "%s [%.3fms] [rows:%v] %s"
	)

	if config.Colorful {
		infoStr = Green + "[info] " + Reset + "%s"
		warnStr = Magenta + "[warn] " + Reset + "%s"
		errStr = Red + "[error] " + Reset + "%s"
		traceStr = Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
		traceWarnStr = Yellow + "%s " + Reset + RedBold + "[%.3fms] " + Yellow + "[rows:%v]" + Magenta + " %s" + Reset
		traceErrStr = MagentaBold + "%s " + Reset + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
	}

	return &gormLogger{
		Config:       config,
		infoStr:      infoStr,
		warnStr:      warnStr,
		errStr:       errStr,
		traceStr:     traceStr,
		traceWarnStr: traceWarnStr,
		traceErrStr:  traceErrStr,
	}
}
