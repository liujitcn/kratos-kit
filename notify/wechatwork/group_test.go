package wechatwork

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	notify "github.com/liujitcn/kratos-kit/notify"
)

func TestGroupSenderPostsRobotMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("key") != "robot-key" {
			t.Errorf("robot key missing from webhook URL")
		}
		var body struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode robot request: %v", err)
		}
		if body.MsgType != "text" || body.Text.Content != "maintenance" {
			t.Errorf("unexpected robot request: %+v", body)
		}
		_, _ = writer.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()
	sender, err := NewGroupSender(GroupConfig{
		Name: "ops-room", URL: server.URL + "?key=robot-key", AllowHTTP: true, AllowPrivateNetwork: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sender.Send(context.Background(), notify.Message{Content: "maintenance"}); err != nil {
		t.Fatal(err)
	}
}
