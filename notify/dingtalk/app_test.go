package dingtalk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cachekit "github.com/liujitcn/kratos-kit/cache"
	notify "github.com/liujitcn/kratos-kit/notify"
)

func TestAppSenderResolvesUnionIDAndSendsWorkNotice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/gettoken":
			_, _ = writer.Write([]byte(`{"errcode":0,"access_token":"token-1","expires_in":7200}`))
		case "/topapi/user/getbyunionid":
			var body map[string]string
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["unionid"] != "union-1" {
				t.Errorf("unexpected union id lookup body: %+v, %v", body, err)
			}
			_, _ = writer.Write([]byte(`{"errcode":0,"result":{"userid":"user-1"}}`))
		case "/topapi/message/corpconversation/asyncsend_v2":
			if request.URL.Query().Get("access_token") != "token-1" {
				t.Errorf("access token missing from send request")
			}
			var body struct {
				AgentID    int64  `json:"agent_id"`
				UserIDList string `json:"userid_list"`
				Msg        struct {
					Text struct {
						Content string `json:"content"`
					} `json:"text"`
				} `json:"msg"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode send request: %v", err)
			}
			if body.AgentID != 12 || body.UserIDList != "user-1" || body.Msg.Text.Content != "hello" {
				t.Errorf("unexpected send request: %+v", body)
			}
			_, _ = writer.Write([]byte(`{"errcode":0,"request_id":"req-1","task_id":22}`))
		default:
			t.Errorf("unexpected request path: %s", request.URL.Path)
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
		Name: "dingtalk-app", AppKey: "key", AppSecret: "secret", AgentID: 12,
		BaseURL: server.URL, AllowHTTP: true, Transport: server.Client(), Cache: cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{UnionID: "union-1"}}, Content: "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.MessageID != "req-1" || len(receipt.AcceptedRecipients) != 1 || receipt.AcceptedRecipients[0] != "user-1" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

// TestAppAndGroupSendersShareProviderType 验证钉钉不同发送方式共用渠道类型。
func TestAppAndGroupSendersShareProviderType(t *testing.T) {
	if (&AppSender{}).Type() != notify.DingTalk || (&GroupSender{}).Type() != notify.DingTalk {
		t.Fatal("钉钉应用和群机器人应使用同一个渠道类型")
	}
	if string(notify.DingTalk) != "dingtalk" {
		t.Fatalf("unexpected provider type value: %q", notify.DingTalk)
	}
}
