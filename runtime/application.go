package runtime

import (
	"sync"

	utilsTranslator "github.com/liujitcn/go-utils/translator"
	"github.com/liujitcn/kratos-kit/cache"
	"github.com/liujitcn/kratos-kit/database/gorm"
	"github.com/liujitcn/kratos-kit/locker"
	"github.com/liujitcn/kratos-kit/oss"
	"github.com/liujitcn/kratos-kit/queue"
	queueData "github.com/liujitcn/kratos-kit/queue/data"
)

// Application 保存业务系统共享的基础设施实例。
type Application struct {
	anyMap map[string]any

	gormClients map[string]*gorm.Client
	cache       cache.Cache
	oss         oss.OSS
	locker      locker.Locker
	queue       queue.Queue
	translator  utilsTranslator.Translator

	mux sync.RWMutex
}

// NewRuntime 创建空的共享运行时。
func NewRuntime() Runtime {
	return &Application{
		anyMap:      make(map[string]any),
		gormClients: make(map[string]*gorm.Client),
	}
}

// SetInterface 按名称保存自定义实例。
func (e *Application) SetInterface(s string, a any) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.anyMap[s] = a
}

// GetInterface 按名称获取自定义实例。
func (e *Application) GetInterface(s string) any {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.anyMap[s]
}

// SetGormClients 设置数据库客户端集合。
func (e *Application) SetGormClients(gormClients map[string]*gorm.Client) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.gormClients = gormClients
}

// GetGormClients 获取数据库客户端集合。
func (e *Application) GetGormClients() map[string]*gorm.Client {
	e.mux.RLock()
	defer e.mux.RUnlock()
	clients := make(map[string]*gorm.Client, len(e.gormClients))
	for name, client := range e.gormClients {
		clients[name] = client
	}
	return clients
}

// GetDefaultGormClient 获取默认数据库客户端。
func (e *Application) GetDefaultGormClient() *gorm.Client {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return e.gormClients[gorm.DefaultClientName]
}

// GetGormClient 按名称获取数据库客户端，未找到时回退到默认客户端。
func (e *Application) GetGormClient(name string) *gorm.Client {
	e.mux.RLock()
	defer e.mux.RUnlock()
	client, ok := e.gormClients[name]
	if !ok || client == nil {
		return e.gormClients[gorm.DefaultClientName]
	}
	return client
}

// SetCache 设置缓存实例。
func (e *Application) SetCache(c cache.Cache) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.cache = c
}

// GetCache 获取缓存实例。
func (e *Application) GetCache() cache.Cache {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.cache
}

// SetOSS 设置对象存储实例。
func (e *Application) SetOSS(oss oss.OSS) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.oss = oss
}

// GetOSS 获取对象存储实例。
func (e *Application) GetOSS() oss.OSS {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.oss
}

// SetLocker 设置分布式锁实例。
func (e *Application) SetLocker(locker locker.Locker) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.locker = locker
}

// GetLocker 获取分布式锁实例。
func (e *Application) GetLocker() locker.Locker {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.locker
}

// SetQueue 设置队列实例。
func (e *Application) SetQueue(c queue.Queue) {
	e.queue = c
}

// GetQueue 获取队列实例。
func (e *Application) GetQueue() queue.Queue {
	return e.queue
}

// SetTranslator 设置翻译器。
func (e *Application) SetTranslator(translatorValue utilsTranslator.Translator) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.translator = translatorValue
}

// GetTranslator 获取翻译器。
func (e *Application) GetTranslator() utilsTranslator.Translator {
	e.mux.RLock()
	defer e.mux.RUnlock()
	return e.translator
}

// GetStreamMessage 创建队列流消息。
func (e *Application) GetStreamMessage(id string, value map[string]interface{}) (queueData.Message, error) {
	return queueData.Message{
		ID:         id,
		Values:     value,
		ErrorCount: 0,
	}, nil
}
