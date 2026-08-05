package swagger

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

// Handler handles swagger UI request.
type Handler struct {
	*Config

	ConfigJson template.JS

	tpl          *template.Template
	staticServer http.Handler
}

// NewHandlerWithConfig 创建 Swagger UI HTTP 处理器，并返回配置或模板初始化错误。
func NewHandlerWithConfig(config *Config, assetsBase, faviconBase string, staticServer http.Handler) (*Handler, error) {
	config.BasePath = strings.TrimSuffix(config.BasePath, "/") + "/"

	h := &Handler{
		Config: config,
	}

	j, err := json.Marshal(h.Config)
	if err != nil {
		return nil, fmt.Errorf("marshal swagger config: %w", err)
	}

	h.ConfigJson = template.JS(j) //nolint:gosec // Data is well-formed.

	h.tpl, err = template.New("index").Parse(IndexTpl(assetsBase, faviconBase, config))
	if err != nil {
		return nil, fmt.Errorf("parse swagger template: %w", err)
	}

	if staticServer != nil {
		h.staticServer = http.StripPrefix(h.BasePath, staticServer)
	}

	return h, nil
}

// ServeHTTP implements http.Handler interface to handle swagger UI request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSuffix(r.URL.Path, "/") != strings.TrimSuffix(h.BasePath, "/") && h.staticServer != nil {
		h.staticServer.ServeHTTP(w, r)

		return
	}

	w.Header().Set("Content-Type", "text/html")

	if err := h.tpl.Execute(w, h); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
