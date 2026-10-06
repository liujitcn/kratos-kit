package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	kratosHTTP "github.com/go-kratos/kratos/v3/transport/http"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TestCreateHttpServerJSSafeInt64 走真实 CreateHttpServer 链路，验证大整数以字符串返回、小整数保持数字、请求端接受两种形态。
func TestCreateHttpServerJSSafeInt64(t *testing.T) {
	cfg := &configv1.Bootstrap{
		Server: &configv1.Server{
			Http: &configv1.Server_Http{Addr: ":0"},
		},
	}
	srv, err := CreateHttpServer(cfg)
	if err != nil {
		t.Fatalf("创建 HTTP 服务失败: %v", err)
	}
	srv.Route("/test").GET("/big", func(ctx kratosHTTP.Context) error {
		return ctx.Returns(&durationpb.Duration{Seconds: 57309943358867204}, nil)
	})
	srv.Route("/test").GET("/small", func(ctx kratosHTTP.Context) error {
		return ctx.Returns(&durationpb.Duration{Seconds: 1}, nil)
	})
	srv.Route("/test").POST("/echo", func(ctx kratosHTTP.Context) error {
		value := &durationpb.Duration{}
		if err := ctx.Bind(value); err != nil {
			return err
		}
		return ctx.Returns(value, nil)
	})

	go func() {
		_ = srv.Start(context.Background())
	}()
	defer func() { _ = srv.Stop(context.Background()) }()

	client := &http.Client{Timeout: 5 * time.Second}
	// Endpoint 在监听建立前会解析到临时端口，必须轮询探测到真实监听地址。
	base := waitReady(t, client, srv, "/test/small")

	assertBody(t, client, base+"/test/big", `"seconds":"57309943358867204"`)
	assertBody(t, client, base+"/test/small", `"seconds":1`)

	resp, err := client.Post(base+"/test/echo", "application/json", strings.NewReader(`{"seconds":"57309943358867204"}`))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	if !strings.Contains(string(body), `"seconds":"57309943358867204"`) {
		t.Errorf("字符串回传输出 = %s, 期望包含字符串形态大整数", body)
	}

	resp, err = client.Post(base+"/test/echo", "application/json", strings.NewReader(`{"seconds":57309943358867204}`))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	if !strings.Contains(string(body), `"seconds":"57309943358867204"`) {
		t.Errorf("数字回传输出 = %s, 期望包含字符串形态大整数", body)
	}
}

// waitReady 轮询等待服务就绪并返回真实服务地址。
func waitReady(t *testing.T, client *http.Client, srv *kratosHTTP.Server, path string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if endpoint, err := srv.Endpoint(); err == nil {
			base := endpoint.String()
			if !strings.HasPrefix(base, "http") {
				base = "http://" + base
			}
			resp, err := client.Get(base + path)
			if err == nil {
				_ = resp.Body.Close()
				return base
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("服务未就绪")
	return ""
}

// assertBody 请求 GET 地址并断言响应包含期望片段。
func assertBody(t *testing.T, client *http.Client, url, want string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("请求 %s 失败: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	if !strings.Contains(string(body), want) {
		t.Errorf("%s 响应 = %s, 期望包含 %s", url, body, want)
	}
	_ = json.Valid(body)
}
