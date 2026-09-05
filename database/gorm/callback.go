package gorm

import (
	"sync"

	"gorm.io/gorm"
)

var (
	registeredCallbackMu sync.RWMutex
	callbackQueries      []func(db *gorm.DB)
	callbackQueryAfters  []func(db *gorm.DB)
	callbackRows         []func(db *gorm.DB)
	callbackRaws         []func(db *gorm.DB)
	callbackCreates      []func(db *gorm.DB)
	callbackCreateAfters []func(db *gorm.DB)
	callbackUpdates      []callbackUpdate
	callbackUpdateAfters []func(db *gorm.DB)
	callbackDeletes      []func(db *gorm.DB)
	callbackDeleteAfters []func(db *gorm.DB)
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

// RegisterCallbackQueryAfter 注册查询完成后的钩子。
func RegisterCallbackQueryAfter(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackQueryAfters = append(callbackQueryAfters, fn)
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

// RegisterCallbackCreateAfter 注册创建完成后的钩子。
func RegisterCallbackCreateAfter(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackCreateAfters = append(callbackCreateAfters, fn)
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

// RegisterCallbackUpdateAfter 注册更新完成后的钩子。
func RegisterCallbackUpdateAfter(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackUpdateAfters = append(callbackUpdateAfters, fn)
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

// RegisterCallbackDeleteAfter 注册删除完成后的钩子。
func RegisterCallbackDeleteAfter(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackDeleteAfters = append(callbackDeleteAfters, fn)
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

// getCallbackQueryAfters 返回已注册的查询完成钩子副本。
func getCallbackQueryAfters() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackQueryAfters) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackQueryAfters))
	copy(dup, callbackQueryAfters)
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

// getCallbackCreateAfters 返回已注册的创建完成钩子副本。
func getCallbackCreateAfters() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackCreateAfters) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackCreateAfters))
	copy(dup, callbackCreateAfters)
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

// getCallbackUpdateAfters 返回已注册的更新完成钩子副本。
func getCallbackUpdateAfters() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackUpdateAfters) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackUpdateAfters))
	copy(dup, callbackUpdateAfters)
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

// getCallbackDeleteAfters 返回已注册的删除完成钩子副本。
func getCallbackDeleteAfters() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackDeleteAfters) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackDeleteAfters))
	copy(dup, callbackDeleteAfters)
	return dup
}
