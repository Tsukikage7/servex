package guardrail_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/safety/guardrail"
)

// mockModel 测试用模拟模型.
type mockModel struct {
	called     bool
	generateFn func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error)
}

func (m *mockModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	m.called = true
	if m.generateFn != nil {
		return m.generateFn(ctx, messages, opts...)
	}
	return &llm.ChatResponse{Message: llm.AssistantMessage("ok")}, nil
}

func (m *mockModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	m.called = true
	return nil, nil
}

// TestMaxLength 验证长度限制护栏.
func TestMaxLength(t *testing.T) {
	guard := guardrail.MaxLength(10)
	ctx := t.Context()

	// 未超出限制，应通过.
	msgs := []llm.Message{llm.UserMessage("hello")}
	if err := guard.Check(ctx, msgs); err != nil {
		t.Errorf("未超出限制应通过，得到错误: %v", err)
	}

	// 正好等于限制，应通过.
	msgs = []llm.Message{llm.UserMessage("helloworld")}
	if err := guard.Check(ctx, msgs); err != nil {
		t.Errorf("等于限制应通过，得到错误: %v", err)
	}

	// 超出限制，应返回 ErrTooLong.
	msgs = []llm.Message{llm.UserMessage("hello world!")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrTooLong) {
		t.Errorf("超出限制期望 ErrTooLong，得到: %v", err)
	}

	// 多条消息累计超出限制.
	msgs = []llm.Message{
		llm.UserMessage("hello"),
		llm.UserMessage("world!"),
	}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrTooLong) {
		t.Errorf("累计超出限制期望 ErrTooLong，得到: %v", err)
	}
}

// TestMaxMessages 验证消息数量限制护栏.
func TestMaxMessages(t *testing.T) {
	guard := guardrail.MaxMessages(3)
	ctx := t.Context()

	// 未超出数量，应通过.
	msgs := []llm.Message{
		llm.UserMessage("a"),
		llm.UserMessage("b"),
		llm.UserMessage("c"),
	}
	if err := guard.Check(ctx, msgs); err != nil {
		t.Errorf("未超出数量应通过，得到错误: %v", err)
	}

	// 超出数量，应返回 ErrTooMany.
	msgs = append(msgs, llm.UserMessage("d"))
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrTooMany) {
		t.Errorf("超出数量期望 ErrTooMany，得到: %v", err)
	}
}

// TestKeywordFilter 验证关键词过滤护栏.
func TestKeywordFilter(t *testing.T) {
	guard := guardrail.KeywordFilter([]string{"spam", "violence"})
	ctx := t.Context()

	// 不含关键词，应通过.
	msgs := []llm.Message{llm.UserMessage("hello world")}
	if err := guard.Check(ctx, msgs); err != nil {
		t.Errorf("不含关键词应通过，得到错误: %v", err)
	}

	// 含关键词（小写），应被拦截.
	msgs = []llm.Message{llm.UserMessage("this is spam content")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrBlocked) {
		t.Errorf("含关键词期望 ErrBlocked，得到: %v", err)
	}

	// 含关键词（大写，测试大小写不敏感），应被拦截.
	msgs = []llm.Message{llm.UserMessage("SPAM detected")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrBlocked) {
		t.Errorf("大写关键词期望 ErrBlocked，得到: %v", err)
	}

	// 含关键词（混合大小写），应被拦截.
	msgs = []llm.Message{llm.UserMessage("SpAm message")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrBlocked) {
		t.Errorf("混合大小写关键词期望 ErrBlocked，得到: %v", err)
	}
}

// TestRegexFilter 验证正则过滤护栏.
func TestRegexFilter(t *testing.T) {
	guard := guardrail.RegexFilter([]string{`\b(bad|evil)\b`, `\d{3}-\d{4}`})
	ctx := t.Context()

	// 不匹配任何模式，应通过.
	msgs := []llm.Message{llm.UserMessage("hello world")}
	if err := guard.Check(ctx, msgs); err != nil {
		t.Errorf("不匹配模式应通过，得到错误: %v", err)
	}

	// 匹配第一个模式，应被拦截.
	msgs = []llm.Message{llm.UserMessage("this is bad content")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrBlocked) {
		t.Errorf("匹配正则期望 ErrBlocked，得到: %v", err)
	}

	// 匹配第二个模式，应被拦截.
	msgs = []llm.Message{llm.UserMessage("call 123-4567")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrBlocked) {
		t.Errorf("匹配正则期望 ErrBlocked，得到: %v", err)
	}
}

// TestPIIDetector_Email 验证邮箱 PII 检测.
func TestPIIDetector_Email(t *testing.T) {
	guard := guardrail.PIIDetector(guardrail.PIIEmail)
	ctx := t.Context()

	// 不含邮箱，应通过.
	msgs := []llm.Message{llm.UserMessage("hello world")}
	if err := guard.Check(ctx, msgs); err != nil {
		t.Errorf("不含邮箱应通过，得到错误: %v", err)
	}

	// 含有效邮箱，应被检测.
	msgs = []llm.Message{llm.UserMessage("my email is user@example.com")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrPIIDetected) {
		t.Errorf("含邮箱期望 ErrPIIDetected，得到: %v", err)
	}
}

