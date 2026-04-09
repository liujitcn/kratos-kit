package logger

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestTrimmedPath 验证模块缓存路径会被压缩为旧版 NcapLog 风格。
func TestTrimmedPath(t *testing.T) {
	var filePath = "/Users/liujun/go/pkg/mod/git.newcapec.cn/03-!co-construction!and!sharing/!n!c!t/sharecomponent/!go/!admin!base.git@v0.0.1-20240513!a-no-notify/method/base_job_log.go:16"
	var except = "[AdminBase]method/base_job_log.go:16"
	var got = trimmedPath(filePath)
	if got != except {
		t.Fatalf("except=%s, got=%s", except, got)
	}
}

// TestTrimmedPathModule 验证本地项目路径会自动转成“项目名/相对路径”。
func TestTrimmedPathModule(t *testing.T) {
	moduleRoot := filepath.Join(t.TempDir(), "backend")
	err := os.MkdirAll(filepath.Join(moduleRoot, "service", "admin", "biz"), 0o755)
	if err != nil {
		t.Fatalf("mkdir module root: %v", err)
	}
	err = os.WriteFile(filepath.Join(moduleRoot, "go.mod"), []byte("module backend\n"), 0o644)
	if err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	moduleRootCache = sync.Map{}
	var filePath = filepath.ToSlash(filepath.Join(moduleRoot, "service", "admin", "biz", "base_job.go")) + ":122"
	var except = "backend/service/admin/biz/base_job.go:122"
	var got = trimmedPath(filePath)
	if got != except {
		t.Fatalf("except=%s, got=%s", except, got)
	}
}

// TestTrimmedPathNormal 验证找不到 go.mod 时回退为原有短路径规则。
func TestTrimmedPathNormal(t *testing.T) {
	moduleRootCache = sync.Map{}
	var filePath = "/Users/liujun/workspace/unknown/service/app/biz/recommend.go:88"
	var except = "recommend.go:88"
	var got = trimmedPath(filePath)
	if got != except {
		t.Fatalf("except=%s, got=%s", except, got)
	}
}

// TestFindCaller 验证会跳过当前日志包和 gorm 包，只返回业务调用位置。
func TestFindCaller(t *testing.T) {
	loggerSourceDir = "/Users/liujun/workspace/shop/kratos-kit/database/gorm/logger/"
	gormSourceDir = "/Users/liujun/go/pkg/mod/gorm.io/gorm@v1.31.1/"

	var frames = map[int]struct {
		file string
		line int
		ok   bool
	}{
		2: {file: "/Users/liujun/workspace/shop/kratos-kit/database/gorm/logger/logger.go", line: 89, ok: true},
		3: {file: "/Users/liujun/go/pkg/mod/gorm.io/gorm@v1.31.1/callbacks/update.go", line: 110, ok: true},
		4: {file: "/Users/liujun/workspace/shop/gorm-kit/repo/base_repo.go", line: 144, ok: true},
		5: {file: "/Users/liujun/workspace/shop/shop/backend/service/admin/task/base_job.go", line: 16, ok: true},
	}

	var got = findCaller(func(skip int) (uintptr, string, int, bool) {
		frame, ok := frames[skip]
		if !ok {
			return 0, "", 0, false
		}
		return 0, frame.file, frame.line, frame.ok
	})
	var except = "/Users/liujun/workspace/shop/shop/backend/service/admin/task/base_job.go:16"
	if got != except {
		t.Fatalf("except=%s, got=%s", except, got)
	}
}

// TestFindCallerDeepStack 验证较深调用栈仍会继续跳过公共仓储层，返回业务调用位置。
func TestFindCallerDeepStack(t *testing.T) {
	loggerSourceDir = "/Users/liujun/workspace/shop/kratos-kit/database/gorm/logger/"
	gormSourceDir = "/Users/liujun/go/pkg/mod/gorm.io/gorm@v1.31.1/"

	var frames = map[int]struct {
		file string
		line int
		ok   bool
	}{
		2:  {file: "/Users/liujun/workspace/shop/kratos-kit/database/gorm/logger/logger.go", line: 89, ok: true},
		3:  {file: "/Users/liujun/go/pkg/mod/gorm.io/gorm@v1.31.1/callbacks/update.go", line: 110, ok: true},
		4:  {file: "/Users/liujun/workspace/shop/gorm-kit/repo/base_repo.go", line: 144, ok: true},
		25: {file: "/Users/liujun/workspace/shop/shop/backend/service/admin/task/base_job.go", line: 16, ok: true},
	}

	var got = findCaller(func(skip int) (uintptr, string, int, bool) {
		frame, ok := frames[skip]
		if !ok {
			return 0, "", 0, false
		}
		return 0, frame.file, frame.line, frame.ok
	})
	var except = "/Users/liujun/workspace/shop/shop/backend/service/admin/task/base_job.go:16"
	if got != except {
		t.Fatalf("except=%s, got=%s", except, got)
	}
}

// TestShouldSkipCallerFile 验证 gorm 包、当前日志包和生成代码会被跳过。
func TestShouldSkipCallerFile(t *testing.T) {
	loggerSourceDir = "/Users/liujun/workspace/shop/kratos-kit/database/gorm/logger/"
	gormSourceDir = "/Users/liujun/go/pkg/mod/gorm.io/gorm@v1.31.1/"

	if !shouldSkipCallerFile("/Users/liujun/workspace/shop/kratos-kit/database/gorm/logger/logger.go") {
		t.Fatalf("expected logger package file to be skipped")
	}
	if !shouldSkipCallerFile("/Users/liujun/go/pkg/mod/gorm.io/gorm@v1.31.1/callbacks/update.go") {
		t.Fatalf("expected gorm package file to be skipped")
	}
	if !shouldSkipCallerFile("/Users/liujun/workspace/shop/gorm-kit/repo/base_repo.go") {
		t.Fatalf("expected generic repo file to be skipped")
	}
	if !shouldSkipCallerFile("/Users/liujun/workspace/shop/shop/backend/pkg/gen/query/order.gen.go") {
		t.Fatalf("expected generated file to be skipped")
	}
	if shouldSkipCallerFile("/Users/liujun/workspace/shop/shop/backend/service/admin/task/base_job.go") {
		t.Fatalf("expected business file not to be skipped")
	}
}

// TestShouldSkipCallerFrameByFunction 验证 trimpath 场景下可通过函数名识别公共仓储层。
func TestShouldSkipCallerFrameByFunction(t *testing.T) {
	if !shouldSkipCallerFrame("base_repo.go", "github.com/liujitcn/gorm-kit/repo.baseRepo[github.com/liujitcn/shop].UpdateById") {
		t.Fatalf("expected generic repo frame to be skipped by function name")
	}
	if shouldSkipCallerFrame("base_job.go", "github.com/liujitcn/shop/backend/service/admin/biz.(*BaseJobUsecase).Update") {
		t.Fatalf("expected business frame not to be skipped by function name")
	}
}
