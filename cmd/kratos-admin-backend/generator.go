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

	"golang.org/x/mod/module"
)

const (
	templateRoot      = "templates/project"
	coreModuleVersion = "v0.1.1"
)

var projectDirectories = []string{
	"api/proto",
	"api/gen/go",
	"data/admin/assets",
	"data/app/assets",
	"data/logs",
	"docs",
	"internal/biz/dto",
	"internal/cmd/server/assets",
	"internal/data/gen/data",
	"internal/data/gen/models",
	"internal/data/gen/query",
	"internal/projectdocs/assets",
	"internal/server/middleware",
	"migration/assets/v0.0.1/mysql",
}

//go:embed templates/project
var projectTemplates embed.FS

type projectInitializer func(string, string) error

// createProject 在当前目录下创建以后端 module 末段命名的项目。
func createProject(modulePath, cwd string) (string, error) {
	return createProjectWithInitializer(modulePath, cwd, initializeProject)
}

// createProjectWithInitializer 渲染项目骨架并执行指定初始化流程。
func createProjectWithInitializer(
	modulePath string,
	cwd string,
	initializer projectInitializer,
) (target string, err error) {
	err = module.CheckPath(modulePath)
	if err != nil {
		return "", fmt.Errorf("无效的 Go module %q: %w", modulePath, err)
	}
	projectName := path.Base(modulePath)
	if projectName == "." || projectName == "/" || projectName == "" {
		return "", fmt.Errorf("无法从 Go module 推导项目目录: %s", modulePath)
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
		"__MODULE_PATH__":         modulePath,
		"__PROJECT_NAME__":        projectName,
		"__CORE_MODULE_VERSION__": coreModuleVersion,
	}
	err = renderProject(target, tokens)
	if err != nil {
		return "", err
	}
	for _, directory := range projectDirectories {
		err = os.MkdirAll(filepath.Join(target, filepath.FromSlash(directory)), 0o755)
		if err != nil {
			return "", fmt.Errorf("创建项目目录 %s: %w", directory, err)
		}
	}
	err = initializer(target, projectName)
	if err != nil {
		return "", err
	}
	initialized = true
	return target, nil
}

// renderProject 将嵌入模板渲染到目标目录。
func renderProject(target string, tokens map[string]string) error {
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
		return nil
	})
}

// initializeProject 生成项目文档与 Wire 产物，并验证项目可以编译。
func initializeProject(target, _ string) error {
	var err error
	err = generateProjectDocuments(target)
	if err != nil {
		return err
	}
	err = runProjectCommand(target, "go", "mod", "tidy")
	if err != nil {
		return err
	}
	err = runProjectCommand(target, "go", "run", "github.com/google/wire/cmd/wire@v0.7.0", ".")
	if err != nil {
		return err
	}
	err = runProjectCommand(target, "go", "test", "./...")
	if err != nil {
		return err
	}
	return nil
}

// generateProjectDocuments 通过 project-docs 生成项目文档目录和嵌入源码。
func generateProjectDocuments(target string) error {
	executable, err := exec.LookPath("project-docs")
	if err == nil {
		return runProjectCommand(target, executable)
	}
	return runProjectCommand(
		target,
		"go",
		"run",
		"github.com/liujitcn/kratos-kit/cmd/project-docs@latest",
	)
}

// runProjectCommand 在生成项目中执行命令并返回完整失败上下文。
func runProjectCommand(target, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Dir = target
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"在 %s 执行 %s %s 失败: %w\n%s",
			target,
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
