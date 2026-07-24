package gorm

import (
	"sync"

	"gorm.io/gorm"
)

var (
	registeredCallbackMu sync.RWMutex
	callbackQueries      []func(db *gorm.DB)
	callbackRows         []func(db *gorm.DB)
	callbackRaws         []func(db *gorm.DB)
	callbackCreates      []func(db *gorm.DB)
	callbackUpdates      []callbackUpdate
	callbackDeletes      []func(db *gorm.DB)
)

type callbackUpdate struct {
	anchor string
	fn     func(db *gorm.DB)
}

// RegisterCallbackQuery 注册查询钩子。
func RegisterCallbackQuery(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackQueries = append(callbackQueries, fn)
}

// RegisterCallbackQueries 批量注册查询钩子。
func RegisterCallbackQueries(fn ...func(g *gorm.DB)) {
	if len(fn) == 0 {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackQueries = append(callbackQueries, fn...)
}

// RegisterCallbackRow 注册单行和流式查询钩子。
func RegisterCallbackRow(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackRows = append(callbackRows, fn)
}

// RegisterCallbackRaw 注册原生 SQL 钩子。
func RegisterCallbackRaw(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackRaws = append(callbackRaws, fn)
}

// RegisterCallbackCreate 注册创建钩子。
func RegisterCallbackCreate(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackCreates = append(callbackCreates, fn)
}

// RegisterCallbackCreates 批量注册创建钩子。
func RegisterCallbackCreates(fn ...func(g *gorm.DB)) {
	if len(fn) == 0 {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackCreates = append(callbackCreates, fn...)
}

// RegisterCallbackUpdate 注册更新钩子。
func RegisterCallbackUpdate(fn func(g *gorm.DB)) {
	RegisterCallbackUpdateBefore("gorm:before_update", fn)
}

// RegisterCallbackUpdateBefore 注册指定更新节点前执行的钩子。
func RegisterCallbackUpdateBefore(anchor string, fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	if anchor == "" {
		anchor = "gorm:before_update"
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackUpdates = append(callbackUpdates, callbackUpdate{anchor: anchor, fn: fn})
}

// RegisterCallbackUpdates 批量注册更新钩子。
func RegisterCallbackUpdates(fn ...func(g *gorm.DB)) {
	if len(fn) == 0 {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	for _, item := range fn {
		if item != nil {
			callbackUpdates = append(callbackUpdates, callbackUpdate{anchor: "gorm:before_update", fn: item})
		}
	}
}

// RegisterCallbackDelete 注册删除钩子。
func RegisterCallbackDelete(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackDeletes = append(callbackDeletes, fn)
}

// RegisterCallbackDeletes 批量注册删除钩子。
func RegisterCallbackDeletes(fn ...func(g *gorm.DB)) {
	if len(fn) == 0 {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackDeletes = append(callbackDeletes, fn...)
}

// getCallbackQueries 返回已注册的查询钩子副本。
func getCallbackQueries() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackQueries) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackQueries))
	copy(dup, callbackQueries)
	return dup
}

// getCallbackRows 返回已注册的单行和流式查询钩子副本。
func getCallbackRows() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackRows) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackRows))
	copy(dup, callbackRows)
	return dup
}

// getCallbackRaws 返回已注册的原生 SQL 钩子副本。
func getCallbackRaws() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackRaws) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackRaws))
	copy(dup, callbackRaws)
	return dup
}

// getCallbackCreates 返回已注册的创建钩子副本。
func getCallbackCreates() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackCreates) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackCreates))
	copy(dup, callbackCreates)
	return dup
}

// getCallbackUpdates 返回已注册的更新钩子副本。
func getCallbackUpdates() []callbackUpdate {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackUpdates) == 0 {
		return nil
	}
	dup := make([]callbackUpdate, len(callbackUpdates))
	copy(dup, callbackUpdates)
	return dup
}

// getCallbackDeletes 返回已注册的删除钩子副本。
func getCallbackDeletes() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackDeletes) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackDeletes))
	copy(dup, callbackDeletes)
	return dup
}
