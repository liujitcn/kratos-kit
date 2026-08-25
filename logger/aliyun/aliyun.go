package aliyun

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/producer"
	"google.golang.org/protobuf/proto"

	"github.com/go-kratos/kratos/v3/log"
	kitlogger "github.com/liujitcn/kratos-kit/logger"
)

// Logger 定义阿里云日志 SDK 适配器能力。
type Logger interface {
	Log(level log.Level, keyvals ...any) error
	GetProducer() *producer.Producer
	Close() error
}

type aliyunLog struct {
	producer *producer.Producer
	opts     *options
}

// NewAliyunLog 根据配置创建阿里云日志适配器。
func NewAliyunLog(options ...Option) (Logger, error) {
	opts := defaultOptions()
	for _, o := range options {
		o(opts)
	}

	producerConfig := producer.GetDefaultProducerConfig()
	producerConfig.Endpoint = opts.endpoint

	producerConfig.CredentialsProvider = sls.NewStaticCredentialsProvider(opts.accessKey, opts.accessSecret, opts.securityToken)

	producerInst, err := producer.NewProducer(producerConfig)
	if err != nil {
		return nil, err
	}
	producerInst.Start()

	return &aliyunLog{
		opts:     opts,
		producer: producerInst,
	}, nil
}

// Log 将统一解析后的结构化日志写入阿里云日志服务。
func (a *aliyunLog) Log(level log.Level, keyvals ...any) error {
	var entry kitlogger.Entry
	var err error
	entry, err = kitlogger.ParseLegacyEntry(keyvals...)
	if err != nil {
		return err
	}

	var contents = make([]*sls.LogContent, 0, len(entry.Fields)+3)
	contents = append(contents, &sls.LogContent{
		Key:   new(slog.LevelKey),
		Value: new(level.String()),
	})
	if entry.Message != "" {
		contents = append(contents, &sls.LogContent{
			Key:   new(slog.MessageKey),
			Value: new(kitlogger.CleanANSI(entry.Message)),
		})
	}
	var caller = kitlogger.FormatFileCaller(entry.Caller)
	if caller != "" {
		contents = append(contents, &sls.LogContent{
			Key:   new(kitlogger.CallerKey),
			Value: new(caller),
		})
	}
	for _, field := range entry.Fields {
		contents = append(contents, &sls.LogContent{
			Key:   new(field.Key),
			Value: new(kitlogger.CleanANSI(toString(field.Value))),
		})
	}

	var logInst = &sls.Log{
		Time:     proto.Uint32(uint32(time.Now().Unix())),
		Contents: contents,
	}
	return a.producer.SendLog(a.opts.project, a.opts.logstore, "", "", logInst)
}

// GetProducer 返回底层阿里云日志 Producer。
func (a *aliyunLog) GetProducer() *producer.Producer {
	return a.producer
}

// Close 关闭阿里云日志 Producer。
func (a *aliyunLog) Close() error {
	return a.producer.Close(5000)
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
