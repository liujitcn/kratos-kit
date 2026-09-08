package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const adminBackendModule = "github.com/liujitcn/kratos-admin/backend"

// projectDependencyResolver 隔离远端版本查询，供生成流程与离线测试使用。
type projectDependencyResolver struct {
	backend  func() (backendDependency, error)
	frontend func(string) (string, error)
}

// backendDependency 表示选定的稳定版本及其本地缓存状态。
type backendDependency struct {
	version string
	cached  bool
}

// resolveBackendDependency 查询发布版本，远端不可用时明确提示并使用完整本地缓存。
func resolveBackendDependency() (backendDependency, error) {
	return resolveBackendDependencyWithQuery(queryDependencyCommand)
}

// resolveBackendDependencyWithQuery 隔离版本查询，在 Git、Go 代理与本地缓存之间降级。
func resolveBackendDependencyWithQuery(query func(string, ...string) ([]byte, error)) (backendDependency, error) {
	cacheDirectory, err := query("go", "env", "GOMODCACHE")
	if err != nil {
		return backendDependency{}, err
	}
	cache := strings.TrimSpace(string(cacheDirectory))
	projectProgress.Printf("检查远端 backend/v* tag（最多等待 20 秒）")
	var output []byte
	output, err = query("git", "ls-remote", "--tags", "--refs", "https://github.com/liujitcn/kratos-admin.git", "backend/v*")
	var version string
	if err == nil {
		version, err = latestBackendVersion(string(output))
	}
	if err != nil {
		failures := []error{err}
		projectProgress.Printf("Git 版本查询失败，尝试 Go 代理（最多等待 20 秒）：%v", err)
		output, err = query("go", "list", "-m", "-json", adminBackendModule+"@latest")
		if err == nil {
			var metadata struct{ Version string }
			err = json.Unmarshal(output, &metadata)
			if err == nil {
				version = metadata.Version
				if !isStableBackendVersion(version) {
					err = fmt.Errorf("Go 代理返回无效的 Backend 稳定版本: %q", version)
				}
			}
		}
		if err != nil {
			failures = append(failures, err)
			projectProgress.Printf("Go 代理版本查询失败，检查完整本地缓存：%v", err)
			version, err = latestCachedBackendVersion(cache)
			if err != nil {
				return backendDependency{}, fmt.Errorf("远端不可用且没有可用的本地 Backend 缓存: %w", errors.Join(append(failures, err)...))
			}
			projectProgress.Printf("无法确认远端最新版本，使用本地缓存 Backend %s 继续生成", version)
			return backendDependency{version: version, cached: true}, nil
		}
		projectProgress.Printf("Go 代理返回 Backend %s", version)
	}
	var cached bool
	cached, err = backendVersionCached(cache, version)
	return backendDependency{version: version, cached: cached}, err
}

// latestCachedBackendVersion 从完整本地缓存中选择当前模块可用的最高稳定版本。
func latestCachedBackendVersion(cacheDirectory string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(cacheDirectory, "cache/download", adminBackendModule, "@v"))
	if err != nil {
		return "", fmt.Errorf("读取 Backend 缓存: %w", err)
	}
	var latest string
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".mod") {
			continue
		}
		version := strings.TrimSuffix(entry.Name(), ".mod")
		if !isStableBackendVersion(version) || (latest != "" && semver.Compare(version, latest) <= 0) {
			continue
		}
		var cached bool
		cached, err = backendVersionCached(cacheDirectory, version)
		if err != nil {
			return "", err
		}
		if cached {
			latest = version
		}
	}
	if latest == "" {
		return "", fmt.Errorf("本地没有完整的 Backend 稳定版本缓存")
	}
	return latest, nil
}

// isStableBackendVersion 判断版本是否适用于无主版本后缀的 Backend 模块。
func isStableBackendVersion(version string) bool {
	return semver.IsValid(version) && semver.Canonical(version) == version && semver.Prerelease(version) == "" && (semver.Major(version) == "v0" || semver.Major(version) == "v1")
}

// resolveFrontendVersion 查询 npm latest 对应的精确版本，让 pnpm 按版本复用 CLI 缓存。
func resolveFrontendVersion(packageName string) (string, error) {
	projectProgress.Printf("检查 %s 的 npm latest 版本（最多等待 20 秒）", packageName)
	output, err := queryDependencyCommand("pnpm", "view", packageName, "dist-tags.latest", "--registry=https://registry.npmjs.org/", "--json")
	if err != nil {
		return "", err
	}
	var version string
	err = json.Unmarshal(output, &version)
	if err != nil {
		return "", fmt.Errorf("解析 %s 的 npm 版本失败: %w", packageName, err)
	}
	if !semver.IsValid("v"+version) || semver.Canonical("v"+version) != "v"+version {
		return "", fmt.Errorf("%s 返回无效的 npm 版本: %q", packageName, version)
	}
	return version, nil
}

// queryDependencyCommand 限时执行只读版本查询，避免版本比较本身无限等待。
func queryDependencyCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Env = append(os.Environ(), "GOWORK=off", "GIT_TERMINAL_PROMPT=0")
	command.WaitDelay = time.Second
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("查询 %s %s 超时: %w", name, strings.Join(args, " "), ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("查询 %s %s 失败: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

// latestBackendVersion 按语义版本选取当前无主版本后缀模块可用的最新稳定 tag。
func latestBackendVersion(tags string) (string, error) {
	var latest string
	for _, line := range strings.Split(tags, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !strings.HasPrefix(fields[1], "refs/tags/backend/") {
			continue
		}
		version := strings.TrimPrefix(fields[1], "refs/tags/backend/")
		// 排除预发布、非规范版本及需要 /v2 等模块后缀的发布。
		if !isStableBackendVersion(version) {
			continue
		}
		if latest == "" || semver.Compare(version, latest) > 0 {
			latest = version
		}
	}
	if latest == "" {
		return "", fmt.Errorf("远端没有可用的 Backend 稳定发布 tag")
	}
	return latest, nil
}

// backendVersionCached 检查指定版本的源码和下载缓存，避免把未完成下载视为缓存命中。
func backendVersionCached(cacheDirectory, version string) (bool, error) {
	download := filepath.Join(cacheDirectory, "cache", "download", adminBackendModule, "@v")
	for _, file := range []string{
		filepath.Join(cacheDirectory, adminBackendModule+"@"+version, "go.mod"),
		filepath.Join(download, version+".mod"),
		filepath.Join(download, version+".zip"),
		filepath.Join(download, version+".ziphash"),
	} {
		info, err := os.Stat(file)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("检查 Backend 模块缓存 %s: %w", file, err)
		}
		if !info.Mode().IsRegular() || info.Size() == 0 {
			return false, nil
		}
	}
	return true, nil
}
