package jwt

import (
	"github.com/golang-jwt/jwt/v5"
)

type Options struct {
	signingMethod jwt.SigningMethod
	keyFunc       jwt.Keyfunc
}

type Option func(d *Options)

// WithSigningMethod set signing method
func WithSigningMethod(alg string) Option {
	return func(o *Options) {
		o.signingMethod = jwt.GetSigningMethod(alg)
	}
}

// WithKey 设置 JWT 签名和验签密钥。
func WithKey(key []byte) Option {
	return func(o *Options) {
		o.keyFunc = func(token *jwt.Token) (any, error) {
			return key, nil
		}
	}
}
