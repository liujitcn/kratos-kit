package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxDocumentContentBytes = 2 << 20

var projectKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

type document struct {
	ID          string `json:"id"`
	ProjectKey  string `json:"project_key"`
	ProjectName string `json:"project_name"`
	Path        string `json:"path"`
	Content     string `json:"content"`
}

type bundle struct {
	Projects []catalogProject `json:"projects"`
}

type catalogProject struct {
	Key         string             `json:"key"`
	Name        string             `json:"name"`
	Documents   []document         `json:"documents"`
	Directories []catalogDirectory `json:"directories"`
}

type catalogDirectory struct {
	Name        string             `json:"name"`
	Path        string             `json:"path"`
	Documents   []document         `json:"documents"`
	Directories []catalogDirectory `json:"directories"`
}

type catalogProjectBuilder struct {
	key         string
	name        string
	documents   []document
	directories map[string]*catalogDirectoryBuilder
}

type catalogDirectoryBuilder struct {
	name        string
	path        string
	documents   []document
	directories map[string]*catalogDirectoryBuilder
}

// newDocument 根据 OpenAPI 项目标识和相对路径创建带稳定 ID 的项目文档。
func newDocument(projectKey, projectName, documentPath, content string) document {
	normalizedProjectKey := strings.TrimSpace(projectKey)
	normalizedProjectName := strings.TrimSpace(projectName)
	normalizedPath := normalizePath(documentPath)
	sum := sha256.Sum256([]byte(normalizedProjectKey + "\x00" + normalizedPath))
	return document{
		ID:          hex.EncodeToString(sum[:16]),
		ProjectKey:  normalizedProjectKey,
		ProjectName: normalizedProjectName,
		Path:        normalizedPath,
		Content:     content,
	}
}

// marshalCatalog 校验项目文档并编码为稳定的目录构建产物。
func marshalCatalog(documents []document) ([]byte, error) {
	normalizedDocuments := make([]document, 0, len(documents))
	documentIDs := make(map[string]struct{}, len(documents))
	documentKeys := make(map[string]struct{}, len(documents))
	projectNames := make(map[string]string)
	var err error
	for _, currentDocument := range documents {
		var normalizedDocument document
		normalizedDocument, err = validateDocument(currentDocument)
		if err != nil {
			return nil, err
		}
		projectName, exists := projectNames[normalizedDocument.ProjectKey]
		if exists && projectName != normalizedDocument.ProjectName {
			return nil, fmt.Errorf(
				"项目文档标识 %q 对应多个项目名称: %q、%q",
				normalizedDocument.ProjectKey,
				projectName,
				normalizedDocument.ProjectName,
			)
		}
		documentKey := normalizedDocument.ProjectKey + "\x00" + normalizedDocument.Path
		if _, exists = documentKeys[documentKey]; exists {
			return nil, fmt.Errorf(
				"项目文档路径重复: %s/%s",
				normalizedDocument.ProjectKey,
				normalizedDocument.Path,
			)
		}
		if _, exists = documentIDs[normalizedDocument.ID]; exists {
			return nil, fmt.Errorf("项目文档 ID 冲突: %s", normalizedDocument.ID)
		}
		projectNames[normalizedDocument.ProjectKey] = normalizedDocument.ProjectName
		documentKeys[documentKey] = struct{}{}
		documentIDs[normalizedDocument.ID] = struct{}{}
		normalizedDocuments = append(normalizedDocuments, normalizedDocument)
	}
	sort.Slice(normalizedDocuments, func(left, right int) bool {
		if normalizedDocuments[left].ProjectKey == normalizedDocuments[right].ProjectKey {
			return normalizedDocuments[left].Path < normalizedDocuments[right].Path
		}
		return normalizedDocuments[left].ProjectKey < normalizedDocuments[right].ProjectKey
	})
	projects := buildCatalogProjects(normalizedDocuments)

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(bundle{Projects: projects})
	if err != nil {
		return nil, fmt.Errorf("编码项目文档目录: %w", err)
	}
	return buffer.Bytes(), nil
}

// buildCatalogProjects 按项目和相对目录构建稳定的文档树。
func buildCatalogProjects(documents []document) []catalogProject {
	builders := make([]catalogProjectBuilder, 0)
	var currentProject *catalogProjectBuilder
	for _, currentDocument := range documents {
		if currentProject == nil || currentProject.key != currentDocument.ProjectKey {
			builders = append(builders, catalogProjectBuilder{
				key:         currentDocument.ProjectKey,
				name:        currentDocument.ProjectName,
				documents:   make([]document, 0),
				directories: make(map[string]*catalogDirectoryBuilder),
			})
			currentProject = &builders[len(builders)-1]
		}
		segments := strings.Split(currentDocument.Path, "/")
		if len(segments) == 1 {
			currentProject.documents = append(currentProject.documents, currentDocument)
			continue
		}
		currentDirectories := currentProject.directories
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

	projects := make([]catalogProject, 0, len(builders))
	for _, builder := range builders {
		projects = append(projects, catalogProject{
			Key:         builder.key,
			Name:        builder.name,
			Documents:   append(make([]document, 0, len(builder.documents)), builder.documents...),
			Directories: buildCatalogDirectories(builder.directories),
		})
	}
	return projects
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

// validateDocument 校验文档字段并重建稳定 ID。
func validateDocument(currentDocument document) (document, error) {
	if !projectKeyPattern.MatchString(strings.TrimSpace(currentDocument.ProjectKey)) {
		return document{}, fmt.Errorf(
			"项目文档标识 %q 必须与 OpenAPI key 一致，只能包含字母、数字、点、下划线和连字符",
			currentDocument.ProjectKey,
		)
	}
	if strings.TrimSpace(currentDocument.ProjectName) == "" {
		return document{}, fmt.Errorf("项目文档缺少项目名称")
	}
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
		currentDocument.ProjectKey,
		currentDocument.ProjectName,
		normalizedPath,
		currentDocument.Content,
	), nil
}

// normalizePath 将各平台文件路径统一为项目内斜杠路径。
func normalizePath(documentPath string) string {
	return strings.TrimPrefix(path.Clean(strings.ReplaceAll(documentPath, "\\", "/")), "./")
}
