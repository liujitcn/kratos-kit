package notify

import (
	"context"
	"errors"
	"testing"
)

type testSender struct {
	name       string
	channel    Type
	messageIDs []string
}

// Name 返回测试发送器的实例名。
func (s *testSender) Name() string { return s.name }

// Type 返回测试发送器的渠道类型。
func (s *testSender) Type() Type { return s.channel }

// Send 返回测试发送器预设的回执。
func (s *testSender) Send(_ context.Context, _ Message) (*Receipt, error) {
	return &Receipt{MessageID: s.messageIDs[0]}, nil
}

func TestManagerSupportsMultipleInstancesPerChannel(t *testing.T) {
	manager, err := NewManager(
		&testSender{name: "finance", channel: Webhook, messageIDs: []string{"1"}},
		&testSender{name: "engineering", channel: Webhook, messageIDs: []string{"2"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := manager.NamesByType(Webhook); len(got) != 2 || got[0] != "engineering" || got[1] != "finance" {
		t.Fatalf("unexpected webhook sender names: %v", got)
	}
	receipt, err := manager.Send(context.Background(), "engineering", Message{Content: "notice"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.MessageID != "2" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestManagerReplaceKeepsCurrentSnapshotOnInvalidInput(t *testing.T) {
	manager, err := NewManager(&testSender{name: "current", channel: Email, messageIDs: []string{"1"}})
	if err != nil {
		t.Fatal(err)
	}
	err = manager.Replace(
		&testSender{name: "replacement", channel: Email},
		&testSender{name: "replacement", channel: Webhook},
	)
	if !errors.Is(err, ErrSenderAlreadyRegistered) {
		t.Fatalf("expected duplicate sender error, got %v", err)
	}
	if _, err = manager.Get("current"); err != nil {
		t.Fatalf("invalid replacement changed the active snapshot: %v", err)
	}
}

func TestManagerRejectsDuplicateRegistration(t *testing.T) {
	manager, err := NewManager(&testSender{name: "primary", channel: Email})
	if err != nil {
		t.Fatal(err)
	}
	err = manager.Register(&testSender{name: "primary", channel: Webhook})
	if !errors.Is(err, ErrSenderAlreadyRegistered) {
		t.Fatalf("expected duplicate sender error, got %v", err)
	}
}
