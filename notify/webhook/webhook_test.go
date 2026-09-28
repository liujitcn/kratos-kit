package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	notify "github.com/liujitcn/kratos-kit/notify"
	"github.com/liujitcn/kratos-kit/notify/internal/platformhttp"
)

func TestSenderPostsJSONAndSignature(t *testing.T) {
	const secret = "webhook-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		var got payload
		if err = json.Unmarshal(body, &got); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		if got.Title != "系统提醒" || got.Content != "服务已恢复" {
			t.Errorf("unexpected payload: %+v", got)
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		if got, want := request.Header.Get(defaultSignatureHeader), "sha256="+hex.EncodeToString(mac.Sum(nil)); got != want {
			t.Errorf("signature mismatch: got %q want %q", got, want)
		}
		writer.Header().Set("X-Message-ID", "hook-1")
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sender, err := New(Config{
		Name: "ops", URL: server.URL, Secret: secret, AllowHTTP: true, AllowPrivateNetwork: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Title: "系统提醒", Content: "服务已恢复", ContentType: notify.Markdown,
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.MessageID != "hook-1" || len(receipt.AcceptedRecipients) != 0 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestNewRequiresHTTPSByDefault(t *testing.T) {
	_, err := New(Config{Name: "unsafe", URL: "http://example.test/hook"})
	if err == nil {
		t.Fatal("expected HTTP URL to be rejected")
	}
}

func TestSenderRejectsPrivateTargetsByDefault(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	sender, err := New(Config{Name: "private", URL: server.URL, AllowHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sender.Send(context.Background(), notify.Message{Content: "notice"}); err == nil {
		t.Fatal("expected private target to be rejected")
	}
}

func TestIsAllowedIPRejectsPrivateAndReservedAddresses(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "192.0.2.1", "198.18.0.1"} {
		if platformhttp.IsAllowedIP(net.ParseIP(value), false) {
			t.Errorf("expected %s to be rejected by default", value)
		}
	}
	if !platformhttp.IsAllowedIP(net.ParseIP("10.0.0.1"), true) {
		t.Fatal("explicit private network allowance should accept private unicast addresses")
	}
	if platformhttp.IsAllowedIP(net.ParseIP("169.254.169.254"), true) {
		t.Fatal("link-local metadata addresses must remain blocked")
	}
}
