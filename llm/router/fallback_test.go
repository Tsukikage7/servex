package router_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/router"
)

// --- 辅助 ---

// flakeyModel 可配置失败次数的模型.
type flakeyModel struct {
	name       string
	failTimes  int
	failErr    error
	calls      int
	streamFail bool
}

func (f *flakeyModel) Generate(context.Context, []llm.Message, ...llm.CallOption) (*llm.ChatResponse, error) {
	f.calls++
	if f.calls <= f.failTimes {
		return nil, f.failErr
	}
	return &llm.ChatResponse{ModelID: f.name, Message: llm.AssistantMessage(f.name)}, nil
}

func (f *flakeyModel) Stream(context.Context, []llm.Message, ...llm.CallOption) (llm.StreamReader, error) {
	f.calls++
	if f.calls <= f.failTimes {
		return nil, f.failErr
	}
	if f.streamFail {
		return nil, io.EOF
	}
	return &mockStream{content: f.name}, nil
}

// --- 用例 ---

func TestFallbackRouter_PrimarySuccess(t *testing.T) {
	primary := &flakeyModel{name: "primary"}
	backup := &flakeyModel{name: "backup"}

	r := router.NewFallbackRouter([]llm.ChatModel{primary, backup})

	resp, err := r.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.ModelID != "primary" {
		t.Errorf("期望 primary,得到 %q", resp.ModelID)
	}
	if backup.calls != 0 {
		t.Error("主成功时不应调用备用")
	}
}

func TestFallbackRouter_PrimaryFails_FallbackToBackup(t *testing.T) {
	primary := &flakeyModel{name: "primary", failTimes: 1, failErr: llm.ErrProviderUnavailable}
	backup := &flakeyModel{name: "backup"}

	var fromIdx, toIdx int
	var hookCalled int
	r := router.NewFallbackRouter([]llm.ChatModel{primary, backup},
		router.WithOnFallback(func(from, to int, err error) {
			fromIdx, toIdx = from, to
			hookCalled++
		}),
	)

	resp, err := r.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.ModelID != "backup" {
		t.Errorf("期望降级到 backup,得到 %q", resp.ModelID)
	}
	if hookCalled != 1 {
		t.Errorf("期望 onFallback 触发 1 次,得到 %d", hookCalled)
	}
	if fromIdx != 0 || toIdx != 1 {
		t.Errorf("期望 from=0 to=1,得到 %d %d", fromIdx, toIdx)
	}
}

func TestFallbackRouter_AllFail_ReturnsLastError(t *testing.T) {
	// 使用 llm.IsRetryable 能识别的错误,确保默认策略会一路降级到最后.
	e1 := fmt.Errorf("provider-1 down: %w", llm.ErrProviderUnavailable)
	e2 := fmt.Errorf("provider-2 down: %w", llm.ErrRateLimited)
	m1 := &flakeyModel{name: "m1", failTimes: 10, failErr: e1}
	m2 := &flakeyModel{name: "m2", failTimes: 10, failErr: e2}
	r := router.NewFallbackRouter([]llm.ChatModel{m1, m2})

	_, err := r.Generate(context.Background(), nil)
	if !errors.Is(err, llm.ErrRateLimited) {
		t.Errorf("期望最后一个错误链含 ErrRateLimited,得到 %v", err)
	}
	if m1.calls != 1 || m2.calls != 1 {
		t.Errorf("期望两个模型各被调用 1 次,得到 m1=%d m2=%d", m1.calls, m2.calls)
	}
}

// TestFallbackRouter_NonRetryable_NoFallback 严格默认策略下,
// 非可重试错误(例如 4xx 鉴权失败)不应触发降级.
func TestFallbackRouter_NonRetryable_NoFallback(t *testing.T) {
	// llm.ErrInvalidAuth 既非 IsRetryable 也非 ErrProviderUnavailable.
	primary := &flakeyModel{name: "primary", failTimes: 10, failErr: llm.ErrInvalidAuth}
	backup := &flakeyModel{name: "backup"}
	r := router.NewFallbackRouter([]llm.ChatModel{primary, backup})

	_, err := r.Generate(context.Background(), nil)
	if !errors.Is(err, llm.ErrInvalidAuth) {
		t.Errorf("期望 ErrInvalidAuth 向上传递,得到 %v", err)
	}
	if backup.calls != 0 {
		t.Error("非可重试错误不应调用 backup")
	}
}

func TestFallbackRouter_ContextCanceled_NoFallback(t *testing.T) {
	primary := &flakeyModel{name: "primary", failTimes: 10, failErr: context.Canceled}
	backup := &flakeyModel{name: "backup"}

	r := router.NewFallbackRouter([]llm.ChatModel{primary, backup})
	_, err := r.Generate(context.Background(), nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("期望 context.Canceled,得到 %v", err)
	}
	if backup.calls != 0 {
		t.Error("context 取消不应降级")
	}
}

func TestFallbackRouter_StreamFallback(t *testing.T) {
	primary := &flakeyModel{name: "primary", failTimes: 1, failErr: llm.ErrProviderUnavailable}
	backup := &flakeyModel{name: "backup"}

	r := router.NewFallbackRouter([]llm.ChatModel{primary, backup})
	stream, err := r.Stream(context.Background(), nil)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer stream.Close()

	chunk, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if chunk.Delta != "backup" {
		t.Errorf("期望 backup,得到 %q", chunk.Delta)
	}
}

func TestFallbackRouter_CustomShouldFallback(t *testing.T) {
	businessErr := errors.New("business rule violation")
	m1 := &flakeyModel{name: "m1", failTimes: 1, failErr: businessErr}
	m2 := &flakeyModel{name: "m2"}

	// 自定义:只对包含 "business" 字样的错误降级.
	r := router.NewFallbackRouter([]llm.ChatModel{m1, m2},
		router.WithShouldFallback(func(err error) bool {
			return err != nil && errors.Is(err, businessErr)
		}),
	)
	resp, err := r.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.ModelID != "m2" {
		t.Errorf("期望降级到 m2,得到 %q", resp.ModelID)
	}
}

func TestFallbackRouter_EmptyModels(t *testing.T) {
	r := router.NewFallbackRouter(nil)
	_, err := r.Generate(context.Background(), nil)
	if !errors.Is(err, router.ErrNoModels) {
		t.Errorf("期望 ErrNoModels,得到 %v", err)
	}
	_, err = r.Stream(context.Background(), nil)
	if !errors.Is(err, router.ErrNoModels) {
		t.Errorf("期望 ErrNoModels,得到 %v", err)
	}
}
