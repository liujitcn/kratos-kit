package bootstrap

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2"
	"github.com/stretchr/testify/assert"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

func initApp(ctx *Context) (*kratos.App, func(), error) {
	app := NewApp(ctx)
	return app, func() {
	}, nil
}

// TestBootstrapWithNameVersion 验证应用信息存在时可正常构建应用实例。
func TestBootstrapWithNameVersion(t *testing.T) {
	serviceName := "test"
	version := "v0.0.1"

	ctx := NewContext(context.Background(), &conf.AppInfo{
		Project: "",
		AppId:   serviceName,
		Version: version,
	})

	app := NewApp(ctx)
	assert.NotNil(t, app)
}

// TestNewInstanceId 验证实例 ID 能正常生成。
func TestNewInstanceId(t *testing.T) {
	instanceId := NewInstanceId("gowind-test-service", "1.0.0", "127.0.0.1", "8000")
	t.Logf("InstanceId: %s", instanceId)
}
