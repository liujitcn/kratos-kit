package wechat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cachekit "github.com/liujitcn/kratos-kit/cache"
	notify "github.com/liujitcn/kratos-kit/notify"
)

func TestOfficialSenderRendersTemplateData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/cgi-bin/token":
			if request.URL.Query().Get("appid") != "app-1" || request.URL.Query().Get("secret") != "secret-1" {
				t.Errorf("unexpected token query: %v", request.URL.Query())
			}
			_, _ = writer.Write([]byte(`{"access_token":"token-1","expires_in":7200}`))
		case "/cgi-bin/message/template/send":
			if request.URL.Query().Get("access_token") != "token-1" {
				t.Errorf("missing access token")
			}
			var body struct {
				ToUser     string                       `json:"touser"`
				TemplateID string                       `json:"template_id"`
				URL        string                       `json:"url"`
				Data       map[string]map[string]string `json:"data"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode template request: %v", err)
			}
			if body.ToUser != "openid-1" || body.TemplateID != "template-1" || body.URL != "https://example.test/task" || body.Data["task"]["value"] != "build" {
				t.Errorf("unexpected template request: %+v", body)
			}
			_, _ = writer.Write([]byte(`{"errcode":0,"msgid":123}`))
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
	sender, err := NewOfficialSender(Config{
		Name: "wechat-official", AppID: "app-1", AppSecret: "secret-1", BaseURL: server.URL + "/cgi-bin",
		AllowHTTP: true, Transport: server.Client(), Cache: cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{OpenID: "openid-1"}},
		Template:   &notify.Template{Code: "template-1", URL: "https://example.test/task", Params: map[string]string{"task": "build"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.MessageID != "123" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}
