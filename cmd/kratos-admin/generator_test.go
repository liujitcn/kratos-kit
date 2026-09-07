package main

import (
	"strings"
	"testing"
)

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
