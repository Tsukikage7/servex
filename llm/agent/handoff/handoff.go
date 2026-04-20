// Package handoff 提供人工接管检测与 Hook 触发能力.
//
// 场景：AI 客服在以下情形下需转人工：用户明确请求（"转人工"/"投诉"等）、
// RAG 低置信度召回、用户重复提问、LLM 判定情绪激烈等.
// 本包通过 Detector 抽象检测策略，Hook 抽象回调副作用（写工单/发消息队列/调 webhook 等）.
//
// 核心组件：
//   - Detector - 检测器接口，返回 Signal 指示是否应转人工及原因
//   - Hook - 检测命中后的副作用触发器
//   - CompositeDetector - 多策略组合：任一命中即触发
//   - KeywordDetector/LowConfidenceDetector/RetryDetector/LLMDetector - 内置 Detector
//   - WebhookHook/FuncHook - 内置 Hook
package handoff

import (
	"context"
	"errors"

	"github.com/Tsukikage7/servex/v2/llm"
)

// 预定义原因常量，便于下游根据 Reason 路由不同处理逻辑.
const (
	// ReasonKeyword 用户消息或回答命中关键词.
	ReasonKeyword = "keyword"
	// ReasonLowConfidence RAG 召回最高 score 低于阈值.
	ReasonLowConfidence = "low_confidence"
	// ReasonRetryExceeded 用户在本 session 重复提问次数超过上限.
	ReasonRetryExceeded = "retry_exceeded"
	// ReasonLLMDetected LLM 判定用户有强烈情绪或明确转人工意图.
	ReasonLLMDetected = "llm_detected"
)

// 预定义错误.
var (
	// ErrNilModel 构造需要 llm.ChatModel 的 Detector 时传入了 nil.
	ErrNilModel = errors.New("handoff: nil chat model")
)

// Signal 检测信号，由 Detector.Detect 返回.
//
// Should 为 true 表示应当触发人工接管；Reason 为语义化的原因标签
// （内置 Detector 使用 Reason* 常量，外部 Detector 可自定义字符串）；
// Meta 是可选的扩展元数据（例如命中的具体关键词、触发时的 score 等），
// 供 Hook 记录到工单或日志使用.
type Signal struct {
	// Should 是否应当转人工.
	Should bool
	// Reason 触发原因标签.
	Reason string
	// Meta 额外元数据，Hook 可按需读取.
	Meta map[string]string
}

// DetectInput 检测器输入，由上层调用方（如 chat handler）组装.
//
// 各 Detector 按需读取字段：KeywordDetector 读取 Question/Answer；
// LowConfidenceDetector 读取 LastScore；RetryDetector 读取 RetryCount；
// LLMDetector 读取 Question（+ History 上下文）.
type DetectInput struct {
	// Question 用户当前问题.
	Question string
	// Answer 模型生成的回答（可为空：在生成前做检测时）.
	Answer string
	// History 对话历史（含当前轮之前的消息）.
	History []llm.Message
	// RetryCount 本 session 内用户重复提问的次数.
	RetryCount int
	// LastScore 上一次 RAG 召回的最高 score（可选，0 表示未提供）.
	LastScore float32
}

// Detector 检测器接口，判断当前状态是否应转人工.
//
// 实现约定：
//   - 未命中时返回 (&Signal{Should:false}, nil)，而非 nil Signal
//   - 内部错误（如 LLM 调用失败）返回非 nil error，Signal 仍应为非 nil（Should 视情况）
//   - Detect 应当是幂等的、线程安全的
type Detector interface {
	Detect(ctx context.Context, input DetectInput) (*Signal, error)
}

// Hook 检测命中后的副作用触发器.
//
// 实现约定：
//   - Fire 内部应当短路（如 HTTP 调用使用带超时的 ctx）避免阻塞主流程
//   - 返回 error 由调用方决定是否仅记日志或向上抛
type Hook interface {
	Fire(ctx context.Context, sig *Signal, input DetectInput) error
}

// notTriggered 预先分配的未命中信号，避免高频路径重复分配.
// 返回前应做深拷贝以防调用方修改 Meta 造成污染——当前 Meta 为 nil，直接返回值是安全的.
func notTriggered() *Signal {
	return &Signal{Should: false}
}
