package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	notify "github.com/liujitcn/kratos-kit/notify"
)

// TLSMode 表示 SMTP 连接的 TLS 模式。
type TLSMode string

const (
	// TLSNone 表示不启用 TLS。
	TLSNone TLSMode = "NONE"
	// TLSStartTLS 表示连接后升级到 TLS。
	TLSStartTLS TLSMode = "START_TLS"
	// TLSSSL 表示连接建立时直接启用 TLS。
	TLSSSL TLSMode = "SSL"
)

// Config 描述一个 SMTP 渠道实例。
type Config struct {
	// Name 是渠道配置实例名，在一个 Manager 中必须唯一。
	Name string
	// Host 是 SMTP 服务器主机名。
	Host string
	// Port 是 SMTP 服务端口。
	Port int
	// Username 是 SMTP 认证用户名。
	Username string
	// Password 是 SMTP 认证密码或授权码。
	Password string
	// From 是发件人邮箱地址。
	From string
	// TLSMode 是连接加密模式；留空默认 START_TLS。
	TLSMode TLSMode
	// Timeout 是 SMTP 单次连接超时；非正数时使用 10 秒。
	Timeout time.Duration
}

// Sender 使用 SMTP 发送邮件。
type Sender struct {
	name     string
	host     string
	address  string
	username string
	password string
	from     *mail.Address
	tlsMode  TLSMode
	timeout  time.Duration
}

// New 创建并校验 SMTP 发送器。
func New(config Config) (*Sender, error) {
	if config.Name == "" || config.Host == "" || strings.ContainsAny(config.Host, "\r\n<>@ ") || config.Port < 1 || config.Port > 65535 {
		return nil, errors.New("notify email: name, host and valid port are required")
	}
	if strings.ContainsAny(config.From, "\r\n") {
		return nil, errors.New("notify email: from address must not contain line breaks")
	}
	from, err := mail.ParseAddress(config.From)
	if err != nil || from.Address == "" || strings.ContainsAny(from.Address, "\r\n") {
		return nil, errors.New("notify email: valid from address is required")
	}
	tlsMode := config.TLSMode
	if tlsMode == "" {
		tlsMode = TLSStartTLS
	}
	if tlsMode != TLSNone && tlsMode != TLSStartTLS && tlsMode != TLSSSL {
		return nil, fmt.Errorf("notify email: unsupported TLS mode %q", tlsMode)
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Sender{
		name: config.Name, host: config.Host,
		address:  net.JoinHostPort(config.Host, strconv.Itoa(config.Port)),
		username: config.Username, password: config.Password, from: from,
		tlsMode: tlsMode, timeout: timeout,
	}, nil
}

// Name 返回 SMTP 渠道配置实例名。
func (s *Sender) Name() string { return s.name }

// Type 返回 SMTP 邮件渠道类型。
func (s *Sender) Type() notify.Type { return notify.Email }

// Send 使用 SMTP 将消息发送给全部收件人。
func (s *Sender) Send(ctx context.Context, message notify.Message) (*notify.Receipt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(message.Recipients) == 0 {
		return nil, errors.New("notify email: at least one recipient is required")
	}
	if strings.ContainsAny(message.Title, "\r\n") {
		return nil, errors.New("notify email: title must not contain line breaks")
	}
	if message.ContentType != "" && message.ContentType != notify.PlainText && message.ContentType != notify.Markdown && message.ContentType != notify.HTML {
		return nil, fmt.Errorf("notify email: unsupported content type %q", message.ContentType)
	}
	recipients := make([]*mail.Address, 0, len(message.Recipients))
	for _, recipient := range message.Recipients {
		value := recipient.Email
		if strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("notify email: invalid recipient address")
		}
		address, err := mail.ParseAddress(value)
		if err != nil || address.Address == "" || strings.ContainsAny(address.Address, "\r\n") {
			return nil, errors.New("notify email: invalid recipient address")
		}
		recipients = append(recipients, address)
	}

	conn, err := (&net.Dialer{Timeout: s.timeout}).DialContext(ctx, "tcp", s.address)
	if err != nil {
		return nil, fmt.Errorf("notify email: connect SMTP: %w", err)
	}
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(s.timeout))
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.host}
	if s.tlsMode == TLSSSL {
		tlsConn := tls.Client(conn, tlsConfig)
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			return nil, fmt.Errorf("notify email: SMTP TLS handshake: %w", err)
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return nil, fmt.Errorf("notify email: initialize SMTP client: %w", err)
	}
	defer client.Close()
	if s.tlsMode == TLSStartTLS {
		if supported, _ := client.Extension("STARTTLS"); !supported {
			return nil, errors.New("notify email: SMTP server does not support STARTTLS")
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return nil, fmt.Errorf("notify email: start TLS: %w", err)
		}
	}
	if s.username != "" {
		if err = client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
			return nil, fmt.Errorf("notify email: SMTP authentication: %w", err)
		}
	}
	if err = client.Mail(s.from.Address); err != nil {
		return nil, fmt.Errorf("notify email: set sender: %w", err)
	}
	for _, recipient := range recipients {
		if err = client.Rcpt(recipient.Address); err != nil {
			return nil, fmt.Errorf("notify email: set recipient: %w", err)
		}
	}
	data, err := client.Data()
	if err != nil {
		return nil, fmt.Errorf("notify email: start message data: %w", err)
	}
	messageID, err := writeMessage(data, s.from, recipients, message, s.host)
	if err != nil {
		_ = data.Close()
		return nil, fmt.Errorf("notify email: write message: %w", err)
	}
	if err = data.Close(); err != nil {
		return nil, fmt.Errorf("notify email: finish message: %w", err)
	}
	accepted := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		accepted = append(accepted, recipient.Address)
	}
	return &notify.Receipt{MessageID: messageID, AcceptedRecipients: accepted}, nil
}

// writeMessage 编码邮件头和正文并写入 SMTP DATA 流。
func writeMessage(writer io.Writer, from *mail.Address, recipients []*mail.Address, message notify.Message, host string) (string, error) {
	messageIDBytes := make([]byte, 16)
	if _, err := rand.Read(messageIDBytes); err != nil {
		return "", err
	}
	messageID := fmt.Sprintf("<%s@%s>", hex.EncodeToString(messageIDBytes), host)
	contentType := "text/plain"
	if message.ContentType == notify.HTML {
		contentType = "text/html"
	}
	formattedRecipients := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		formattedRecipients = append(formattedRecipients, recipient.String())
	}
	if _, err := fmt.Fprintf(writer,
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n",
		from.String(), strings.Join(formattedRecipients, ", "), mime.QEncoding.Encode("UTF-8", message.Title),
		time.Now().Format(time.RFC1123Z), messageID, contentType,
	); err != nil {
		return "", err
	}
	encoder := quotedprintable.NewWriter(writer)
	if _, err := io.WriteString(encoder, message.Content); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	return messageID, nil
}
