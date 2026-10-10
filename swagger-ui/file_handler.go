package swaggerUI

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

type openApiFileHandler struct {
	Content []byte
}

func (h *openApiFileHandler) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	_, _ = writer.Write(h.Content)
}

// loadOpenApiFile 读取 OpenAPI 文件内容。
func (h *openApiFileHandler) loadOpenApiFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	content, err := io.ReadAll(file)
	closeErr := file.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close OpenAPI file: %w", closeErr)
	}
	return content, nil
}

func (h *openApiFileHandler) LoadFile(filePath string) error {
	content, err := h.loadOpenApiFile(filePath)
	if err != nil {
		return err
	}

	h.Content = content
	return nil
}
