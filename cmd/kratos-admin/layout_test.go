package main

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
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
		"templates/project/Makefile": "Makefile",
		"templates/backend/Makefile": "backend/Makefile",
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

// TestFrontendOwnedByCLI 验证 Go 骨架不生成任何前端文件。
func TestFrontendOwnedByCLI(t *testing.T) {
	target, err := createProjectWithOptions(projectOptions{projectName: "orders"}, t.TempDir(), func(string, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var entries []os.DirEntry
	entries, err = os.ReadDir(filepath.Join(target, "frontend"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("Go 不应生成前端文件: %v", entries)
	}
}

// TestMultipleBusinessModules 验证多模块目录、Wire 汇总、RPC 及发布清单完整生成。
func TestMultipleBusinessModules(t *testing.T) {
	var selected string
	target, err := createProjectWithOptions(projectOptions{projectName: "github.com/example/shop", frontendModule: "system,order"}, t.TempDir(), func(_ string, modules string) error { selected = modules; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if selected != "system,order" {
		t.Fatalf("初始化模块清单错误: %s", selected)
	}
	for _, module := range []string{"system", "order"} {
		for _, file := range []string{
			"backend/internal/biz/" + module + "/init.go",
			"backend/internal/service/" + module + "/admin/v1/init.go",
			"backend/internal/server/" + module + "/app/v1/init.go",
			"backend/api/buf.admin." + module + ".typescript.gen.yaml",
			"backend/api/buf.uni-app." + module + ".typescript.gen.yaml",
			"backend/api/buf.taro-app." + module + ".typescript.gen.yaml",
		} {
			_, err = os.Stat(filepath.Join(target, file))
			if err != nil {
				t.Errorf("缺少模块入口 %s: %v", file, err)
			}
		}
	}
	for _, layer := range []string{"biz", "service", "server"} {
		var content []byte
		content, err = os.ReadFile(filepath.Join(target, "backend/internal", layer, "init.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, module := range []string{"system", "order"} {
			if !strings.Contains(string(content), "github.com/example/shop/backend/internal/"+layer+"/"+module) {
				t.Errorf("Wire 未汇总 %s/%s", layer, module)
			}
		}
		_, err = parser.ParseFile(token.NewFileSet(), "init.go", content, parser.AllErrors)
		if err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("python3", "-c", `import runpy; data=runpy.run_path('scripts/tag_release.py'); files=data['PACKAGE_FILES']; assert len(files)==6; assert all(any('/'+m+'/' in str(p) for p in files) for m in ['system','order'])`)
	command.Dir = target
	var output []byte
	output, err = command.CombinedOutput()
	if err != nil {
		t.Fatalf("发布清单错误: %v\n%s", err, output)
	}
}

// TestBusinessModuleArguments 验证默认、多个模块、重复参数与非法模块名。
func TestBusinessModuleArguments(t *testing.T) {
	for _, input := range []string{"system,system", "system,", "../system", "core", "System"} {
		_, err := parseBusinessModules(input)
		if err == nil {
			t.Errorf("应拒绝非法模块清单: %q", input)
		}
	}
	var output bytes.Buffer
	err := run([]string{"create", "demo", "--modules", "system", "--frontend-module", "system"}, &output)
	if err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("重复参数未合并校验: %v", err)
	}

}
