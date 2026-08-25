package nats

import (
	"context"
	"slices"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/liujitcn/kratos-kit/broker"
)

type optionsKey struct{}
type drainConnectionKey struct{}
type jetStreamContextOptionsKey struct{}
type headersKey struct{}
type publishMessageIDKey struct{}
type publishExpectedStreamKey struct{}
type publishExpectedLastSequenceKey struct{}
type publishExpectedLastSequencePerSubjectKey struct{}
type publishExpectedLastMessageIDKey struct{}
type publishRawOptionsKey struct{}
type subscribeDurableKey struct{}
type subscribeDeliverAllKey struct{}
type subscribeDeliverLastKey struct{}
type subscribeDeliverNewKey struct{}
type subscribeStartSequenceKey struct{}
type subscribeStartTimeKey struct{}
type subscribeAckWaitKey struct{}
type subscribeMaxAckPendingKey struct{}
type subscribeBindStreamKey struct{}
type subscribeReplayInstantKey struct{}
type subscribeDescriptionKey struct{}
type subscribeManualAckKey struct{}
type subscribePullKey struct{}
type subscribePullBatchSizeKey struct{}
type subscribeRawOptionsKey struct{}

// Options 注入完整的 NATS SDK 连接选项。
func Options(options nats.Options) broker.Option {
	return broker.OptionContextWithValue(optionsKey{}, options)
}

// DrainConnection 要求断开时先排空订阅和待发送消息。
func DrainConnection() broker.Option {
	return broker.OptionContextWithValue(drainConnectionKey{}, true)
}

// JetStreamContextOptions 注入创建 JetStream context 时使用的 SDK 选项。
func JetStreamContextOptions(options ...nats.JSOpt) broker.Option {
	return broker.OptionContextWithValue(jetStreamContextOptionsKey{}, slices.Clone(options))
}

// WithHeaders 为发布消息追加支持多值的 NATS header。
func WithHeaders(headers map[string][]string) broker.PublishOption {
	return broker.PublishContextWithValue(headersKey{}, cloneHeaders(headers))
}

// WithRequestHeaders 为请求消息追加支持多值的 NATS header。
func WithRequestHeaders(headers map[string][]string) broker.RequestOption {
	return broker.RequestContextWithValue(headersKey{}, cloneHeaders(headers))
}

// WithMessageID 设置 JetStream 去重消息标识。
func WithMessageID(id string) broker.PublishOption {
	return broker.PublishContextWithValue(publishMessageIDKey{}, id)
}

// WithMsgId 兼容上游命名并设置 JetStream 去重消息标识。
func WithMsgId(id string) broker.PublishOption {
	return WithMessageID(id)
}

// WithExpectedStream 设置 JetStream 期望写入的 stream。
func WithExpectedStream(stream string) broker.PublishOption {
	return broker.PublishContextWithValue(publishExpectedStreamKey{}, stream)
}

// WithExpectStream 兼容上游命名并设置期望写入的 stream。
func WithExpectStream(stream string) broker.PublishOption {
	return WithExpectedStream(stream)
}

// WithExpectedLastSequence 设置 JetStream 期望的最后序列号。
func WithExpectedLastSequence(sequence uint64) broker.PublishOption {
	return broker.PublishContextWithValue(publishExpectedLastSequenceKey{}, sequence)
}

// WithExpectLastSequence 兼容上游命名并设置期望的最后序列号。
func WithExpectLastSequence(sequence uint64) broker.PublishOption {
	return WithExpectedLastSequence(sequence)
}

// WithExpectedLastSequencePerSubject 设置 JetStream subject 期望的最后序列号。
func WithExpectedLastSequencePerSubject(sequence uint64) broker.PublishOption {
	return broker.PublishContextWithValue(publishExpectedLastSequencePerSubjectKey{}, sequence)
}

// WithExpectLastSequencePerSubject 兼容上游命名并设置 subject 期望的最后序列号。
func WithExpectLastSequencePerSubject(sequence uint64) broker.PublishOption {
	return WithExpectedLastSequencePerSubject(sequence)
}

