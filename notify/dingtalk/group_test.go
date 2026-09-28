package dingtalk

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

func TestGroupSenderSignsAndSendsMarkdown(t *testing.T) {
	const secret = "robot-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		timestamp := request.URL.Query().Get("timestamp")
		parsed, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || parsed == 0 {
			t.Errorf("invalid signature timestamp %q", timestamp)
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(timestamp + "\n" + secret))
		if got, want := request.URL.Query().Get("sign"), base64.StdEncoding.EncodeToString(mac.Sum(nil)); got != want {
			t.Errorf("signature mismatch: got %q want %q", got, want)
		}
		var body struct {
			MsgType  string `json:"msgtype"`
			Markdown struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			} `json:"markdown"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode robot request: %v", err)
		}
		if body.MsgType != "markdown" || body.Markdown.Title != "notice" || body.Markdown.Text != "**hello**" {
			t.Errorf("unexpected robot request: %+v", body)
		}
		_, _ = writer.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	sender, err := NewGroupSender(GroupConfig{
		Name: "ops", URL: server.URL, Secret: secret, AllowHTTP: true, AllowPrivateNetwork: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = sender.Send(context.Background(), notify.Message{Title: "notice", Content: "**hello**", ContentType: notify.Markdown})
	if err != nil {
		t.Fatal(err)
	}
}
