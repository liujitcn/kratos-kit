package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// completeFrontendWorkspaces 为官方 CLI 的业务 workspace 补齐 Admin 共用检查和构建入口。
func completeFrontendWorkspaces(target, frontendModule string) error {
	tokens := map[string]string{
		"__PROJECT_NAME__":    filepath.Base(target),
		"__FRONTEND_MODULE__": frontendModule,
	}
	frontendTarget := filepath.Join(target, "frontend")
	err := renderTemplates(frontendTarget, "templates/frontend", tokens)
	if err != nil {
		return err
	}
	for _, cli := range frontendCLIs {
		workspace := filepath.Join(frontendTarget, cli.name)
		var content []byte
		content, err = projectTemplates.ReadFile("templates/metadata/" + cli.name + ".json")
		if err != nil {
			return err
		}
		err = mergeFrontendPackage(filepath.Join(workspace, "package.json"), []byte(replaceTokens(string(content), tokens)))
		if err != nil {
			return err
		}
		moduleTarget := filepath.Join(workspace, "packages", "modules", frontendModule)
		for _, directory := range []string{"src/api", "src/rpc", "src/locales", "test"} {
			directoryPath := filepath.Join(moduleTarget, directory)
			err = os.MkdirAll(directoryPath, 0o755)
			if err != nil {
				return err
			}
			var entries []os.DirEntry
			entries, err = os.ReadDir(directoryPath)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				err = os.WriteFile(filepath.Join(directoryPath, ".gitkeep"), nil, 0o644)
				if err != nil {
					return err
				}
			}
		}
		for _, locale := range []string{"zh-CN", "en-US", "zh-TW", "ja-JP"} {
			localePath := filepath.Join(moduleTarget, "src/locales", locale+".json")
			_, err = os.Stat(localePath)
			if err == nil {
				continue
			}
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			err = os.WriteFile(localePath, []byte("{}\n"), 0o644)
			if err != nil {
				return err
			}
		}
		if cli.name == "admin" {
			vitePath := filepath.Join(workspace, "apps/admin/vite.config.ts")
			content, err = os.ReadFile(vitePath)
			if err != nil {
				return err
			}
			const marker = "optimizeDependencies: adminModuleOptimizeDependencies"
			if !strings.Contains(string(content), marker) {
				return fmt.Errorf("管理端 CLI 的 Vite 模板已变化，无法设置 H5 输出目录: %s", vitePath)
			}
			content = []byte(strings.Replace(string(content), marker, marker+",\n  outputDirectory: \"../../../../backend/data/admin\"", 1))
			err = os.WriteFile(vitePath, content, 0o644)
			if err != nil {
				return err
			}
		} else {
			// 业务模块只打包自己的源码或构建入口，不发布来自 npm 的框架包。
			modulePatch := []byte(`{"scripts":{"build":"pnpm pack --pack-destination ../../../dist/npm"}}`)
			if cli.name == "taro-app" {
				modulePatch = []byte(`{"scripts":{"build":"pnpm build:entries && pnpm pack --pack-destination ../../../dist/npm"}}`)
			}
			err = mergeFrontendPackage(filepath.Join(moduleTarget, "package.json"), modulePatch)
			if err != nil {
				return err
			}
		}
		if cli.name == "taro-app" {
			packagePath := filepath.Join(workspace, "apps/taro-app/package.json")
			content, err = os.ReadFile(packagePath)
			if err != nil {
				return err
			}
			if !strings.Contains(string(content), "KRATOS_TARO_OUTPUT_ROOT=dist/build/h5") {
				return fmt.Errorf("Taro CLI 构建配置已变化，无法设置 H5 输出目录: %s", packagePath)
			}
			content = []byte(strings.Replace(string(content), "KRATOS_TARO_OUTPUT_ROOT=dist/build/h5", "KRATOS_TARO_OUTPUT_ROOT=../../../../backend/data/taro-app", 1))
			err = os.WriteFile(packagePath, content, 0o644)
			if err != nil {
				return err
			}
		}
	}
	return runProjectCommandInDirectory(target, ".", "python3", "scripts/sync_locales.py", "--write")
}

// mergeFrontendPackage 合并工具链字段，保留官方 CLI 生成的运行依赖和其他元数据。
func mergeFrontendPackage(packagePath string, patch []byte) error {
	content, err := os.ReadFile(packagePath)
	if err != nil {
		return fmt.Errorf("读取前端 package.json: %w", err)
	}
	var manifest map[string]json.RawMessage
	err = json.Unmarshal(content, &manifest)
	if err != nil {
		return err
	}
	var changes map[string]json.RawMessage
	err = json.Unmarshal(patch, &changes)
	if err != nil {
		return err
	}
	for field, value := range changes {
		if field == "scripts" || field == "devDependencies" {
			entries := make(map[string]json.RawMessage)
			if current := manifest[field]; current != nil {
				err = json.Unmarshal(current, &entries)
				if err != nil {
					return err
				}
			}
			var updates map[string]json.RawMessage
			err = json.Unmarshal(value, &updates)
			if err != nil {
				return err
			}
			for name, entry := range updates {
				entries[name] = entry
			}
			value, err = json.Marshal(entries)
			if err != nil {
				return err
			}
		}
		manifest[field] = value
	}
	content, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(packagePath, append(content, '\n'), 0o644)
}
