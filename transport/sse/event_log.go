package sse

import (
	"strconv"
	"time"
)

type EventLog []*Event

// Add 将有效事件写入重放日志，并在调用方未指定时生成事件标识。
func (e *EventLog) Add(ev *Event) {
	if !ev.hasContent() {
		return
	}

	if len(ev.ID) == 0 {
		ev.ID = []byte(e.currentIndex())
	}
	ev.timestamp = time.Now()
	*e = append(*e, ev)
}

// Clear 清空事件重放日志。
func (e *EventLog) Clear() {
	*e = nil
}

// Replay 从订阅者最后收到的事件之后开始重放。
func (e *EventLog) Replay(s *Subscriber) {
	start := 0
	if s.eventId != "" {
		start = len(*e)
		for i := range *e {
			if string((*e)[i].ID) == s.eventId {
				start = i + 1
				break
			}
		}
	}
	for i := start; i < len(*e); i++ {
		s.connection <- (*e)[i]
	}
}

// currentIndex 返回大于现有数值标识的下一个自动事件标识。
func (e *EventLog) currentIndex() string {
	next := len(*e) + 1
	var err error
	for _, event := range *e {
		var eventID int
		eventID, err = strconv.Atoi(string(event.ID))
		if err == nil && eventID >= next {
			next = eventID + 1
		}
	}
	return strconv.Itoa(next)
}
