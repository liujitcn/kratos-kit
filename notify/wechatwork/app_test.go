package wechatwork

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cachekit "github.com/liujitcn/kratos-kit/cache"
	notify "github.com/liujitcn/kratos-kit/notify"
)

func TestAppSenderGetsTokenAndSendsToUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/cgi-bin/gettoken":
			if request.URL.Query().Get("corpid") != "corp-1" || request.URL.Query().Get("corpsecret") != "secret-1" {
				t.Errorf("unexpected token query: %v", request.URL.Query())
			}
			_, _ = writer.Write([]byte(`{"errcode":0,"access_token":"token-1","expires_in":7200}`))
		case "/cgi-bin/message/send":
			if request.URL.Query().Get("access_token") != "token-1" {
				t.Errorf("missing access token")
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode message request: %v", err)
			}
			text, _ := body["text"].(map[string]any)
			if body["touser"] != "user-1" || body["agentid"] != float64(7) || text["content"] != "hello" {
				t.Errorf("unexpected message request: %+v", body)
			}
			_, _ = writer.Write([]byte(`{"errcode":0,"msgid":"msg-1"}`))
		default:
			t.Errorf("unexpected request path %s", request.URL.Path)
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	cache, cleanup, err := cachekit.NewCache(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	sender, err := NewAppSender(AppConfig{
		Name: "wecom-app", CorpID: "corp-1", CorpSecret: "secret-1", AgentID: 7,
		BaseURL: server.URL + "/cgi-bin", AllowHTTP: true, Transport: server.Client(), Cache: cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{UserID: "user-1"}}, Content: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.MessageID != "msg-1" || len(receipt.AcceptedRecipients) != 1 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestAppSenderReportsInvalidUsers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/cgi-bin/gettoken" {
			_, _ = writer.Write([]byte(`{"errcode":0,"access_token":"token-1","expires_in":7200}`))
			return
		}
		_, _ = writer.Write([]byte(`{"errcode":0,"msgid":"msg-2","invaliduser":"missing-user"}`))
	}))
	defer server.Close()
	cache, cleanup, err := cachekit.NewCache(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	sender, err := NewAppSender(AppConfig{
		Name: "wecom-app", CorpID: "corp-1", CorpSecret: "secret-1", AgentID: 7,
		BaseURL: server.URL + "/cgi-bin", AllowHTTP: true, Transport: server.Client(), Cache: cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{UserID: "valid-user"}, {UserID: "missing-user"}}, Content: "hello",
	})
	if err == nil {
		t.Fatal("expected an error for the invalid user")
	}
	if receipt.MessageID != "msg-2" || len(receipt.AcceptedRecipients) != 1 || receipt.AcceptedRecipients[0] != "valid-user" {
		t.Fatalf("unexpected partial receipt: %+v", receipt)
	}
}
