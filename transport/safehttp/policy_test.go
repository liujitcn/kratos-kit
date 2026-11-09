package safehttp

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestOutboundBoundaries 验证映射地址、元数据、URL 用户信息和默认私网不能绕过出站限制。
func TestOutboundBoundaries(t *testing.T) {
	policy, err := NewPolicy(true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"http://127.0.0.1", "http://[::ffff:127.0.0.1]", "http://10.1.2.3", "http://169.254.169.254", "http://100.100.100.200", "file:///etc/passwd", "http://user:pass@8.8.8.8"} {
		var target *url.URL
		target, err = url.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		if policy.ValidateURL(t.Context(), target) == nil {
			t.Fatalf("越界地址被允许: %s", value)
		}
	}
}

// TestApprovedPrivateEndpoint 验证部署放行后私网服务及同源重定向仍可使用。
func TestApprovedPrivateEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirect" {
			http.Redirect(writer, request, "/ok", http.StatusFound)
			return
		}
		_, err := io.WriteString(writer, "approved")
		if err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	policy, err := NewPolicy(false, nil, []string{"127.0.0.1/32"})
	if err != nil {
		t.Fatal(err)
	}
	var response *http.Response
	response, err = policy.Client(time.Second).Get(server.URL + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var data []byte
	data, err = io.ReadAll(response.Body)
	if err != nil || string(data) != "approved" {
		t.Fatalf("合法服务不可用: %s, %v", data, err)
	}
}

// TestPinnedDNS 验证主机授权不能豁免私网检查，拨号只使用验证过的 IP。
func TestPinnedDNS(t *testing.T) {
	policy, err := NewPolicy(false, []string{"cluster.example:6443"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	policy.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("127.0.0.1")}}, nil
	}
	_, err = policy.DialContext(t.Context(), "tcp", "cluster.example:6443")
	if err == nil {
		t.Fatal("混合私网 DNS 结果未拒绝")
	}
	policy.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	policy.dial = func(_ context.Context, _, address string) (net.Conn, error) {
		if address != "8.8.8.8:6443" {
			t.Fatalf("拨号未固定 IP: %s", address)
		}
		return nil, io.EOF
	}
	_, err = policy.DialContext(t.Context(), "tcp", "cluster.example:6443")
	if err == nil {
		t.Fatal("受控拨号错误未返回")
	}
}

// TestProxyPinnedTarget 验证代理隧道使用固定 IP，同时保留业务主机名和认证代理能力。
func TestProxyPinnedTarget(t *testing.T) {
	hits := 0
	originServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		hits++
		if !strings.HasPrefix(request.Host, "cluster.example:") {
			t.Error("原始业务主机名丢失")
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer originServer.Close()
	_, port, err := net.SplitHostPort(originServer.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect || request.Host != "127.0.0.1:"+port {
			t.Error("代理目的地址未固定")
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		upstream, err := net.Dial("tcp", request.Host)
		if err != nil {
			t.Error(err)
			return
		}
		defer upstream.Close()
		var connection net.Conn
		var buffered *bufio.ReadWriter
		connection, buffered, err = writer.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		_, err = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		if err == nil {
			err = buffered.Flush()
		}
		if err != nil {
			t.Error(err)
			return
		}
		go func() { _, _ = io.Copy(upstream, buffered) }()
		_, _ = io.Copy(connection, upstream)
	}))
	defer proxy.Close()
	var policy *Policy
	policy, err = NewPolicy(false, []string{"cluster.example:" + port}, []string{"127.0.0.1/32"})
	if err != nil {
		t.Fatal(err)
	}
	policy.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	var proxyURL, origin *url.URL
	proxyURL, err = url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	origin, err = url.Parse("http://cluster.example:" + port)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return policy.DialProxy(ctx, network, address, proxyURL, nil)
	}
	client := &http.Client{Transport: policy.GuardOrigin(transport, origin), Timeout: time.Second}
	defer client.CloseIdleConnections()
	var response *http.Response
	response, err = client.Get(origin.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if hits != 1 {
		t.Fatal("合法代理请求不可用")
	}
	proxyURL.Scheme = "socks5"
	proxyURL.User = url.UserPassword("test", "test")
	if policy.ValidateProxy(t.Context(), proxyURL) != nil {
		t.Fatal("已授权的认证 SOCKS5 代理被拒绝")
	}
}
