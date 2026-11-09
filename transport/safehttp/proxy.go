package safehttp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"

	netproxy "golang.org/x/net/proxy"
)

// DialProxy 经批准的代理连接已固定的目标 IP，业务传输继续使用原始 Host、SNI 和证书。
func (p *Policy) DialProxy(ctx context.Context, network, address string, proxy *url.URL, proxyTLS *tls.Config) (net.Conn, error) {
	err := p.ValidateProxy(ctx, proxy)
	if err != nil {
		return nil, err
	}
	var host, port string
	host, port, err = net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var addresses []netip.Addr
	addresses, err = p.resolve(ctx, host, port)
	if err != nil {
		return nil, err
	}
	for _, ip := range addresses {
		target := net.JoinHostPort(ip.String(), port)
		var connection net.Conn
		if proxy.Scheme == "socks5" || proxy.Scheme == "socks5h" {
			var dialer netproxy.Dialer
			configured := *proxy
			configured.Scheme = "socks5"
			dialer, err = netproxy.FromURL(&configured, policyDialer{p})
			if err != nil {
				return nil, err
			}
			connection, err = dialer.(netproxy.ContextDialer).DialContext(ctx, network, target)
		} else {
			connection, err = p.connectProxy(ctx, network, target, proxy, proxyTLS)
		}
		if err == nil {
			return connection, nil
		}
	}
	return nil, fmt.Errorf("代理连接目标失败: %w", err)
}

// connectProxy 通过 HTTP CONNECT 固定真实目的地址，限制握手读取并在取消时关闭连接。
func (p *Policy) connectProxy(ctx context.Context, network, target string, proxy *url.URL, proxyTLS *tls.Config) (connection net.Conn, err error) {
	port := proxy.Port()
	if port == "" {
		port = "80"
		if proxy.Scheme == "https" {
			port = "443"
		}
	}
	connection, err = p.DialContext(ctx, network, net.JoinHostPort(proxy.Hostname(), port))
	if err != nil {
		return nil, err
	}
	raw := connection
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	defer stop()
	defer func() {
		if err != nil {
			_ = raw.Close()
		}
	}()
	if proxy.Scheme == "https" {
		configuration := &tls.Config{MinVersion: tls.VersionTLS12}
		if proxyTLS != nil {
			configuration = proxyTLS.Clone()
		}
		configuration.ServerName = proxy.Hostname()
		configuration.NextProtos = []string{"http/1.1"}
		secured := tls.Client(connection, configuration)
		err = secured.HandshakeContext(ctx)
		if err != nil {
			return nil, err
		}
		connection = secured
	}
	request := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: target}, Host: target, Header: make(http.Header)}
	if proxy.User != nil {
		password, _ := proxy.User.Password()
		request.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(proxy.User.Username()+":"+password)))
	}
	err = request.Write(connection)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(io.LimitReader(connection, 64*1024))
	var response *http.Response
	response, err = http.ReadResponse(reader, request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("代理拒绝 CONNECT，状态码 %d", response.StatusCode)
	}
	var buffered []byte
	buffered, err = reader.Peek(reader.Buffered())
	if err != nil {
		return nil, err
	}
	return &proxyConnection{Conn: connection, reader: io.MultiReader(bytes.NewReader(bytes.Clone(buffered)), connection)}, nil
}

// policyDialer 为 SOCKS5 握手提供同一安全拨号策略。
type policyDialer struct{ policy *Policy }

// Dial 兼容代理库同步拨号接口，业务使用支持取消的 DialContext。
func (d policyDialer) Dial(network, address string) (net.Conn, error) {
	return d.policy.DialContext(context.Background(), network, address)
}

// DialContext 对代理地址应用 DNS 和网段检查。
func (d policyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.policy.DialContext(ctx, network, address)
}

// proxyConnection 保留 CONNECT 读取缓冲区，同时恢复无限制的业务读取。
type proxyConnection struct {
	net.Conn
	reader io.Reader
}

// Read 先消费握手缓冲区，避免丢弃代理已传来的业务字节。
func (c *proxyConnection) Read(data []byte) (int, error) { return c.reader.Read(data) }
