package provider

import (
	"net/url"
	"strconv"
	"strings"
)

// JoinScopes 使用指定分隔符拼接 OAuth scope。
func JoinScopes(scopes []string, sep string) string {
	return strings.Join(scopes, sep)
}

// ChooseScopes 按覆盖参数、配置参数、默认参数的优先级选择 scope。
func ChooseScopes(configured []string, override []string, defaults []string) []string {
	switch {
	case len(override) > 0:
		return override
	case len(configured) > 0:
		return configured
	default:
		return defaults
	}
}

// BuildAuthURL 将查询参数追加到授权地址。
func BuildAuthURL(endpoint string, params url.Values) string {
	u, _ := url.Parse(endpoint)
	q := u.Query()
	for k, vs := range params {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// MergeParams 将额外参数覆盖合并到目标参数中。
func MergeParams(dst url.Values, src url.Values) {
	for k, vs := range src {
		dst.Del(k)
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

// SetPKCEAuthParams 设置授权地址中的 PKCE 参数。
func SetPKCEAuthParams(params url.Values, challenge *PKCEChallenge) {
	if challenge == nil {
		return
	}
	params.Set("code_challenge", challenge.Challenge)
	params.Set("code_challenge_method", challenge.Method)
}

// SetPKCETokenParams 设置换取 Token 请求中的 PKCE 参数。
func SetPKCETokenParams(params url.Values, challenge *PKCEChallenge) {
	if challenge == nil || challenge.Verifier == "" {
		return
	}
	params.Set("code_verifier", challenge.Verifier)
}

// StringifyID 将第三方平台返回的 ID 转换为字符串。
func StringifyID(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	default:
		return ""
	}
}
