package tencent

import (
	"context"
	"testing"

	notify "github.com/liujitcn/kratos-kit/notify"
	v20210111 "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

type fakeSMSClient struct {
	request  *v20210111.SendSmsRequest
	response *v20210111.SendSmsResponse
}

// SendSmsWithContext 记录 SDK 请求并返回预设响应。
func (c *fakeSMSClient) SendSmsWithContext(_ context.Context, request *v20210111.SendSmsRequest) (*v20210111.SendSmsResponse, error) {
	c.request = request
	return c.response, nil
}

// TestSendBuildsProviderRequestAndReturnsReceipt 验证腾讯云请求映射和发送回执。
func TestSendBuildsProviderRequestAndReturnsReceipt(t *testing.T) {
	requestID, statusCode, phone := "request-1", "Ok", "+8613800000000"
	api := &fakeSMSClient{response: &v20210111.SendSmsResponse{Response: &v20210111.SendSmsResponseParams{
		RequestId:     &requestID,
		SendStatusSet: []*v20210111.SendStatus{{PhoneNumber: &phone, Code: &statusCode}},
	}}}
	sender := &Sender{name: "tencent", appID: "app-1", signName: "notice", client: api}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{Phone: phone}},
		Template:   &notify.Template{Code: "1001", OrderedParams: []string{"1234", "5"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(api.request.PhoneNumberSet) != 1 || *api.request.PhoneNumberSet[0] != phone || *api.request.SmsSdkAppId != "app-1" || *api.request.SignName != "notice" || *api.request.TemplateId != "1001" {
		t.Fatalf("unexpected SMS request: %+v", api.request)
	}
	if len(api.request.TemplateParamSet) != 2 || *api.request.TemplateParamSet[0] != "1234" || *api.request.TemplateParamSet[1] != "5" {
		t.Fatalf("unexpected template parameters: %+v", api.request.TemplateParamSet)
	}
	if receipt.MessageID != requestID || len(receipt.AcceptedRecipients) != 1 || receipt.AcceptedRecipients[0] != phone {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}
