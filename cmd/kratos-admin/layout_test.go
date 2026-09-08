package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestProjectResourceLayout 验证资源入口与各业务层的 Wire 初始化文件完整。
func TestProjectResourceLayout(t *testing.T) {
	target, err := createProjectWithOptions(projectOptions{projectName: "orders", frontendModule: "orders"}, t.TempDir(), func(string, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"backend/internal/const/app.go", "backend/internal/data/init.go",
		"backend/internal/module/wire.go", "backend/internal/module/resources.go",
		"backend/internal/i18n/i18n.go", "backend/internal/i18n/assets/zh-CN.json",
		"backend/internal/openapi/openapi.go", "backend/internal/openapi/assets/openapi.yaml",
		"backend/migration/migration.go", "backend/migration/assets/v0.0.1/mysql/README.md",
		"backend/internal/biz/init.go", "backend/internal/service/init.go", "backend/api/proto/.gitkeep",
		"backend/internal/server/init.go", "backend/internal/task/init.go", "backend/internal/config/init.go",
		"backend/internal/biz/orders/init.go", "backend/internal/biz/orders/admin/init.go", "backend/internal/biz/orders/app/init.go",
		"backend/internal/service/orders/init.go", "backend/internal/service/orders/admin/init.go", "backend/internal/service/orders/admin/v1/init.go",
		"backend/internal/service/orders/app/init.go", "backend/internal/service/orders/app/v1/init.go",
		"backend/internal/server/orders/init.go", "backend/internal/server/orders/admin/init.go", "backend/internal/server/orders/admin/v1/init.go",
		"backend/internal/server/orders/app/init.go", "backend/internal/server/orders/app/v1/init.go",
		"scripts/githooks/pre-commit", "scripts/generate_openapi_locales.py", "scripts/sync_locales.py",
	} {
		_, err = os.Stat(filepath.Join(target, path))
		if err != nil {
			t.Errorf("缺少项目入口 %s: %v", path, err)
		}
	}
	for _, path := range []string{"migration", "backend/internal/module/assets", "scripts/backend.sh"} {
		_, err = os.Stat(filepath.Join(target, path))
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("仍包含旧结构 %s: %v", path, err)
		}
	}
	var info os.FileInfo
	info, err = os.Stat(filepath.Join(target, "scripts/githooks/pre-commit"))
	if err != nil || info.Mode()&0o111 == 0 {
		t.Fatal("Git hook 没有可执行权限")
	}
}

// TestMakefileTargetsMatchAdmin 验证生成器三层 Make 目标与指定 Admin 基准一致。
func TestMakefileTargetsMatchAdmin(t *testing.T) {
	reference := os.Getenv("KRATOS_ADMIN_SOURCE_DIR")
	if reference == "" {
		t.Skip("设置 KRATOS_ADMIN_SOURCE_DIR 对照 Admin Make 目标")
	}
	targetPattern := regexp.MustCompile(`(?m)^([a-zA-Z][a-zA-Z0-9_-]*):`)
	for template, source := range map[string]string{
		"templates/project/Makefile":          "Makefile",
		"templates/backend/Makefile":          "backend/Makefile",
		"templates/project/frontend/Makefile": "frontend/Makefile",
	} {
		content, err := projectTemplates.ReadFile(template)
		if err != nil {
			t.Fatal(err)
		}
		var upstream []byte
		upstream, err = os.ReadFile(filepath.Join(reference, source))
		if err != nil {
			t.Fatal(err)
		}
		var actual, expected []string
		for _, match := range targetPattern.FindAllStringSubmatch(string(content), -1) {
			actual = append(actual, match[1])
		}
		for _, match := range targetPattern.FindAllStringSubmatch(string(upstream), -1) {
			expected = append(expected, match[1])
		}
		slices.Sort(actual)
		slices.Sort(expected)
		if !slices.Equal(actual, expected) {
			t.Errorf("%s 目标不同:\n实际: %v\n上游: %v", source, actual, expected)
		}
	}
}

// TestFrontendWorkspaceCompletion 验证补齐前端工具链时保留 CLI 依赖并统一 H5 输出。
func TestFrontendWorkspaceCompletion(t *testing.T) {
	target, err := createProjectWithOptions(projectOptions{projectName: "orders", frontendModule: "orders"}, t.TempDir(), func(string, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"admin", "uni-app", "taro-app"} {
		for path, content := range map[string]string{
			"package.json":                         `{"scripts":{"dev":"keep-existing-command"},"dependencies":{"@example/keep":"1.0.0"}}`,
			"packages/modules/orders/package.json": `{"name":"@local/orders","version":"0.0.1","private":true}`,
			"apps/" + name + "/package.json":       `{"scripts":{"build:h5":"cross-env KRATOS_TARO_OUTPUT_ROOT=dist/build/h5 build"}}`,
		} {
			output := filepath.Join(target, "frontend", name, path)
			err = os.MkdirAll(filepath.Dir(output), 0o755)
			if err != nil {
				t.Fatal(err)
			}
			err = os.WriteFile(output, []byte(content), 0o644)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	config := filepath.Join(target, "frontend/admin/apps/admin/vite.config.ts")
	err = os.WriteFile(config, []byte("export default { optimizeDependencies: adminModuleOptimizeDependencies };"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = completeFrontendWorkspaces(target, "orders")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"admin", "uni-app", "taro-app"} {
		var content []byte
		content, err = os.ReadFile(filepath.Join(target, "frontend", name, "package.json"))
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Scripts      map[string]string
			Dependencies map[string]string
		}
		err = json.Unmarshal(content, &manifest)
		if err != nil {
			t.Fatal(err)
		}
		if manifest.Scripts["test"] == "" || manifest.Scripts["dev"] != "keep-existing-command" || manifest.Dependencies["@example/keep"] != "1.0.0" {
			t.Errorf("%s 未正确合并 CLI 元数据: %s", name, content)
		}
		_, err = os.Stat(filepath.Join(target, "frontend", name, "packages/modules/orders/src/locales/generated.ts"))
		if err != nil {
			t.Errorf("%s 语言注册文件未通过脚本生成: %v", name, err)
		}
	}
	var content []byte
	content, err = os.ReadFile(config)
	if err != nil || !strings.Contains(string(content), "backend/data/admin") {
		t.Fatal("管理端输出路径未对齐")
	}
	content, err = os.ReadFile(filepath.Join(target, "frontend/taro-app/apps/taro-app/package.json"))
	if err != nil || !strings.Contains(string(content), "backend/data/taro-app") {
		t.Fatal("Taro 输出路径未对齐")
	}
}
