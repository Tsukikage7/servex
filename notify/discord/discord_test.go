package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/Tsukikage7/servex/v2/notify"
)

// mockDiscordClient 用于测试的 mock discordClient.
type mockDiscordClient struct {
	// sentChannels 记录每次调用的 channelID.
	sentChannels []string
	// sentContents 记录每次调用的消息正文.
	sentContents []string
	// responses 按顺序返回的响应错误，nil 表示成功.
	responses []error
	// nextID 下一条消息自增 ID字符串形式.
	nextID int
	// callCount 已调用次数.
	callCount int
	// closeCalled 是否调用过 Close.
	closeCalled bool
}

func (m *mockDiscordClient) ChannelMessageSend(channelID string, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	m.sentChannels = append(m.sentChannels, channelID)
	m.sentContents = append(m.sentContents, content)
	idx := m.callCount
	m.callCount++
	if idx < len(m.responses) && m.responses[idx] != nil {
		return nil, m.responses[idx]
	}
	m.nextID++
	return &discordgo.Message{ID: string(rune('0' + m.nextID))}, nil
}

func (m *mockDiscordClient) Close() error {
	m.closeCalled = true
	return nil
}

func TestSend_SingleSuccess(t *testing.T) {
	mock := &mockDiscordClient{}
	s := &Sender{client: mock}

	msg := &notify.Message{
		Channel: ChannelDiscord,
		To:      []string{"channel-123"},
		Body:    "hello discord",
	}
	result, err := s.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}
	if result == nil {
		t.Fatal("期望 result 不为 nil")
	}
	if result.MessageID == "" {
		t.Error("期望 MessageID 不为空")
	}
	if result.Channel != ChannelDiscord {
		t.Errorf("期望 Channel=discord，实际: %s", result.Channel)
	}
	if result.Error != nil {
		t.Errorf("期望 Result.Error 为 nil，实际: %v", result.Error)
	}
	if len(mock.sentChannels) != 1 {
		t.Errorf("期望发送 1 条消息，实际: %d", len(mock.sentChannels))
	}
	if mock.sentChannels[0] != "channel-123" {
		t.Errorf("期望 channelID=channel-123，实际: %s", mock.sentChannels[0])
	}
	if mock.sentContents[0] != "hello discord" {
		t.Errorf("期望 content=hello discord，实际: %s", mock.sentContents[0])
	}
}

func TestSend_MultiplePartialFailure(t *testing.T) {
	sendErr := errors.New("发送失败")
	mock := &mockDiscordClient{
		// 第一条成功，第二条失败，第三条成功
		responses: []error{nil, sendErr, nil},
	}
	s := &Sender{client: mock}

	msg := &notify.Message{
		Channel: ChannelDiscord,
		To:      []string{"ch-1", "ch-2", "ch-3"},
		Body:    "批量消息",
	}
	result, err := s.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("期望有错误，实际无错误")
	}
	if !errors.Is(err, sendErr) {
		t.Errorf("期望错误包含 sendErr，实际: %v", err)
	}
	if result == nil {
		t.Fatal("期望 result 不为 nil")
	}
	// lastID 应为最后一条成功消息的 ID非空
	if result.MessageID == "" {
		t.Error("期望 MessageID 不为空（至少有一条成功）")
	}
	if result.Error == nil {
		t.Error("期望 Result.Error 不为 nil")
	}
	if result.Channel != ChannelDiscord {
		t.Errorf("期望 Channel=discord，实际: %s", result.Channel)
	}
	// 应尝试发送全部 3 条
	if mock.callCount != 3 {
		t.Errorf("期望尝试发送 3 条，实际: %d", mock.callCount)
	}
}

func TestChannel(t *testing.T) {
	s := &Sender{client: &mockDiscordClient{}}
	if s.Channel() != ChannelDiscord {
		t.Errorf("期望 Channel=discord，实际: %s", s.Channel())
	}
	if string(s.Channel()) != "discord" {
		t.Errorf("期望渠道字符串值为 discord，实际: %s", s.Channel())
	}
}

func TestClose(t *testing.T) {
	mock := &mockDiscordClient{}
	s := &Sender{client: mock}
	if err := s.Close(); err != nil {
		t.Errorf("期望 Close 返回 nil，实际: %v", err)
	}
	if !mock.closeCalled {
		t.Error("期望 client.Close() 被调用")
	}
}
