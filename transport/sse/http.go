package sse

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) prepareHeaderForSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	for k, v := range s.headers {
		w.Header().Set(k, v)
	}
}

// ServeHTTP 从请求中解析 stream ID 并输出对应的 SSE 事件流。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	streamID, err := s.resolveStreamID(r)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if streamID == "" {
		writeError(w, "Please specify a stream!", http.StatusInternalServerError)
		return
	}

	s.ServeStreamHTTP(w, r, StreamID(streamID))
}

// ServeStreamHTTP 直接使用给定的 stream ID 输出 SSE 事件流。
func (s *Server) ServeStreamHTTP(w http.ResponseWriter, r *http.Request, streamID StreamID) {
	var err error
	var statusCode int
	statusCode, err = s.serveStreamHTTP(w, r, streamID)
	if err != nil {
		writeError(w, err.Error(), statusCode)
	}
}

// serveStreamHTTP 执行单条 stream 的 SSE 输出流程。
func (s *Server) serveStreamHTTP(w http.ResponseWriter, r *http.Request, streamID StreamID) (int, error) {
	flusher, exist := w.(http.Flusher)
	if !exist {
		return http.StatusInternalServerError, fmt.Errorf("Streaming unsupported!")
	}

	if streamID == "" {
		return http.StatusInternalServerError, fmt.Errorf("Please specify a stream!")
	}

	s.prepareHeaderForSSE(w)

	stream := s.streamMgr.Get(streamID)
	if stream == nil {
		if !s.autoStream {
			return http.StatusInternalServerError, fmt.Errorf("Stream not found!")
		}

		stream = s.CreateStream(streamID)
	}

	eventId := 0
	if id := r.Header.Get("Last-Event-ID"); id != "" {
		var err error
		eventId, err = strconv.Atoi(id)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("Last-Event-ID must be a number!")
		}
	}

	sub := stream.addSubscriber(eventId, r.URL)

	go func() {
		<-r.Context().Done()

		sub.close()

		if s.autoStream && !s.autoReplay && stream.getSubscriberCount() == 0 {
			s.streamMgr.RemoveWithID(streamID)
		}
	}()

	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for ev := range sub.connection {
		if len(ev.Data) == 0 && len(ev.Comment) == 0 {
			break
		}

		if s.eventTTL != 0 && time.Now().After(ev.timestamp.Add(s.eventTTL)) {
			continue
		}

		if len(ev.Data) > 0 {
			_, _ = writeData(w, FieldId, ev.ID)

			if s.splitData {
				sd := bytes.Split(ev.Data, []byte("\n"))
				for i := range sd {
					_, _ = writeData(w, FieldData, sd[i])
				}
			} else {
				if bytes.HasPrefix(ev.Data, []byte(":")) {
					_, _ = fmt.Fprintf(w, "%s\n", ev.Data)
				} else {
					_, _ = writeData(w, FieldData, ev.Data)
				}
			}

			if len(ev.Event) > 0 {
				_, _ = writeData(w, FieldEvent, ev.Event)
			}

			if len(ev.Retry) > 0 {
				_, _ = writeData(w, FieldRetry, ev.Retry)
			}
		}

		if len(ev.Comment) > 0 {
			_, _ = writeData(w, "", ev.Comment)
		}

		_, _ = fmt.Fprint(w, "\n")

		flusher.Flush()
	}

	return http.StatusOK, nil
}

// resolveStreamID 从请求中解析 stream ID。
func (s *Server) resolveStreamID(r *http.Request) (string, error) {
	if s.streamIDResolver != nil {
		return s.streamIDResolver(r)
	}
	if r == nil || r.URL == nil {
		return "", fmt.Errorf("request url is nil")
	}
	return r.URL.Query().Get(s.streamIdKey), nil
}
