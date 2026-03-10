package memory

import (
	"errors"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type strItem struct {
	Value   string
	Expired time.Time
}

type mapItem struct {
	Value   map[string]string
	Expired time.Time
}

type Memory struct {
	strItems map[string]*strItem
	strMutex sync.RWMutex
	mapItems map[string]*mapItem
	mapMutex sync.RWMutex
}

// NewMemory memory模式
func NewMemory() (*Memory, func(), error) {
	return &Memory{
			strItems: make(map[string]*strItem),
			mapItems: make(map[string]*mapItem),
		}, func() {
			log.Info("cache memory cleanup...")
		}, nil
}

func (s *Memory) Connect() error {
	if s.strItems == nil || s.mapItems == nil {
		return errors.New("memory connect fail")
	}
	return nil
}

func (s *Memory) DisConnect() error {
	s.strItems = nil
	s.mapItems = nil
	return nil
}

func (s *Memory) Get(key string) (string, error) {
	s.strMutex.RLock()
	defer s.strMutex.RUnlock()

	item, ok := s.strItems[key]
	if !ok {
		return "", errors.New("key not found")
	}
	if time.Now().After(item.Expired) {
		delete(s.strItems, key)
		return "", errors.New("key expired")
	}
	return item.Value, nil
}

func (s *Memory) Set(key, value string, expire time.Duration) error {
	s.strMutex.Lock()
	defer s.strMutex.Unlock()

	item := &strItem{
		Value:   value,
		Expired: time.Now().Add(expire),
	}

	s.strItems[key] = item

	return nil
}

func (s *Memory) Del(key string) error {
	s.strMutex.Lock()
	defer s.strMutex.Unlock()

	delete(s.strItems, key)
	return nil
}

func (s *Memory) Expire(key string, dur time.Duration) error {
	s.strMutex.Lock()
	defer s.strMutex.Unlock()

	item, ok := s.strItems[key]
	if !ok {
		return errors.New("key not found")
	}
	item.Expired = time.Now().Add(dur)

	s.strItems[key] = item
	return nil
}

func (s *Memory) Exists(key string) bool {
	s.strMutex.RLock()
	defer s.strMutex.RUnlock()

	item, ok := s.strItems[key]
	if !ok {
		return false
	}

	if time.Now().After(item.Expired) {
		delete(s.strItems, key)
		return false
	}
	return true
}

func (s *Memory) HGetAll(key string) (map[string]string, error) {
	s.mapMutex.RLock()
	defer s.mapMutex.RUnlock()

	item, ok := s.mapItems[key]
	if !ok {
		return nil, errors.New("key not found")
	}
	if time.Now().After(item.Expired) {
		return nil, errors.New("key expired")
	}
	return item.Value, nil
}

func (s *Memory) HGet(key, field string) (string, error) {
	s.mapMutex.RLock()
	defer s.mapMutex.RUnlock()

	item, ok := s.mapItems[key]
	if !ok {
		return "", errors.New("key not found")
	}
	if time.Now().After(item.Expired) {
		return "", errors.New("key expired")
	}
	var itemValue string
	itemValue, ok = item.Value[field]
	if !ok {
		return "", errors.New("field not found")
	}
	return itemValue, nil
}

func (s *Memory) HSet(key, field, value string) error {
	s.mapMutex.Lock()
	defer s.mapMutex.Unlock()

	item, ok := s.mapItems[key]
	if !ok {
		item = &mapItem{
			Value:   make(map[string]string),
			Expired: time.Now().AddDate(100, 0, 0),
		}
	}

	item.Value[field] = value

	s.mapItems[key] = item
	return nil
}

func (s *Memory) HDel(key, field string) error {
	s.mapMutex.Lock()
	defer s.mapMutex.Unlock()

	item, ok := s.mapItems[key]
	if !ok {
		return errors.New("key not found")
	}
	if time.Now().After(item.Expired) {
		return errors.New("key expired")
	}

	delete(item.Value, field)

	s.mapItems[key] = item
	return nil
}

func (s *Memory) HExists(key, field string) error {
	s.mapMutex.RLock()
	defer s.mapMutex.RUnlock()

	item, ok := s.mapItems[key]
	if !ok {
		return errors.New("key not found")
	}
	if time.Now().After(item.Expired) {
		return errors.New("key expired")
	}

	_, ok = item.Value[field]
	if !ok {
		return errors.New("field not found")
	}
	return nil
}
