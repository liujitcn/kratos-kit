package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/mod/module"
)

const (
	projectTemplateRoot = "templates/project"
	backendTemplateRoot = "templates/backend"
)

var projectDirectories = []string{
	"backend/data",
}

var frontendCLIs = []frontendCLI{
	{name: "admin", packageName: "@liujitcn/kratos-admin-cli@latest"},
	{name: "uni-app", packageName: "@liujitcn/kratos-uni-app-cli@latest"},
	{name: "taro-app", packageName: "@liujitcn/kratos-taro-app-cli@latest"},
}

//go:embed templates
var projectTemplates embed.FS

type projectInitializer func(string, string) error

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

// createProjectWithOptions 按项目名称、后端 module 和前端 module 创建完整项目。
func createProjectWithOptions(options projectOptions, cwd string, initializer projectInitializer) (target string, err error) {
	projectName := path.Base(filepath.Clean(options.projectName))
	if projectName == "." || projectName == "/" || projectName == "" {
		return "", fmt.Errorf("无效的项目名称: %s", options.projectName)
	}
	modulePath := options.modulePath
	if modulePath == "" {
		modulePath = "github.com/example/" + projectName + "/backend"
	}
	err = module.CheckPath(modulePath)
	if err != nil {
		return "", fmt.Errorf("无效的 Go module %q: %w", modulePath, err)
	}
	frontendModule := options.frontendModule
	if frontendModule == "" {
		frontendModule = "app"
	}
	if strings.ContainsAny(frontendModule, "/, \\ \t\r\n") {
		return "", fmt.Errorf("无效的前端 module 名称: %s", frontendModule)
	}
	target = filepath.Join(cwd, projectName)
	_, err = os.Stat(target)
	if err == nil {
		return "", fmt.Errorf("目标目录已存在，拒绝覆盖: %s", target)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("检查目标目录 %s: %w", target, err)
	}

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
		cleanupErr := os.RemoveAll(cleanupTarget)
		if cleanupErr != nil && err == nil {
			err = fmt.Errorf("清理未完成项目 %s: %w", cleanupTarget, cleanupErr)
		}
	}()

	tokens := map[string]string{
		"__MODULE_PATH__":     modulePath,
		"__PROJECT_NAME__":    projectName,
		"__PACKAGE_NAME__":    projectPackageName(projectName),
		"__FRONTEND_MODULE__": frontendModule,
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
	for _, directory := range projectDirectories {
		err = os.MkdirAll(filepath.Join(target, filepath.FromSlash(directory)), 0o755)
		if err != nil {
			return "", fmt.Errorf("创建项目目录 %s: %w", directory, err)
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

// renderTemplates 将嵌入模板目录渲染到目标目录。
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
		renderedPath := replaceTokens(filepath.ToSlash(relativePath), tokens)
		renderedPath = renameTemplateFile(renderedPath)
		outputPath := filepath.Join(target, filepath.FromSlash(renderedPath))
		if entry.IsDir() {
			err = os.MkdirAll(outputPath, 0o755)
			if err != nil {
				return fmt.Errorf("创建模板目录 %s: %w", renderedPath, err)
			}
			return nil
		}
		var content []byte
		content, err = projectTemplates.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("读取模板 %s: %w", templatePath, err)
		}
		content = []byte(replaceTokens(string(content), tokens))
		err = os.WriteFile(outputPath, content, 0o644)
		if err != nil {
			return fmt.Errorf("写入模板文件 %s: %w", renderedPath, err)
		}
		if strings.HasSuffix(renderedPath, ".sh") {
			err = os.Chmod(outputPath, 0o755)
			if err != nil {
				return fmt.Errorf("设置模板脚本权限 %s: %w", renderedPath, err)
			}
		}
		return nil
	})
}

// initializeProject 生成前端、Wire 产物，并验证完整项目可以编译。
func initializeProject(target, frontendModule string) error {
	var err error
	for _, cli := range frontendCLIs {
		err = runProjectCommand(
			target,
			"pnpm",
			"dlx",
			cli.packageName,
			"create",
			filepath.Join(target, "frontend", cli.name),
			"--module",
			frontendModule,
		)
		if err != nil {
			return fmt.Errorf("生成 %s 前端失败: %w", cli.name, err)
		}
	}
	backendTarget := filepath.Join(target, "backend")
	err = runProjectCommand(backendTarget, "go", "mod", "tidy")
	if err != nil {
		return err
	}
	err = runProjectCommandInDirectory(
		backendTarget,
		"internal/cmd/server",
		"go",
		"run",
		"github.com/google/wire/cmd/wire@v0.7.0",
		".",
	)
	if err != nil {
		return err
	}
	err = runProjectCommand(backendTarget, "go", "test", "./...")
	if err != nil {
		return err
	}
	return nil
}

// runProjectCommand 在生成项目中执行命令并返回完整失败上下文。
func runProjectCommand(target, name string, args ...string) error {
	return runProjectCommandInDirectory(target, ".", name, args...)
}

// runProjectCommandInDirectory 在生成项目指定目录执行命令并返回完整失败上下文。
func runProjectCommandInDirectory(target, directory, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Dir = filepath.Join(target, filepath.FromSlash(directory))
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"在 %s 执行 %s %s 失败: %w\n%s",
			command.Dir,
			name,
			strings.Join(args, " "),
			err,
			strings.TrimSpace(string(output)),
		)
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
