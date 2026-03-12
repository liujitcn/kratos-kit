package pprof

import (
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

func NewPprof(cfg *conf.Pprof) (Pprof, error) {
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
