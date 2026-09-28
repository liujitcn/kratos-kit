package platformhttp

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"time"

	httpx "github.com/liujitcn/go-utils/http"
	notify "github.com/liujitcn/kratos-kit/notify"
)

var restrictedPrefixes = []netip.Prefix{
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

// SafeClient 创建禁用代理和重定向、并在拨号时检查解析 IP 的客户端。
func SafeClient(timeout time.Duration, allowPrivate bool) *httpx.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: nil, DialContext: safeDialContext(timeout, allowPrivate)},
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return Client(notify.HTTPTransport(client), timeout)
}

// IsAllowedIP 判断出站地址是否符合默认公共网络策略。
func IsAllowedIP(ip net.IP, allowPrivate bool) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if address.IsUnspecified() || address.IsMulticast() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() {
		return false
	}
	if address.String() == "100.100.100.200" || address.String() == "fd00:ec2::254" {
		return false
	}
	for _, prefix := range restrictedPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	if (address.IsLoopback() || address.IsPrivate()) && !allowPrivate {
		return false
	}
	if allowPrivate {
		return true
	}
	return address.IsGlobalUnicast()
}

// safeDialContext 校验 DNS 返回的所有地址并固定拨号 IP，降低 DNS 重绑定风险。
func safeDialContext(timeout time.Duration, allowPrivate bool) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, net.InvalidAddrError("host resolved to no IP addresses")
		}
		for _, item := range ips {
			if !IsAllowedIP(item.IP, allowPrivate) {
				return nil, net.InvalidAddrError("host resolved to a restricted IP address")
			}
		}
		var dialErr error
		for _, item := range ips {
			conn, connectErr := dialer.DialContext(ctx, network, net.JoinHostPort(item.IP.String(), port))
			if connectErr == nil {
				return conn, nil
			}
			dialErr = connectErr
		}
		return nil, dialErr
	}
}
