package handoff_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/v2/llm/agent/handoff"
)

// ExampleNewKeywordDetector 展示关键词检测器：用户说"转人工"触发.
func ExampleNewKeywordDetector() {
	d := handoff.NewKeywordDetector([]string{"转人工", "投诉"})
	sig, _ := d.Detect(context.Background(), handoff.DetectInput{Question: "我要转人工客服"})
	fmt.Println(sig.Should)
	fmt.Println(sig.Reason)
	fmt.Println(sig.Meta["matched"])
	// Output:
	// true
	// keyword
	// 转人工
}

// ExampleNewLowConfidenceDetector 展示基于 RAG 召回置信度的检测.
func ExampleNewLowConfidenceDetector() {
	d := handoff.NewLowConfidenceDetector(0.5)
	sig, _ := d.Detect(context.Background(), handoff.DetectInput{LastScore: 0.3})
	fmt.Println(sig.Should, sig.Reason)
	// Output:
	// true low_confidence
}

// ExampleNewRetryDetector 展示重复提问次数检测.
func ExampleNewRetryDetector() {
	d := handoff.NewRetryDetector(3)
	sig, _ := d.Detect(context.Background(), handoff.DetectInput{RetryCount: 3})
	fmt.Println(sig.Should, sig.Reason)
	// Output:
	// true retry_exceeded
}

// ExampleNewCompositeDetector 展示多策略组合：任一命中即触发，短路执行.
func ExampleNewCompositeDetector() {
	c := handoff.NewCompositeDetector(
		handoff.NewKeywordDetector([]string{"转人工"}),
		handoff.NewLowConfidenceDetector(0.35),
		handoff.NewRetryDetector(3),
	)
	sig, _ := c.Detect(context.Background(), handoff.DetectInput{
		Question:  "查不到订单",
		LastScore: 0.2,
	})
	fmt.Println(sig.Should, sig.Reason)
	// Output:
	// true low_confidence
}

// ExampleNewFuncHook 展示把任意函数包装为 Hook，用于写工单/发消息队列.
func ExampleNewFuncHook() {
	h, _ := handoff.NewFuncHook(func(_ context.Context, sig *handoff.Signal, _ handoff.DetectInput) error {
		fmt.Printf("create ticket: %s\n", sig.Reason)
		return nil
	})
	_ = h.Fire(context.Background(),
		&handoff.Signal{Should: true, Reason: handoff.ReasonKeyword},
		handoff.DetectInput{Question: "转人工"})
	// Output:
	// create ticket: keyword
}

// ExampleNewWebhookHook 展示把检测信号 POST 到外部 webhook.
func ExampleNewWebhookHook() {
	// 用 httptest.Server 模拟外部 webhook 接收端.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Printf("webhook ok (body len > 0: %v, content-type=%s)\n",
			len(body) > 0, r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	h := handoff.NewWebhookHook(srv.URL, handoff.WithHeader("Authorization", "Bearer x"))
	err := h.Fire(context.Background(),
		&handoff.Signal{Should: true, Reason: handoff.ReasonKeyword, Meta: map[string]string{"matched": "转人工"}},
		handoff.DetectInput{Question: "转人工"},
	)
	fmt.Println("err:", err)
	// Output:
	// webhook ok (body len > 0: true, content-type=application/json)
	// err: <nil>
}
