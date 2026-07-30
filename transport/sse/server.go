package sse

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/go-kratos/kratos/v3/encoding"
	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
	kratosTransport "github.com/go-kratos/kratos/v3/transport"

	"github.com/gorilla/mux"

	"github.com/liujitcn/kratos-kit/broker"
	"github.com/liujitcn/kratos-kit/transport"
)

type MessagePayload any

var (
	_ kratosTransport.Server     = (*Server)(nil)
	_ kratosTransport.Endpointer = (*Server)(nil)
	_ http.Handler               = (*Server)(nil)
)

type Server struct {
	*http.Server

	lis      net.Listener
	tlsConf  *tls.Config
	endpoint *url.URL

	network          string
	address          string
	path             string
	streamIdKey      string
	streamIDResolver StreamIDResolver

	timeout time.Duration

	err   error
	codec encoding.Codec

	router      *mux.Router
	strictSlash bool

	headers    map[string]string
	eventTTL   time.Duration
	bufferSize int

	encodeBase64    bool
	splitData       bool
	autoStream      bool
	autoReplay      bool
	corsAllowOrigin string

	subscribeFunc   SubscriberFunction
	unsubscribeFunc SubscriberFunction
	authorizeFunc   AuthorizeFunc
	tokenExtractor  TokenExtractor

	streamMgr *StreamManager
}

// NewServer 创建独立监听端口的 SSE 服务端。
func NewServer(opts ...ServerOption) *Server {
	srv := newServer(opts...)
	srv.err = srv.listen()
	return srv
}

// newServer 初始化 SSE 服务端基础配置，不主动占用监听端口。
func newServer(opts ...ServerOption) *Server {
	srv := &Server{
		network:     "tcp",
		address:     ":0",
		timeout:     1 * time.Second,
		router:      mux.NewRouter(),
		strictSlash: true,
		path:        "/",
		streamIdKey: "stream",

		bufferSize:   DefaultBufferSize,
		encodeBase64: false,

		autoStream:      false,
		autoReplay:      true,
		corsAllowOrigin: "*",
		headers:         map[string]string{},
		tokenExtractor:  DefaultTokenExtractor,

		streamMgr: NewStreamManager(),
	}

	srv.streamIDResolver = ResolveStreamIDFromQuery(srv.streamIdKey)

	srv.init(opts...)

	return srv
}

func (s *Server) Name() string {
	return KindSSE
}

func (s *Server) Start(ctx context.Context) error {
	if err := s.listenAndEndpoint(); err != nil {
		return err
	}

	if s.err != nil {
		return s.err
	}

	s.BaseContext = func(net.Listener) context.Context {
		return ctx
	}

	log.Info("server listening", "addr", s.lis.Addr().String())

	s.HandleServeHTTP(s.path)

	var err error
	if s.tlsConf != nil {
		err = s.ServeTLS(s.lis, "", "")
	} else {
		err = s.Serve(s.lis)
	}
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	log.Info("server stopping...")

	s.streamMgr.Clean()

	err := s.Shutdown(ctx)
	s.err = nil

	log.Info("server stopped.")

	return err
}

func (s *Server) Endpoint() (*url.URL, error) {
	if err := s.listenAndEndpoint(); err != nil {
		return nil, err
	}
	return s.endpoint, nil
}

func (s *Server) listenAndEndpoint() error {
	if s.lis == nil {
		lis, err := net.Listen(s.network, s.address)
		if err != nil {
			return err
		}
		s.lis = lis
	}

	if s.endpoint == nil {
		// 如果传入的是完整的ip地址，则不需要调整。
		// 如果传入的只有端口号，则会调整为完整的地址，但，IP地址或许会不正确。
		addr, err := transport.AdjustAddress(s.address, s.lis)
		if err != nil {
			s.err = err
			return err
		}

		s.endpoint = transport.NewRegistryEndpoint(KindSSE, addr)
	}

	return nil
}

func (s *Server) Handle(path string, h http.Handler) {
	s.router.Handle(path, h)
}

func (s *Server) HandlePrefix(prefix string, h http.Handler) {
	s.router.PathPrefix(prefix).Handler(h)
}

func (s *Server) HandleFunc(path string, h http.HandlerFunc) {
	s.router.HandleFunc(path, h)
}

func (s *Server) HandleHeader(key, val string, h http.HandlerFunc) {
	s.router.Headers(key, val).Handler(h)
}

func (s *Server) HandleServeHTTP(path string) {
	s.router.HandleFunc(path, s.ServeHTTP)
}

func (s *Server) init(opts ...ServerOption) {
	for _, o := range opts {
		o(s)
	}

	s.router.StrictSlash(s.strictSlash)
	s.router.NotFoundHandler = http.DefaultServeMux
	s.router.MethodNotAllowedHandler = http.DefaultServeMux

	s.Server = &http.Server{
		Handler:   s.router,
		TLSConfig: s.tlsConf,
	}
}

