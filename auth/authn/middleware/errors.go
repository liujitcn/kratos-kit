package middleware

import "github.com/go-kratos/kratos/v3/errors"

const (
	reason string = "UNAUTHORIZED"
)

var (
	ErrUnauthorized = errors.Unauthorized(reason, "unauthorized access")
)
