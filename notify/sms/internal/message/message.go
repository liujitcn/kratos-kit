package message

import (
	"errors"

	notify "github.com/liujitcn/kratos-kit/notify"
)

// Recipients 校验短信模板和手机号并提取接收号码。
func Recipients(value notify.Message) ([]string, error) {
	if value.Template == nil || value.Template.Code == "" {
		return nil, errors.New("notify sms: approved template code is required")
	}
	if len(value.Recipients) == 0 {
		return nil, errors.New("notify sms: at least one recipient is required")
	}
	phones := make([]string, 0, len(value.Recipients))
	for _, recipient := range value.Recipients {
		if recipient.Phone == "" {
			return nil, errors.New("notify sms: recipient phone is required")
		}
		phones = append(phones, recipient.Phone)
	}
	return phones, nil
}

// OrderedParams 校验短信服务商要求的位置参数。
func OrderedParams(template *notify.Template) ([]string, error) {
	if template == nil {
		return nil, errors.New("notify sms: template is required")
	}
	if len(template.OrderedParams) > 0 {
		return append([]string(nil), template.OrderedParams...), nil
	}
	if len(template.Params) > 0 {
		return nil, errors.New("notify sms: ordered template parameters are required by this provider")
	}
	return nil, nil
}
