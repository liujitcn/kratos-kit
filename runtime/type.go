package runtime

import (
	"github.com/liujitcn/go-utils/translator"
	"github.com/liujitcn/kratos-kit/cache"
	"github.com/liujitcn/kratos-kit/database/gorm"
	"github.com/liujitcn/kratos-kit/locker"
	"github.com/liujitcn/kratos-kit/oss"
	"github.com/liujitcn/kratos-kit/queue"
	"github.com/liujitcn/kratos-kit/queue/data"
)

// Runtime 定义业务系统共享基础设施实例的存取能力。
type Runtime interface {
	// SetInterface 按名称保存自定义实例。
	SetInterface(string, any)
	// GetInterface 按名称获取自定义实例。
	GetInterface(string) any

	// SetGormClients 设置数据库客户端集合。
	SetGormClients(clients map[string]*gorm.Client)
	// GetGormClients 获取数据库客户端集合。
	GetGormClients() map[string]*gorm.Client
	// GetDefaultGormClient 获取默认数据库客户端。
	GetDefaultGormClient() *gorm.Client
	// GetGormClient 按名称获取数据库客户端，未找到时回退到默认客户端。
	GetGormClient(name string) *gorm.Client

	// SetCache 设置缓存实例。
	SetCache(cache.Cache)
	// GetCache 获取缓存实例。
	GetCache() cache.Cache

	// SetOSS 设置对象存储实例。
	SetOSS(oss.OSS)
	// GetOSS 获取对象存储实例。
	GetOSS() oss.OSS

	// SetLocker 设置分布式锁实例。
	SetLocker(locker.Locker)
	// GetLocker 获取分布式锁实例。
	GetLocker() locker.Locker

	// SetQueue 设置队列实例。
	SetQueue(queue.Queue)
	// GetQueue 获取队列实例。
	GetQueue() queue.Queue

	// SetTranslator 设置翻译器。
	SetTranslator(translator.Translator)
	// GetTranslator 获取翻译器。
	GetTranslator() translator.Translator

	// GetStreamMessage 创建队列流消息。
	GetStreamMessage(id string, value map[string]interface{}) (data.Message, error)
}
