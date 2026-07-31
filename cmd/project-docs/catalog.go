package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxDocumentContentBytes = 2 << 20

type document struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	UpdatedAt string `json:"updated_at"`
}

type bundle struct {
	Documents   []document         `json:"documents"`
	Directories []catalogDirectory `json:"directories"`
}

type catalogDirectory struct {
	Name        string             `json:"name"`
	Path        string             `json:"path"`
	Documents   []document         `json:"documents"`
	Directories []catalogDirectory `json:"directories"`
}

type catalogBuilder struct {
	documents   []document
	directories map[string]*catalogDirectoryBuilder
}

type catalogDirectoryBuilder struct {
	name        string
	path        string
	documents   []document
	directories map[string]*catalogDirectoryBuilder
}

// newDocument 根据相对路径创建构建期项目文档。
func newDocument(documentPath, content, updatedAt string) document {
	return document{
		Path:      normalizePath(documentPath),
		Content:   content,
		UpdatedAt: updatedAt,
	}
}

// marshalCatalog 校验项目文档并编码为稳定的目录构建产物。
func marshalCatalog(documents []document) ([]byte, error) {
	normalizedDocuments := make([]document, 0, len(documents))
	documentPaths := make(map[string]struct{}, len(documents))
	var err error
	for _, currentDocument := range documents {
		var normalizedDocument document
		normalizedDocument, err = validateDocument(currentDocument)
		if err != nil {
			return nil, err
		}
		if _, exists := documentPaths[normalizedDocument.Path]; exists {
			return nil, fmt.Errorf("项目文档路径重复: %s", normalizedDocument.Path)
		}
		documentPaths[normalizedDocument.Path] = struct{}{}
		normalizedDocuments = append(normalizedDocuments, normalizedDocument)
	}
	sort.Slice(normalizedDocuments, func(left, right int) bool {
		return normalizedDocuments[left].Path < normalizedDocuments[right].Path
	})

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(buildCatalog(normalizedDocuments))
	if err != nil {
		return nil, fmt.Errorf("编码项目文档目录: %w", err)
	}
	return buffer.Bytes(), nil
}

// buildCatalog 按相对目录构建稳定的文档树。
func buildCatalog(documents []document) bundle {
	builder := catalogBuilder{
		documents:   make([]document, 0),
		directories: make(map[string]*catalogDirectoryBuilder),
	}
	for _, currentDocument := range documents {
		segments := strings.Split(currentDocument.Path, "/")
		if len(segments) == 1 {
			builder.documents = append(builder.documents, currentDocument)
			continue
		}
		currentDirectories := builder.directories
		directoryPath := ""
		var parentDirectory *catalogDirectoryBuilder
		for _, directoryName := range segments[:len(segments)-1] {
			directoryPath = path.Join(directoryPath, directoryName)
			directory, exists := currentDirectories[directoryName]
			if !exists {
				directory = &catalogDirectoryBuilder{
					name:        directoryName,
					path:        directoryPath,
					documents:   make([]document, 0),
					directories: make(map[string]*catalogDirectoryBuilder),
				}
				currentDirectories[directoryName] = directory
			}
			parentDirectory = directory
			currentDirectories = directory.directories
		}
		parentDirectory.documents = append(parentDirectory.documents, currentDocument)
	}
	return bundle{
		Documents:   builder.documents,
		Directories: buildCatalogDirectories(builder.directories),
	}
}

// buildCatalogDirectories 将目录构建节点递归转换为按名称排序的目录树。
func buildCatalogDirectories(builders map[string]*catalogDirectoryBuilder) []catalogDirectory {
	names := make([]string, 0, len(builders))
	for name := range builders {
		names = append(names, name)
	}
	sort.Strings(names)

	directories := make([]catalogDirectory, 0, len(names))
	for _, name := range names {
		builder := builders[name]
		directories = append(directories, catalogDirectory{
			Name:        builder.name,
			Path:        builder.path,
			Documents:   append(make([]document, 0, len(builder.documents)), builder.documents...),
			Directories: buildCatalogDirectories(builder.directories),
		})
	}
	return directories
}

// validateDocument 校验并规范化构建期文档字段。
func validateDocument(currentDocument document) (document, error) {
	normalizedPath := normalizePath(currentDocument.Path)
	if normalizedPath == "." ||
		normalizedPath == "" ||
		path.IsAbs(normalizedPath) ||
		strings.HasPrefix(normalizedPath, "../") {
		return document{}, fmt.Errorf("项目文档路径无效: %q", currentDocument.Path)
	}
	if !utf8.ValidString(currentDocument.Content) {
		return document{}, fmt.Errorf("项目文档不是有效 UTF-8: %s", normalizedPath)
	}
	if len(currentDocument.Content) > maxDocumentContentBytes {
		return document{}, fmt.Errorf("项目文档超过 2 MiB: %s", normalizedPath)
	}
	return newDocument(
		normalizedPath,
		currentDocument.Content,
		currentDocument.UpdatedAt,
	), nil
}

// normalizePath 将各平台文件路径统一为项目内斜杠路径。
func normalizePath(documentPath string) string {
	return strings.TrimPrefix(path.Clean(strings.ReplaceAll(documentPath, "\\", "/")), "./")
}
