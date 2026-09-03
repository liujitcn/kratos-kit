package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateProjectWithOptions 验证完整项目模板、后端模板和前端参数的渲染结果。
func TestCreateProjectWithOptions(t *testing.T) {
	root := t.TempDir()
	var receivedFrontendModule string
	target, err := createProjectWithOptions(
		projectOptions{
			projectName:    "shop-admin",
			modulePath:     "github.com/acme/shop-admin/backend",
			frontendModule: "shop",
		},
		root,
		func(target, frontendModule string) error {
			receivedFrontendModule = frontendModule
			return os.MkdirAll(filepath.Join(target, "frontend"), 0o755)
		},
	)
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}
	if receivedFrontendModule != "shop" {
		t.Fatalf("前端 module = %q, want shop", receivedFrontendModule)
	}
	for _, file := range []string{
		"Makefile",
		"README.md",
		"scripts/backend.sh",
		"frontend/Makefile",
		"frontend/scripts/reinstall_dependencies.sh",
		"backend/bootstrap.go",
		"backend/scripts/docker-entrypoint.sh",
		"backend/internal/module/module.go",
		"backend/internal/cmd/server/wire.go",
	} {
		path := filepath.Join(target, file)
		if strings.HasSuffix(file, string(filepath.Separator)) {
			if _, err = os.Stat(path); err != nil {
				t.Fatalf("项目目录 %s 不存在: %v", file, err)
			}
			continue
		}
		if _, err = os.Stat(path); err != nil {
			t.Fatalf("项目文件 %s 不存在: %v", file, err)
		}
	}
	var content []byte
	content, err = os.ReadFile(filepath.Join(target, "backend", "go.mod"))
	if err != nil {
		t.Fatalf("读取后端 go.mod 失败: %v", err)
	}
	if !strings.Contains(string(content), "module github.com/acme/shop-admin/backend") {
		t.Fatalf("后端 go.mod 未渲染指定 module: %s", content)
	}
	content, err = os.ReadFile(filepath.Join(target, "backend", "bootstrap.go"))
	if err != nil {
		t.Fatalf("读取 Admin bootstrap.go 失败: %v", err)
	}
	if !strings.Contains(string(content), "adminbackend.ProviderSet") {
		t.Fatalf("默认项目未组合 Admin ProviderSet: %s", content)
	}
	content, err = os.ReadFile(filepath.Join(target, "backend", "configs", "data.yaml"))
	if err != nil {
		t.Fatalf("读取 Admin data.yaml 失败: %v", err)
	}
	if !strings.Contains(string(content), "driver: mysql") {
		t.Fatalf("默认项目未使用 Admin MySQL 配置: %s", content)
	}
	if _, err = os.Stat(filepath.Join(target, "docker-compose.yaml")); err != nil {
		t.Fatalf("默认项目未生成基础设施编排文件: %v", err)
	}
	info, err := os.Stat(filepath.Join(target, "scripts", "backend.sh"))
	if err != nil {
		t.Fatalf("读取后端脚本失败: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatal("后端脚本没有执行权限")
	}
}

// TestCreateProjectFailureCleansTarget 验证初始化失败时完整项目目录会被清理。
func TestCreateProjectFailureCleansTarget(t *testing.T) {
	root := t.TempDir()
	failure := errors.New("frontend cli failed")
	_, err := createProjectWithOptions(
		projectOptions{projectName: "broken-admin"},
		root,
		func(string, string) error { return failure },
	)
	if !errors.Is(err, failure) {
		t.Fatalf("错误 = %v, want %v", err, failure)
	}
	if _, err = os.Stat(filepath.Join(root, "broken-admin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("失败项目仍存在，stat error = %v", err)
	}
}
