package huawei

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smsapi/v1/model"
	notify "github.com/liujitcn/kratos-kit/notify"
)

type fakeSMSClient struct {
	request   *model.BatchSendSmsRequest
	requests  []*model.BatchSendSmsRequest
	response  *model.BatchSendSmsResponse
	responses []*model.BatchSendSmsResponse
}

// BatchSendSms 记录 SDK 请求并返回预设响应。
func (c *fakeSMSClient) BatchSendSms(request *model.BatchSendSmsRequest) (*model.BatchSendSmsResponse, error) {
	c.request = request
	c.requests = append(c.requests, request)
	if len(c.responses) > 0 {
		response := c.responses[0]
		c.responses = c.responses[1:]
		return response, nil
	}
	return c.response, nil
}

// TestSendBuildsProviderRequestAndReturnsReceipt 验证华为云请求映射和发送回执。
func TestSendBuildsProviderRequestAndReturnsReceipt(t *testing.T) {
	status, messageID, phone := "000000", "sms-1", "+8613800000000"
	api := &fakeSMSClient{response: &model.BatchSendSmsResponse{
		Code:   &status,
		Result: &[]model.SmsId{{Status: &status, SmsMsgId: &messageID, OriginTo: &phone}},
	}}
	sender := &Sender{name: "huawei", from: "1069", sign: "notice", client: api}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{Phone: phone}},
		Template:   &notify.Template{Code: "template-abc123", OrderedParams: []string{"1234", "5"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := api.request.Body
	if body.From.Content != "1069" || body.To.Content != phone || body.TemplateId.Content != "template-abc123" || body.Signature.Content != "notice" || body.TemplateParas.Content != `["1234","5"]` {
		t.Fatalf("unexpected SMS request: %+v", body)
	}
	if receipt.MessageID != messageID || len(receipt.AcceptedRecipients) != 1 || receipt.AcceptedRecipients[0] != phone {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

// TestSendSplitsRequestsAtProviderLimit 验证超过服务商上限时会分批发送并合并回执。
func TestSendSplitsRequestsAtProviderLimit(t *testing.T) {
	phones := make([]string, maxPhonesPerRequest+1)
	recipients := make([]notify.Recipient, 0, len(phones))
	for index := range phones {
		phones[index] = fmt.Sprintf("+86138%08d", index)
		recipients = append(recipients, notify.Recipient{Phone: phones[index]})
	}

	success := "000000"
	makeResponse := func(batch []string, messageID string) *model.BatchSendSmsResponse {
		statuses := make([]model.SmsId, 0, len(batch))
		for _, phone := range batch {
			phoneValue := phone
			statusValue := success
			idValue := messageID
			statuses = append(statuses, model.SmsId{Status: &statusValue, SmsMsgId: &idValue, OriginTo: &phoneValue})
		}
		return &model.BatchSendSmsResponse{Code: &success, Result: &statuses}
	}
	api := &fakeSMSClient{responses: []*model.BatchSendSmsResponse{
		makeResponse(phones[:maxPhonesPerRequest], "sms-1"),
		makeResponse(phones[maxPhonesPerRequest:], "sms-2"),
	}}
	sender := &Sender{name: "huawei", from: "1069", sign: "notice", client: api}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: recipients,
		Template:   &notify.Template{Code: "template-abc123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(api.requests) != 2 {
		t.Fatalf("expected two request batches, got %d", len(api.requests))
	}
	firstBatch, ok := api.requests[0].Body.To.Content.(string)
	if !ok || len(strings.Split(firstBatch, ",")) != maxPhonesPerRequest {
		t.Fatalf("unexpected first request batch: %+v", api.requests[0].Body.To.Content)
	}
	secondBatch, ok := api.requests[1].Body.To.Content.(string)
	if !ok || secondBatch != phones[maxPhonesPerRequest] {
		t.Fatalf("unexpected request batches: %+v", api.requests)
	}
	if receipt.MessageID != "sms-1" || len(receipt.AcceptedRecipients) != len(phones) {
		t.Fatalf("unexpected merged receipt: %+v", receipt)
	}
}

// TestSendReportsPartialRecipientFailure 验证华为云逐收件人失败状态不会被记为已受理。
func TestSendReportsPartialRecipientFailure(t *testing.T) {
	success, rejected, firstPhone, secondPhone := "000000", "E000000", "+8613800000000", "+8613900000000"
	api := &fakeSMSClient{response: &model.BatchSendSmsResponse{
		Code: &success,
		Result: &[]model.SmsId{
			{Status: &success, OriginTo: &firstPhone},
			{Status: &rejected, OriginTo: &secondPhone},
		},
	}}
	sender := &Sender{name: "huawei", from: "1069", sign: "notice", client: api}
	receipt, err := sender.Send(context.Background(), notify.Message{
		Recipients: []notify.Recipient{{Phone: firstPhone}, {Phone: secondPhone}},
		Template:   &notify.Template{Code: "12345"},
	})
	if err == nil || len(receipt.AcceptedRecipients) != 1 || receipt.AcceptedRecipients[0] != firstPhone {
		t.Fatalf("expected one accepted recipient and a partial error, got receipt=%+v error=%v", receipt, err)
	}
}
