package middleware

import (
	"context"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Logging 返回记录请求日志的中间件.
// 记录内容：模型名称、prompt tokens、completion tokens、总耗时.
func Logging(log logger.Logger) Middleware {
	return func(next llm.ChatModel) llm.ChatModel {
		return Wrap(
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
				start := time.Now()
				o := llm.ApplyOptions(opts)
				model := o.Model

				resp, err := next.Generate(ctx, messages, opts...)

				elapsed := time.Since(start)
				fields := []logger.Field{
					{Key: "model", Value: model},
					{Key: "duration_ms", Value: elapsed.Milliseconds()},
					{Key: "messages", Value: len(messages)},
				}
				if err != nil {
					fields = append(fields, logger.Field{Key: "error", Value: err.Error()})
					log.With(fields...).Error("ai generate 失败")
				} else {
					fields = append(fields,
						logger.Field{Key: "finish_reason", Value: resp.FinishReason},
						logger.Field{Key: "prompt_tokens", Value: resp.Usage.PromptTokens},
						logger.Field{Key: "completion_tokens", Value: resp.Usage.CompletionTokens},
						logger.Field{Key: "total_tokens", Value: resp.Usage.TotalTokens},
					)
					log.With(fields...).Info("ai generate 完成")
				}
				return resp, err
			},
			func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
				start := time.Now()
				o := llm.ApplyOptions(opts)
				model := o.Model

				reader, err := next.Stream(ctx, messages, opts...)

				elapsed := time.Since(start)
				fields := []logger.Field{
					{Key: "model", Value: model},
					{Key: "duration_ms", Value: elapsed.Milliseconds()},
					{Key: "messages", Value: len(messages)},
				}
				if err != nil {
					fields = append(fields, logger.Field{Key: "error", Value: err.Error()})
					log.With(fields...).Error("ai stream 失败")
				} else {
					log.With(fields...).Info("ai stream 已建立")
				}
				return reader, err
			},
		)
	}
}
