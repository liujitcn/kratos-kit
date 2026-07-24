package eino

import (
	einoPrompt "github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// FromMessages 根据消息模板列表创建 ChatTemplate。
func FromMessages(formatType schema.FormatType, templates ...schema.MessagesTemplate) *einoPrompt.DefaultChatTemplate {
	return einoPrompt.FromMessages(formatType, templates...)
}

// FromAgenticMessages 根据 Agentic 消息模板列表创建 AgenticChatTemplate。
func FromAgenticMessages(formatType schema.FormatType, templates ...schema.AgenticMessagesTemplate) *einoPrompt.DefaultAgenticChatTemplate {
	return einoPrompt.FromAgenticMessages(formatType, templates...)
}

// SystemMessage 创建一个系统角色的消息模板。
func SystemMessage(content string) *schema.Message {
	return &schema.Message{Role: schema.System, Content: content}
}

// SystemAgenticMessage 创建一个系统角色的 Agentic 消息模板。
func SystemAgenticMessage(content string) *schema.AgenticMessage {
	return schema.SystemAgenticMessage(content)
}

// UserMessage 创建一个用户角色的消息模板。
func UserMessage(content string) *schema.Message {
	return &schema.Message{Role: schema.User, Content: content}
}

// UserAgenticMessage 创建一个用户角色的 Agentic 消息模板。
func UserAgenticMessage(content string) *schema.AgenticMessage {
	return schema.UserAgenticMessage(content)
}

// AssistantMessage 创建一个助手角色的消息模板。
func AssistantMessage(content string) *schema.Message {
	return &schema.Message{Role: schema.Assistant, Content: content}
}

// AssistantAgenticMessage 创建一个助手角色的 Agentic 消息模板。
func AssistantAgenticMessage(content string) *schema.AgenticMessage {
	return &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			{
				Type: schema.ContentBlockTypeAssistantGenText,
				AssistantGenText: &schema.AssistantGenText{
					Text: content,
				},
			},
		},
	}
}

// MessagesPlaceholder 创建一个消息占位符，用于在模板中插入动态消息列表。
func MessagesPlaceholder(variableName string, optional bool) schema.MessagesTemplate {
	return schema.MessagesPlaceholder(variableName, optional)
}

// AgenticMessagesPlaceholder 创建一个 Agentic 消息占位符，用于在模板中插入动态消息列表。
func AgenticMessagesPlaceholder(variableName string, optional bool) schema.AgenticMessagesTemplate {
	return schema.AgenticMessagesPlaceholder(variableName, optional)
}
