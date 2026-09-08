package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"golang.org/x/mod/module"
)

const (
	projectTemplateRoot = "templates/project"
	backendTemplateRoot = "templates/backend"
)

var projectDirectories = []string{
	"backend/api/proto",
	"backend/api/gen/go",
	"backend/internal/data/gen",
	"backend/internal/service",
	"backend/internal/server",
	"backend/internal/task",
	"backend/internal/biz",
	"backend/internal/config",
	"backend/api/proto/__FRONTEND_MODULE__/admin/v1",
	"backend/api/proto/__FRONTEND_MODULE__/app/v1",
	"backend/api/proto/__FRONTEND_MODULE__/config/v1",
	"backend/internal/biz/__FRONTEND_MODULE__/admin",
	"backend/internal/biz/__FRONTEND_MODULE__/app",
	"backend/internal/service/__FRONTEND_MODULE__/admin/v1",
	"backend/internal/service/__FRONTEND_MODULE__/app/v1",
	"backend/internal/server/__FRONTEND_MODULE__/admin/v1",
	"backend/internal/server/__FRONTEND_MODULE__/app/v1",
}

var frontendCLIs = []frontendCLI{
	{name: "admin", packageName: "@liujitcn/kratos-admin-cli"},
	{name: "uni-app", packageName: "@liujitcn/kratos-uni-app-cli"},
	{name: "taro-app", packageName: "@liujitcn/kratos-taro-app-cli"},
}

//go:embed all:templates
var projectTemplates embed.FS

type projectInitializer func(string, string) error

type projectCommandRunner func(string, string, string, ...string) error

type frontendCLI struct {
	name        string
	packageName string
}

type projectOptions struct {
	projectName    string
	modulePath     string
	frontendModule string
}

// createProject 在当前目录下创建完整的前后端项目。
func createProject(projectName, cwd string) (string, error) {
	return createProjectWithOptions(projectOptions{projectName: projectName}, cwd, initializeProject)
}

// createProjectWithInitializer 渲染完整项目骨架并执行指定初始化流程。
func createProjectWithInitializer(
	projectName string,
	cwd string,
	initializer projectInitializer,
) (target string, err error) {
	return createProjectWithOptions(projectOptions{projectName: projectName}, cwd, initializer)
}

// createProjectWithOptions 推导项目模块名称，输出模板阶段进度并创建完整项目。
func createProjectWithOptions(options projectOptions, cwd string, initializer projectInitializer) (target string, err error) {
	projectName := path.Base(filepath.Clean(options.projectName))
	if projectName == "." || projectName == "/" || projectName == "" {
		return "", fmt.Errorf("无效的项目名称: %s", options.projectName)
	}
	modulePath := options.modulePath
	if modulePath == "" {
		modulePath = "github.com/example/" + projectName + "/backend"
		if strings.Contains(options.projectName, "/") {
			modulePath = options.projectName + "/backend"
		}
	}
	err = module.CheckPath(modulePath)
	if err != nil {
		return "", fmt.Errorf("无效的 Go module %q: %w", modulePath, err)
	}
	var modules []string
	modules, err = parseBusinessModules(options.frontendModule)
	if err != nil {
		return "", err
	}
	frontendModule := strings.Join(modules, ",")
	target = filepath.Join(cwd, projectName)
	_, err = os.Stat(target)
	if err == nil {
		return "", fmt.Errorf("目标目录已存在，拒绝覆盖: %s", target)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("检查目标目录 %s: %w", target, err)
	}

	projectProgress.Printf("[1/5] 创建项目骨架：%s（Go module：%s）", target, modulePath)
	err = os.Mkdir(target, 0o755)
	if err != nil {
		return "", fmt.Errorf("创建目标目录 %s: %w", target, err)
	}
	cleanupTarget := target
	initialized := false
	defer func() {
		if initialized {
			return
		}
		projectProgress.Printf("生成失败，清理未完成项目：%s", cleanupTarget)
		cleanupErr := os.RemoveAll(cleanupTarget)
		if cleanupErr != nil && err == nil {
			err = fmt.Errorf("清理未完成项目 %s: %w", cleanupTarget, cleanupErr)
		}
	}()

	tokens := map[string]string{
		"__MODULE_PATH__":      modulePath,
		"__PROJECT_NAME__":     projectName,
		"__PACKAGE_NAME__":     projectPackageName(projectName),
		"__BUSINESS_PACKAGE__": projectPackageName(modules[0]),
		"__FRONTEND_MODULE__":  modules[0],
		"__MODULES__":          frontendModule,
		"__MODULES_SPACE__":    strings.Join(modules, " "),
		"__MODULES_PYTHON__":   "\"" + strings.Join(modules, "\", \"") + "\",",
		"__DATABASE_NAME__":    projectPackageName(projectName),
	}
	err = renderTemplates(target, projectTemplateRoot, tokens)
	if err != nil {
		return "", err
	}
	backendTarget := filepath.Join(target, "backend")
	err = os.MkdirAll(backendTarget, 0o755)
	if err != nil {
		return "", fmt.Errorf("创建后端目录: %w", err)
	}
	err = renderTemplates(backendTarget, backendTemplateRoot, tokens)
	if err != nil {
		return "", err
	}
	for _, directoryTemplate := range projectDirectories {
		for _, moduleName := range modules {
			tokens["__FRONTEND_MODULE__"] = moduleName
			directory := directoryTemplate
			directory = replaceTokens(directory, tokens)
			path := filepath.Join(target, filepath.FromSlash(directory))
			err = os.MkdirAll(path, 0o755)
			if err != nil {
				return "", fmt.Errorf("创建项目目录 %s: %w", directory, err)
			}
			var entries []os.DirEntry
			entries, err = os.ReadDir(path)
			if err != nil {
				return "", err
			}
			if len(entries) == 0 {
				err = os.WriteFile(filepath.Join(path, ".gitkeep"), nil, 0o644)
				if err != nil {
					return "", err
				}
			}
		}
	}
	err = os.MkdirAll(filepath.Join(target, "frontend"), 0o755)
	if err != nil {
		return "", fmt.Errorf("创建前端目录: %w", err)
	}
	err = initializer(target, frontendModule)
	if err != nil {
		return "", err
	}
	initialized = true
	return target, nil
}

