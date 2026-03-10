package memory

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/liujitcn/kratos-kit/queue/data"
)

type queueChan chan data.Message

type Memory struct {
	queue   *sync.Map
	wait    sync.WaitGroup
	mutex   sync.RWMutex
	PoolNum int64
}

// NewMemory 内存模式
func NewMemory(poolNum int64) *Memory {
	return &Memory{
		queue:   new(sync.Map),
		PoolNum: poolNum,
	}
}

func (s *Memory) Append(stream string, message data.Message) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	v, ok := s.queue.Load(stream)

	if !ok {
		v = s.makeQueue()
		s.queue.Store(stream, v)
	}

	var q queueChan
	switch v.(type) {
	case queueChan:
		q = v.(queueChan)
	default:
		q = s.makeQueue()
		s.queue.Store(stream, q)
	}
	go func(gm data.Message, gq queueChan) {
		if len(gm.ID) == 0 {
			gm.ID = uuid.New().String()
		}
		gq <- gm
	}(message, q)
	return nil
}

func (s *Memory) Register(name string, fn data.ConsumerFunc) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	v, ok := s.queue.Load(name)
	if !ok {
		v = s.makeQueue()
		s.queue.Store(name, v)
	}
	var q queueChan
	switch v.(type) {
	case queueChan:
		q = v.(queueChan)
	default:
		q = s.makeQueue()
		s.queue.Store(name, q)
	}
	go func(out queueChan, gf data.ConsumerFunc) {
		var err error
		for message := range q {
			err = gf(message)
			if err != nil {
				if message.ErrorCount < 3 {
					message.ErrorCount = message.ErrorCount + 1
					// 每次间隔时长放大
					i := time.Second * time.Duration(message.ErrorCount)
					time.Sleep(i)
					out <- message
				}
				err = nil
			}
		}
	}(q, fn)
}

func (s *Memory) Run() {
	s.wait.Add(1)
	s.wait.Wait()
}

func (s *Memory) Shutdown() {
	s.wait.Done()
}

func (s *Memory) makeQueue() queueChan {
	if s.PoolNum <= 0 {
		return make(queueChan)
	}
	return make(queueChan, s.PoolNum)
}
