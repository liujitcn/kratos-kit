package gorm

import (
	"sync"

	"gorm.io/gorm"
)

var (
	registeredCallbackMu sync.RWMutex
	callbackCreates      []func(db *gorm.DB)
	callbackUpdates      []func(db *gorm.DB)
)

// RegisterCallbackCreate 注册注册钩子
func RegisterCallbackCreate(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackCreates = append(callbackCreates, fn)
}

// RegisterCallbackCreates 注册钩子
func RegisterCallbackCreates(fn ...func(g *gorm.DB)) {
	if len(fn) == 0 {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackCreates = append(callbackCreates, fn...)
}

// getCallbackCreates 返回已注册的创建钩子（线程安全）
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

// RegisterCallbackUpdate 注册注册钩子
func RegisterCallbackUpdate(fn func(g *gorm.DB)) {
	if fn == nil {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackUpdates = append(callbackUpdates, fn)
}

// RegisterCallbackUpdates 注册钩子
func RegisterCallbackUpdates(fn ...func(g *gorm.DB)) {
	if len(fn) == 0 {
		return
	}
	registeredCallbackMu.Lock()
	defer registeredCallbackMu.Unlock()
	callbackUpdates = append(callbackUpdates, fn...)
}

// getCallbackUpdates 返回已注册的创建钩子（线程安全）
func getCallbackUpdates() []func(g *gorm.DB) {
	registeredCallbackMu.RLock()
	defer registeredCallbackMu.RUnlock()
	if len(callbackUpdates) == 0 {
		return nil
	}
	dup := make([]func(g *gorm.DB), len(callbackUpdates))
	copy(dup, callbackUpdates)
	return dup
}
