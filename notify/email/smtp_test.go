package email

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	notify "github.com/liujitcn/kratos-kit/notify"
)

func TestSenderDeliversSMTPMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	messageReceived := make(chan string, 1)
	serverError := make(chan error, 1)
	go serveSMTP(listener, messageReceived, serverError)
	port := listener.Addr().(*net.TCPAddr).Port
	sender, err := New(Config{
		Name: "local-test", Host: "localhost", Port: port,
		From: "system@example.com", TLSMode: TLSNone, Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{Email: "user@example.com"}}, Title: "系统通知", Content: "发送成功",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.AcceptedRecipients) != 1 || receipt.AcceptedRecipients[0] != "user@example.com" || receipt.MessageID == "" {
		t.Fatalf("unexpected SMTP receipt: %+v", receipt)
	}
	select {
	case raw := <-messageReceived:
		if !strings.Contains(raw, "Subject: =?UTF-8?q?") || !strings.Contains(raw, "=E5=8F=91=E9=80=81=E6=88=90=E5=8A=9F") {
			t.Fatalf("message was not correctly encoded: %q", raw)
		}
	case err = <-serverError:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("SMTP server did not receive message")
	}
}

func TestNewRejectsInvalidTLSMode(t *testing.T) {
	_, err := New(Config{Name: "mail", Host: "localhost", Port: 25, From: "a@example.com", TLSMode: "AUTO"})
	if err == nil {
		t.Fatal("expected invalid TLS mode error")
	}
}

func serveSMTP(listener net.Listener, received chan<- string, serverError chan<- error) {
	conn, err := listener.Accept()
	if err != nil {
		serverError <- err
		return
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	if err = smtpReply(writer, "220 localhost ESMTP"); err != nil {
		serverError <- err
		return
	}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			serverError <- readErr
			return
		}
		command := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(command, "EHLO "):
			_, err = writer.WriteString("250-localhost\r\n250 OK\r\n")
			if err == nil {
				err = writer.Flush()
			}
		case strings.HasPrefix(command, "MAIL FROM:"), strings.HasPrefix(command, "RCPT TO:"):
			err = smtpReply(writer, "250 OK")
		case command == "DATA":
			err = smtpReply(writer, "354 End data with <CR><LF>.<CR><LF>")
			if err == nil {
				var message strings.Builder
				for {
					var bodyLine string
					bodyLine, err = reader.ReadString('\n')
					if err != nil {
						serverError <- err
						return
					}
					if bodyLine == ".\r\n" {
						break
					}
					message.WriteString(bodyLine)
				}
				received <- message.String()
				err = smtpReply(writer, "250 queued")
			}
		case command == "QUIT":
			err = smtpReply(writer, "221 bye")
			if err != nil {
				serverError <- err
			}
			return
		default:
			err = smtpReply(writer, fmt.Sprintf("250 OK: %s", command))
		}
		if err != nil {
			serverError <- err
			return
		}
	}
}

func smtpReply(writer *bufio.Writer, response string) error {
	if _, err := writer.WriteString(response + "\r\n"); err != nil {
		return err
	}
	return writer.Flush()
}
