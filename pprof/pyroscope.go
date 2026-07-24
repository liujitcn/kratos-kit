package pprof

import (
	"github.com/go-kratos/kratos/v3/log"
	"github.com/grafana/pyroscope-go"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

// Pyroscope 封装 Pyroscope 性能采样器实例。
type Pyroscope struct {
	cfg       *configv1.Pprof_Pyroscope
	pyroscope *pyroscope.Profiler
}

// NewPyroscope 创建一个服务监控
func NewPyroscope(cfg *configv1.Pprof_Pyroscope) (Pprof, error) {
	if cfg == nil {
		return nil, nil
	}
	return &Pyroscope{
		cfg: cfg,
	}, nil
}

// Start 启动 Pyroscope 性能采样上报。
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
		DisableGCRuns:     p.cfg.GetDisableGcRuns(),
		HTTPHeaders:       p.cfg.GetHttpHeaders(),
	})
	if err != nil {
		log.Error("pyroscope.Start failed", "error", err)
	}
}

// Stop 停止 Pyroscope 性能采样上报。
func (p *Pyroscope) Stop() {
	var err error
	// 仅当采样器已成功启动时才尝试停止，避免空指针调用。
	if p.pyroscope != nil {
		err = p.pyroscope.Stop()
	}
	if err != nil {
		log.Error("pyroscope.Stop failed", "error", err)
	} else {
		log.Info("pyroscope.Stop: ok")
	}
}

func profileTypes(profileTypes []string) []pyroscope.ProfileType {
	res := make([]pyroscope.ProfileType, 0)
	for _, item := range profileTypes {
		res = append(res, pyroscope.ProfileType(item))
	}
	return res
}
