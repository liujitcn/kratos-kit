package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

// TestGeneratedBackendBuilds 验证单模块项目通过公开 Admin 边界生成、测试和构建。
func TestGeneratedBackendBuilds(t *testing.T) {
	if os.Getenv("KRATOS_ADMIN_INTEGRATION") != "1" {
		t.Skip("设置 KRATOS_ADMIN_INTEGRATION=1 运行后端生成集成测试")
	}
	runner := func(target string, directory string, name string, args ...string) error {
		if name == "pnpm" {
			return nil
		}
		return runProjectCommandInDirectory(target, directory, name, args...)
	}
	initializer := func(target string, frontendModule string) error {
		// 未发布的跨仓库修复仅通过测试显式指定，不写入模板或正式初始化流程。
		for environment, modulePath := range map[string]string{
			"KRATOS_ADMIN_BACKEND_DIR": "github.com/liujitcn/kratos-admin/backend",
			"KRATOS_CORE_DIR":          "github.com/liujitcn/kratos-core",
			"KRATOS_KIT_REDACT_DIR":    "github.com/liujitcn/kratos-kit/redact",
			"KRATOS_KIT_GRPC_DIR":      "github.com/liujitcn/kratos-kit/server/grpc",
		} {
			directory := os.Getenv(environment)
			if directory == "" {
				continue
			}
			var err error
			directory, err = filepath.Abs(directory)
			if err != nil {
				return err
			}
			err = runProjectCommandInDirectory(filepath.Join(target, "backend"), ".", "go", "mod", "edit", "-replace="+modulePath+"="+directory)
			if err != nil {
				return err
			}
		}
		return initializeProjectWithRunner(target, frontendModule, runner, projectDependencyResolver{backend: resolveBackendDependency, frontend: testFrontendVersion})
	}
	for _, modulePath := range []string{"github.com/example/test/backend", "github.com/acme/test/backend/v2"} {
		t.Run(modulePath, func(t *testing.T) {
			target, err := createProjectWithOptions(projectOptions{projectName: "test", modulePath: modulePath}, t.TempDir(), initializer)
			if err != nil {
				t.Fatalf("生成独立后端失败: %v", err)
			}
			var generated *ast.File
			generated, err = parser.ParseFile(token.NewFileSet(), filepath.Join(target, "backend/internal/cmd/server/wire_gen.go"), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("解析宿主 Wire 产物失败: %v", err)
			}
			for _, spec := range generated.Imports {
				var importPath string
				importPath, err = strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatalf("解析 import 路径失败: %v", err)
				}
				if strings.HasPrefix(importPath, "github.com/liujitcn/kratos-admin/backend/internal/") {
					t.Fatalf("外部项目不应引用 Admin 内部包: %s", importPath)
				}
			}
			err = runProjectCommandInDirectory(filepath.Join(target, "backend"), ".", "make", "test")
			if err != nil {
				t.Fatalf("项目 Makefile 测试失败: %v", err)
			}
			err = runProjectCommandInDirectory(filepath.Join(target, "backend"), ".", "make", "build")
			if err != nil {
				t.Fatalf("项目 Makefile 构建失败: %v", err)
			}
		})
	}
}

