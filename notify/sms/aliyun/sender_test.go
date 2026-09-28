package aliyun

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/dara"
	notify "github.com/liujitcn/kratos-kit/notify"
)

type fakeSMSClient struct {
	request  *client.SendSmsRequest
	response *client.SendSmsResponse
}

// SendSmsWithContext 记录 SDK 请求并返回预设响应。
func (c *fakeSMSClient) SendSmsWithContext(_ context.Context, request *client.SendSmsRequest, _ *dara.RuntimeOptions) (*client.SendSmsResponse, error) {
	c.request = request
	return c.response, nil
}

// TestSendBuildsProviderRequestAndReturnsReceipt 验证阿里云请求映射和发送回执。
func TestSendBuildsProviderRequestAndReturnsReceipt(t *testing.T) {
	code, messageID, requestID := "OK", "biz-1", "request-1"
	api := &fakeSMSClient{response: &client.SendSmsResponse{Body: &client.SendSmsResponseBody{
		Code: &code, BizId: &messageID, RequestId: &requestID,
	}}}
	sender := &Sender{name: "aliyun", sign: "notice", client: api}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{Phone: "+8613800000000"}},
		Template:   &notify.Template{Code: "SMS_1", Params: map[string]string{"code": "1234"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if api.request.GetPhoneNumbers() == nil || *api.request.GetPhoneNumbers() != "+8613800000000" {
		t.Fatalf("unexpected phone numbers: %+v", api.request.GetPhoneNumbers())
	}
	if api.request.GetSignName() == nil || *api.request.GetSignName() != "notice" || api.request.GetTemplateCode() == nil || *api.request.GetTemplateCode() != "SMS_1" {
		t.Fatalf("unexpected SMS request: %+v", api.request)
	}
	var params map[string]string
	if err = json.Unmarshal([]byte(*api.request.GetTemplateParam()), &params); err != nil {
		t.Fatal(err)
	}
	if params["code"] != "1234" || receipt.MessageID != "biz-1" || len(receipt.AcceptedRecipients) != 1 {
		t.Fatalf("unexpected params or receipt: params=%v receipt=%+v", params, receipt)
	}
}
