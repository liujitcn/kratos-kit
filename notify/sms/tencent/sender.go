package tencent

import (
	"context"
	"errors"
	"fmt"

	notify "github.com/liujitcn/kratos-kit/notify"
	"github.com/liujitcn/kratos-kit/notify/sms/internal/message"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	v20210111 "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

const maxPhonesPerRequest = 200

// Config 描述腾讯云短信渠道配置。
type Config struct {
	// Name 是渠道配置实例名。
	Name string
	// SecretID 是腾讯云 SecretId。
	SecretID string
	// SecretKey 是腾讯云 SecretKey。
	SecretKey string
	// SDKAppID 是短信应用 ID。
	SDKAppID string
	// SignName 是已审核通过的短信签名。
	SignName string
	// Region 是腾讯云短信服务区域；留空使用 ap-guangzhou。
	Region string
	// Endpoint 用于覆盖腾讯云短信服务地址。
	Endpoint string
}

// Sender 使用腾讯云短信 SDK 发送短信。
type Sender struct {
	name     string
	appID    string
	signName string
	client   smsClient
}

type smsClient interface {
	// SendSmsWithContext 调用腾讯云短信发送接口。
	SendSmsWithContext(context.Context, *v20210111.SendSmsRequest) (*v20210111.SendSmsResponse, error)
}

// New 创建腾讯云短信发送器。
func New(config Config) (*Sender, error) {
	if config.Name == "" || config.SecretID == "" || config.SecretKey == "" || config.SDKAppID == "" || config.SignName == "" {
		return nil, errors.New("notify sms tencent: name, credentials, sdk app id and sign name are required")
	}
	region := config.Region
	if region == "" {
		region = "ap-guangzhou"
	}
	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.Endpoint = "sms.tencentcloudapi.com"
	if config.Endpoint != "" {
		clientProfile.HttpProfile.Endpoint = config.Endpoint
	}
	sdkClient, err := v20210111.NewClient(common.NewCredential(config.SecretID, config.SecretKey), region, clientProfile)
	if err != nil {
		return nil, fmt.Errorf("notify sms tencent: create client: %w", err)
	}
	return &Sender{name: config.Name, appID: config.SDKAppID, signName: config.SignName, client: sdkClient}, nil
}

// Name 返回腾讯云短信渠道实例名。
func (s *Sender) Name() string { return s.name }

// Type 返回短信渠道类型。
func (s *Sender) Type() notify.Type { return notify.SMS }

// Send 按服务商模板顺序发送腾讯云短信。
func (s *Sender) Send(ctx context.Context, value notify.Message) (*notify.Receipt, error) {
	phones, err := message.Recipients(value)
	if err != nil {
		return nil, err
	}
	params, err := message.OrderedParams(value.Template)
	if err != nil {
		return nil, err
	}
	requestParams := make([]*string, 0, len(params))
	for _, param := range params {
		value := param
		requestParams = append(requestParams, &value)
	}
	receipt := &notify.Receipt{}
	for start := 0; start < len(phones); start += maxPhonesPerRequest {
		end := min(start+maxPhonesPerRequest, len(phones))
		phoneParams := make([]*string, 0, end-start)
		for _, phone := range phones[start:end] {
			value := phone
			phoneParams = append(phoneParams, &value)
		}
		templateID := value.Template.Code
		appID := s.appID
		signName := s.signName
		request := v20210111.NewSendSmsRequest()
		request.PhoneNumberSet = phoneParams
		request.SmsSdkAppId = &appID
		request.TemplateId = &templateID
		request.SignName = &signName
		request.TemplateParamSet = requestParams
		response, sendErr := s.client.SendSmsWithContext(ctx, request)
		if sendErr != nil {
			return receipt, fmt.Errorf("notify sms tencent: send request: %w", sendErr)
		}
		if response == nil || response.Response == nil {
			return receipt, errors.New("notify sms tencent: empty provider response")
		}
		if response.Response.RequestId != nil {
			receipt.MessageID = *response.Response.RequestId
		}
		for _, status := range response.Response.SendStatusSet {
			if status == nil || status.Code == nil || *status.Code != "Ok" {
				return receipt, errors.New("notify sms tencent: provider rejected one or more recipients")
			}
			if status.PhoneNumber != nil {
				receipt.AcceptedRecipients = append(receipt.AcceptedRecipients, *status.PhoneNumber)
			}
		}
	}
	return receipt, nil
}
