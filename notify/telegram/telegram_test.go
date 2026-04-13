package telegram

import (
	"context"
	"errors"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Tsukikage7/servex/v2/notify"
)

// mockBotClient 用于测试的 mock botClient.
type mockBotClient struct {
	// sentMessages 记录已发送的消息.
	sentMessages []tgbotapi.Chattable
	// responses 按顺序返回的响应，nil 表示成功.
	responses []error
	// nextID 下一条消息的 ID.
	nextID int
	// callCount 已调用次数.
	callCount int
}

func (m *mockBotClient) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sentMessages = append(m.sentMessages, c)
	idx := m.callCount
	m.callCount++
	if idx < len(m.responses) && m.responses[idx] != nil {
		return tgbotapi.Message{}, m.responses[idx]
	}
	m.nextID++
	return tgbotapi.Message{MessageID: m.nextID}, nil
}

func TestSend_SingleSuccess(t *testing.T) {
	mock := &mockBotClient{}
	s := &Sender{client: mock}

	msg := &notify.Message{
		Channel: ChannelTelegram,
		To:      []string{"123456"},
		Body:    "hello",
	}
	result, err := s.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if result.MessageID != "1" {
		t.Errorf("期望 MessageID=1，实际: %s", result.MessageID)
	}
	if result.Channel != ChannelTelegram {
		t.Errorf("期望 Channel=telegram，实际: %s", result.Channel)
	}
	if len(mock.sentMessages) != 1 {
		t.Errorf("期望发送 1 条消息，实际: %d", len(mock.sentMessages))
	}
}

func TestSend_MultiplePartialFailure(t *testing.T) {
	sendErr := errors.New("发送失败")
	mock := &mockBotClient{
		// 第一条成功，第二条失败，第三条成功
		responses: []error{nil, sendErr, nil},
	}
	s := &Sender{client: mock}

	msg := &notify.Message{
		Channel: ChannelTelegram,
		To:      []string{"111", "222", "333"},
		Body:    "批量消息",
	}
	result, err := s.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("期望有错误，实际无错误")
	}
	if !errors.Is(err, sendErr) {
		t.Errorf("期望错误包含 sendErr，实际: %v", err)
	}
	// lastID 应为最后一条成功消息的 ID（第三条，nextID=2）
	if result.MessageID == "" {
		t.Error("期望 MessageID 不为空（至少有一条成功）")
	}
	if result.Error == nil {
		t.Error("期望 Result.Error 不为 nil")
	}
	// 应仍尝试发送全部 3 条（跳过无效 chatID 外，有效的都要尝试）
	if len(mock.sentMessages) != 3 {
		t.Errorf("期望尝试发送 3 条，实际: %d", len(mock.sentMessages))
	}
}

func TestSend_InvalidChatID(t *testing.T) {
	mock := &mockBotClient{}
	s := &Sender{client: mock}

	msg := &notify.Message{
		Channel: ChannelTelegram,
		To:      []string{"not-a-number"},
		Body:    "测试",
	}
	result, err := s.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("期望有错误，实际无错误")
	}
	if result == nil {
		t.Fatal("期望 result 不为 nil")
	}
	// 没有成功发送，MessageID 应为空
	if result.MessageID != "" {
		t.Errorf("期望 MessageID 为空，实际: %s", result.MessageID)
	}
	// mock 不应被调用
	if len(mock.sentMessages) != 0 {
		t.Errorf("期望没有实际发送，实际发送了 %d 条", len(mock.sentMessages))
	}
}

func TestChannel(t *testing.T) {
	s := &Sender{client: &mockBotClient{}}
	if s.Channel() != ChannelTelegram {
		t.Errorf("期望 Channel=telegram，实际: %s", s.Channel())
	}
	if string(s.Channel()) != "telegram" {
		t.Errorf("期望渠道字符串值为 telegram，实际: %s", s.Channel())
	}
}

func TestClose(t *testing.T) {
	s := &Sender{client: &mockBotClient{}}
	if err := s.Close(); err != nil {
		t.Errorf("期望 Close 返回 nil，实际: %v", err)
	}
}
