// Command project-docs 收集当前项目约定范围内的 README 和 docs Markdown 文档。
package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultProjectDocsPath = "internal/projectdocs"
	backendProjectDocsPath = "backend/internal/projectdocs"
	maxSourcePathDepth     = 3
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

// main 执行当前项目文档收集命令。
func main() {
	err := run()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run 收集当前项目文档并写入约定的目录构建产物。
func run() error {
	if len(os.Args) > 1 {
		return fmt.Errorf("project-docs 不接受命令行参数")
	}
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("读取当前项目目录: %w", err)
	}
	projectDocsPath := defaultProjectDocsPath
	var backendInfo fs.FileInfo
	backendInfo, err = os.Stat(filepath.Join(root, "backend"))
	if err == nil {
		if backendInfo.IsDir() {
			projectDocsPath = backendProjectDocsPath
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查 Backend 目录: %w", err)
	}
	outputPath := filepath.Join(root, projectDocsPath, "assets", "catalog.json")
	goOutputPath := filepath.Join(root, projectDocsPath, "catalog_gen.go")
	documents, err := scanSource(root)
	if err != nil {
		return err
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

// scanSource 扫描当前项目约定范围内的 Markdown 文档。
func scanSource(root string) ([]document, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("读取项目根目录: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("项目根目录不是目录: %s", root)
	}

	documents := make([]document, 0)
	err = filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, entryErr error) error {
		return collectSourceEntry(root, filePath, entry, entryErr, &documents)
	})
	if err != nil {
		return nil, fmt.Errorf("扫描项目文档: %w", err)
	}
	return documents, nil
}

// collectSourceEntry 处理目录遍历中的单个文件并追加可收集文档。
func collectSourceEntry(
	root string,
	filePath string,
	entry fs.DirEntry,
	entryErr error,
	documents *[]document,
) error {
	if entryErr != nil {
		return entryErr
	}
	relativePath, err := filepath.Rel(root, filePath)
	if err != nil {
		return err
	}
	if entry.IsDir() {
		if relativePath != "." {
			segments := strings.Split(filepath.ToSlash(relativePath), "/")
			if len(segments) >= maxSourcePathDepth || shouldSkipDirectory(relativePath) {
				return filepath.SkipDir
			}
		}
		return nil
	}
	if !shouldCollect(relativePath) {
		return nil
	}
	var currentDocument document
	currentDocument, err = readDocument(filePath, relativePath, entry)
	if err != nil {
		return err
	}
	*documents = append(*documents, currentDocument)
	return nil
}

// readDocument 读取、校验并转换单篇 Markdown 文档及其更新时间。
func readDocument(filePath, relativePath string, entry fs.DirEntry) (document, error) {
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
		normalizedPath,
		string(content),
		info.ModTime().UTC().Format(time.RFC3339),
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
func shouldSkipDirectory(relativePath string) bool {
	normalizedPath := filepath.ToSlash(relativePath)
	_, excluded := excludedDirectories[filepath.Base(normalizedPath)]
	return excluded
}

// shouldCollect 判断三层范围内的文件是否为 README 或 docs Markdown。
func shouldCollect(relativePath string) bool {
	normalizedPath := filepath.ToSlash(relativePath)
	segments := strings.Split(normalizedPath, "/")
	if len(segments) > maxSourcePathDepth {
		return false
	}
	if filepath.Base(normalizedPath) == "README.md" {
		return true
	}
	if !strings.EqualFold(filepath.Ext(normalizedPath), ".md") {
		return false
	}
	for _, segment := range segments[:len(segments)-1] {
		if segment == "docs" {
			return true
		}
	}
	return false
}