// renderTemplates 渲染公共模板，并为每个业务模块展开带模块占位符的路径。
func renderTemplates(target, templateRoot string, tokens map[string]string) error {
	return fs.WalkDir(projectTemplates, templateRoot, func(templatePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(templateRoot, templatePath)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}
		modules := []string{tokens["__FRONTEND_MODULE__"]}
		if strings.Contains(relativePath, "__FRONTEND_MODULE__") && tokens["__MODULES__"] != "" {
			modules = strings.Split(tokens["__MODULES__"], ",")
		}
		for _, moduleName := range modules {
			current := maps.Clone(tokens)
			current["__FRONTEND_MODULE__"] = moduleName
			current["__BUSINESS_PACKAGE__"] = projectPackageName(moduleName)
			renderedPath := renameTemplateFile(replaceTokens(filepath.ToSlash(relativePath), current))
			outputPath := filepath.Join(target, filepath.FromSlash(renderedPath))
			if entry.IsDir() {
				err = os.MkdirAll(outputPath, 0o755)
			} else {
				var content []byte
				content, err = projectTemplates.ReadFile(templatePath)
				if err != nil {
					return err
				}
				var parsed *template.Template
				parsed, err = template.New(templatePath).Delims("[[[", "]]]").Parse(string(content))
				if err != nil {
					return err
				}
				moduleList := tokens["__MODULES__"]
				if moduleList == "" {
					moduleList = tokens["__FRONTEND_MODULE__"]
				}
				var rendered bytes.Buffer
				err = parsed.Execute(&rendered, map[string]any{"Modules": strings.Split(moduleList, ","), "GoModule": tokens["__MODULE_PATH__"]})
				if err != nil {
					return err
				}
				content = []byte(replaceTokens(rendered.String(), current))
				mode := os.FileMode(0o644)
				if strings.HasSuffix(renderedPath, ".sh") || strings.HasPrefix(string(content), "#!") {
					mode = 0o755
				}
				err = os.WriteFile(outputPath, content, mode)
			}
			if err != nil {
				return fmt.Errorf("生成模板 %s: %w", renderedPath, err)
			}
		}
		return nil
	})
}

// initializeProject 输出初始化阶段进度，生成并验证后端后补齐前端工具链。
func initializeProject(target, frontendModule string) error {
	err := initializeProjectWithRunner(target, frontendModule, runProjectCommandInDirectory, projectDependencyResolver{backend: resolveBackendDependency, frontend: resolveFrontendVersion})
	if err != nil {
		return err
	}
	err = normalizeFrontendModules(target, strings.Split(frontendModule, ","))
	if err != nil {
		return err
	}
	projectProgress.Printf("[5/5] 补齐前端工具链与语言注册文件")
	return completeFrontendWorkspaces(target, frontendModule)
}

