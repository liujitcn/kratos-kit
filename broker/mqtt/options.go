package mqtt

import (
	"context"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/liujitcn/kratos-kit/broker"
)

type authKey struct{}
type clientIDKey struct{}
type cleanSessionKey struct{}
type keepAliveKey struct{}
type connectTimeoutKey struct{}
type autoReconnectKey struct{}
type resumeSubsKey struct{}
type orderMattersKey struct{}
type writeTimeoutKey struct{}
type connectRetryIntervalKey struct{}
type maxReconnectIntervalKey struct{}
type qosKey struct{}

type authOptions struct {
	username string
	password string
}

// WithAuth 设置 MQTT 客户端用户名和密码。
func WithAuth(username, password string) broker.Option {
	return broker.OptionContextWithValue(authKey{}, authOptions{username: username, password: password})
}

// WithClientID 设置 MQTT 客户端 ID。
func WithClientID(clientID string) broker.Option {
	return broker.OptionContextWithValue(clientIDKey{}, clientID)
}

// WithCleanSession 设置 MQTT Clean Session 标志。
func WithCleanSession(clean bool) broker.Option {
	return broker.OptionContextWithValue(cleanSessionKey{}, clean)
}

// WithKeepAlive 设置 MQTT 心跳周期。
func WithKeepAlive(keepAlive time.Duration) broker.Option {
	return broker.OptionContextWithValue(keepAliveKey{}, keepAlive)
}

// WithConnectTimeout 设置 MQTT 连接超时时间。
func WithConnectTimeout(timeout time.Duration) broker.Option {
	return broker.OptionContextWithValue(connectTimeoutKey{}, timeout)
}

// WithAutoReconnect 设置 MQTT 是否自动重连。
func WithAutoReconnect(autoReconnect bool) broker.Option {
	return broker.OptionContextWithValue(autoReconnectKey{}, autoReconnect)
}

// WithResumeSubs 设置 MQTT 重连后是否恢复订阅。
func WithResumeSubs(resumeSubs bool) broker.Option {
	return broker.OptionContextWithValue(resumeSubsKey{}, resumeSubs)
}

// WithOrderMatters 设置 MQTT 是否保持消息顺序。
func WithOrderMatters(orderMatters bool) broker.Option {
	return broker.OptionContextWithValue(orderMattersKey{}, orderMatters)
}

// WithWriteTimeout 设置 MQTT 写操作超时时间。
func WithWriteTimeout(timeout time.Duration) broker.Option {
	return broker.OptionContextWithValue(writeTimeoutKey{}, timeout)
}

// WithConnectRetryInterval 设置 MQTT 连接重试间隔。
func WithConnectRetryInterval(interval time.Duration) broker.Option {
	return broker.OptionContextWithValue(connectRetryIntervalKey{}, interval)
}

// WithMaxReconnectInterval 设置 MQTT 最大重连间隔。
func WithMaxReconnectInterval(interval time.Duration) broker.Option {
	return broker.OptionContextWithValue(maxReconnectIntervalKey{}, interval)
}

// WithQoS 设置 MQTT 发布或订阅的默认 QoS。
func WithQoS(qos byte) broker.SubscribeOption {
	return broker.SubscribeContextWithValue(qosKey{}, qos)
}

// WithPublishQoS 设置 MQTT 发布消息的 QoS。
func WithPublishQoS(qos byte) broker.PublishOption {
	return broker.PublishContextWithValue(qosKey{}, qos)
}

func authFromContext(ctx context.Context) (authOptions, bool) {
	value, ok := ctx.Value(authKey{}).(authOptions)
	return value, ok
}

func clientIDFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(clientIDKey{}).(string)
	return value, ok
}

func cleanSessionFromContext(ctx context.Context) (bool, bool) {
	value, ok := ctx.Value(cleanSessionKey{}).(bool)
	return value, ok
}

func keepAliveFromContext(ctx context.Context) (time.Duration, bool) {
	value, ok := ctx.Value(keepAliveKey{}).(time.Duration)
	return value, ok
}

