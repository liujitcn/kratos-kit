package zookeeper

import (
	"context"
	"errors"
	"path"
	"time"

	"github.com/go-zookeeper/zk"
	"golang.org/x/sync/singleflight"

	"github.com/go-kratos/kratos/v3/registry"
)

var (
	_ registry.Registrar = (*Registry)(nil)
	_ registry.Discovery = (*Registry)(nil)
)

// Registry 是 ZooKeeper 注册发现实现。
type Registry struct {
	opts *options
	conn *zk.Conn

	group singleflight.Group
}

// New 创建 ZooKeeper 注册发现实例。
func New(conn *zk.Conn, opts ...Option) *Registry {
	opt := &options{
		namespace: "/microservices",
	}
	for _, o := range opts {
		o(opt)
	}
	return &Registry{
		opts: opt,
		conn: conn,
	}
}

// Register 将服务实例注册到 ZooKeeper。
func (r *Registry) Register(_ context.Context, service *registry.ServiceInstance) error {
	var (
		data []byte
		err  error
	)
	if err = r.ensureName(r.opts.namespace, []byte(""), 0); err != nil {
		return err
	}
	serviceNamePath := path.Join(r.opts.namespace, service.Name)
	if err = r.ensureName(serviceNamePath, []byte(""), 0); err != nil {
		return err
	}
	if data, err = marshal(service); err != nil {
		return err
	}
	servicePath := path.Join(serviceNamePath, service.ID)
	if err = r.ensureName(servicePath, data, zk.FlagEphemeral); err != nil {
		return err
	}
	go r.reRegister(servicePath, data)
	return nil
}

// Deregister 从 ZooKeeper 注销服务实例。
func (r *Registry) Deregister(ctx context.Context, service *registry.ServiceInstance) error {
	ch := make(chan error, 1)
	servicePath := path.Join(r.opts.namespace, service.Name, service.ID)
	go func() {
		err := r.conn.Delete(servicePath, -1)
		ch <- err
	}()
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-ch:
	}
	return err
}

// GetService 从 ZooKeeper 获取指定服务实例列表。
func (r *Registry) GetService(_ context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	instances, err, _ := r.group.Do(serviceName, func() (interface{}, error) {
		serviceNamePath := path.Join(r.opts.namespace, serviceName)
		servicesID, _, err := r.conn.Children(serviceNamePath)
		if err != nil {
			return nil, err
		}
		items := make([]*registry.ServiceInstance, 0, len(servicesID))
		for _, service := range servicesID {
			servicePath := path.Join(serviceNamePath, service)
			serviceInstanceByte, _, err := r.conn.Get(servicePath)
			if err != nil {
				return nil, err
			}
			item, err := unmarshal(serviceInstanceByte)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	})
	if err != nil {
		return nil, err
	}
	return instances.([]*registry.ServiceInstance), nil
}

// Watch 监听指定服务实例变化。
func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	prefix := path.Join(r.opts.namespace, serviceName)
	return newWatcher(ctx, prefix, serviceName, r.conn)
}

// ensureName 确保 ZooKeeper 节点存在，不存在时创建并写入数据。
func (r *Registry) ensureName(path string, data []byte, flags int32) error {
	exists, stat, err := r.conn.Exists(path)
	if err != nil {
		return err
	}
	// 临时节点在会话恢复后需要重新处理，避免服务异常退出后旧节点残留导致创建冲突。
	if flags&zk.FlagEphemeral == zk.FlagEphemeral {
		err = r.conn.Delete(path, stat.Version)
		if err != nil && !errors.Is(err, zk.ErrNoNode) {
			return err
		}
		exists = false
	}
	if !exists {
		if len(r.opts.user) > 0 && len(r.opts.password) > 0 {
			_, err = r.conn.Create(path, data, flags, zk.DigestACL(zk.PermAll, r.opts.user, r.opts.password))
		} else {
			_, err = r.conn.Create(path, data, flags, zk.WorldACL(zk.PermAll))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// reRegister 在 ZooKeeper 连接恢复后重新注册数据节点。
func (r *Registry) reRegister(path string, data []byte) {
	sessionID := r.conn.SessionID()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		cur := r.conn.SessionID()
		// 会话 ID 变化说明连接已恢复为新会话，需要重新创建临时节点。
		if cur > 0 && sessionID != cur {
			if err := r.ensureName(path, data, zk.FlagEphemeral); err != nil {
				return
			}
			sessionID = cur
		}
	}
}
