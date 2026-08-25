package tencent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	cls "github.com/tencentcloud/tencentcloud-cls-sdk-go"
	"google.golang.org/protobuf/proto"

	"github.com/go-kratos/kratos/v3/log"
	kitlogger "github.com/liujitcn/kratos-kit/logger"
)

// Logger 定义腾讯云日志 SDK 适配器能力。
type Logger interface {
	Log(level log.Level, keyvals ...any) error
	GetProducer() *cls.AsyncProducerClient
	Close() error
}

type tencentLog struct {
	producer *cls.AsyncProducerClient
	opts     *options
}

// NewTencentLogger 创建腾讯云 CLS 日志适配器。
func NewTencentLogger(options ...Option) (Logger, error) {
	opts := defaultOptions()
	for _, o := range options {
		o(opts)
	}
	producerConfig := cls.GetDefaultAsyncProducerClientConfig()
	producerConfig.AccessKeyID = opts.accessKey
	producerConfig.AccessKeySecret = opts.accessSecret
	producerConfig.Endpoint = opts.endpoint
	producerInst, err := cls.NewAsyncProducerClient(producerConfig)
	if err != nil {
		return nil, err
	}
	producerInst.Start()
	return &tencentLog{
		producer: producerInst,
		opts:     opts,
	}, nil
}

// Log 将统一解析后的结构化日志写入腾讯云 CLS。
func (t *tencentLog) Log(level log.Level, keyvals ...any) error {
	var entry kitlogger.Entry
	var err error
	entry, err = kitlogger.ParseLegacyEntry(keyvals...)
	if err != nil {
		return err
	}

	var contents = make([]*cls.Log_Content, 0, len(entry.Fields)+3)
	contents = append(contents, &cls.Log_Content{
		Key:   new(slog.LevelKey),
		Value: new(level.String()),
	})
	if entry.Message != "" {
		contents = append(contents, &cls.Log_Content{
			Key:   new(slog.MessageKey),
			Value: new(kitlogger.CleanANSI(entry.Message)),
		})
	}
	var caller = kitlogger.FormatFileCaller(entry.Caller)
	if caller != "" {
		contents = append(contents, &cls.Log_Content{
			Key:   new(kitlogger.CallerKey),
			Value: new(caller),
		})
	}
	for _, field := range entry.Fields {
		contents = append(contents, &cls.Log_Content{
			Key:   new(field.Key),
			Value: new(kitlogger.CleanANSI(toString(field.Value))),
		})
	}

	var logInst = &cls.Log{
		Time:     proto.Int64(time.Now().Unix()),
		Contents: contents,
	}
	return t.producer.SendLog(t.opts.topicID, logInst, nil)
}

// GetProducer 返回底层腾讯云日志 Producer。
func (t *tencentLog) GetProducer() *cls.AsyncProducerClient {
	return t.producer
}

// Close 关闭腾讯云日志 Producer。
func (t *tencentLog) Close() error {
	return t.producer.Close(5000)
}

// toString 将任意字段值转换为字符串。
func toString(v any) string {
	var key string
	if v == nil {
		return key
	}
	switch v := v.(type) {
	case float64:
		key = strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		key = strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		key = strconv.Itoa(v)
	case uint:
		key = strconv.FormatUint(uint64(v), 10)
	case int8:
		key = strconv.Itoa(int(v))
	case uint8:
		key = strconv.FormatUint(uint64(v), 10)
	case int16:
		key = strconv.Itoa(int(v))
	case uint16:
		key = strconv.FormatUint(uint64(v), 10)
	case int32:
		key = strconv.Itoa(int(v))
	case uint32:
		key = strconv.FormatUint(uint64(v), 10)
	case int64:
		key = strconv.FormatInt(v, 10)
	case uint64:
		key = strconv.FormatUint(v, 10)
	case string:
		key = v
	case bool:
		key = strconv.FormatBool(v)
	case []byte:
		key = string(v)
	case fmt.Stringer:
		key = v.String()
	default:
		newValue, _ := json.Marshal(v)
		key = string(newValue)
	}
	return key
}
