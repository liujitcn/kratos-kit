package clientip

import (
	"net/http/httptest"
	"testing"
)

// TestProxyTrust 验证只从受信代理链最右侧提取真实客户端，不接受直连伪造的转发头。
func TestProxyTrust(t *testing.T) {
	t.Setenv("SECURITY_TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	request := httptest.NewRequest("GET", "http://example.test", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	if result := FromRequest(request); result != "192.0.2.10" {
		t.Fatalf("直连头被信任: %s", result)
	}
	request.RemoteAddr = "10.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 192.0.2.10, 10.0.0.2")
	if result := FromRequest(request); result != "192.0.2.10" {
		t.Fatalf("代理链解析错误: %s", result)
	}
	t.Setenv("SECURITY_TRUSTED_PROXY_CIDRS", "invalid")
	if result := FromRequest(request); result != "10.0.0.1" {
		t.Fatalf("无效配置扩大信任: %s", result)
	}
}
