package zap

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/go-kratos/kratos/v2/log"
)

type testWriteSyncer struct {
	output []string
}

func (x *testWriteSyncer) Write(p []byte) (n int, err error) {
	x.output = append(x.output, string(p))
	return len(p), nil
}

func (x *testWriteSyncer) Sync() error {
	return nil
}

// TestLogger 验证基础日志写入和非法 keyvals 处理。
func TestLogger(t *testing.T) {
	syncer := &testWriteSyncer{}
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), syncer, zap.DebugLevel)
	logger := NewZapLogger(core)

	defer func() { _ = logger.Close() }()

	zlog := log.NewHelper(logger)

	zlog.Debugw("log", "debug")
	zlog.Infow("log", "info")
	zlog.Warnw("log", "warn")
	zlog.Errorw("log", "error")
	zlog.Errorw("log", "error", "except warn")
	zlog.Info("hello world")

	except := []string{
		"{\"level\":\"debug\",\"msg\":\"\",\"log\":\"debug\"}\n",
		"{\"level\":\"info\",\"msg\":\"\",\"log\":\"info\"}\n",
		"{\"level\":\"warn\",\"msg\":\"\",\"log\":\"warn\"}\n",
		"{\"level\":\"error\",\"msg\":\"\",\"log\":\"error\"}\n",
		"{\"level\":\"warn\",\"msg\":\"Keyvalues must appear in pairs: [log error except warn]\"}\n",
		"{\"level\":\"info\",\"msg\":\"hello world\"}\n", // not {"level":"info","msg":"","msg":"hello world"}
	}
	for i, s := range except {
		if s != syncer.output[i] {
			t.Logf("except=%s, got=%s", s, syncer.output[i])
			t.Fail()
		}
	}
}

// TestLoggerCallerAndStandardFields 验证 caller 提升为主输出列且标准字段被过滤。
func TestLoggerCallerAndStandardFields(t *testing.T) {
	syncer := &testWriteSyncer{}
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		CallerKey:   "caller",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
		EncodeCaller: func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(caller.FullPath())
		},
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), syncer, zap.DebugLevel)
	logger := NewZapLogger(core)

	defer func() { _ = logger.Close() }()

	helper := log.NewHelper(logger)
	helper.Log(log.LevelInfo,
		"msg", "hello world",
		"caller", "biz/service.go:18",
		"service.id", "app",
		"trace_id", "",
		"span_id", "span-1",
		"extra", "value",
	)

	if len(syncer.output) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(syncer.output))
	}

	except := "{\"level\":\"info\",\"caller\":\"biz/service.go:18\",\"msg\":\"hello world\",\"span_id\":\"span-1\",\"extra\":\"value\"}\n"
	if syncer.output[0] != except {
		t.Fatalf("except=%s, got=%s", except, syncer.output[0])
	}
}

// TestLoggerFatalLevelMapping 验证 Kratos fatal 级别会正确映射到 zap fatal。
func TestLoggerFatalLevelMapping(t *testing.T) {
	syncer := &testWriteSyncer{}
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), syncer, zap.DebugLevel)
	logger := NewZapLogger(core)

	defer func() { _ = logger.Close() }()

	if err := logger.Log(log.LevelFatal, "msg", "fatal log"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(syncer.output) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(syncer.output))
	}

	except := "{\"level\":\"fatal\",\"msg\":\"fatal log\"}\n"
	if syncer.output[0] != except {
		t.Fatalf("except=%s, got=%s", except, syncer.output[0])
	}
}

// TestLoggerInferCallerFromMessage 验证 SQL 风格消息前缀会覆盖默认 caller。
func TestLoggerInferCallerFromMessage(t *testing.T) {
	syncer := &testWriteSyncer{}
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		CallerKey:   "caller",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
		EncodeCaller: func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(caller.FullPath())
		},
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), syncer, zap.DebugLevel)
	logger := NewZapLogger(core)

	defer func() { _ = logger.Close() }()

	helper := log.NewHelper(logger)
	helper.Log(log.LevelDebug,
		"msg", "[AdminBase]method/base_job_log.go:16 [50.161ms] [rows:1] INSERT INTO test",
		"caller", "gorm/logger/logger.go:88",
	)

	if len(syncer.output) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(syncer.output))
	}

	except := "{\"level\":\"debug\",\"caller\":\"[AdminBase]method/base_job_log.go:16\",\"msg\":\"[AdminBase]method/base_job_log.go:16 [50.161ms] [rows:1] INSERT INTO test\"}\n"
	if syncer.output[0] != except {
		t.Fatalf("except=%s, got=%s", except, syncer.output[0])
	}
}

// TestLoggerInferCallerFromColoredMessage 验证带 ANSI 颜色的 SQL 消息也能提取真实 caller。
func TestLoggerInferCallerFromColoredMessage(t *testing.T) {
	syncer := &testWriteSyncer{}
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		CallerKey:   "caller",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
		EncodeCaller: func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(caller.FullPath())
		},
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), syncer, zap.DebugLevel)
	logger := NewZapLogger(core)

	defer func() { _ = logger.Close() }()

	helper := log.NewHelper(logger)
	helper.Log(log.LevelDebug,
		"msg", "\u001b[32m[AdminBase]method/base_job_log.go:16 \u001b[0m\u001b[33m[13.475ms] \u001b[34;1m[rows:1]\u001b[0m INSERT INTO test",
		"caller", "gorm/logger/logger.go:88",
	)

	if len(syncer.output) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(syncer.output))
	}

	except := "{\"level\":\"debug\",\"caller\":\"[AdminBase]method/base_job_log.go:16\",\"msg\":\"\\u001b[32m[AdminBase]method/base_job_log.go:16 \\u001b[0m\\u001b[33m[13.475ms] \\u001b[34;1m[rows:1]\\u001b[0m INSERT INTO test\"}\n"
	if syncer.output[0] != except {
		t.Fatalf("except=%s, got=%s", except, syncer.output[0])
	}
}

