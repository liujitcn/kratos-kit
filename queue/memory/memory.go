package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"uuid"

	"github.com/liujitcn/kratos-kit/queue/data"
	"github.com/liujitcn/kratos-kit/transport/hptimer"
)

type queueChan chan data.Message

type Memory struct {
	queue     *sync.Map
	wait      sync.WaitGroup
	mutex     sync.RWMutex
	PoolNum   int64
	running   atomic.Bool
	closing   chan struct{}
	closeOnce sync.Once
	consumers sync.WaitGroup
	delayed   *hptimer.HighPrecisionTimer
	plans     sync.Map
}

// NewMemory 内存模式
func NewMemory(poolNum int64) *Memory {
	memory := &Memory{
		queue:   new(sync.Map),
		PoolNum: poolNum,
		closing: make(chan struct{}),
	}
	memory.delayed = hptimer.NewHighPrecisionTimer(nil)
	memory.delayed.Start()
	return memory
}

// Append 追加消息到指定内存队列。
func (s *Memory) Append(stream string, message data.Message) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.isClosed() {
		return errors.New("queue is shut down")
	}

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
			gm.ID = uuid.NewV4().String()
		}
		select {
		case gq <- gm:
		case <-s.closing:
		}
	}(message, q)
	return nil
}

// Schedule 按指定时间向内存队列投递消息；相同流和消息编号会覆盖原计划。
func (s *Memory) Schedule(stream string, message data.Message, executeAt time.Time) error {
	if stream == "" {
		return errors.New("queue stream is empty")
	}
	if message.ID == "" {
		return errors.New("delayed message id is empty")
	}
	if s.isClosed() {
		return errors.New("queue is shut down")
	}
	taskID := hptimer.TimerTaskID(delayedTaskID(stream, message.ID))
	if _, exists := s.plans.Load(taskID); exists {
		_ = s.delayed.RemoveTask(taskID)
	}
	added := s.delayed.AddTask(hptimer.NewTimerTask(taskID, executeAt,
		hptimer.WithCallback(func(_ context.Context) error {
			s.plans.Delete(taskID)
			return s.Append(stream, message)
		}),
	))
	if added == "" {
		return fmt.Errorf("schedule delayed message failed: %s", taskID)
	}
	s.plans.Store(taskID, struct{}{})
	return nil
}

// Cancel 取消指定流中尚未触发的内存延迟消息。
func (s *Memory) Cancel(stream string, messageID string) error {
	if stream == "" || messageID == "" {
		return nil
	}
	taskID := hptimer.TimerTaskID(delayedTaskID(stream, messageID))
	if _, exists := s.plans.LoadAndDelete(taskID); exists {
		_ = s.delayed.RemoveTask(taskID)
	}
	return nil
}

// Register 注册指定队列的消费处理函数。
func (s *Memory) Register(name string, fn data.ConsumerFunc) {
	if fn == nil {
		return
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.isClosed() {
		return
	}
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
	s.consumers.Add(1)
	go s.consume(q, fn)
}

// Run 启动内存队列阻塞等待，直到收到关闭信号。
func (s *Memory) Run() {
	s.mutex.Lock()
	if s.isClosed() || !s.running.CompareAndSwap(false, true) {
		s.mutex.Unlock()
		return
	}
	s.wait.Add(1)
	s.mutex.Unlock()
	// 仅在首次启动时增加等待计数，避免重复 Run 导致 Shutdown 次数无法匹配。
	s.wait.Wait()
	s.consumers.Wait()
}

// Shutdown 关闭内存队列阻塞等待。
func (s *Memory) Shutdown() {
	s.mutex.Lock()
	s.closeOnce.Do(func() { close(s.closing) })
	s.mutex.Unlock()
	if s.delayed != nil {
		s.delayed.Stop()
	}
	// 只有在 Run 已经成功进入等待态时才允许 Done，避免出现负数计数 panic。
	if !s.running.CompareAndSwap(true, false) {
		return
	}
	s.wait.Done()
}

// delayedTaskID 生成内存延迟任务的进程内稳定编号。
func delayedTaskID(stream string, messageID string) string {
	return stream + "\x00" + messageID
}

// Wait 等待已注册的内存队列消费者退出。
func (s *Memory) Wait() {
	s.consumers.Wait()
}

// consume 执行消费者并在队列关闭或重试等待取消时退出。
func (s *Memory) consume(q queueChan, consumer data.ConsumerFunc) {
	defer s.consumers.Done()
	for {
		select {
		case <-s.closing:
			return
		case message := <-q:
			err := consumer(message)
			if err == nil || message.ErrorCount >= 3 {
				continue
			}
			message.ErrorCount++
			timer := time.NewTimer(time.Second * time.Duration(message.ErrorCount))
			select {
			case <-s.closing:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-timer.C:
				select {
				case q <- message:
				case <-s.closing:
					return
				}
			}
		}
	}
}

// isClosed 判断内存队列是否已进入停止状态。
func (s *Memory) isClosed() bool {
	select {
	case <-s.closing:
		return true
	default:
		return false
	}
}

// makeQueue 创建内存消息通道。
func (s *Memory) makeQueue() queueChan {
	if s.PoolNum <= 0 {
		return make(queueChan)
	}
	return make(queueChan, s.PoolNum)
}