func connectTimeoutFromContext(ctx context.Context) (time.Duration, bool) {
	value, ok := ctx.Value(connectTimeoutKey{}).(time.Duration)
	return value, ok
}

func autoReconnectFromContext(ctx context.Context) (bool, bool) {
	value, ok := ctx.Value(autoReconnectKey{}).(bool)
	return value, ok
}

func resumeSubsFromContext(ctx context.Context) (bool, bool) {
	value, ok := ctx.Value(resumeSubsKey{}).(bool)
	return value, ok
}

func orderMattersFromContext(ctx context.Context) (bool, bool) {
	value, ok := ctx.Value(orderMattersKey{}).(bool)
	return value, ok
}

func writeTimeoutFromContext(ctx context.Context) (time.Duration, bool) {
	value, ok := ctx.Value(writeTimeoutKey{}).(time.Duration)
	return value, ok
}

func connectRetryIntervalFromContext(ctx context.Context) (time.Duration, bool) {
	value, ok := ctx.Value(connectRetryIntervalKey{}).(time.Duration)
	return value, ok
}

func maxReconnectIntervalFromContext(ctx context.Context) (time.Duration, bool) {
	value, ok := ctx.Value(maxReconnectIntervalKey{}).(time.Duration)
	return value, ok
}

func qosFromSubscribeOptions(options broker.SubscribeOptions) byte {
	if value, ok := options.Context.Value(qosKey{}).(byte); ok && value <= 2 {
		return value
	}
	return 0
}

func qosFromPublishOptions(options broker.PublishOptions) byte {
	if value, ok := options.Context.Value(qosKey{}).(byte); ok && value <= 2 {
		return value
	}
	return 0
}

func applyClientOptions(options broker.Options, clientOptions *mqtt.ClientOptions) {
	addresses := options.Addrs
	if len(addresses) == 0 {
		addresses = []string{"tcp://127.0.0.1:1883"}
	}
	for _, address := range addresses {
		clientOptions.AddBroker(address)
	}
	if auth, ok := authFromContext(options.Context); ok {
		clientOptions.SetUsername(auth.username)
		clientOptions.SetPassword(auth.password)
	}
	if clientID, ok := clientIDFromContext(options.Context); ok {
		clientOptions.SetClientID(clientID)
	}
	if cleanSession, ok := cleanSessionFromContext(options.Context); ok {
		clientOptions.SetCleanSession(cleanSession)
	}
	if keepAlive, ok := keepAliveFromContext(options.Context); ok && keepAlive > 0 {
		clientOptions.SetKeepAlive(keepAlive)
	}
	if connectTimeout, ok := connectTimeoutFromContext(options.Context); ok && connectTimeout > 0 {
		clientOptions.SetConnectTimeout(connectTimeout)
	}
	if autoReconnect, ok := autoReconnectFromContext(options.Context); ok {
		clientOptions.SetAutoReconnect(autoReconnect)
	}
	if resumeSubs, ok := resumeSubsFromContext(options.Context); ok {
		clientOptions.SetResumeSubs(resumeSubs)
	}
	if orderMatters, ok := orderMattersFromContext(options.Context); ok {
		clientOptions.SetOrderMatters(orderMatters)
	}
	if writeTimeout, ok := writeTimeoutFromContext(options.Context); ok && writeTimeout > 0 {
		clientOptions.SetWriteTimeout(writeTimeout)
	}
	if connectRetryInterval, ok := connectRetryIntervalFromContext(options.Context); ok && connectRetryInterval > 0 {
		clientOptions.SetConnectRetryInterval(connectRetryInterval)
	}
	if maxReconnectInterval, ok := maxReconnectIntervalFromContext(options.Context); ok && maxReconnectInterval > 0 {
		clientOptions.SetMaxReconnectInterval(maxReconnectInterval)
	}
	if options.TLSConfig != nil {
		clientOptions.SetTLSConfig(options.TLSConfig)
	}
}
