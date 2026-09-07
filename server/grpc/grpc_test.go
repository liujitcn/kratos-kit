package grpc

import (
	"context"
	"testing"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
)

// TestCreateGrpcServerWithOptions 验证每次创建仅应用自身选项，不改变原始服务器类型。
func TestCreateGrpcServerWithOptions(t *testing.T) {
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			called := 0
			server, err := CreateGrpcServerWithOptions(nil, []kratosgrpc.ServerOption{func(*kratosgrpc.Server) { called++ }})
			if err != nil {
				t.Fatal(err)
			}
			if server == nil || called != 1 {
				t.Fatalf("服务选项未按实例应用: called=%d", called)
			}
			err = server.Stop(context.Background())
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
