package runtime

import (
	"github.com/liujitcn/kratos-kit/cache"
	"github.com/liujitcn/kratos-kit/database/gorm"
	"github.com/liujitcn/kratos-kit/locker"
	"github.com/liujitcn/kratos-kit/oss"
	"github.com/liujitcn/kratos-kit/queue"
	queueData "github.com/liujitcn/kratos-kit/queue/data"
)

type Runtime interface {
	SetInterface(string, any)
	GetInterface(string) any

	SetGormClient(client *gorm.Client)
	GetGormClient() *gorm.Client

	SetCache(cache.Cache)
	GetCache() cache.Cache

	SetOSS(oss.OSS)
	GetOSS() oss.OSS

	SetLocker(locker.Locker)
	GetLocker() locker.Locker

	SetQueue(queue.Queue)
	GetQueue() queue.Queue

	GetStreamMessage(id string, value map[string]interface{}) (queueData.Message, error)
}
