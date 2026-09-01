package memory

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/liujitcn/kratos-kit/cache/store"
)

type strItem struct {
	Value     string
	Expired   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type mapItem struct {
	Value     map[string]string
	Expired   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
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

// List 返回内存缓存中的字符串和 Hash 条目及其元数据。
func (s *Memory) List() ([]store.Entry, error) {
	now := time.Now()
	entries := make([]store.Entry, 0)
	s.strMutex.Lock()
	for key, item := range s.strItems {
		if !item.Expired.IsZero() && now.After(item.Expired) {
			delete(s.strItems, key)
			continue
		}
		entries = append(entries, store.Entry{Key: key, Type: "string", Value: item.Value, TTL: ttl(item.Expired), ExpiresAt: item.Expired, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt})
	}
	s.strMutex.Unlock()
	s.mapMutex.Lock()
	for key, item := range s.mapItems {
		if !item.Expired.IsZero() && now.After(item.Expired) {
			delete(s.mapItems, key)
			continue
		}
		fields := make(map[string]string, len(item.Value))
		for field, value := range item.Value {
			fields[field] = value
		}
		entries = append(entries, store.Entry{Key: key, Type: "hash", Fields: fields, TTL: ttl(item.Expired), ExpiresAt: item.Expired, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt})
	}
	s.mapMutex.Unlock()
	return entries, nil
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
	s.strMutex.Lock()
	defer s.strMutex.Unlock()

	item, ok := s.strItems[key]
	if !ok {
		return "", errors.New("key not found")
	}
	if !item.Expired.IsZero() && time.Now().After(item.Expired) {
		delete(s.strItems, key)
		return "", errors.New("key expired")
	}
	return item.Value, nil
}

// Incr 原子递增内存缓存中的数值键。
func (s *Memory) Incr(key string) (int64, error) {
	s.strMutex.Lock()
	defer s.strMutex.Unlock()
	item, ok := s.strItems[key]
	if ok && !item.Expired.IsZero() && time.Now().After(item.Expired) {
		delete(s.strItems, key)
		ok = false
	}
	var value int64
	var err error
	if ok {
		value, err = strconv.ParseInt(item.Value, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	value++
	now := time.Now()
	if !ok {
		item = &strItem{CreatedAt: now}
	}
	item.Value = strconv.FormatInt(value, 10)
	item.UpdatedAt = now
	item.Expired = time.Time{}
	s.strItems[key] = item
	return value, nil
}

// GetDel 原子读取并删除缓存值。
func (s *Memory) GetDel(key string) (string, error) {
	s.strMutex.Lock()
	defer s.strMutex.Unlock()

	item, ok := s.strItems[key]
	if !ok {
		return "", errors.New("key not found")
	}
	delete(s.strItems, key)
	if !item.Expired.IsZero() && time.Now().After(item.Expired) {
		return "", errors.New("key expired")
	}
	return item.Value, nil
}

func (s *Memory) Set(key, value string, expire time.Duration) error {
	s.strMutex.Lock()
	defer s.strMutex.Unlock()

	now := time.Now()
	item := &strItem{Value: value, Expired: expiration(expire), CreatedAt: now, UpdatedAt: now}
	if previous, ok := s.strItems[key]; ok {
		if previous.Expired.IsZero() || now.Before(previous.Expired) {
			item.CreatedAt = previous.CreatedAt
		}
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
	item.Expired = expiration(dur)
	item.UpdatedAt = time.Now()

	s.strItems[key] = item
	return nil
}

func (s *Memory) Exists(key string) bool {
	s.strMutex.Lock()
	defer s.strMutex.Unlock()

	item, ok := s.strItems[key]
	if !ok {
		return false
	}

	if !item.Expired.IsZero() && time.Now().After(item.Expired) {
		delete(s.strItems, key)
		return false
	}
	return true
}

func (s *Memory) HGetAll(key string) (map[string]string, error) {
	s.mapMutex.Lock()
	defer s.mapMutex.Unlock()

	item, ok := s.mapItems[key]
	if ok && !item.Expired.IsZero() && time.Now().After(item.Expired) {
		delete(s.mapItems, key)
		return nil, errors.New("key expired")
	}
	if !ok {
		return nil, errors.New("key not found")
	}
	return item.Value, nil
}

func (s *Memory) HGet(key, field string) (string, error) {
	s.mapMutex.Lock()
	defer s.mapMutex.Unlock()

	item, ok := s.mapItems[key]
	if ok && !item.Expired.IsZero() && time.Now().After(item.Expired) {
		delete(s.mapItems, key)
		return "", errors.New("key expired")
	}
	if !ok {
		return "", errors.New("key not found")
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
	if ok && !item.Expired.IsZero() && time.Now().After(item.Expired) {
		delete(s.mapItems, key)
		ok = false
	}
	if !ok {
		now := time.Now()
		item = &mapItem{
			Value:     make(map[string]string),
			CreatedAt: now,
		}
	}

	item.Value[field] = value
	item.UpdatedAt = time.Now()

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
	if !item.Expired.IsZero() && time.Now().After(item.Expired) {
		return errors.New("key expired")
	}

	delete(item.Value, field)
	item.UpdatedAt = time.Now()

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
	if !item.Expired.IsZero() && time.Now().After(item.Expired) {
		return errors.New("key expired")
	}

	_, ok = item.Value[field]
	if !ok {
		return errors.New("field not found")
	}
	return nil
}

func expiration(expire time.Duration) time.Time {
	if expire <= 0 {
		return time.Time{}
	}
	return time.Now().Add(expire)
}

func ttl(expired time.Time) time.Duration {
	if expired.IsZero() {
		return -1
	}
	return time.Until(expired)
}
