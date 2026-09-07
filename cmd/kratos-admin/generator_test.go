package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestInitializeProjectRefreshesFrontendCLIs 验证前端生成使用官方源并刷新 dlx 执行缓存。
func TestInitializeProjectRefreshesFrontendCLIs(t *testing.T) {
	target := t.TempDir()
	var commands [][]string
	runner := func(commandTarget string, directory string, name string, args ...string) error {
		if name != "pnpm" {
			return nil
		}
		if commandTarget != target || directory != "." {
			t.Fatalf("前端命令工作目录错误: %s, %s", commandTarget, directory)
		}
		commands = append(commands, args)
		return nil
	}

	err := initializeProjectWithRunner(target, "orders", runner)
	if err != nil {
		t.Fatalf("初始化项目命令失败: %v", err)
	}
	frontends := []string{"admin", "uni-app", "taro-app"}
	if len(commands) != len(frontends) {
		t.Fatalf("前端命令数量错误: %d", len(commands))
	}
	for index, frontend := range frontends {
		expected := []string{
			"--config.@liujitcn:registry=https://registry.npmjs.org/",
			"--config.dlx-cache-max-age=0",
			"dlx",
			"@liujitcn/kratos-" + frontend + "-cli@latest",
			"create",
			filepath.Join(target, "frontend", frontend),
			"--module",
			"orders",
		}
		if !slices.Equal(commands[index], expected) {
			t.Errorf("%s 前端命令参数错误:\n实际: %v\n预期: %v", frontend, commands[index], expected)
		}
	}
}

// TestInitializeProjectDoesNotOverrideAdminAPI 验证项目初始化不会覆盖 Backend 声明的 API 版本。
func TestInitializeProjectDoesNotOverrideAdminAPI(t *testing.T) {
	var commands []string
	runner := func(_ string, directory string, name string, args ...string) error {
		commands = append(commands, strings.Join(append([]string{directory, name}, args...), " "))
		return nil
	}

	err := initializeProjectWithRunner("/tmp/test-project", "app", runner)
	if err != nil {
		t.Fatalf("初始化项目命令失败: %v", err)
	}
	backendGetCount := 0
	for _, command := range commands {
		if strings.Contains(command, "go get github.com/liujitcn/kratos-admin/backend@latest") {
			backendGetCount++
		}
		if strings.Contains(command, "go get github.com/liujitcn/kratos-admin/backend/api@") {
			t.Fatalf("不应强制覆盖 Admin API 版本: %s", command)
		}
	}
	if backendGetCount != 1 {
		t.Fatalf("Backend latest 安装命令数量错误: %d", backendGetCount)
	}
}
