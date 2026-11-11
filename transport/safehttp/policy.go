package safehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Policy 定义服务端出站目标范围，私网和回环地址必须显式授权。
type Policy struct {
	public   bool
	hosts    map[string]struct{}
	networks []netip.Prefix
	lookup   func(context.Context, string) ([]net.IPAddr, error)
	dial     func(context.Context, string, string) (net.Conn, error)
}

var restrictedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// NewPolicy 创建出站策略；主机可带端口，CIDR 不允许覆盖链路本地或元数据地址。
func NewPolicy(allowPublic bool, hosts, cidrs []string) (*Policy, error) {
	policy := &Policy{public: allowPublic, hosts: make(map[string]struct{}), lookup: net.DefaultResolver.LookupIPAddr}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	policy.dial = dialer.DialContext
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		if strings.ContainsAny(host, "/@?#*") {
			return nil, fmt.Errorf("出站白名单必须为完整主机名或主机:端口: %s", host)
		}
		policy.hosts[strings.TrimSuffix(host, ".")] = struct{}{}
	}
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("出站 CIDR 无效: %w", err)
		}
		policy.networks = append(policy.networks, prefix.Masked())
	}
	return policy, nil
}

// ValidateURL 在发送凭据或请求体之前校验协议、端口、主机和全部解析地址。
func (p *Policy) ValidateURL(ctx context.Context, target *url.URL) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" || target.User != nil || target.Opaque != "" {
		return fmt.Errorf("出站地址必须为不含用户信息的 HTTP(S) URL")
	}
	port := target.Port()
	if port == "" {
		port = "443"
		if target.Scheme == "http" {
			port = "80"
		}
	}
	_, err := p.resolve(ctx, target.Hostname(), port)
	return err
}

// ValidateProxy 保留 HTTP(S) 和 SOCKS5 代理认证，同时校验实际代理目标。
func (p *Policy) ValidateProxy(ctx context.Context, target *url.URL) error {
	if target == nil {
		return fmt.Errorf("代理地址不能为空")
	}
	checked := *target
	checked.User = nil
	if checked.Scheme == "socks5" || checked.Scheme == "socks5h" {
		checked.Scheme = "http"
		if checked.Port() == "" {
			checked.Host = net.JoinHostPort(checked.Hostname(), "1080")
		}
	}
	return p.ValidateURL(ctx, &checked)
}

// DialContext 重新校验 DNS 并固定连接到已通过检查的 IP，不将主机名交给拨号器再次解析。
func (p *Policy) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("出站连接仅支持 TCP")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("出站地址无效: %w", err)
	}
	var ips []netip.Addr
	ips, err = p.resolve(ctx, host, port)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		var connection net.Conn
		connection, err = p.dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return connection, nil
		}
	}
	return nil, fmt.Errorf("出站连接失败: %w", err)
}

// resolve 校验目的主机及所有解析结果，任一地址越界时拒绝整个请求。
func (p *Policy) resolve(ctx context.Context, host, port string) ([]netip.Addr, error) {
	if p == nil {
		return nil, fmt.Errorf("出站安全策略未初始化")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, fmt.Errorf("出站端口无效")
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	_, hostAllowed := p.hosts[host]
	if !hostAllowed {
		_, hostAllowed = p.hosts[net.JoinHostPort(host, port)]
	}
	var address netip.Addr
	address, err = netip.ParseAddr(host)
	if err == nil {
		address = address.Unmap()
		if !p.allows(address, hostAllowed) {
			return nil, fmt.Errorf("出站地址不在授权范围")
		}
		return []netip.Addr{address}, nil
	}
	if !p.public && !hostAllowed {
		return nil, fmt.Errorf("出站主机不在授权范围")
	}
	var resolved []net.IPAddr
	resolved, err = p.lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("解析出站主机失败: %w", err)
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("出站主机没有可用地址")
	}
	ips := make([]netip.Addr, 0, len(resolved))
	for _, item := range resolved {
		ip, ok := netip.AddrFromSlice(item.IP)
		if !ok || item.Zone != "" || !p.allows(ip.Unmap(), hostAllowed) {
			return nil, fmt.Errorf("出站主机解析到未授权地址")
		}
		ips = append(ips, ip.Unmap())
	}
	return ips, nil
}

// allows 检查 IP 的真实范围，域名白名单不能绕过私网授权或元数据保护。
func (p *Policy) allows(address netip.Addr, hostAllowed bool) bool {
	if !address.IsValid() || address.Zone() != "" || address.IsUnspecified() || address.IsMulticast() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() {
		return false
	}
	if address.String() == "100.100.100.200" || address.String() == "fd00:ec2::254" {
		return false
	}
	for _, prefix := range p.networks {
		if prefix.Contains(address) {
			return true
		}
	}
	if address.IsLoopback() || address.IsPrivate() {
		return false
	}
	for _, prefix := range restrictedNetworks {
		if prefix.Contains(address) {
			return false
		}
	}
	return address.IsGlobalUnicast() && (p.public || hostAllowed)
}

// Transport 克隆现有传输配置并注入安全拨号，保留 TLS、证书和连接池配置。
func (p *Policy) Transport(base *http.Transport) http.RoundTripper {
	transport := base.Clone()
	transport.Proxy = nil
	transport.DialContext = p.DialContext
	transport.DialTLSContext = nil
	return p.Guard(transport)
}

// Guard 包装请求校验；自定义或 SDK 传输仍须同时使用 DialContext 固定连接 IP。
func (p *Policy) Guard(base http.RoundTripper) http.RoundTripper {
	return &guardedTransport{policy: p, base: base}
}

// GuardOrigin 限制认证请求保持同源，避免 SDK 重定向把凭据重新附加到其他服务。
func (p *Policy) GuardOrigin(base http.RoundTripper, origin *url.URL) http.RoundTripper {
	return &guardedTransport{policy: p, base: base, origin: origin.Scheme + "://" + origin.Host}
}

// Client 创建使用安全传输的客户端，同源重定向逐跳校验，跨源跳转拒绝携带业务凭据。
func (p *Policy) Client(timeout time.Duration) *http.Client {
	return &http.Client{Transport: p.Transport(http.DefaultTransport.(*http.Transport)), Timeout: timeout, CheckRedirect: p.CheckRedirect}
}

// CheckRedirect 保留同源重定向并重新校验目标，最多允许十次跳转。
func (p *Policy) CheckRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 10 || (len(via) > 0 && (request.URL.Scheme != via[0].URL.Scheme || request.URL.Host != via[0].URL.Host)) {
		return fmt.Errorf("拒绝跨源或过多的出站重定向")
	}
	return p.ValidateURL(request.Context(), request.URL)
}

// guardedTransport 在认证传输发出请求之前检查出站目标。
type guardedTransport struct {
	policy *Policy
	base   http.RoundTripper
	origin string
}

// RoundTrip 校验请求目的地址并交给原传输实现发送。
func (t *guardedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.origin != "" && request.URL.Scheme+"://"+request.URL.Host != t.origin {
		return nil, fmt.Errorf("认证请求不能重定向到其他来源")
	}
	err := t.policy.ValidateURL(request.Context(), request.URL)
	if err != nil {
		return nil, err
	}
	return t.base.RoundTrip(request)
}

// CloseIdleConnections 透传空闲连接清理，临时客户端结束时不遗留连接池。
func (t *guardedTransport) CloseIdleConnections() {
	if transport, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}
