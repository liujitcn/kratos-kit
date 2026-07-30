package sse

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"time"
)

type Event struct {
	timestamp time.Time
	ID        []byte
	Data      []byte
	Event     []byte
	Retry     []byte
	Comment   []byte
}

// EventMetaOption 设置 SSE 事件元数据。
type EventMetaOption func(event *Event)

// WithEventID 设置事件重连标识。
func WithEventID(id string) EventMetaOption {
	return func(event *Event) {
		if id != "" {
			event.ID = []byte(id)
		}
	}
}

// WithEventName 设置事件名称。
func WithEventName(name string) EventMetaOption {
	return func(event *Event) {
		if name != "" {
			event.Event = []byte(name)
		}
	}
}

// WithEventRetry 设置客户端重连间隔字段。
func WithEventRetry(retry string) EventMetaOption {
	return func(event *Event) {
		if retry != "" {
			event.Retry = []byte(retry)
		}
	}
}

// WithEventComment 设置事件注释。
func WithEventComment(comment string) EventMetaOption {
	return func(event *Event) {
		if comment != "" {
			event.Comment = []byte(comment)
		}
	}
}

func (e *Event) hasContent() bool {
	return len(e.ID) > 0 || len(e.Data) > 0 || len(e.Event) > 0 || len(e.Retry) > 0 || len(e.Comment) > 0
}

func (e *Event) encodeBase64() {
	dataLen := len(e.Data)
	if dataLen > 0 {
		output := make([]byte, base64.StdEncoding.EncodedLen(dataLen))
		base64.StdEncoding.Encode(output, e.Data)
		e.Data = output
	}
}

type EventStreamReader struct {
	scanner *bufio.Scanner
}

func NewEventStreamReader(eventStream io.Reader, maxBufferSize int) *EventStreamReader {
	scanner := bufio.NewScanner(eventStream)
	initBufferSize := minPosInt(4096, maxBufferSize)
	scanner.Buffer(make([]byte, initBufferSize), maxBufferSize)

	split := func(data []byte, atEOF bool) (int, []byte, error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		if i, nLen := containsDoubleNewline(data); i >= 0 {
			return i + nLen, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
	scanner.Split(split)

	return &EventStreamReader{
		scanner: scanner,
	}
}

func (e *EventStreamReader) ReadEvent() ([]byte, error) {
	if e.scanner.Scan() {
		event := e.scanner.Bytes()
		return event, nil
	}
	if err := e.scanner.Err(); err != nil {
		if err == context.Canceled {
			return nil, io.EOF
		}
		return nil, err
	}
	return nil, io.EOF
}
