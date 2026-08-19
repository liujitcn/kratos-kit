package fluent

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"

	"github.com/fluent/fluent-logger-golang/fluent"

	"github.com/go-kratos/kratos/v3/log"
	kitlogger "github.com/liujitcn/kratos-kit/logger"
)

// Logger 封装 Fluent 日志 SDK。
type Logger struct {
	opts options
	log  *fluent.Fluent
}

// NewFluentLogger 根据目标地址和选项创建 Fluent 日志适配器。
// 支持以下目标地址：
//
//	tcp://127.0.0.1:24224
//	unix://var/run/fluent/fluent.sock
func NewFluentLogger(addr string, opts ...Option) (*Logger, error) {
	var option = options{}
	for _, o := range opts {
		o(&option)
	}
	var parsedURL *url.URL
	var err error
	parsedURL, err = url.Parse(addr)
	if err != nil {
		return nil, err
	}
	var config = fluent.Config{
		Timeout:            option.timeout,
		WriteTimeout:       option.writeTimeout,
		BufferLimit:        option.bufferLimit,
		RetryWait:          option.retryWait,
		MaxRetry:           option.maxRetry,
		MaxRetryWait:       option.maxRetryWait,
		TagPrefix:          option.tagPrefix,
		Async:              option.async,
		ForceStopAsyncSend: option.forceStopAsyncSend,
	}
	switch parsedURL.Scheme {
	case "tcp":
		var host string
		var port string
		host, port, err = net.SplitHostPort(parsedURL.Host)
		if err != nil {
			return nil, err
		}
		config.FluentPort, err = strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
		config.FluentNetwork = parsedURL.Scheme
		config.FluentHost = host
	case "unix":
		config.FluentNetwork = parsedURL.Scheme
		config.FluentSocketPath = parsedURL.Path
	default:
		return nil, fmt.Errorf("unknown network: %s", parsedURL.Scheme)
	}
	var fluentLogger *fluent.Fluent
	fluentLogger, err = fluent.New(config)
	if err != nil {
		return nil, err
	}
	return &Logger{
		opts: option,
		log:  fluentLogger,
	}, nil
}

// Log 将统一解析后的结构化日志写入 Fluent。
func (l *Logger) Log(level log.Level, keyvals ...any) error {
	var entry kitlogger.Entry
	var err error
	entry, err = kitlogger.ParseLegacyEntry(keyvals...)
	if err != nil {
		return err
	}

	var data = make(map[string]string, len(entry.Fields)+3)
	data[slog.LevelKey] = level.String()
	if entry.Message != "" {
		data[slog.MessageKey] = kitlogger.CleanANSI(entry.Message)
	}
	var caller = kitlogger.FormatFileCaller(entry.Caller)
	if caller != "" {
		data[kitlogger.CallerKey] = caller
	}
	for _, field := range entry.Fields {
		data[field.Key] = kitlogger.CleanANSI(fmt.Sprint(field.Value))
	}

	return l.log.Post(level.String(), data)
}

// Close 关闭 Fluent 日志连接。
func (l *Logger) Close() error {
	return l.log.Close()
}