// initializeProjectWithRunner 按发布版本复用缓存，输出阶段进度并执行生成及验证命令。
func initializeProjectWithRunner(target, frontendModule string, runner projectCommandRunner, resolve projectDependencyResolver) error {
	var err error
	for _, cli := range frontendCLIs {
		projectProgress.Printf("[2/5] 生成 %s 前端（业务 module：%s）", cli.name, frontendModule)
		var version string
		version, err = resolve.frontend(cli.packageName)
		if err != nil {
			return err
		}
		projectProgress.Printf("使用 %s CLI %s，复用 pnpm 可用缓存", cli.name, version)
		// 精确版本隔离不同发布的 CLI 缓存，不再强制清空 dlx 缓存。
		args := []string{"--config.@liujitcn:registry=https://registry.npmjs.org/", "dlx", cli.packageName + "@" + version, "create", filepath.Join(target, "frontend", cli.name)}
		for _, moduleName := range frontendModuleAliases(strings.Split(frontendModule, ",")) {
			args = append(args, "--module", moduleName)
		}
		err = runner(target, ".", "pnpm", args...)
		if err != nil {
			return fmt.Errorf("生成 %s 前端失败: %w", cli.name, err)
		}
	}
	backendTarget := filepath.Join(target, "backend")
	projectProgress.Printf("[3/5] 对比 Backend 发布 tag 与本地 Go 模块缓存")
	var dependency backendDependency
	dependency, err = resolve.backend()
	if err != nil {
		return err
	}
	if dependency.cached {
		projectProgress.Printf("Backend %s 已缓存，跳过 go get", dependency.version)
		err = runner(backendTarget, ".", "go", "mod", "edit", "-require="+adminBackendModule+"@"+dependency.version)
	} else {
		projectProgress.Printf("Backend %s 尚未缓存，获取指定版本", dependency.version)
		err = runner(backendTarget, ".", "go", "get", adminBackendModule+"@"+dependency.version)
	}
	if err != nil {
		return fmt.Errorf("设置 Admin Backend 依赖失败: %w", err)
	}
	err = runner(backendTarget, ".", "go", "mod", "tidy")
	if err != nil {
		return err
	}
	projectProgress.Printf("[4/5] 生成 Wire、格式化并测试后端")
	// 创建项目时直接运行固定版本 Wire，不依赖用户预先安装全局生成工具。
	for _, directory := range []string{"internal/module", "internal/cmd/server"} {
		err = runner(backendTarget, directory, "go", "run", "github.com/google/wire/cmd/wire@v0.7.0", ".")
		if err != nil {
			return err
		}
	}
	err = runner(backendTarget, ".", "go", "fmt", "./...")
	if err != nil {
		return err
	}
	err = runner(backendTarget, ".", "go", "test", "./...")
	if err != nil {
		return err
	}
	return nil
}

// replaceTokens 替换路径或文件内容中的全部模板占位符。
func replaceTokens(value string, tokens map[string]string) string {
	for token, replacement := range tokens {
		value = strings.ReplaceAll(value, token, replacement)
	}
	return value
}

// renameTemplateFile 将可发布的占位文件名还原为点文件。
func renameTemplateFile(filePath string) string {
	if strings.HasSuffix(filePath, ".tmpl") {
		return strings.TrimSuffix(filePath, ".tmpl")
	}
	switch path.Base(filePath) {
	case "gitignore":
		return path.Join(path.Dir(filePath), ".gitignore")
	case "dockerignore":
		return path.Join(path.Dir(filePath), ".dockerignore")
	default:
		return filePath
	}
}

// projectPackageName 将项目目录名转换为合法且稳定的 Go 包名。
func projectPackageName(projectName string) string {
	var builder strings.Builder
	for _, character := range projectName {
		switch {
		case character >= 'a' && character <= 'z':
			builder.WriteRune(character)
		case character >= 'A' && character <= 'Z':
			builder.WriteRune(unicode.ToLower(character))
		case character >= '0' && character <= '9' && builder.Len() > 0:
			builder.WriteRune(character)
		}
	}
	if builder.Len() == 0 {
		return "project"
	}
	packageName := builder.String()
	if packageName[0] >= '0' && packageName[0] <= '9' {
		return "project" + packageName
	}
	return packageName
}
