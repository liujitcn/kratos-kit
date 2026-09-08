package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// parseBusinessModules 校验业务模块清单，未指定时仅生成 system 模块。
func parseBusinessModules(value string) ([]string, error) {
	if value == "" {
		return []string{"system"}, nil
	}
	modules := strings.Split(value, ",")
	pattern := regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	seen := make(map[string]bool)
	for _, name := range modules {
		if !pattern.MatchString(name) || name == "core" {
			return nil, fmt.Errorf("无效的业务模块名 %q：使用小写字母、数字或连字符，不能使用 core", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("业务模块名重复: %s", name)
		}
		seen[name] = true
	}
	return modules, nil
}

// frontendModuleAliases 为上游保留名 system 分配不与业务清单冲突的生成名称。
func frontendModuleAliases(modules []string) []string {
	aliases := slices.Clone(modules)
	for index, name := range modules {
		if name != "system" {
			continue
		}
		alias := "local-system"
		for slices.Contains(modules, alias) {
			alias += "-local"
		}
		aliases[index] = alias
	}
	return aliases
}

// normalizeFrontendModules 将上游临时模块名还原，并把本地 system 作为内置管理模块扩展。
func normalizeFrontendModules(target string, modules []string) error {
	index := slices.Index(modules, "system")
	if index < 0 {
		return nil
	}
	alias := frontendModuleAliases(modules)[index]
	var title strings.Builder
	for _, part := range strings.Split(alias, "-") {
		title.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	for _, cli := range frontendCLIs {
		workspace := filepath.Join(target, "frontend", cli.name)
		var paths []string
		err := filepath.WalkDir(workspace, func(file string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if strings.Contains(filepath.Base(file), alias) {
				paths = append(paths, file)
			}
			if entry.IsDir() {
				return nil
			}
			content, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			if !utf8.Valid(content) || !bytes.Contains(content, []byte(alias)) {
				return nil
			}
			var info fs.FileInfo
			info, err = entry.Info()
			if err != nil {
				return err
			}
			content = bytes.ReplaceAll(content, []byte(alias), []byte("system"))
			content = bytes.ReplaceAll(content, []byte(title.String()), []byte("System"))
			return os.WriteFile(file, content, info.Mode().Perm())
		})
		if err != nil {
			return err
		}
		// 先改深层路径，避免父目录改名使待处理的子路径失效。
		slices.SortFunc(paths, func(a, b string) int { return len(b) - len(a) })
		for _, file := range paths {
			err = os.Rename(file, filepath.Join(filepath.Dir(file), strings.ReplaceAll(filepath.Base(file), alias, "system")))
			if err != nil {
				return err
			}
		}
		if cli.name != "admin" {
			continue
		}
		moduleFile := filepath.Join(workspace, "packages/modules/system/src/module.ts")
		var content []byte
		content, err = os.ReadFile(moduleFile)
		if err != nil {
			return err
		}
		original := "  name: \"system\",\n  views: viewModules"
		if !strings.Contains(string(content), original) {
			return fmt.Errorf("上游 system 模块模板已变化: %s", moduleFile)
		}
		content = []byte("import { systemAdminModule as baseSystemAdminModule } from \"@liujitcn/kratos-admin-system\";\n" + strings.Replace(string(content), original, "  ...baseSystemAdminModule,\n  views: { ...baseSystemAdminModule.views, ...viewModules }", 1))
		err = os.WriteFile(moduleFile, content, 0o644)
		if err != nil {
			return err
		}
		manifestFile := filepath.Join(workspace, "apps/admin/src/module-manifest.ts")
		content, err = os.ReadFile(manifestFile)
		if err != nil {
			return err
		}
		// 本地 system 已包含上游视图，只保留一个运行时 system 注册入口。
		pattern := regexp.MustCompile(`(?s)  \{\s*packageName: "@liujitcn/kratos-admin-system",.*?\},\n`)
		if !pattern.Match(content) {
			return fmt.Errorf("上游管理端模块清单已变化: %s", manifestFile)
		}
		content = pattern.ReplaceAll(content, nil)
		content = bytes.Replace(content, []byte(`packageName: "@system/admin-module",`), []byte(`packageName: "@system/admin-module",
    optimizeDependencies: ["swagger-ui-dist/swagger-ui-bundle.js"],`), 1)
		err = os.WriteFile(manifestFile, content, 0o644)
		if err != nil {
			return err
		}
		var packageContent []byte
		packageContent, err = os.ReadFile(filepath.Join(workspace, "apps/admin/package.json"))
		if err != nil {
			return err
		}
		// 依赖版本与宿主保持一致，由同一 CLI 生成的 manifest 提供。
		var manifest struct {
			Dependencies map[string]string `json:"dependencies"`
		}
		err = json.Unmarshal(packageContent, &manifest)
		if err != nil {
			return err
		}
		var patch []byte
		patch, err = json.Marshal(map[string]any{"dependencies": map[string]string{"@liujitcn/kratos-admin-system": manifest.Dependencies["@liujitcn/kratos-admin-system"]}})
		if err != nil {
			return err
		}
		err = mergeFrontendPackage(filepath.Join(workspace, "packages/modules/system/package.json"), patch)
		if err != nil {
			return err
		}
	}
	return nil
}