// TestProjectNaming 验证项目名和仓库路径生成正确的模块、导入与业务目录。
func TestProjectNaming(t *testing.T) {
	for _, input := range []string{"test", "github.com/example/test", "github.com/acme/test"} {
		t.Run(input, func(t *testing.T) {
			var businessModule string
			target, err := createProjectWithOptions(projectOptions{projectName: input}, t.TempDir(), func(_ string, name string) error {
				businessModule = name
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Base(target) != "test" || businessModule != "system" {
				t.Fatalf("项目名或业务模块错误: %s, %s", target, businessModule)
			}
			expectedModule := "github.com/example/test/backend"
			if strings.Contains(input, "/") {
				expectedModule = input + "/backend"
			}
			for _, file := range []string{"backend/go.mod", "backend/bootstrap.go"} {
				var content []byte
				content, err = os.ReadFile(filepath.Join(target, file))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(content), expectedModule) {
					t.Errorf("%s 未使用预期 Go module: %s", file, expectedModule)
				}
			}
			for _, directory := range []string{"backend/api/proto/system/admin/v1", "backend/internal/biz/system/admin"} {
				_, err = os.Stat(filepath.Join(target, directory))
				if err != nil {
					t.Errorf("业务目录未使用默认 system 模块: %s: %v", directory, err)
				}
			}
			for _, directory := range []string{"adapter", "client", "backups", "codegen", "data", "logs"} {
				_, err = os.Stat(filepath.Join(target, "backend", directory))
				if !errors.Is(err, os.ErrNotExist) {
					t.Errorf("不应预创建目录 %s: %v", directory, err)
				}
			}
		})
	}
}

// TestGeneratedSingleModule 验证生成项目保留业务模块名且不引入额外宿主模块或工作区。
func TestGeneratedSingleModule(t *testing.T) {
	for _, modulePath := range []string{"github.com/example/orders/backend", "github.com/acme/orders/backend/v2"} {
		t.Run(modulePath, func(t *testing.T) {
			target, err := createProjectWithOptions(projectOptions{projectName: "orders", modulePath: modulePath}, t.TempDir(), func(string, string) error { return nil })
			if err != nil {
				t.Fatalf("渲染项目失败: %v", err)
			}
			var content []byte
			content, err = os.ReadFile(filepath.Join(target, "backend/go.mod"))
			if err != nil {
				t.Fatalf("读取业务模块失败: %v", err)
			}
			var projectModule *modfile.File
			projectModule, err = modfile.Parse("go.mod", content, nil)
			if err != nil {
				t.Fatalf("解析业务模块失败: %v", err)
			}
			if projectModule.Module.Mod.Path != modulePath {
				t.Fatalf("业务 module 路径错误: %s", projectModule.Module.Mod.Path)
			}
			if len(projectModule.Replace) != 0 {
				t.Fatalf("生成项目不应包含本地替换: %v", projectModule.Replace)
			}
			for _, relativePath := range []string{"backend/go.work", "backend/internal/cmd/server/go.mod"} {
				_, err = os.Stat(filepath.Join(target, relativePath))
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("生成项目不应包含 %s: %v", relativePath, err)
				}
			}
		})
	}
}

// TestInitializeProjectChecksSingleModule 验证单模块后端在 Wire 生成后执行完整测试。
func TestInitializeProjectChecksSingleModule(t *testing.T) {
	var commands []string
	runner := func(_ string, directory string, name string, args ...string) error {
		if name == "go" || name == "make" {
			commands = append(commands, strings.Join(append([]string{directory, name}, args...), " "))
		}
		return nil
	}
	err := initializeProjectWithRunner(t.TempDir(), "app", runner, testDependencyResolver())
	if err != nil {
		t.Fatalf("初始化项目失败: %v", err)
	}
	expected := []string{
		". go mod edit -require=github.com/liujitcn/kratos-admin/backend@v0.0.37",
		". go mod tidy",
		"internal/module go run github.com/google/wire/cmd/wire@v0.7.0 .",
		"internal/cmd/server go run github.com/google/wire/cmd/wire@v0.7.0 .",
		". go fmt ./...",
		". go test ./...",
	}
	if !slices.Equal(commands, expected) {
		t.Fatalf("后端初始化命令错误:\n实际: %v\n预期: %v", commands, expected)
	}
}

// TestInitializeProjectRefreshesFrontendCLIs 验证前端生成使用官方源与精确版本，不再强制刷新缓存。
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

	err := initializeProjectWithRunner(target, "orders", runner, testDependencyResolver())
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
			"dlx",
			"@liujitcn/kratos-" + frontend + "-cli@0.0.37",
			"create",
			filepath.Join(target, "frontend", frontend),
			"--kratos-project",
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

	err := initializeProjectWithRunner("/tmp/test-project", "app", runner, testDependencyResolver())
	if err != nil {
		t.Fatalf("初始化项目命令失败: %v", err)
	}
	backendGetCount := 0
	for _, command := range commands {
		if strings.Contains(command, "go mod edit -require=github.com/liujitcn/kratos-admin/backend@v0.0.37") {
			backendGetCount++
		}
		if strings.Contains(command, "go get github.com/liujitcn/kratos-admin/backend/api@") {
			t.Fatalf("不应强制覆盖 Admin API 版本: %s", command)
		}
	}
	if backendGetCount != 1 {
		t.Fatalf("Backend 缓存版本设置命令数量错误: %d", backendGetCount)
	}
}

// testDependencyResolver 为生成流程测试提供已缓存的固定版本。
func testDependencyResolver() projectDependencyResolver {
	return projectDependencyResolver{
		backend:  func() (backendDependency, error) { return backendDependency{version: "v0.0.37", cached: true}, nil },
		frontend: testFrontendVersion,
	}
}

// testFrontendVersion 为前端命令测试提供固定 npm 版本。
func testFrontendVersion(string) (string, error) { return "0.0.37", nil }