func (s *Server) listen() error {
	if s.lis == nil {
		lis, err := net.Listen(s.network, s.address)
		if err != nil {
			return err
		}
		s.lis = lis
	}

	return nil
}

// Publish 向指定流发布事件。
func (s *Server) Publish(_ context.Context, streamId StreamID, event *Event) {
	stream := s.streamMgr.Get(streamId)
	if stream == nil {
		return
	}

	select {
	case <-stream.quit:
	case stream.event <- s.process(event):
	}
}

// TryPublish 尝试向指定流非阻塞发布事件。
func (s *Server) TryPublish(_ context.Context, streamId StreamID, event *Event) bool {
	stream := s.streamMgr.Get(streamId)
	if stream == nil {
		return false
	}

	select {
	case <-stream.quit:
		return false
	case stream.event <- s.process(event):
		return true
	default:
		return false
	}
}

// PublishData 编码数据并发布到指定流。
func (s *Server) PublishData(ctx context.Context, streamId StreamID, data MessagePayload) error {
	event, err := s.marshalEvent(data)
	if err != nil {
		return err
	}
	s.Publish(ctx, streamId, event)
	return nil
}

// PublishDataWithEventName 编码数据、设置事件名称并发布到指定流。
func (s *Server) PublishDataWithEventName(ctx context.Context, streamId StreamID, eventName string, data MessagePayload) error {
	return s.PublishDataWithMeta(ctx, streamId, data, WithEventName(eventName))
}

// PublishDataWithMeta 编码数据、设置事件元数据并发布到指定流。
func (s *Server) PublishDataWithMeta(ctx context.Context, streamId StreamID, data MessagePayload, opts ...EventMetaOption) error {
	event, err := s.marshalEvent(data)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(event)
	}
	s.Publish(ctx, streamId, event)
	return nil
}

// Notify 向所有流广播事件。
func (s *Server) Notify(_ context.Context, event *Event) {
	s.streamMgr.Range(func(stream *Stream) {
		if stream == nil {
			return
		}

		select {
		case <-stream.quit:
		case stream.event <- s.process(event):
		}
	})
}

// NotifyData 编码数据并向所有流广播。
func (s *Server) NotifyData(ctx context.Context, data MessagePayload) error {
	event, err := s.marshalEvent(data)
	if err != nil {
		return err
	}
	s.Notify(ctx, event)
	return nil
}

// NotifyDataWithEventName 编码数据、设置事件名称并向所有流广播。
func (s *Server) NotifyDataWithEventName(ctx context.Context, eventName string, data MessagePayload) error {
	return s.NotifyDataWithMeta(ctx, data, WithEventName(eventName))
}

// NotifyDataWithMeta 编码数据、设置事件元数据并向所有流广播。
func (s *Server) NotifyDataWithMeta(ctx context.Context, data MessagePayload, opts ...EventMetaOption) error {
	event, err := s.marshalEvent(data)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(event)
	}
	s.Notify(ctx, event)
	return nil
}

// createStream 创建并启动一个事件流。
func (s *Server) createStream(streamId StreamID) *Stream {
	stream := newStream(streamId, s.bufferSize, s.autoReplay, s.autoStream, s.subscribeFunc, s.unsubscribeFunc)
	stream.run()
	return stream
}

// CreateStream 创建事件流，已存在时返回原实例。
func (s *Server) CreateStream(streamId StreamID) *Stream {
	stream := s.streamMgr.Get(streamId)
	if stream != nil {
		return stream
	}

	stream = s.createStream(streamId)
	return s.streamMgr.Add(stream)
}

// GetStream 返回指定事件流，不存在时返回 nil。
func (s *Server) GetStream(streamId StreamID) *Stream {
	return s.streamMgr.Get(streamId)
}

// RemoveStream 关闭并移除指定事件流。
func (s *Server) RemoveStream(streamId StreamID) {
	s.streamMgr.RemoveWithID(streamId)
}

// StreamCount 返回当前事件流数量。
func (s *Server) StreamCount() int {
	return s.streamMgr.Count()
}

// process 复制事件并应用服务端传输转换，避免广播时修改调用方对象。
func (s *Server) process(event *Event) *Event {
	if event == nil {
		event = &Event{}
	}
	processed := *event
	processed.timestamp = time.Now()
	if s.encodeBase64 {
		processed.encodeBase64()
	}
	return &processed
}

// marshalEvent 将业务数据编码为 SSE 事件。
func (s *Server) marshalEvent(data MessagePayload) (*Event, error) {
	event := &Event{}
	if data == nil {
		return event, nil
	}
	encoded, err := broker.Marshal(s.codec, data)
	if err != nil {
		return nil, err
	}
	event.Data = encoded
	return event, nil
}
