package pprof

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/grafana/pyroscope-go"
	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

type Pyroscope struct {
	cfg       *conf.Pprof_Pyroscope
	pyroscope *pyroscope.Profiler
}

// NewPyroscope 创建一个服务监控
func NewPyroscope(cfg *conf.Pprof_Pyroscope) (Pprof, error) {
	if cfg == nil {
		return nil, nil
	}
	return &Pyroscope{
		cfg: cfg,
	}, nil
}

func (p *Pyroscope) Start() {
	var err error
	p.pyroscope, err = pyroscope.Start(pyroscope.Config{
		ApplicationName:   p.cfg.GetApplicationName(),
		Tags:              p.cfg.GetTags(),
		ServerAddress:     p.cfg.GetServerAddress(),
		BasicAuthUser:     p.cfg.GetBasicAuthUser(),
		BasicAuthPassword: p.cfg.GetBasicAuthPassword(),
		TenantID:          p.cfg.GetTenantId(),
		UploadRate:        p.cfg.GetUploadRate().AsDuration(),
		ProfileTypes:      profileTypes(p.cfg.GetProfileTypes()),
		DisableGCRuns:     p.cfg.GetDisableGCRuns(),
		HTTPHeaders:       p.cfg.GetHttpHeaders(),
	})
	if err != nil {
		log.Errorf("pyroscope.Start: %v", err)
	} else {
		log.Infof("pyroscope.Start: ok")
	}
}

func (p *Pyroscope) Stop() {
	var err error
	if p.pyroscope != nil {
		err = p.pyroscope.Stop()
	}
	if err != nil {
		log.Errorf("pyroscope.Stop: %v", err)
	} else {
		log.Infof("pyroscope.Stop: ok")
	}
}

func profileTypes(profileTypes []string) []pyroscope.ProfileType {
	res := make([]pyroscope.ProfileType, 0)
	for _, item := range profileTypes {
		res = append(res, pyroscope.ProfileType(item))
	}
	return res
}
