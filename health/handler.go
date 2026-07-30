package health

import (
	"encoding/json"
	"net/http"
)

// Handler 将 [Health] 聚合器暴露为 HTTP JSON 端点。
//
// 适用于 Kubernetes livenessProbe / readinessProbe:
//
//	srv.GET("/healthz",  health.NewLivenessHandler().ServeHTTP)
//	srv.GET("/readyz",   health.NewHandler(h).ServeHTTP)
//
// Liveness 探针只检查进程是否存活（始终返回 200）。
// Readiness 探针执行所有注册的依赖检查器，任一失败返回 503。
type Handler struct {
	health *Health
}

// NewHandler 创建一个 readiness 检查 HTTP handler。
//
// 请求时执行 [Health.Check]，
// 状态为 StatusUp 返回 200，StatusUnknown 返回 200，
// StatusDown 返回 503。
//
// 响应体为 JSON 格式的 [Result]，包含每个检查器的状态明细。
func NewHandler(h *Health) *Handler {
	return &Handler{health: h}
}

// ServeHTTP 实现 http.Handler 接口。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	result := h.health.Check(r.Context())

	statusCode := http.StatusOK
	if result.Status == StatusDown {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	resp := handlerResponse{
		Status: result.Status.String(),
	}
	if result.Message != "" {
		resp.Message = result.Message
	}
	if len(result.Details) > 0 {
		resp.Checks = result.Details
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// handlerResponse 是 HTTP 端点的 JSON 响应结构。
type handlerResponse struct {
	Status  string         `json:"status"`
	Message string         `json:"message,omitempty"`
	Checks  map[string]any `json:"checks,omitempty"`
}

// --- Liveness ---

// livenessHandler 是一个始终返回 200 的简单 handler，
// 只要进程能响应 HTTP 请求，就认为存活。
type livenessHandler struct{}

// NewLivenessHandler 创建一个 liveness 检查 HTTP handler。
//
// Liveness 探针只关心进程是否存活——只要 HTTP 服务器在运行，
// 就认为进程存活。不检查任何外部依赖。
//
// 适用于 Kubernetes livenessProbe。
func NewLivenessHandler() http.Handler {
	return &livenessHandler{}
}

// ServeHTTP 返回存活状态。
func (*livenessHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "up"})
}