// WithExpectedLastMessageID 设置 JetStream 期望的最后消息标识。
func WithExpectedLastMessageID(id string) broker.PublishOption {
	return broker.PublishContextWithValue(publishExpectedLastMessageIDKey{}, id)
}

// WithExpectLastMsgId 兼容上游命名并设置期望的最后消息标识。
func WithExpectLastMsgId(id string) broker.PublishOption {
	return WithExpectedLastMessageID(id)
}

// WithPublishRawOptions 追加原生 JetStream 发布选项。
func WithPublishRawOptions(options ...nats.PubOpt) broker.PublishOption {
	return broker.PublishContextWithValue(publishRawOptionsKey{}, slices.Clone(options))
}

// WithPublishRawOpts 兼容上游命名并追加原生 JetStream 发布选项。
func WithPublishRawOpts(options ...nats.PubOpt) broker.PublishOption {
	return WithPublishRawOptions(options...)
}

// WithDurable 设置 JetStream durable consumer 名称。
func WithDurable(name string) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeDurableKey{}, name)
}

// WithDeliverAll 要求 JetStream 从最早可用消息开始投递。
func WithDeliverAll() broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeDeliverAllKey{}, true)
}

// WithDeliverLast 要求 JetStream 从最后一条消息开始投递。
func WithDeliverLast() broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeDeliverLastKey{}, true)
}

// WithDeliverNew 要求 JetStream 仅投递新消息。
func WithDeliverNew() broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeDeliverNewKey{}, true)
}

// WithStartSequence 设置 JetStream 起始序列号。
func WithStartSequence(sequence uint64) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeStartSequenceKey{}, sequence)
}

// WithStartTime 设置 JetStream 起始时间。
func WithStartTime(start time.Time) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeStartTimeKey{}, start)
}

// WithSubscribeAckWait 设置 JetStream 等待确认的时间。
func WithSubscribeAckWait(wait time.Duration) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeAckWaitKey{}, wait)
}

// WithSubscribeMaxAckPending 设置 JetStream 最大未确认消息数。
func WithSubscribeMaxAckPending(max int) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeMaxAckPendingKey{}, max)
}

// WithBindStream 将 JetStream consumer 绑定到指定 stream。
func WithBindStream(stream string) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeBindStreamKey{}, stream)
}

// WithReplayInstant 要求 JetStream 尽快重放历史消息。
func WithReplayInstant() broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeReplayInstantKey{}, true)
}

// WithConsumerDescription 设置 JetStream consumer 描述。
func WithConsumerDescription(description string) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeDescriptionKey{}, description)
}

// WithManualAck 要求调用方通过 Event.Ack 手动确认消息。
func WithManualAck() broker.SubscribeOption {
	return func(options *broker.SubscribeOptions) {
		broker.DisableAutoAck()(options)
		broker.SubscribeContextWithValue(subscribeManualAckKey{}, true)(options)
	}
}

// WithPullSubscribe 启用 JetStream pull consumer。
func WithPullSubscribe() broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribePullKey{}, true)
}

// WithPullBatchSize 设置 JetStream pull consumer 每次拉取的消息数。
func WithPullBatchSize(size int) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribePullBatchSizeKey{}, size)
}

// WithSubscribeRawOptions 追加原生 JetStream 订阅选项。
func WithSubscribeRawOptions(options ...nats.SubOpt) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(subscribeRawOptionsKey{}, slices.Clone(options))
}

// WithSubscribeRawOpts 兼容上游命名并追加原生 JetStream 订阅选项。
func WithSubscribeRawOpts(options ...nats.SubOpt) broker.SubscribeOption {
	return WithSubscribeRawOptions(options...)
}

// contextValue 从 option context 中读取指定类型的值。
func contextValue[T any](ctx context.Context, key any) (T, bool) {
	var zero T
	if ctx == nil {
		return zero, false
	}
	value, ok := ctx.Value(key).(T)
	return value, ok
}

// cloneHeaders 复制多值 header，隔离调用方后续修改。
func cloneHeaders(headers map[string][]string) map[string][]string {
	if headers == nil {
		return nil
	}
	cloned := make(map[string][]string, len(headers))
	for key, values := range headers {
		cloned[key] = slices.Clone(values)
	}
	return cloned
}
