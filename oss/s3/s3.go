package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	// ErrNilConfig 表示 S3 配置为空。
	ErrNilConfig = errors.New("s3: config is nil")
	// ErrNilClient 表示 S3 客户端为空。
	ErrNilClient = errors.New("s3: client is nil")
	// ErrEmptyBucket 表示 Bucket 为空。
	ErrEmptyBucket = errors.New("s3: bucket is empty")
	// ErrEmptyObjectKey 表示对象 Key 为空。
	ErrEmptyObjectKey = errors.New("s3: object key is empty")
	// ErrNilObjectBody 表示上传内容为空。
	ErrNilObjectBody = errors.New("s3: object body is nil")
	// ErrStreamingUploadUnsupported 表示自定义 S3 客户端未提供分片上传能力。
	ErrStreamingUploadUnsupported = errors.New("s3: client does not support streaming upload")
)

// Config 描述 S3 客户端配置。
type Config struct {
	// Endpoint 指定自定义 S3 服务地址。
	Endpoint string
	// Region 指定 AWS 区域。
	Region string
	// AccessKey 指定访问密钥。
	AccessKey string
	// SecretKey 指定秘密密钥。
	SecretKey string
	// Token 指定临时凭证令牌。
	Token string
	// UseSSL 指定自定义 Endpoint 未带协议时是否使用 HTTPS。
	UseSSL bool
	// ForcePathStyle 指定是否使用路径风格 URL。
	ForcePathStyle bool
	// Bucket 指定存储空间名称。
	Bucket string
	// RootDirectory 指定对象根目录。
	RootDirectory string
}

// Client 定义对象存储使用的 S3 SDK 能力。
type Client interface {
	// PutObject 上传一个对象。
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	// GetObject 下载一个对象。
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	// DeleteObject 删除一个对象。
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// uploader 定义非 seekable 输入使用的分片上传能力。
type uploader interface {
	Upload(context.Context, *s3.PutObjectInput, ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

// NewClient 根据配置创建 AWS S3 SDK 客户端。
func NewClient(ctx context.Context, cfg *Config) (*s3.Client, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	loadOpts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if cfg.AccessKey != "" || cfg.SecretKey != "" || cfg.Token != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, cfg.Token),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load AWS config: %w", err)
	}
	var endpoint string
	endpoint, err = normalizeEndpoint(cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.ForcePathStyle
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
	}), nil
}

// Storage 提供指定 Bucket 下的对象读写能力。
type Storage struct {
	client        Client
	uploader      uploader
	bucket        string
	rootDirectory string
}

// NewStorage 创建 S3 对象存储。
func NewStorage(ctx context.Context, cfg *Config) (*Storage, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if cfg.Bucket == "" {
		return nil, ErrEmptyBucket
	}
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Storage{
		client:        client,
		uploader:      manager.NewUploader(client),
		bucket:        cfg.Bucket,
		rootDirectory: cfg.RootDirectory,
	}, nil
}

// Client 返回底层 S3 客户端接口。
func (s *Storage) Client() Client {
	return s.client
}

// Bucket 返回当前 Bucket。
func (s *Storage) Bucket() string {
	return s.bucket
}

// PutObject 上传一个对象。
func (s *Storage) PutObject(ctx context.Context, key string, body io.Reader, contentType string) (*s3.PutObjectOutput, error) {
	if s.client == nil {
		return nil, ErrNilClient
	}
	if s.bucket == "" {
		return nil, ErrEmptyBucket
	}
	if key == "" {
		return nil, ErrEmptyObjectKey
	}
	if isNilReader(body) {
		return nil, ErrNilObjectBody
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	seeker, seekable := body.(io.ReadSeeker)
	if !seekable {
		return s.putObjectStream(ctx, key, input)
	}
	size, err := readerSize(seeker)
	if err != nil {
		return nil, fmt.Errorf("s3: prepare object %s: %w", key, err)
	}
	input.ContentLength = aws.Int64(size)
	var output *s3.PutObjectOutput
	output, err = s.client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("s3: put s3://%s/%s: %w", s.bucket, key, err)
	}
	return output, nil
}

// putObjectStream 使用 AWS 分片 uploader 流式上传非 seekable 请求体。
func (s *Storage) putObjectStream(ctx context.Context, key string, input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	objectUploader := s.uploader
	if objectUploader == nil {
		uploadClient, ok := s.client.(manager.UploadAPIClient)
		if !ok {
			return nil, ErrStreamingUploadUnsupported
		}
		objectUploader = manager.NewUploader(uploadClient)
	}
	output, err := objectUploader.Upload(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("s3: put s3://%s/%s: %w", s.bucket, key, err)
	}
	return uploadOutput(output), nil
}

// GetObject 下载一个对象，调用方负责关闭返回值的 Body。
func (s *Storage) GetObject(ctx context.Context, key string) (*s3.GetObjectOutput, error) {
	if s.client == nil {
		return nil, ErrNilClient
	}
	if s.bucket == "" {
		return nil, ErrEmptyBucket
	}
	if key == "" {
		return nil, ErrEmptyObjectKey
	}
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3: get s3://%s/%s: %w", s.bucket, key, err)
	}
	return output, nil
}

