package feishu

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	notify "github.com/liujitcn/kratos-kit/notify"
)

func TestGroupSenderSignsMessage(t *testing.T) {
	const secret = "feishu-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		timestamp := request.URL.Query().Get("timestamp")
		if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
			t.Errorf("invalid timestamp: %q", timestamp)
		}
		mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
		_, _ = mac.Write(nil)
		if got, want := request.URL.Query().Get("sign"), base64.StdEncoding.EncodeToString(mac.Sum(nil)); got != want {
			t.Errorf("signature mismatch: got %q want %q", got, want)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode robot payload: %v", err)
		}
		if body["msg_type"] != "text" {
			t.Errorf("unexpected robot payload: %+v", body)
		}
		_, _ = writer.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer server.Close()

	sender, err := NewGroupSender(GroupConfig{
		Name: "feishu-room", URL: server.URL, Secret: secret, AllowHTTP: true, AllowPrivateNetwork: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sender.Send(context.Background(), notify.Message{Content: "hello"}); err != nil {
		t.Fatal(err)
	}
}
