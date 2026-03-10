package config

import (
	"reflect"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

var (
	muBC         sync.RWMutex
	initOnce     sync.Once
	configList   []proto.Message
	configSet    map[uintptr]struct{}
	commonConfig *conf.Bootstrap
)

func GetBootstrapConfig() *conf.Bootstrap {
	initBootstrapConfig()
	muBC.RLock()
	defer muBC.RUnlock()
	return commonConfig
}

// RegisterConfig 注册配置（去重、并发安全）
// 传入值应为指针类型，例如 &conf.SomeConfig{}
func RegisterConfig(c proto.Message) {
	if c == nil {
		return
	}
	initBootstrapConfig()

	muBC.Lock()
	defer muBC.Unlock()
	addConfigLocked(c)
}

// initBootstrapConfig 初始化引导配置（仅执行一次）
func initBootstrapConfig() {
	initOnce.Do(func() {
		muBC.Lock()
		defer muBC.Unlock()

		// 初始化集合与列表
		configList = make([]proto.Message, 0)
		configSet = make(map[uintptr]struct{})

		if commonConfig == nil {
			commonConfig = &conf.Bootstrap{}
		}

		// 按需添加根与子配置，使用去重函数
		addConfigLocked(commonConfig)

		if commonConfig.GetServer() == nil {
			commonConfig.Server = &conf.Server{}
		}
		addConfigLocked(commonConfig.Server)

		if commonConfig.GetClient() == nil {
			commonConfig.Client = &conf.Client{}
		}
		addConfigLocked(commonConfig.Client)

		if commonConfig.GetData() == nil {
			commonConfig.Data = &conf.Data{}
		}
		addConfigLocked(commonConfig.Data)

		if commonConfig.GetTrace() == nil {
			commonConfig.Trace = &conf.Tracer{}
		}
		addConfigLocked(commonConfig.Trace)

		if commonConfig.GetLogger() == nil {
			commonConfig.Logger = &conf.Logger{}
		}
		addConfigLocked(commonConfig.Logger)

		if commonConfig.GetRegistry() == nil {
			commonConfig.Registry = &conf.Registry{}
		}
		addConfigLocked(commonConfig.Registry)

		if commonConfig.GetOss() == nil {
			commonConfig.Oss = &conf.OSS{}
		}
		addConfigLocked(commonConfig.Oss)

		if commonConfig.GetNotify() == nil {
			commonConfig.Notify = &conf.Notification{}
		}
		addConfigLocked(commonConfig.Notify)

		if commonConfig.GetAuthn() == nil {
			commonConfig.Authn = &conf.Authentication{}
		}
		addConfigLocked(commonConfig.Authn)

		if commonConfig.GetAuthz() == nil {
			commonConfig.Authz = &conf.Authorization{}
		}
		addConfigLocked(commonConfig.Authz)
	})
}

// addConfigLocked 假定已持有 muBC 锁，添加时会去重并确保参数为指针
func addConfigLocked(c proto.Message) {
	if c == nil {
		return
	}
	v := reflect.ValueOf(c)
	if !v.IsValid() || v.Kind() != reflect.Ptr || v.IsNil() {
		// 只接受非 nil 的指针类型
		return
	}
	addr := v.Pointer()
	if _, exists := configSet[addr]; exists {
		return
	}
	configList = append(configList, c)
	configSet[addr] = struct{}{}
}