// DeleteObject 删除一个对象。
func (s *Storage) DeleteObject(ctx context.Context, key string) error {
	if s.client == nil {
		return ErrNilClient
	}
	if s.bucket == "" {
		return ErrEmptyBucket
	}
	if key == "" {
		return ErrEmptyObjectKey
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3: delete s3://%s/%s: %w", s.bucket, key, err)
	}
	return nil
}

// Upload 上传本地文件并返回相对对象路径。
func (s *Storage) Upload(fileName string, filePath string, localFile string) (string, error) {
	return s.UploadContext(context.Background(), fileName, filePath, localFile)
}

// UploadContext 使用指定 context 上传本地文件。
func (s *Storage) UploadContext(ctx context.Context, fileName string, filePath string, localFile string) (string, error) {
	file, err := os.Open(localFile)
	if err != nil {
		return "", fmt.Errorf("s3: open %s: %w", localFile, err)
	}
	key := s.objectKey(fileName, filePath)
	_, err = s.PutObject(ctx, key, file, "application/octet-stream")
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", fmt.Errorf("s3: close %s: %w", localFile, closeErr)
	}
	return s.relativePath(key), nil
}

// UploadByByte 上传内存数据并返回相对对象路径。
func (s *Storage) UploadByByte(fileName string, filePath string, fileByte []byte) (string, error) {
	return s.UploadByByteContext(context.Background(), fileName, filePath, fileByte)
}

// UploadByByteContext 使用指定 context 上传内存数据。
func (s *Storage) UploadByByteContext(ctx context.Context, fileName string, filePath string, fileByte []byte) (string, error) {
	key := s.objectKey(fileName, filePath)
	_, err := s.PutObject(ctx, key, bytes.NewReader(fileByte), "application/octet-stream")
	if err != nil {
		return "", err
	}
	return s.relativePath(key), nil
}

// GetFileByte 下载对象内容。
func (s *Storage) GetFileByte(filePath string) ([]byte, error) {
	return s.GetFileByteContext(context.Background(), filePath)
}

// GetFileByteContext 使用指定 context 下载对象内容。
func (s *Storage) GetFileByteContext(ctx context.Context, filePath string) ([]byte, error) {
	output, err := s.GetObject(ctx, path.Join(s.rootDirectory, filePath))
	if err != nil {
		return nil, err
	}
	var value []byte
	value, err = io.ReadAll(output.Body)
	closeErr := output.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("s3: read object %s: %w", filePath, err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("s3: close object %s: %w", filePath, closeErr)
	}
	return value, nil
}

// DeleteFile 删除对象。
func (s *Storage) DeleteFile(filePath string) error {
	return s.DeleteFileContext(context.Background(), filePath)
}

// DeleteFileContext 使用指定 context 删除对象。
func (s *Storage) DeleteFileContext(ctx context.Context, filePath string) error {
	return s.DeleteObject(ctx, path.Join(s.rootDirectory, filePath))
}

// objectKey 生成带根目录的对象 Key。
func (s *Storage) objectKey(fileName string, filePath string) string {
	return path.Join(s.rootDirectory, filePath, filepath.Base(fileName))
}

// relativePath 将完整 Key 转换为与现有 OSS 实现一致的相对路径。
func (s *Storage) relativePath(key string) string {
	root := strings.TrimSuffix(path.Clean(s.rootDirectory), "/")
	if root == "." || root == "" {
		return key
	}
	return strings.TrimPrefix(key, root)
}

// normalizeEndpoint 补齐并校验自定义 S3 endpoint。
func normalizeEndpoint(endpoint string, useSSL bool) (string, error) {
	if endpoint == "" {
		return "", nil
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		scheme := "http"
		if useSSL {
			scheme = "https"
		}
		endpoint = scheme + "://" + endpoint
	}
	_, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return "", fmt.Errorf("s3: invalid endpoint: %w", err)
	}
	return endpoint, nil
}

// isNilReader 判断 Reader 接口是否包含类型化 nil。
func isNilReader(body io.Reader) bool {
	if body == nil {
		return true
	}
	value := reflect.ValueOf(body)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// readerSize 计算 Reader 从当前位置到结尾的字节数并恢复位置。
func readerSize(reader io.ReadSeeker) (int64, error) {
	current, err := reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	var end int64
	end, err = reader.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	if _, err = reader.Seek(current, io.SeekStart); err != nil {
		return 0, err
	}
	return end - current, nil
}

// uploadOutput 将分片 uploader 输出映射为现有 PutObject 返回类型。
func uploadOutput(output *manager.UploadOutput) *s3.PutObjectOutput {
	result := &s3.PutObjectOutput{
		ChecksumCRC32:        output.ChecksumCRC32,
		ChecksumCRC32C:       output.ChecksumCRC32C,
		ChecksumCRC64NVME:    output.ChecksumCRC64NVME,
		ChecksumSHA1:         output.ChecksumSHA1,
		ChecksumSHA256:       output.ChecksumSHA256,
		ChecksumType:         output.ChecksumType,
		ETag:                 output.ETag,
		Expiration:           output.Expiration,
		RequestCharged:       output.RequestCharged,
		SSEKMSKeyId:          output.SSEKMSKeyId,
		ServerSideEncryption: output.ServerSideEncryption,
		VersionId:            output.VersionID,
	}
	if output.BucketKeyEnabled {
		result.BucketKeyEnabled = aws.Bool(true)
	}
	return result
}
