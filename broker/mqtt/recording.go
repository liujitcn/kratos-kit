package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
	"unicode/utf8"

	paho "github.com/eclipse/paho.golang/paho"
)

const (
	// DirectionPublish 表示平台向 MQTT Broker 发布消息。
	DirectionPublish = "PUBLISH"
	// DirectionSubscribe 表示平台从 MQTT Broker 收到订阅消息。
	DirectionSubscribe = "SUBSCRIBE"
	// StatusHandled 表示订阅消息处理成功。
	StatusHandled = "HANDLED"
	// StatusSent 表示发布消息已得到 Broker 确认。
	StatusSent = "SENT"
	// StatusFailed 表示消息发布或处理失败。
	StatusFailed = "FAILED"
)

// MessageRecord 描述一次 MQTT 发布或订阅消息事件。
type MessageRecord struct {
	Direction     string `json:"direction"`
	Topic         string `json:"topic"`
	PayloadRaw    []byte `json:"payload_raw"`
	PayloadJSON   string `json:"payload_json,omitempty"`
	PayloadSize   int64  `json:"payload_size"`
	PayloadFormat string `json:"payload_format"`
	QoS           byte   `json:"qos"`
	Retained      bool   `json:"retained"`
	Duplicate     bool   `json:"duplicate"`
	MessageID     string `json:"message_id"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"error_message,omitempty"`
	LatencyMs     int64  `json:"latency_ms"`
	CreatedAt     int64  `json:"created_at"`
	HandledAt     int64  `json:"handled_at"`
}

// Recorder 接收 MQTT 消息事件，具体持久化由宿主实现。
type Recorder interface {
	Record(context.Context, MessageRecord)
}

// Stats 描述 MQTT 客户端连接、订阅和消息处理快照。
type Stats struct {
	Connected               bool
	Address                 string
	ClientID                string
	ConnectedAt             time.Time
	DisconnectedAt          time.Time
	RegisteredSubscriptions uint64
	ActiveSubscriptions     uint64
	Published               uint64
	PublishPending          uint64
	MaxPublishPending       uint64
	PublishErrors           uint64
	Received                uint64
	Handled                 uint64
	ReceiveInFlight         uint64
	MaxReceiveInFlight      uint64
	ReceiveErrors           uint64
	SubscribeErrors         uint64
	LastError               string
}

// StatsProvider 提供 MQTT 客户端运行统计快照。
type StatsProvider interface {
	Stats() Stats
}

var _ StatsProvider = (*mqttBroker)(nil)

// Stats 返回 MQTT 客户端运行统计快照。
func (b *mqttBroker) Stats() Stats {
	options := b.Options()
	b.mu.RLock()
	connected := b.connected
	b.mu.RUnlock()
	b.statsMu.RLock()
	lastError := b.lastError
	connectedAt := b.connectedAt
	disconnectedAt := b.disconnectedAt
	b.statsMu.RUnlock()
	clientID, _ := clientIDFromContext(options.Context)
	return Stats{
		Connected:          connected,
		Address:            b.Address(),
		ClientID:           clientID,
		ConnectedAt:        connectedAt,
		DisconnectedAt:     disconnectedAt,
		Published:          b.published.Load(),
		PublishPending:     b.publishPending.Load(),
		MaxPublishPending:  b.maxPublishPending.Load(),
		PublishErrors:      b.publishErrors.Load(),
		Received:           b.received.Load(),
		Handled:            b.handled.Load(),
		ReceiveInFlight:    b.receiveInFlight.Load(),
		MaxReceiveInFlight: b.maxReceiveInFlight.Load(),
		ReceiveErrors:      b.receiveErrors.Load(),
		SubscribeErrors:    b.subscribeErrors.Load(),
		LastError:          lastError,
	}
}

