package aliyun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/dara"
	notify "github.com/liujitcn/kratos-kit/notify"
	"github.com/liujitcn/kratos-kit/notify/sms/internal/message"
)

// Config 描述阿里云短信渠道配置。
type Config struct {
	// Name 是渠道配置实例名。
	Name string
	// AccessKeyID 是阿里云访问密钥 ID。
	AccessKeyID string
	// AccessKeySecret 是阿里云访问密钥。
	AccessKeySecret string
	// SignName 是已审核通过的短信签名。
	SignName string
	// Region 是短信服务区域；留空使用 cn-hangzhou。
	Region string
	// Endpoint 用于专有网络或测试环境覆盖服务地址。
	Endpoint string
}

// Sender 使用阿里云短信 V2 SDK 发送短信。
type Sender struct {
	name   string
	sign   string
	client smsClient
}

type smsClient interface {
	// SendSmsWithContext 调用阿里云短信发送接口。
	SendSmsWithContext(context.Context, *client.SendSmsRequest, *dara.RuntimeOptions) (*client.SendSmsResponse, error)
}

// New 创建阿里云短信发送器。
func New(config Config) (*Sender, error) {
	if config.Name == "" || config.AccessKeyID == "" || config.AccessKeySecret == "" || config.SignName == "" {
		return nil, errors.New("notify sms aliyun: name, access key, secret and sign name are required")
	}
	region := config.Region
	if region == "" {
		region = "cn-hangzhou"
	}
	sdkConfig := &utils.Config{
		AccessKeyId:     dara.String(config.AccessKeyID),
		AccessKeySecret: dara.String(config.AccessKeySecret),
		RegionId:        dara.String(region),
	}
	if config.Endpoint != "" {
		sdkConfig.Endpoint = dara.String(config.Endpoint)
	}
	sdkClient, err := client.NewClient(sdkConfig)
	if err != nil {
		return nil, fmt.Errorf("notify sms aliyun: create client: %w", err)
	}
	return &Sender{name: config.Name, sign: config.SignName, client: sdkClient}, nil
}

// Name 返回阿里云短信渠道实例名。
func (s *Sender) Name() string { return s.name }

// Type 返回短信渠道类型。
func (s *Sender) Type() notify.Type { return notify.SMS }

// Send 使用审核模板向收件人发送阿里云短信。
func (s *Sender) Send(ctx context.Context, value notify.Message) (*notify.Receipt, error) {
	phones, err := message.Recipients(value)
	if err != nil {
		return nil, err
	}
	params := ""
	if len(value.Template.Params) > 0 {
		encoded, marshalErr := json.Marshal(value.Template.Params)
		if marshalErr != nil {
			return nil, fmt.Errorf("notify sms aliyun: encode template params: %w", marshalErr)
		}
		params = string(encoded)
	}
	request := (&client.SendSmsRequest{}).
		SetPhoneNumbers(strings.Join(phones, ",")).
		SetSignName(s.sign).
		SetTemplateCode(value.Template.Code)
	if params != "" {
		request.SetTemplateParam(params)
	}
	response, err := s.client.SendSmsWithContext(ctx, request, &dara.RuntimeOptions{})
	if err != nil {
		return nil, fmt.Errorf("notify sms aliyun: send request: %w", err)
	}
	if response == nil || response.Body == nil {
		return nil, errors.New("notify sms aliyun: empty provider response")
	}
	body := response.Body
	if body.GetCode() == nil || *body.GetCode() != "OK" {
		return nil, fmt.Errorf("notify sms aliyun: provider rejected request: %s", dereference(body.GetCode()))
	}
	messageID := dereference(body.GetBizId())
	if messageID == "" {
		messageID = dereference(body.GetRequestId())
	}
	return &notify.Receipt{MessageID: messageID, AcceptedRecipients: phones}, nil
}

// dereference 返回字符串指针的值。
func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
