package clientip

import (
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// FromRequest 仅在 TCP 对端属于受信代理时从右向左解析转发链，默认忽略客户端可伪造的头。
func FromRequest(request *http.Request) string {
	return FromRequestWithTrustedProxyCIDRs(request)
}

// FromRequestWithTrustedProxyCIDRs 使用调用方提供的受信代理网段解析转发链。
func FromRequestWithTrustedProxyCIDRs(request *http.Request, trustedProxyCIDRs ...string) string {
	address, err := netip.ParseAddrPort(request.RemoteAddr)
	var peer netip.Addr
	if err == nil {
		peer = address.Addr().Unmap()
	} else {
		peer, err = netip.ParseAddr(request.RemoteAddr)
		if err != nil {
			return request.RemoteAddr
		}
		peer = peer.Unmap()
	}
	var trusted []netip.Prefix
	for _, network := range trustedProxyCIDRs {
		network = strings.TrimSpace(network)
		if network == "" {
			continue
		}
		var prefix netip.Prefix
		prefix, err = netip.ParsePrefix(network)
		if err != nil {
			// 配置错误时保留 TCP 对端，绝不降级为信任任意转发头。
			return peer.String()
		}
		trusted = append(trusted, prefix.Masked())
	}
	forwarded := request.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		forwarded = request.Header.Get("X-Real-IP")
	}
	for _, hop := range slices.Backward(strings.Split(forwarded, ",")) {
		if !slices.ContainsFunc(trusted, func(prefix netip.Prefix) bool { return prefix.Contains(peer) }) {
			break
		}
		var candidate netip.Addr
		// 转发链的逗号分隔语法允许空白，只规范化 HTTP 地址项。
		candidate, err = netip.ParseAddr(strings.TrimSpace(hop))
		if err != nil {
			break
		}
		peer = candidate.Unmap()
	}
	return peer.String()
}
