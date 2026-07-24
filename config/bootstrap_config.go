package config

import (
	"reflect"
	"sync"

	"google.golang.org/protobuf/proto"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

var (
	muBC         sync.RWMutex
	initOnce     sync.Once
	configList   []proto.Message
	configSet    map[uintptr]struct{}
	commonConfig *configv1.Bootstrap
)

func GetBootstrapConfig() *configv1.Bootstrap {
	initBootstrapConfig()
	muBC.RLock()
	defer muBC.RUnlock()
	return commonConfig
}

// RegisterConfig 注册配置（去重、并发安全）
// 传入值应为指针类型，例如 &configv1.SomeConfig{}
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
			commonConfig = &configv1.Bootstrap{}
		}

		// 按需添加根与子配置，使用去重函数
		addConfigLocked(commonConfig)

		if commonConfig.GetServer() == nil {
			commonConfig.Server = &configv1.Server{}
		}
		addConfigLocked(commonConfig.Server)

		if commonConfig.GetClient() == nil {
			commonConfig.Client = &configv1.Client{}
		}
		addConfigLocked(commonConfig.Client)

		if commonConfig.GetData() == nil {
			commonConfig.Data = &configv1.Data{}
		}
		addConfigLocked(commonConfig.Data)

		if commonConfig.GetTrace() == nil {
			commonConfig.Trace = &configv1.Tracer{}
		}
		addConfigLocked(commonConfig.Trace)

		if commonConfig.GetLogger() == nil {
			commonConfig.Logger = &configv1.Logger{}
		}
		addConfigLocked(commonConfig.Logger)

		if commonConfig.GetRegistry() == nil {
			commonConfig.Registry = &configv1.Registry{}
		}
		addConfigLocked(commonConfig.Registry)

		if commonConfig.GetOss() == nil {
			commonConfig.Oss = &configv1.Oss{}
		}
		addConfigLocked(commonConfig.Oss)

		if commonConfig.GetNotify() == nil {
			commonConfig.Notify = &configv1.Notification{}
		}
		addConfigLocked(commonConfig.Notify)

		if commonConfig.GetAuthn() == nil {
			commonConfig.Authn = &configv1.Authentication{}
		}
		addConfigLocked(commonConfig.Authn)

		if commonConfig.GetAuthz() == nil {
			commonConfig.Authz = &configv1.Authorization{}
		}
		addConfigLocked(commonConfig.Authz)

		if commonConfig.GetOauth() == nil {
			commonConfig.Oauth = &configv1.OAuth{}
		}
		addConfigLocked(commonConfig.Oauth)
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
