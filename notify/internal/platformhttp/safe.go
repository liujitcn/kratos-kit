package platformhttp

import (
	"context"
	"net"
	"net/url"
	"time"

	httpx "github.com/liujitcn/go-utils/http"
	"github.com/liujitcn/kratos-kit/transport/safehttp"
)

// SafeClient 复用公共安全出站策略，保留已有私网配置并始终阻断元数据地址。
func SafeClient(timeout time.Duration, allowPrivate bool) *httpx.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return Client(notificationPolicy(allowPrivate).Client(timeout), timeout)
}

// IsAllowedIP 按公共策略校验单个目的 IP，包括映射地址及保留地址。
func IsAllowedIP(ip net.IP, allowPrivate bool) bool {
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(ip.String(), "80")}
	return notificationPolicy(allowPrivate).ValidateURL(context.Background(), target) == nil
}

// notificationPolicy 将原有可信私网开关转换为具体网段，不放行链路本地及元数据目标。
func notificationPolicy(allowPrivate bool) *safehttp.Policy {
	var networks []string
	if allowPrivate {
		networks = []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "fc00::/7", "::1/128"}
	}
	policy, err := safehttp.NewPolicy(true, nil, networks)
	if err != nil {
		// 所有 CIDR 都是编译期常量，解析失败意味着程序本身配置错误。
		panic(err)
	}
	return policy
}
