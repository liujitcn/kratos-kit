package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

func TestCreateHttpServerBlocksDebugRoutesWhenPprofDisabled(t *testing.T) {
	server, err := CreateHttpServer(&configv1.Bootstrap{
		Server: &configv1.Server{
			Http: &configv1.Server_Http{EnablePprof: false},
		},
	})
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

	for _, path := range []string{"/debug", "/debug/pprof/", "/debug/vars"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("路径 %s 返回状态码 %d，期望 %d", path, response.Code, http.StatusNotFound)
		}
	}
}

func TestCreateHttpServerRegistersPprofRoutesWhenEnabled(t *testing.T) {
	server, err := CreateHttpServer(&configv1.Bootstrap{
		Server: &configv1.Server{
			Http: &configv1.Server_Http{EnablePprof: true},
		},
	})
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/debug/profile?seconds=1", nil)
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("启用 pprof 后 /debug/profile 返回状态码 %d，期望 %d", response.Code, http.StatusOK)
	}
}
