package runtime

import (
	"sync"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/liujitcn/go-utils/id"
	"github.com/liujitcn/kratos-kit/cache"
	"github.com/liujitcn/kratos-kit/database/gorm"
	"github.com/liujitcn/kratos-kit/locker"
	"github.com/liujitcn/kratos-kit/oss"
	"github.com/liujitcn/kratos-kit/queue"
	queueData "github.com/liujitcn/kratos-kit/queue/data"
)

type Application struct {
	snowflake *id.Snowflake
	anyMap    map[string]any

	gormClient *gorm.Client
	cache      cache.Cache
	oss        oss.OSS
	locker     locker.Locker
	queue      queue.Queue

	mux sync.RWMutex
}

// NewRuntime 默认值
func NewRuntime() Runtime {
	sf, err := id.NewSnowflake()
	if err != nil {
		log.Error("NewRuntime err:", err)
	}
	return &Application{
		snowflake: sf,
		anyMap:    make(map[string]any),
	}
}

func (e *Application) GetSnowflake() *id.Snowflake {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.snowflake
}

func (e *Application) SetInterface(s string, a any) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.anyMap[s] = a
}

func (e *Application) GetInterface(s string) any {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.anyMap[s]
}

func (e *Application) SetGormClient(gormClient *gorm.Client) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.gormClient = gormClient
}

func (e *Application) GetGormClient() *gorm.Client {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.gormClient
}

// SetCache 设置缓存
func (e *Application) SetCache(c cache.Cache) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.cache = c
}

// GetCache 获取缓存
func (e *Application) GetCache() cache.Cache {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.cache
}

func (e *Application) SetOSS(oss oss.OSS) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.oss = oss
}

func (e *Application) GetOSS() oss.OSS {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.oss
}

func (e *Application) SetLocker(locker locker.Locker) {
	e.mux.Lock()
	defer e.mux.Unlock()
	e.locker = locker
}

func (e *Application) GetLocker() locker.Locker {
	e.mux.Lock()
	defer e.mux.Unlock()
	return e.locker
}

// SetQueue 设置队列适配器
func (e *Application) SetQueue(c queue.Queue) {
	e.queue = c
}

// GetQueue 获取队列适配器
func (e *Application) GetQueue() queue.Queue {
	return e.queue
}

// GetStreamMessage 获取队列需要用的message
func (e *Application) GetStreamMessage(id string, value map[string]interface{}) (queueData.Message, error) {
	return queueData.Message{
		ID:         id,
		Values:     value,
		ErrorCount: 0,
	}, nil
}