// TestPIIDetector_Phone 验证中国手机号 PII 检测.
func TestPIIDetector_Phone(t *testing.T) {
	guard := guardrail.PIIDetector(guardrail.PIIPhone)
	ctx := t.Context()

	// 不含手机号，应通过.
	msgs := []llm.Message{llm.UserMessage("call me")}
	if err := guard.Check(ctx, msgs); err != nil {
		t.Errorf("不含手机号应通过，得到错误: %v", err)
	}

	// 含有效中国手机号，应被检测.
	msgs = []llm.Message{llm.UserMessage("我的电话是 13812345678")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrPIIDetected) {
		t.Errorf("含手机号期望 ErrPIIDetected，得到: %v", err)
	}
}

// TestPIIDetector_IDCard 验证身份证号 PII 检测.
func TestPIIDetector_IDCard(t *testing.T) {
	guard := guardrail.PIIDetector(guardrail.PIIIDCard)
	ctx := t.Context()

	// 不含身份证号，应通过.
	msgs := []llm.Message{llm.UserMessage("no id here")}
	if err := guard.Check(ctx, msgs); err != nil {
		t.Errorf("不含身份证号应通过，得到错误: %v", err)
	}

	// 含有效身份证号，应被检测.
	msgs = []llm.Message{llm.UserMessage("身份证：110101199003072317")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrPIIDetected) {
		t.Errorf("含身份证号期望 ErrPIIDetected，得到: %v", err)
	}
}

// TestPIIDetector_CreditCard 验证信用卡号 PII 检测.
func TestPIIDetector_CreditCard(t *testing.T) {
	guard := guardrail.PIIDetector(guardrail.PIICreditCard)
	ctx := t.Context()

	// 不含信用卡号，应通过.
	msgs := []llm.Message{llm.UserMessage("no card here")}
	if err := guard.Check(ctx, msgs); err != nil {
		t.Errorf("不含信用卡号应通过，得到错误: %v", err)
	}

	// 含有效信用卡号（无分隔符），应被检测.
	msgs = []llm.Message{llm.UserMessage("card: 4111111111111111")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrPIIDetected) {
		t.Errorf("含信用卡号（无分隔符）期望 ErrPIIDetected，得到: %v", err)
	}

	// 含有效信用卡号（连字符分隔），应被检测.
	msgs = []llm.Message{llm.UserMessage("card: 4111-1111-1111-1111")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrPIIDetected) {
		t.Errorf("含信用卡号（连字符分隔）期望 ErrPIIDetected，得到: %v", err)
	}

	// 含有效信用卡号（空格分隔），应被检测.
	msgs = []llm.Message{llm.UserMessage("card: 4111 1111 1111 1111")}
	if err := guard.Check(ctx, msgs); !errors.Is(err, guardrail.ErrPIIDetected) {
		t.Errorf("含信用卡号（空格分隔）期望 ErrPIIDetected，得到: %v", err)
	}
}

// TestMiddleware 验证护栏中间件：输入护栏拦截后不应调用底层模型.
func TestMiddleware(t *testing.T) {
	ctx := t.Context()

	// 输入护栏拦截，不应调用底层模型.
	t.Run("输入护栏拦截", func(t *testing.T) {
		mock := &mockModel{}
		mw := guardrail.Middleware(
			guardrail.WithInputGuards(guardrail.KeywordFilter([]string{"forbidden"})),
		)
		wrapped := mw(mock)

		msgs := []llm.Message{llm.UserMessage("this contains forbidden word")}
		_, err := wrapped.Generate(ctx, msgs)
		if !errors.Is(err, guardrail.ErrBlocked) {
			t.Errorf("输入护栏应返回 ErrBlocked，得到: %v", err)
		}
		if mock.called {
			t.Error("输入护栏拦截后不应调用底层模型")
		}
	})

	// 输入护栏通过，应调用底层模型.
	t.Run("输入护栏通过", func(t *testing.T) {
		mock := &mockModel{}
		mw := guardrail.Middleware(
			guardrail.WithInputGuards(guardrail.KeywordFilter([]string{"forbidden"})),
		)
		wrapped := mw(mock)

		msgs := []llm.Message{llm.UserMessage("safe message")}
		resp, err := wrapped.Generate(ctx, msgs)
		if err != nil {
			t.Errorf("输入护栏通过应返回响应，得到错误: %v", err)
		}
		if resp == nil || resp.Message.Content != "ok" {
			t.Errorf("期望响应内容为 'ok'，得到: %v", resp)
		}
		if !mock.called {
			t.Error("输入护栏通过后应调用底层模型")
		}
	})

	// 输出护栏拦截模型响应.
	t.Run("输出护栏拦截", func(t *testing.T) {
		mock := &mockModel{
			generateFn: func(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
				return &llm.ChatResponse{Message: llm.AssistantMessage("response with spam")}, nil
			},
		}
		mw := guardrail.Middleware(
			guardrail.WithOutputGuards(guardrail.KeywordFilter([]string{"spam"})),
		)
		wrapped := mw(mock)

		msgs := []llm.Message{llm.UserMessage("safe input")}
		_, err := wrapped.Generate(ctx, msgs)
		if !errors.Is(err, guardrail.ErrBlocked) {
			t.Errorf("输出护栏应返回 ErrBlocked，得到: %v", err)
		}
	})

	// 无护栏配置，应正常透传.
	t.Run("无护栏透传", func(t *testing.T) {
		mock := &mockModel{}
		mw := guardrail.Middleware()
		wrapped := mw(mock)

		msgs := []llm.Message{llm.UserMessage("anything")}
		resp, err := wrapped.Generate(ctx, msgs)
		if err != nil {
			t.Errorf("无护栏应正常通过，得到错误: %v", err)
		}
		if resp == nil {
			t.Error("期望有响应，得到 nil")
		}
	})
}
