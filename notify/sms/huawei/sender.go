package huawei

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/def"
	v1 "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smsapi/v1"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smsapi/v1/model"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/smsapi/v1/region"
	notify "github.com/liujitcn/kratos-kit/notify"
	"github.com/liujitcn/kratos-kit/notify/sms/internal/message"
)

const maxPhonesPerRequest = 500

// Config 描述华为云消息&短信渠道配置。
type Config struct {
	// Name 是渠道配置实例名。
	Name string
	// AppKey 是华为云短信应用 AppKey。
	AppKey string
	// AppSecret 是华为云短信应用 AppSecret。
	AppSecret string
	// Region 是短信服务区域，支持 cn-north-4 和 cn-south-1。
	Region string
	// Endpoint 用于专有网络或测试环境覆盖服务地址。
	Endpoint string
	// From 是短信发送方号码。
	From string
	// Signature 是短信签名。
	Signature string
	// Timeout 是 SDK 请求超时；非正数时使用 10 秒。
	Timeout time.Duration
}

// Sender 使用华为云 SMSAPI SDK 发送短信。
type Sender struct {
	name   string
	from   string
	sign   string
	client smsClient
}

type smsClient interface {
	// BatchSendSms 调用华为云批量短信发送接口。
	BatchSendSms(*model.BatchSendSmsRequest) (*model.BatchSendSmsResponse, error)
}

// New 创建华为云短信发送器。
func New(configValue Config) (*Sender, error) {
	if configValue.Name == "" || configValue.AppKey == "" || configValue.AppSecret == "" || configValue.From == "" || configValue.Signature == "" {
		return nil, errors.New("notify sms huawei: name, app credentials, sender number and signature are required")
	}
	regionID := configValue.Region
	if regionID == "" {
		regionID = "cn-north-4"
	}
	regionValue, err := region.SafeValueOf(regionID)
	if err != nil {
		return nil, fmt.Errorf("notify sms huawei: unsupported region %q: %w", regionID, err)
	}
	credential := v1.NewSMSApiCredentialsBuilder().WithAk(configValue.AppKey).WithSk(configValue.AppSecret).Build()
	httpConfig := config.DefaultHttpConfig().WithRetries(0)
	timeout := configValue.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	httpConfig.WithTimeout(timeout)
	builder := v1.SMSApiClientBuilder().WithCredential(credential).WithRegion(regionValue).WithHttpConfig(httpConfig)
	if configValue.Endpoint != "" {
		builder.WithEndpoint(configValue.Endpoint)
	}
	httpClient, err := builder.SafeBuild()
	if err != nil {
		return nil, fmt.Errorf("notify sms huawei: create client: %w", err)
	}
	return &Sender{name: configValue.Name, from: configValue.From, sign: configValue.Signature, client: v1.NewSMSApiClient(httpClient)}, nil
}

// Name 返回华为云短信渠道实例名。
func (s *Sender) Name() string { return s.name }

// Type 返回短信渠道类型。
func (s *Sender) Type() notify.Type { return notify.SMS }

// Send 按服务商模板参数顺序发送华为云短信。
func (s *Sender) Send(ctx context.Context, value notify.Message) (*notify.Receipt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	phones, err := message.Recipients(value)
	if err != nil {
		return nil, err
	}
	var params []string
	params, err = message.OrderedParams(value.Template)
	if err != nil {
		return nil, err
	}
	var paramJSON []byte
	paramJSON, err = json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("notify sms huawei: encode template params: %w", err)
	}
	receipt := &notify.Receipt{}
	for start := 0; start < len(phones); start += maxPhonesPerRequest {
		if err = ctx.Err(); err != nil {
			return receipt, err
		}
		end := min(start+maxPhonesPerRequest, len(phones))
		body := &model.BatchSendSmsRequestBody{
			From:          def.NewMultiPart(s.from),
			To:            def.NewMultiPart(strings.Join(phones[start:end], ",")),
			TemplateId:    def.NewMultiPart(value.Template.Code),
			TemplateParas: def.NewMultiPart(string(paramJSON)),
			Signature:     def.NewMultiPart(s.sign),
		}
		var response *model.BatchSendSmsResponse
		response, err = s.client.BatchSendSms(&model.BatchSendSmsRequest{Body: body})
		if err != nil {
			return receipt, fmt.Errorf("notify sms huawei: send request: %w", err)
		}
		if response == nil || response.Code == nil || *response.Code != "000000" {
			description := "empty provider response"
			if response != nil && response.Description != nil {
				description = *response.Description
			}
			return receipt, fmt.Errorf("notify sms huawei: provider rejected request: %s", description)
		}
		acceptedBeforeBatch := len(receipt.AcceptedRecipients)
		var failedStatus string
		if response.Result != nil {
			for _, item := range *response.Result {
				if item.Status == nil || *item.Status != "000000" {
					if failedStatus == "" {
						failedStatus = "unknown"
						if item.Status != nil {
							failedStatus = *item.Status
						}
					}
					continue
				}
				if item.SmsMsgId != nil && receipt.MessageID == "" {
					receipt.MessageID = *item.SmsMsgId
				}
				if item.OriginTo != nil {
					receipt.AcceptedRecipients = append(receipt.AcceptedRecipients, *item.OriginTo)
				}
			}
		}
		if failedStatus != "" {
			return receipt, fmt.Errorf("notify sms huawei: provider rejected one or more recipients with status %s", failedStatus)
		}
		if len(receipt.AcceptedRecipients) == acceptedBeforeBatch {
			return receipt, errors.New("notify sms huawei: provider response contains no recipient status")
		}
	}
	return receipt, nil
}
