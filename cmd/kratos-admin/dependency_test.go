package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLatestBackendVersion 验证子模块 tag 按语义版本比较，排除预发布与其他模块版本。
func TestLatestBackendVersion(t *testing.T) {
	version, err := latestBackendVersion("hash refs/tags/backend/v0.0.9\nhash refs/tags/v9.0.0\nhash refs/tags/backend/v0.0.37\nhash refs/tags/backend/v0.0.38-rc.1\nhash refs/tags/backend/v2.0.0\nhash refs/tags/backend/v0.0.36\n")
	if err != nil || version != "v0.0.37" {
		t.Fatalf("未正确选择 Backend 稳定版本: %s, %v", version, err)
	}
	_, err = latestBackendVersion("hash refs/tags/v0.0.37\nhash refs/tags/backend/v1.0.0-beta.1")
	if err == nil {
		t.Fatal("没有稳定发布时不应继续生成")
	}
}

// TestBackendVersionCached 验证只有指定版本的完整缓存可跳过获取。
func TestBackendVersionCached(t *testing.T) {
	directory := t.TempDir()
	version := "v0.0.37"
	download := filepath.Join(directory, "cache/download", adminBackendModule, "@v")
	for _, file := range []string{
		filepath.Join(directory, adminBackendModule+"@"+version, "go.mod"),
		filepath.Join(download, version+".mod"),
		filepath.Join(download, version+".zip"),
		filepath.Join(download, version+".ziphash"),
	} {
		cached, err := backendVersionCached(directory, version)
		if err != nil || cached {
			t.Fatalf("不完整缓存被视为命中: %v, %v", cached, err)
		}
		err = os.MkdirAll(filepath.Dir(file), 0o755)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(file, []byte("cached"), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	cached, err := backendVersionCached(directory, version)
	if err != nil || !cached {
		t.Fatalf("未命中完整缓存: %v, %v", cached, err)
	}
	cached, err = backendVersionCached(directory, "v0.0.38")
	if err != nil || cached {
		t.Fatalf("其他版本缓存不应命中: %v, %v", cached, err)
	}
}

// TestInitializeDependencyCache 验证缓存命中、版本缺失与查询失败时的命令选择。
func TestInitializeDependencyCache(t *testing.T) {
	for _, cached := range []bool{true, false} {
		var commands []string
		runner := func(_ string, _ string, name string, args ...string) error {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return nil
		}
		resolver := testDependencyResolver()
		resolver.backend = func() (backendDependency, error) {
			return backendDependency{version: "v0.0.38", cached: cached}, nil
		}
		err := initializeProjectWithRunner(t.TempDir(), "test", runner, resolver)
		if err != nil {
			t.Fatal(err)
		}
		expected := "go get " + adminBackendModule + "@v0.0.38"
		if cached {
			expected = "go mod edit -require=" + adminBackendModule + "@v0.0.38"
		}
		if len(commands) < 4 || commands[3] != expected {
			t.Fatalf("缓存状态 %v 使用错误命令: %v", cached, commands)
		}
		resolver.backend = func() (backendDependency, error) {
			return backendDependency{}, errors.New("版本查询失败")
		}
		commands = nil
		err = initializeProjectWithRunner(t.TempDir(), "test", runner, resolver)
		if err == nil || len(commands) != 3 {
			t.Fatalf("版本未知时不应运行 Go 命令: %v, %v", commands, err)
		}
	}
}

// TestBackendQueryFallback 验证 Git 超时后代理可接替，双重失败时只使用完整稳定缓存。
func TestBackendQueryFallback(t *testing.T) {
	directory := t.TempDir()
	download := filepath.Join(directory, "cache/download", adminBackendModule, "@v")
	var err error
	for _, version := range []string{"v0.0.9", "v0.0.37", "v0.0.38-rc.1", "v2.0.0"} {
		for _, file := range []string{
			filepath.Join(directory, adminBackendModule+"@"+version, "go.mod"),
			filepath.Join(download, version+".mod"),
			filepath.Join(download, version+".zip"),
			filepath.Join(download, version+".ziphash"),
		} {
			err = os.MkdirAll(filepath.Dir(file), 0o755)
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(file, []byte("cached"), 0o644)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	err = os.WriteFile(filepath.Join(download, "v0.0.99.mod"), []byte("incomplete"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for _, proxyAvailable := range []bool{true, false} {
		query := func(name string, args ...string) ([]byte, error) {
			if name == "git" {
				return nil, context.DeadlineExceeded
			}
			if args[0] == "env" {
				return []byte(directory), nil
			}
			if proxyAvailable {
				return []byte(`{"Version":"v0.0.40"}`), nil
			}
			return nil, errors.New("代理不可用")
		}
		var dependency backendDependency
		dependency, err = resolveBackendDependencyWithQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		if proxyAvailable {
			if dependency.version != "v0.0.40" || dependency.cached {
				t.Fatalf("代理结果错误: %+v", dependency)
			}
		} else if dependency.version != "v0.0.37" || !dependency.cached {
			t.Fatalf("本地降级版本错误: %+v", dependency)
		}
	}
}

// TestBackendQueryFailureWithoutCache 验证远端不可用且本地无缓存时保留失败原因。
func TestBackendQueryFailureWithoutCache(t *testing.T) {
	directory := t.TempDir()
	query := func(name string, args ...string) ([]byte, error) {
		if name == "go" && args[0] == "env" {
			return []byte(directory), nil
		}
		return nil, context.DeadlineExceeded
	}
	_, err := resolveBackendDependencyWithQuery(query)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "没有可用的本地") {
		t.Fatalf("缺少明确的失败原因: %v", err)
	}
}
