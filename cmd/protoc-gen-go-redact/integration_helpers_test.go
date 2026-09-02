package main

import (
	"os/exec"
	"testing"
)

// skipWithoutProtoc 在本机未安装 protoc 时跳过需要外部编译器的测试。
func skipWithoutProtoc(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("未找到 protoc，跳过协议生成测试")
	}
}
