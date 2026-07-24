package sse

type ClientOption func(o *Client)

// WithEndpoint 设置 SSE 客户端请求地址。
func WithEndpoint(uri string) ClientOption {
	return func(c *Client) {
		c.URL = uri
	}
}
