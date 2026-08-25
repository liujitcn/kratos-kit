package mtls

import (
	"context"
	"crypto/x509"
	"errors"

	"github.com/go-kratos/kratos/v3/transport"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/liujitcn/kratos-kit/auth/authn/engine"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

var (
	// ErrMissingPeerCertificate 表示请求上下文中没有客户端证书。
	ErrMissingPeerCertificate = errors.New("mtls: peer certificate is required")
	// ErrMissingPeerSubject 表示客户端证书中没有可用身份。
	ErrMissingPeerSubject = errors.New("mtls: peer subject is required")
)

type peerCertificateKey struct{}

// ContextWithPeerCertificate 把客户端证书写入上下文，供自定义传输使用。
func ContextWithPeerCertificate(ctx context.Context, certificate *x509.Certificate) context.Context {
	return context.WithValue(ctx, peerCertificateKey{}, certificate)
}

// PeerCertificateFromContext 从上下文读取客户端证书。
func PeerCertificateFromContext(ctx context.Context) (*x509.Certificate, bool) {
	certificate, ok := ctx.Value(peerCertificateKey{}).(*x509.Certificate)
	return certificate, ok
}

// CertValidator 校验证书身份并返回关联声明。
type CertValidator func(subject string) (map[string]any, bool)

// SubjectExtractor 从证书提取业务身份。
type SubjectExtractor func(certificate *x509.Certificate) string

// Option 配置 mTLS 认证器。
type Option func(*options)

type options struct {
	trustedSubjects map[string]struct{}
	validator       CertValidator
	extractor       SubjectExtractor
}

// WithTrustedSubject 添加受信任的证书身份。
func WithTrustedSubject(subject string) Option {
	return func(options *options) {
		if options.trustedSubjects == nil {
			options.trustedSubjects = make(map[string]struct{})
		}
		options.trustedSubjects[subject] = struct{}{}
	}
}

// WithTrustedSubjects 配置受信任的证书身份集合。
func WithTrustedSubjects(subjects ...string) Option {
	return func(options *options) {
		options.trustedSubjects = make(map[string]struct{}, len(subjects))
		for _, subject := range subjects {
			options.trustedSubjects[subject] = struct{}{}
		}
	}
}

// WithValidator 配置外部证书身份校验函数。
func WithValidator(validator CertValidator) Option {
	return func(options *options) {
		options.validator = validator
	}
}

// WithSubjectExtractor 配置证书身份提取函数。
func WithSubjectExtractor(extractor SubjectExtractor) Option {
	return func(options *options) {
		options.extractor = extractor
	}
}

// Authenticator 校验 mTLS 客户端证书。
type Authenticator struct {
	options *options
}

var _ engine.RequestAuthenticator = (*Authenticator)(nil)
var _ engine.TokenAuthenticator = (*Authenticator)(nil)
var _ engine.AuthenticatorCloser = (*Authenticator)(nil)

// NewAuthenticator 创建 mTLS 认证器。
func NewAuthenticator(opts ...Option) (*Authenticator, error) {
	options := &options{extractor: defaultSubjectExtractor}
	for _, option := range opts {
		option(options)
	}
	return &Authenticator{options: options}, nil
}

// Authenticate 从 Kratos HTTP transport 或 gRPC peer 读取客户端证书并校验。
func (a *Authenticator) Authenticate(ctx context.Context, _ engine.ContextType, _ any) (*engine.AuthClaims, error) {
	certificate, ok := peerCertificate(ctx)
	if !ok {
		return nil, ErrMissingPeerCertificate
	}
	subject := a.options.extractor(certificate)
	if subject == "" {
		return nil, ErrMissingPeerSubject
	}
	return a.authenticateSubject(subject)
}

// AuthenticateToken 直接校验证书身份字符串。
func (a *Authenticator) AuthenticateToken(subject string) (*engine.AuthClaims, error) {
	if subject == "" {
		return nil, ErrMissingPeerSubject
	}
	return a.authenticateSubject(subject)
}

// Close 释放认证器资源。
func (a *Authenticator) Close() {}

// authenticateSubject 校验证书身份。
func (a *Authenticator) authenticateSubject(subject string) (*engine.AuthClaims, error) {
	if a.options.validator != nil {
		claims, valid := a.options.validator(subject)
		if !valid {
			return nil, engine.ErrUnauthenticated
		}
		authClaims := engine.AuthClaims(claims)
		return &authClaims, nil
	}
	if len(a.options.trustedSubjects) > 0 {
		if _, ok := a.options.trustedSubjects[subject]; !ok {
			return nil, engine.ErrUnauthenticated
		}
	}
	claims := engine.AuthClaims{engine.ClaimFieldSubject: subject}
	return &claims, nil
}

// peerCertificate 从自定义上下文、HTTP TLS 状态或 gRPC peer 中提取证书。
func peerCertificate(ctx context.Context) (*x509.Certificate, bool) {
	if certificate, ok := PeerCertificateFromContext(ctx); ok {
		return certificate, true
	}
	if transporter, ok := transport.FromServerContext(ctx); ok {
		if httpTransporter, matched := transporter.(http.Transporter); matched {
			request := httpTransporter.Request()
			if request.TLS != nil && len(request.TLS.PeerCertificates) > 0 {
				return request.TLS.PeerCertificates[0], true
			}
		}
	}
	peerInfo, ok := peer.FromContext(ctx)
	if !ok {
		return nil, false
	}
	switch tlsInfo := peerInfo.AuthInfo.(type) {
	case credentials.TLSInfo:
		if len(tlsInfo.State.PeerCertificates) > 0 {
			return tlsInfo.State.PeerCertificates[0], true
		}
	case *credentials.TLSInfo:
		if len(tlsInfo.State.PeerCertificates) > 0 {
			return tlsInfo.State.PeerCertificates[0], true
		}
	}
	return nil, false
}

// defaultSubjectExtractor 按 URI SAN、DNS SAN、Common Name 顺序提取身份。
func defaultSubjectExtractor(certificate *x509.Certificate) string {
	if len(certificate.URIs) > 0 {
		return certificate.URIs[0].String()
	}
	if len(certificate.DNSNames) > 0 {
		return certificate.DNSNames[0]
	}
	return certificate.Subject.CommonName
}
