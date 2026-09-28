package notify

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

var (
	// ErrSenderNotFound 表示未注册指定名称的发送渠道实例。
	ErrSenderNotFound = errors.New("notify: sender not found")
	// ErrSenderNameRequired 表示发送渠道实例名称为空。
	ErrSenderNameRequired = errors.New("notify: sender name is required")
	// ErrSenderTypeRequired 表示发送渠道类型为空。
	ErrSenderTypeRequired = errors.New("notify: sender type is required")
	// ErrSenderAlreadyRegistered 表示发送渠道实例名称重复。
	ErrSenderAlreadyRegistered = errors.New("notify: sender already registered")
)

// Manager 管理按配置实例名注册的通知发送渠道。
type Manager struct {
	mutex   sync.RWMutex
	senders map[string]Sender
}

// NewManager 创建通知发送渠道管理器。
func NewManager(senders ...Sender) (*Manager, error) {
	registry, err := buildSenders(senders)
	if err != nil {
		return nil, err
	}
	return &Manager{senders: registry}, nil
}

// Register 注册一个发送渠道实例；同名实例不会被覆盖。
func (m *Manager) Register(sender Sender) error {
	if err := validateSender(sender); err != nil {
		return err
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, exists := m.senders[sender.Name()]; exists {
		return fmt.Errorf("%w: %s", ErrSenderAlreadyRegistered, sender.Name())
	}
	m.senders[sender.Name()] = sender
	return nil
}

// Replace 使用完整的新配置原子替换当前发送渠道实例集合。
func (m *Manager) Replace(senders ...Sender) error {
	replacement, err := buildSenders(senders)
	if err != nil {
		return err
	}
	m.mutex.Lock()
	m.senders = replacement
	m.mutex.Unlock()
	return nil
}

// Get 根据配置实例名获取发送渠道。
func (m *Manager) Get(name string) (Sender, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	sender, ok := m.senders[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSenderNotFound, name)
	}
	return sender, nil
}

// Send 将通知发送到指定配置实例。
func (m *Manager) Send(ctx context.Context, name string, message Message) (*Receipt, error) {
	sender, err := m.Get(name)
	if err != nil {
		return nil, err
	}
	return sender.Send(ctx, message)
}

// Names 返回已注册的渠道配置实例名。
func (m *Manager) Names() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	names := make([]string, 0, len(m.senders))
	for name := range m.senders {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// NamesByType 返回指定渠道类型的配置实例名。
func (m *Manager) NamesByType(channelType Type) []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	names := make([]string, 0, len(m.senders))
	for name, sender := range m.senders {
		if sender.Type() == channelType {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// validateSender 校验发送器的管理标识。
func validateSender(sender Sender) error {
	if sender == nil {
		return ErrSenderNameRequired
	}
	if sender.Name() == "" {
		return ErrSenderNameRequired
	}
	if sender.Type() == "" {
		return ErrSenderTypeRequired
	}
	return nil
}

// buildSenders 构造无名称冲突的发送器快照。
func buildSenders(senders []Sender) (map[string]Sender, error) {
	replacement := make(map[string]Sender, len(senders))
	for _, sender := range senders {
		if err := validateSender(sender); err != nil {
			return nil, err
		}
		if _, exists := replacement[sender.Name()]; exists {
			return nil, fmt.Errorf("%w: %s", ErrSenderAlreadyRegistered, sender.Name())
		}
		replacement[sender.Name()] = sender
	}
	return replacement, nil
}
