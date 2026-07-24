package pprof

import (
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

func NewPprof(cfg *configv1.Pprof) (Pprof, error) {
	if cfg == nil {
		return nil, nil
	}
	switch cfg.Type {
	default:
		fallthrough
	case "pyroscope":
		return NewPyroscope(cfg.Pyroscope)
	}
}
