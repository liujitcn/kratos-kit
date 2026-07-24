package zookeeper

// Option 配置 ZooKeeper 注册发现实例。
type Option func(o *options)

type options struct {
	namespace string
	user      string
	password  string
}

// WithRootPath 设置注册信息根路径。
func WithRootPath(path string) Option {
	return func(o *options) { o.namespace = path }
}

// WithDigestACL 设置 ZooKeeper Digest ACL 认证信息。
func WithDigestACL(user string, password string) Option {
	return func(o *options) {
		o.user = user
		o.password = password
	}
}
