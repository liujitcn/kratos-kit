// Command project-docs 收集一个或多个项目的指定 README 和根 docs Markdown 文档。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	defaultOutputPath      = "internal/projectdocs/assets/catalog.json"
	defaultGoOutputPath    = "internal/projectdocs/catalog_gen.go"
	maxSourceDocumentBytes = 2 << 20
)

var excludedDirectories = map[string]struct{}{
	".git":         {},
	".idea":        {},
	".turbo":       {},
	".vscode":      {},
	"build":        {},
	"data":         {},
	"dist":         {},
	"node_modules": {},
	"vendor":       {},
}

type source struct {
	key  string
	name string
	root string
}

// main 执行多项目文档收集命令。
func main() {
	err := run()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run 解析参数、收集文档并写入目录构建产物。
func run() error {
	sources := make([]source, 0)
	outputPath := defaultOutputPath
	var goOutputPath string
	flag.Func("source", "文档来源，格式为 OpenAPI-key:项目名称=项目根目录，可重复传入", func(value string) error {
		project, root, found := strings.Cut(value, "=")
		if !found || strings.TrimSpace(project) == "" || strings.TrimSpace(root) == "" {
			return fmt.Errorf("无效的 --source %q，格式应为 OpenAPI-key:项目名称=项目根目录", value)
		}
		key, name, hasName := strings.Cut(project, ":")
		if !hasName {
			name = key
		}
		sources = append(sources, source{
			key:  strings.TrimSpace(key),
			name: strings.TrimSpace(name),
			root: strings.TrimSpace(root),
		})
		return nil
	})
	flag.StringVar(&outputPath, "output", defaultOutputPath, "输出 catalog.json 路径")
	flag.StringVar(&goOutputPath, "go-output", "", "输出 go:embed Go 文件路径；标准 catalog.json 路径下默认自动生成")
	flag.Parse()
	if len(sources) == 0 {
		return fmt.Errorf("必须至少提供一个 --source")
	}
	if goOutputPath == "" && filepath.Clean(outputPath) == filepath.Clean(defaultOutputPath) {
		goOutputPath = defaultGoOutputPath
	}

	documents := make([]document, 0)
	var err error
	for _, item := range sources {
		var sourceDocuments []document
		sourceDocuments, err = scanSource(item)
		if err != nil {
			return err
		}
		documents = append(documents, sourceDocuments...)
	}
	var data []byte
	data, err = marshalCatalog(documents)
	if err != nil {
		return err
	}
	err = writeFileIfChanged(outputPath, data)
	if err != nil {
		return err
	}
	if goOutputPath != "" {
		var goSource []byte
		goSource, err = generateCatalogGoSource(goOutputPath, outputPath)
		if err != nil {
			return err
		}
		err = writeFileIfChanged(goOutputPath, goSource)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(
			os.Stdout,
			"已收集 %d 篇项目文档到 %s，并生成 %s\n",
			len(documents),
			outputPath,
			goOutputPath,
		)
		return nil
	}
	_, _ = fmt.Fprintf(os.Stdout, "已收集 %d 篇项目文档到 %s\n", len(documents), outputPath)
	return nil
}

// scanSource 扫描单个项目来源中的指定 README 和根 docs Markdown 文档。
func scanSource(item source) ([]document, error) {
	root, err := filepath.Abs(item.root)
	if err != nil {
		return nil, fmt.Errorf("解析项目 %s 根目录: %w", item.key, err)
	}
	var info fs.FileInfo
	info, err = os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("读取项目 %s 根目录: %w", item.key, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("项目 %s 根目录不是目录: %s", item.key, root)
	}

	documents := make([]document, 0)
	err = filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, entryErr error) error {
		return collectSourceEntry(item, root, filePath, entry, entryErr, &documents)
	})
	if err != nil {
		return nil, fmt.Errorf("扫描项目 %s 文档: %w", item.key, err)
	}
	return documents, nil
}

// collectSourceEntry 处理目录遍历中的单个文件并追加可收集文档。
func collectSourceEntry(
	item source,
	root string,
	filePath string,
	entry fs.DirEntry,
	entryErr error,
	documents *[]document,
) error {
	if entryErr != nil {
		return entryErr
	}
	if entry.IsDir() {
		if filePath != root && shouldSkipDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		return nil
	}
	relativePath, err := filepath.Rel(root, filePath)
	if err != nil {
		return err
	}
	if !shouldCollect(relativePath) {
		return nil
	}
	var currentDocument document
	currentDocument, err = readDocument(item.key, item.name, filePath, relativePath, entry)
	if err != nil {
		return err
	}
	*documents = append(*documents, currentDocument)
	return nil
}

// readDocument 读取、校验并转换单篇 Markdown 文档。
func readDocument(projectKey, projectName, filePath, relativePath string, entry fs.DirEntry) (document, error) {
	info, err := entry.Info()
	if err != nil {
		return document{}, err
	}
	if info.Size() > maxSourceDocumentBytes {
		return document{}, fmt.Errorf("文档超过 2 MiB: %s", relativePath)
	}
	var content []byte
	content, err = os.ReadFile(filePath)
	if err != nil {
		return document{}, err
	}
	if !utf8.Valid(content) {
		return document{}, fmt.Errorf("文档不是有效 UTF-8: %s", relativePath)
	}
	normalizedPath := filepath.ToSlash(relativePath)
	return newDocument(
		projectKey,
		projectName,
		normalizedPath,
		string(content),
	), nil
}

// writeFileIfChanged 仅在内容变化时原子替换生成文件。
func writeFileIfChanged(outputPath string, data []byte) error {
	current, err := os.ReadFile(outputPath)
	if err == nil && bytes.Equal(current, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取现有生成文件 %s: %w", outputPath, err)
	}
	err = os.MkdirAll(filepath.Dir(outputPath), 0o755)
	if err != nil {
		return fmt.Errorf("创建生成目录 %s: %w", filepath.Dir(outputPath), err)
	}
	var tempFile *os.File
	tempFile, err = os.CreateTemp(filepath.Dir(outputPath), ".project-docs-*")
	if err != nil {
		return fmt.Errorf("创建临时生成文件: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	err = tempFile.Chmod(0o644)
	if err == nil {
		_, err = tempFile.Write(data)
	}
	if err == nil {
		err = tempFile.Close()
	}
	if err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("写入生成文件 %s: %w", outputPath, err)
	}
	err = os.Rename(tempPath, outputPath)
	if err != nil {
		return fmt.Errorf("替换生成文件 %s: %w", outputPath, err)
	}
	return nil
}

// shouldSkipDirectory 判断目录是否属于依赖、构建或运行时产物。
func shouldSkipDirectory(name string) bool {
	_, excluded := excludedDirectories[name]
	return excluded
}

// shouldCollect 判断相对路径是否属于约定的 README 或根 docs Markdown。
func shouldCollect(relativePath string) bool {
	normalizedPath := filepath.ToSlash(relativePath)
	switch normalizedPath {
	case "README.md", "frontend/admin/README.md", "frontend/app/README.md":
		return true
	}
	if !strings.EqualFold(filepath.Ext(normalizedPath), ".md") {
		return false
	}
	return strings.HasPrefix(normalizedPath, "docs/")
}
