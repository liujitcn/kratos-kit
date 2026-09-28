package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cachekit "github.com/liujitcn/kratos-kit/cache"
	notify "github.com/liujitcn/kratos-kit/notify"
)

func TestAppSenderGetsTenantTokenAndSendsByOpenID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			var body map[string]string
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode token request: %v", err)
			}
			if body["app_id"] != "app-1" || body["app_secret"] != "secret-1" {
				t.Errorf("unexpected token request: %+v", body)
			}
			_, _ = writer.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
		case "/open-apis/im/v1/messages":
			if request.Header.Get("Authorization") != "Bearer tenant-token" || request.URL.Query().Get("receive_id_type") != "open_id" {
				t.Errorf("missing auth or receive id type: %v", request.Header)
			}
			var body struct {
				ReceiveID string `json:"receive_id"`
				MsgType   string `json:"msg_type"`
				Content   string `json:"content"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode message request: %v", err)
			}
			var content map[string]string
			if err := json.Unmarshal([]byte(body.Content), &content); err != nil {
				t.Errorf("decode nested message content: %v", err)
			}
			if body.ReceiveID != "ou-1" || body.MsgType != "text" || content["text"] != "hello" {
				t.Errorf("unexpected message request: %+v, content=%+v", body, content)
			}
			_, _ = writer.Write([]byte(`{"code":0,"data":{"message_id":"msg-1"}}`))
		default:
			t.Errorf("unexpected path %s", request.URL.Path)
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	cache, cleanup, err := cachekit.NewCache(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	sender, err := NewAppSender(Config{
		Name: "feishu-app", AppID: "app-1", AppSecret: "secret-1", BaseURL: server.URL + "/open-apis",
		AllowHTTP: true, Transport: server.Client(), Cache: cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{OpenID: "ou-1"}}, Content: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.MessageID != "msg-1" || len(receipt.AcceptedRecipients) != 1 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}