// SetRecorder 设置 MQTT 消息事件记录器。
func (b *mqttBroker) SetRecorder(recorder Recorder) {
	b.recorderMu.Lock()
	b.recorder = recorder
	b.recorderMu.Unlock()
}

// recordMessage 投递一条 MQTT 消息事件给宿主记录器。
func (b *mqttBroker) recordMessage(ctx context.Context, record MessageRecord) {
	b.recorderMu.RLock()
	recorder := b.recorder
	b.recorderMu.RUnlock()
	if recorder != nil {
		recorder.Record(ctx, record)
	}
}

// beginPublish 记录一个等待 Broker 确认的发布消息。
func (b *mqttBroker) beginPublish() {
	pending := b.publishPending.Add(1)
	b.updateMax(&b.maxPublishPending, pending)
}

// finishPublish 记录一个已结束等待的发布消息。
func (b *mqttBroker) finishPublish() {
	b.publishPending.Add(^uint64(0))
}

// recordError 更新最近一次 MQTT 错误。
func (b *mqttBroker) recordError(err error) {
	if err == nil {
		return
	}
	b.statsMu.Lock()
	b.lastError = err.Error()
	b.statsMu.Unlock()
}

// recordPublishError 记录发布失败和最近错误。
func (b *mqttBroker) recordPublishError(err error) {
	if err == nil {
		return
	}
	b.publishErrors.Add(1)
	b.recordError(err)
}

// updateMax 更新原子峰值计数。
func (b *mqttBroker) updateMax(target *atomic.Uint64, value uint64) {
	for current := target.Load(); value > current; {
		if target.CompareAndSwap(current, value) {
			return
		}
		current = target.Load()
	}
}

// recordPublishMessage 生成并投递一条发布消息记录。
func (b *mqttBroker) recordPublishMessage(ctx context.Context, topic string, payload []byte, qos byte, status string, recordErr error, startedAt time.Time) {
	payload = clonePayload(payload)
	format, payloadJSON := inspectPayload(payload)
	record := MessageRecord{Direction: DirectionPublish, Topic: topic, PayloadRaw: payload, PayloadJSON: payloadJSON, PayloadSize: int64(len(payload)), PayloadFormat: format, QoS: qos, Status: status, CreatedAt: startedAt.UnixMilli(), HandledAt: time.Now().UnixMilli(), LatencyMs: time.Since(startedAt).Milliseconds()}
	if recordErr != nil {
		record.ErrorMessage = recordErr.Error()
	}
	b.recordMessage(ctx, record)
}

// recordSubscribeMessage 生成并投递一条订阅消息记录。
func (b *mqttBroker) recordSubscribeMessage(ctx context.Context, raw *paho.Publish, status string, recordErr error, startedAt time.Time) {
	payload := clonePayload(raw.Payload)
	format, payloadJSON := inspectPayload(payload)
	record := MessageRecord{Direction: DirectionSubscribe, Topic: raw.Topic, PayloadRaw: payload, PayloadJSON: payloadJSON, PayloadSize: int64(len(payload)), PayloadFormat: format, QoS: raw.QoS, Retained: raw.Retain, MessageID: fmt.Sprintf("%d", raw.PacketID), Status: status, CreatedAt: startedAt.UnixMilli(), HandledAt: time.Now().UnixMilli(), LatencyMs: time.Since(startedAt).Milliseconds()}
	if recordErr != nil {
		record.ErrorMessage = recordErr.Error()
	}
	b.recordMessage(ctx, record)
}

// clonePayload 复制消息内容，并将空消息规范化为零长度非 nil 切片，保证可安全写入非空数据库字段。
func clonePayload(payload []byte) []byte {
	return append([]byte{}, payload...)
}

// inspectPayload 识别消息格式并提取规范化 JSON。
func inspectPayload(payload []byte) (string, string) {
	if !utf8.Valid(payload) {
		return "BINARY", ""
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return "TEXT", ""
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return "JSON", ""
	}
	return "JSON", string(normalized)
}
