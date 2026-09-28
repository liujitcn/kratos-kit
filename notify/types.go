package notify

import (
	"context"
	"net/http"
)

// HTTPTransport 是可注入给 go-utils/http 的底层标准库传输客户端。
type HTTPTransport = *http.Client

// Type 表示通知发送渠道类型。
type Type string

const (
	// Email 表示 SMTP 邮件渠道。
	Email Type = "email"
	// Webhook 表示通用 HTTP Webhook 渠道。
	Webhook Type = "webhook"
	// SMS 表示短信渠道。
	SMS Type = "sms"
	// DingTalk 表示钉钉通知渠道。
	DingTalk Type = "dingtalk"
	// FeishuApp 表示飞书应用消息。
	FeishuApp Type = "feishu_app"
	// FeishuGroup 表示飞书群机器人消息。
	FeishuGroup Type = "feishu_group"
	// WechatOfficial 表示微信公众号模板消息。
	WechatOfficial Type = "wechat_official"
	// WechatWorkApp 表示企业微信应用消息。
	WechatWorkApp Type = "wechat_work_app"
	// WechatWorkGroup 表示企业微信群机器人消息。
	WechatWorkGroup Type = "wechat_work_group"
)

// ContentType 表示通知正文格式。
type ContentType string

const (
	// PlainText 表示纯文本正文。
	PlainText ContentType = "TEXT"
	// Markdown 表示 Markdown 正文。
	Markdown ContentType = "MARKDOWN"
	// HTML 表示 HTML 正文。
	HTML ContentType = "HTML"
)

// Recipient 描述一个渠道的接收人标识；每个渠道只读取自身需要的标识字段。
type Recipient struct {
	// Email 是邮件接收地址。
	Email string
	// Phone 是短信接收手机号，格式由短信服务商要求决定。
	Phone string
	// OpenID 是微信、飞书等应用范围内的用户标识。
	OpenID string
	// UnionID 是平台开发者或组织范围内的跨应用用户标识。
	UnionID string
	// UserID 是钉钉或企业微信通讯录中的用户标识。
	UserID string
	// ChatID 是飞书应用中的群会话标识。
	ChatID string
}

// Template 描述短信或平台模板消息所需的模板信息。
type Template struct {
	// Code 是服务商或平台审核通过的模板编码。
	Code string
	// Params 是按参数名映射的模板值。
	Params map[string]string
	// OrderedParams 是服务商要求按位置传递的模板值。
	OrderedParams []string
	// URL 是模板消息附带的跳转地址。
	URL string
}

// Message 描述一次通知发送内容。
type Message struct {
	// Recipients 是接收人标识；群机器人通常不需要填写。
	Recipients []Recipient
	// Title 是通知标题；邮件将其用作主题。
	Title string
	// Content 是通知正文。
	Content string
	// ContentType 是正文格式；留空时按纯文本处理。
	ContentType ContentType
	// Template 是短信或平台模板消息配置；普通文本消息可以为空。
	Template *Template
	// Metadata 是随通知传递的业务扩展字段。
	Metadata map[string]string
}

// Receipt 描述渠道确认接受的发送结果。
type Receipt struct {
	// MessageID 是渠道或接收端返回的消息标识。
	MessageID string
	// AcceptedRecipients 是渠道确认接受的接收目标。
	AcceptedRecipients []string
}

// Sender 定义一个可配置的通知发送渠道实例。
type Sender interface {
	// Name 返回渠道配置实例名。
	Name() string
	// Type 返回渠道类型。
	Type() Type
	// Send 发送通知并返回渠道受理结果。
	Send(context.Context, Message) (*Receipt, error)
}
