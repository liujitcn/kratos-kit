package sse

import "sync"

type StreamMap map[StreamID]*Stream

type StreamManager struct {
	streams StreamMap
	mtx     sync.RWMutex
}

// NewStreamManager 创建并初始化事件流管理器。
func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(StreamMap),
	}
}

// Clean 关闭并移除全部事件流。
func (s *StreamManager) Clean() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, v := range s.streams {
		v.close()
	}
	s.streams = make(StreamMap)
}

// Count 返回当前事件流数量。
func (s *StreamManager) Count() int {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return len(s.streams)
}

// Get 返回指定事件流，不存在时返回 nil。
func (s *StreamManager) Get(streamId StreamID) *Stream {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	c, _ := s.streams[streamId]
	return c
}

// Exist 判断指定事件流是否存在。
func (s *StreamManager) Exist(streamId StreamID) bool {
	stream := s.Get(streamId)
	return stream != nil
}

// Range 遍历调用时刻的事件流快照。
func (s *StreamManager) Range(fn func(*Stream)) {
	s.mtx.RLock()
	streams := make([]*Stream, 0, len(s.streams))
	for _, stream := range s.streams {
		streams = append(streams, stream)
	}
	s.mtx.RUnlock()

	for _, stream := range streams {
		fn(stream)
	}
}

// Add 添加事件流；同名流已存在时关闭传入实例并返回原实例。
func (s *StreamManager) Add(stream *Stream) *Stream {
	if stream == nil {
		return nil
	}

	s.mtx.Lock()
	existing := s.streams[stream.StreamID()]
	if existing == nil {
		s.streams[stream.StreamID()] = stream
	}
	s.mtx.Unlock()

	if existing != nil {
		stream.close()
		return existing
	}
	return stream
}

// RemoveWithID 关闭并移除指定事件流。
func (s *StreamManager) RemoveWithID(streamId StreamID) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.streams[streamId] != nil {
		s.streams[streamId].close()
		delete(s.streams, streamId)
	}
}

// Remove 关闭并移除指定事件流实例。
func (s *StreamManager) Remove(stream *Stream) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for k, v := range s.streams {
		if stream == v {
			//LogInfo("remove stream: ", stream.StreamID())
			s.streams[k].close()
			delete(s.streams, k)
			return
		}
	}
}
