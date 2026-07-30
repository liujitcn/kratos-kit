package cache

import "github.com/liujitcn/kratos-kit/cache/store"

// ErrNotFound 表示缓存键不存在或已经过期。
var ErrNotFound = store.ErrNotFound

// Item 是现代缓存接口的批量写入项。
type Item = store.Item

// Store 是支持 context、GetDel、SetNX 和批量操作的缓存接口。
type Store = store.Store
