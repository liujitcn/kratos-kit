package oss

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kratos/kratos/v3/config"
)

const defaultPollInterval = 30 * time.Second

// Client 定义配置源使用的 S3 客户端能力。
type Client interface {
	GetObject(context.Context, *awss3.GetObjectInput, ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	HeadObject(context.Context, *awss3.HeadObjectInput, ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error)
}

// Option 配置 S3 对象存储配置源。
type Option func(*options)

type options struct {
	ctx          context.Context
	bucket       string
	key          string
	pollInterval time.Duration
	format       string
}

// WithContext 设置 S3 请求使用的父 context。
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		}
	}
}

// WithBucket 设置 S3 Bucket。
func WithBucket(bucket string) Option {
	return func(o *options) {
		o.bucket = bucket
	}
}

// WithKey 设置保存配置内容的对象 Key。
func WithKey(key string) Option {
	return func(o *options) {
		o.key = key
	}
}

// WithPollInterval 设置对象变更轮询间隔。
func WithPollInterval(interval time.Duration) Option {
	return func(o *options) {
		if interval > 0 {
			o.pollInterval = interval
		}
	}
}

// WithFormat 显式设置配置格式。
func WithFormat(format string) Option {
	return func(o *options) {
		o.format = strings.TrimPrefix(format, ".")
	}
}

// New 创建 S3 对象存储配置源。
func New(client Client, opts ...Option) (config.Source, error) {
	if client == nil {
		return nil, errors.New("config/oss: client is nil")
	}

	cfg := &options{
		ctx:          context.Background(),
		pollInterval: defaultPollInterval,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.bucket == "" {
		return nil, errors.New("config/oss: bucket is empty")
	}
	if cfg.key == "" {
		return nil, errors.New("config/oss: key is empty")
	}
	return &source{
		client:  client,
		options: cfg,
	}, nil
}

type source struct {
	mu              sync.Mutex
	client          Client
	options         *options
	lastBody        []byte
	lastFingerprint string
}

// Load 下载当前对象内容。
func (s *source) Load() ([]*config.KeyValue, error) {
	value, fingerprint, err := s.load(s.options.ctx)
	if err != nil {
		return nil, err
	}
	s.storeSnapshot(value, fingerprint)
	return []*config.KeyValue{s.keyValue(value)}, nil
}

// Watch 创建对象内容轮询 Watcher。
func (s *source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(s.options.ctx)
	return &watcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
		ticker: time.NewTicker(s.options.pollInterval),
	}, nil
}

// load 下载对象并返回内容指纹。
func (s *source) load(ctx context.Context) ([]byte, string, error) {
	output, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.options.bucket),
		Key:    aws.String(s.options.key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("config/oss: get s3://%s/%s: %w", s.options.bucket, s.options.key, err)
	}

	var value []byte
	value, err = io.ReadAll(output.Body)
	closeErr := output.Body.Close()
	if err != nil {
		return nil, "", fmt.Errorf("config/oss: read s3://%s/%s: %w", s.options.bucket, s.options.key, err)
	}
	if closeErr != nil {
		return nil, "", fmt.Errorf("config/oss: close s3://%s/%s: %w", s.options.bucket, s.options.key, closeErr)
	}
	return value, objectFingerprint(output.ETag, output.VersionId, output.LastModified, output.ContentLength), nil
}

// head 获取当前对象内容指纹。
func (s *source) head(ctx context.Context) (string, error) {
	output, err := s.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(s.options.bucket),
		Key:    aws.String(s.options.key),
	})
	if err != nil {
		return "", fmt.Errorf("config/oss: head s3://%s/%s: %w", s.options.bucket, s.options.key, err)
	}
	return objectFingerprint(output.ETag, output.VersionId, output.LastModified, output.ContentLength), nil
}

// snapshot 返回上次成功加载的对象状态。
func (s *source) snapshot() ([]byte, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.lastBody), s.lastFingerprint
}

// storeSnapshot 保存上次成功加载的对象状态。
func (s *source) storeSnapshot(value []byte, fingerprint string) {
	s.mu.Lock()
	s.lastBody = bytes.Clone(value)
	s.lastFingerprint = fingerprint
	s.mu.Unlock()
}

// keyValue 将对象内容转换为 Kratos KeyValue。
func (s *source) keyValue(value []byte) *config.KeyValue {
	format := s.options.format
	if format == "" {
		format = strings.TrimPrefix(path.Ext(s.options.key), ".")
	}
	return &config.KeyValue{
		Key:    s.options.key,
		Value:  value,
		Format: format,
	}
}

type watcher struct {
	source *source
	ctx    context.Context
	cancel context.CancelFunc
	ticker *time.Ticker
}

// Next 等待 S3 对象内容发生变化。
func (w *watcher) Next() ([]*config.KeyValue, error) {
	for {
		select {
		case <-w.ctx.Done():
			return nil, w.ctx.Err()
		case <-w.ticker.C:
			fingerprint, err := w.source.head(w.ctx)
			if err != nil {
				return nil, err
			}
			lastBody, lastFingerprint := w.source.snapshot()
			if fingerprint != "" && fingerprint == lastFingerprint {
				continue
			}

			var value []byte
			value, fingerprint, err = w.source.load(w.ctx)
			if err != nil {
				return nil, err
			}
			w.source.storeSnapshot(value, fingerprint)
			if bytes.Equal(value, lastBody) {
				continue
			}
			return []*config.KeyValue{w.source.keyValue(value)}, nil
		}
	}
}

// Stop 停止 S3 对象配置轮询。
func (w *watcher) Stop() error {
	w.ticker.Stop()
	w.cancel()
	return nil
}

// objectFingerprint 组合 S3 可用的对象版本字段。
func objectFingerprint(etag, versionID *string, lastModified *time.Time, contentLength *int64) string {
	if etag == nil && versionID == nil && lastModified == nil && contentLength == nil {
		return ""
	}
	return fmt.Sprintf(
		"%s\x00%s\x00%d\x00%d",
		aws.ToString(etag),
		aws.ToString(versionID),
		aws.ToTime(lastModified).UnixNano(),
		aws.ToInt64(contentLength),
	)
}