// TestNormalizeCallerPathModule 验证本地项目 caller 会统一输出为“项目名/相对路径”。
func TestNormalizeCallerPathModule(t *testing.T) {
	moduleRootCache = sync.Map{}

	var moduleRoot = filepath.Join(t.TempDir(), "backend")
	var err = os.MkdirAll(filepath.Join(moduleRoot, "service", "admin", "biz"), 0o755)
	if err != nil {
		t.Fatalf("mkdir module root: %v", err)
	}
	err = os.WriteFile(filepath.Join(moduleRoot, "go.mod"), []byte("module backend\n"), 0o644)
	if err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	var got = normalizeCallerPath(filepath.Join(moduleRoot, "service", "admin", "biz", "base_job.go"))
	var except = "backend/service/admin/biz/base_job.go"
	if got != except {
		t.Fatalf("except=%s, got=%s", except, got)
	}
}

// TestNormalizeCallerPathNestedModule 验证嵌套模块会优先使用仓库根 go.mod，而不是当前子模块 go.mod。
func TestNormalizeCallerPathNestedModule(t *testing.T) {
	moduleRootCache = sync.Map{}

	var repoRoot = filepath.Join(t.TempDir(), "kratos-kit")
	var err = os.MkdirAll(filepath.Join(repoRoot, "database", "gorm"), 0o755)
	if err != nil {
		t.Fatalf("mkdir repo root: %v", err)
	}
	err = os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755)
	if err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	err = os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module github.com/liujitcn/kratos-kit\n"), 0o644)
	if err != nil {
		t.Fatalf("write root go.mod: %v", err)
	}
	err = os.WriteFile(filepath.Join(repoRoot, "database", "gorm", "go.mod"), []byte("module github.com/liujitcn/kratos-kit/database/gorm\n"), 0o644)
	if err != nil {
		t.Fatalf("write nested go.mod: %v", err)
	}

	var got = normalizeCallerPath(filepath.Join(repoRoot, "database", "gorm", "fill_callback.go"))
	var except = "kratos-kit/database/gorm/fill_callback.go"
	if got != except {
		t.Fatalf("except=%s, got=%s", except, got)
	}
}

// TestNormalizeCallerPathPkgMod 验证普通 pkg/mod 模块会输出模块名加相对路径。
func TestNormalizeCallerPathPkgMod(t *testing.T) {
	moduleRootCache = sync.Map{}

	var tempDir = t.TempDir()
	var moduleRoot = filepath.Join(tempDir, "pkg", "mod", "github.com", "go-kratos", "kratos", "v2@v2.9.2")
	var err = os.MkdirAll(filepath.Join(moduleRoot, "transport", "grpc"), 0o755)
	if err != nil {
		t.Fatalf("mkdir module root: %v", err)
	}
	err = os.WriteFile(filepath.Join(moduleRoot, "go.mod"), []byte("module github.com/go-kratos/kratos/v2\n"), 0o644)
	if err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	var got = normalizeCallerPath(filepath.Join(moduleRoot, "transport", "grpc", "server.go"))
	var except = "kratos/transport/grpc/server.go"
	if got != except {
		t.Fatalf("except=%s, got=%s", except, got)
	}
}

// TestNormalizeCallerPathModuleCache 验证模块缓存 caller 仍保留模块名裁剪格式。
func TestNormalizeCallerPathModuleCache(t *testing.T) {
	moduleRootCache = sync.Map{}

	var file = "/Users/liujun/go/pkg/mod/git.newcapec.cn/03-!co-construction!and!sharing/!n!c!t/sharecomponent/!go/!admin!base.git@v0.0.1-20240513!a-no-notify/method/base_job_log.go"
	var except = "[AdminBase]method/base_job_log.go"
	var got = normalizeCallerPath(file)
	if got != except {
		t.Fatalf("except=%s, got=%s", except, got)
	}
}

// TestFormatConsoleCallerPath 验证控制台 caller 会保留绝对路径，便于点击跳转源码。
func TestFormatConsoleCallerPath(t *testing.T) {
	var file = "/Users/liujun/workspace/shop/shop/backend/service/admin/biz/base_job.go"
	var got = formatConsoleCallerPath(file)
	if got != file {
		t.Fatalf("except=%s, got=%s", file, got)
	}
}

// TestParseCallerKeepAbsolutePath 验证解析 caller 字段时不会提前裁短绝对路径。
func TestParseCallerKeepAbsolutePath(t *testing.T) {
	var caller = parseCaller("/Users/liujun/workspace/shop/shop/backend/service/admin/biz/base_job.go:229")
	if !caller.Defined {
		t.Fatalf("expected caller defined")
	}
	if caller.File != "/Users/liujun/workspace/shop/shop/backend/service/admin/biz/base_job.go" {
		t.Fatalf("unexpected caller file: %s", caller.File)
	}
	if caller.Line != 229 {
		t.Fatalf("unexpected caller line: %d", caller.Line)
	}
}
