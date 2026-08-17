package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type existingDocumentCatalog struct {
	Documents   []document         `json:"documents"`
	Directories []catalogDirectory `json:"directories"`
}

// mergeCachedLocalizedDocuments 复用源文未变化文档的已有语言版本。
func mergeCachedLocalizedDocuments(outputPath string, documents []document) ([]document, error) {
	cached, err := loadExistingDocumentCatalog(outputPath)
	if err != nil {
		return nil, err
	}
	for index := range documents {
		currentDocument := &documents[index]
		cachedDocument, exists := cached[currentDocument.Path]
		if !exists || cachedDocument.Content != currentDocument.Content {
			continue
		}
		if len(cachedDocument.LocalizedContents) == 0 {
			continue
		}
		if currentDocument.LocalizedContents == nil {
			currentDocument.LocalizedContents = make(map[string]string, len(cachedDocument.LocalizedContents))
		}
		for locale, content := range cachedDocument.LocalizedContents {
			if !hasLocalizedContent(currentDocument.LocalizedContents, locale) {
				currentDocument.LocalizedContents[locale] = content
			}
		}
	}
	return documents, nil
}

// loadExistingDocumentCatalog 读取上一次生成的文档目录。
func loadExistingDocumentCatalog(outputPath string) (map[string]document, error) {
	data, err := os.ReadFile(outputPath)
	if os.IsNotExist(err) {
		return map[string]document{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取已有项目文档目录: %w", err)
	}
	var catalog existingDocumentCatalog
	err = json.Unmarshal(data, &catalog)
	if err != nil {
		return nil, fmt.Errorf("解析已有项目文档目录: %w", err)
	}
	documents := make(map[string]document)
	for _, currentDocument := range catalog.Documents {
		documents[normalizePath(currentDocument.Path)] = currentDocument
	}
	for _, directory := range catalog.Directories {
		collectCatalogDirectoryDocuments(directory, documents)
	}
	return documents, nil
}

// collectCatalogDirectoryDocuments 递归收集目录节点中的已有文档。
func collectCatalogDirectoryDocuments(directory catalogDirectory, documents map[string]document) {
	for _, currentDocument := range directory.Documents {
		documents[normalizePath(currentDocument.Path)] = currentDocument
	}
	for _, child := range directory.Directories {
		collectCatalogDirectoryDocuments(child, documents)
	}
}
