package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/gateway/apikey"
	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/transport/response"
)

// chatCompletionRequest 是 Chat Completions 请求载荷.
type chatCompletionRequest struct {
	Model       string       `json:"model"`
	Messages    []messageReq `json:"messages"`
	Temperature *float64     `json:"temperature,omitzero"`
	MaxTokens   *int         `json:"max_tokens,omitzero"`
	Stream      bool         `json:"stream"`
}

// messageReq 是 Chat Completions 消息载荷.
type messageReq struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatCompletionResponse 是 Chat Completions 响应载荷.
type chatCompletionResponse struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []choice  `json:"choices"`
	Usage   usageResp `json:"usage"`
}

// choice 单条候选结果.
type choice struct {
	Index        int        `json:"index"`
	Message      messageReq `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

// usageResp token 用量统计.
type usageResp struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// modelInfo 是模型列表响应中的模型信息.
type modelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// modelsResponse 是模型列表响应载荷.
type modelsResponse struct {
	Object string      `json:"object"`
	Data   []modelInfo `json:"data"`
}

func (p *Gateway) writeError(w http.ResponseWriter, r *http.Request, code response.Code, msg string) {
	err := code.ToError()
	if msg != "" && code.HTTPStatus() >= http.StatusInternalServerError {
		p.logInternalError(r, msg)
	}
	_ = response.WriteLocalizedError(w, r, err)
}

func (p *Gateway) logInternalError(r *http.Request, msg string) {
	if p.log == nil {
		return
	}
	p.log.With(
		logger.String("path", r.URL.Path),
		logger.String("message", msg),
	).Error("AI 网关内部错误")
}

// handleChatCompletion 处理 POST /v1/chat/completions 请求.
func (p *Gateway) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	// 1. 限制请求体大小10MB并解析 JSON
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var req chatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if p.log != nil {
			p.log.With(logger.Err(err)).Error("AI 网关解析请求体失败")
		}
		p.writeError(w, r, response.CodeInvalidParam, "无效的请求体")
		return
	}
	if req.Model == "" {
		p.writeError(w, r, response.CodeMissingParam, "model 字段不能为空")
		return
	}

	// 2. 若设置了 API Key 管理器，从 context 中获取已验证的 Key
	var keyID string
	if p.keyMgr != nil {
		key, ok := apikey.FromContext(r.Context())
		if !ok {
			p.writeError(w, r, response.CodeUnauthorized, apikey.ErrMissingKey.Error())
			return
		}
		keyID = key.ID
	}

	// 3. 若设置了内容审核器，检查消息内容
	if p.moderator != nil {
		msgs := convertMessages(req.Messages)
		result, err := p.moderator.ModerateMessages(r, msgs)
		if err != nil {
			if p.log != nil {
				p.log.With(logger.Err(err)).Error("AI 网关内容审核失败")
			}
			p.writeError(w, r, response.CodeInternal, "内容审核失败")
			return
		}
		if result.Flagged {
			p.writeError(w, r, response.CodeInvalidParam, "内容审核未通过: "+result.Reason)
			return
		}
	}

	// 4. 按模型名称路由到对应 Provider
	model, err := p.Route(req.Model)
	if err != nil {
		if p.log != nil {
			p.log.With(logger.String("model", req.Model), logger.Err(err)).Error("AI 网关路由模型失败")
		}
		p.writeError(w, r, response.CodeNotFound, "路由失败")
		return
	}

	// 5. 将 messageReq 转换为 []llm.Message
	messages := convertMessages(req.Messages)

	// 构建调用选项
	var callOpts []llm.CallOption
	callOpts = append(callOpts, llm.WithModel(req.Model))
	if req.Temperature != nil {
		callOpts = append(callOpts, llm.WithTemperature(*req.Temperature))
	}
	if req.MaxTokens != nil {
		callOpts = append(callOpts, llm.WithMaxTokens(*req.MaxTokens))
	}

	// 6. 根据 stream 字段决定调用方式
	if req.Stream {
		p.handleStream(w, r, model, messages, callOpts, req.Model, keyID)
		return
	}

	p.handleGenerate(w, r, model, messages, callOpts, req.Model, keyID)
}

// handleGenerate 处理非流式聊天补全请求.
func (p *Gateway) handleGenerate(
	w http.ResponseWriter, r *http.Request,
	model llm.ChatModel, messages []llm.Message,
	callOpts []llm.CallOption, modelName, keyID string,
) {
	resp, err := model.Generate(r.Context(), messages, callOpts...)
	if err != nil {
		if p.log != nil {
			p.log.With(logger.String("model", modelName), logger.Err(err)).Error("AI 网关模型调用失败")
		}
		p.writeError(w, r, response.CodeUpstreamError, "模型调用失败，请稍后重试")
		return
	}

	// 8. 若设置了计费引擎，记录 token 用量
	if p.billing != nil && keyID != "" {
		if berr := p.billing.Record(r.Context(), keyID, resp.ModelID, resp.Usage); berr != nil {
			if p.log != nil {
				p.log.With(logger.Err(berr)).Error("AI 网关计费记录失败")
			}
		}
	}

	// 构造 Chat Completions 响应载荷。
	result := chatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []choice{
			{
				Index: 0,
				Message: messageReq{
					Role:    string(resp.Message.Role),
					Content: resp.Message.Content,
				},
				FinishReason: resp.FinishReason,
			},
		},
		Usage: usageResp{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	_ = response.WriteSuccess(w, result)
}

// handleStream 处理流式聊天补全请求，以 SSE 格式输出.
func (p *Gateway) handleStream(
	w http.ResponseWriter, r *http.Request,
	model llm.ChatModel, messages []llm.Message,
	callOpts []llm.CallOption, modelName, keyID string,
) {
	reader, err := model.Stream(r.Context(), messages, callOpts...)
	if err != nil {
		if p.log != nil {
			p.log.With(logger.String("model", modelName), logger.Err(err)).Error("AI 网关流式模型调用失败")
		}
		p.writeError(w, r, response.CodeUpstreamError, "流式模型调用失败，请稍后重试")
		return
	}
	defer reader.Close()

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, canFlush := w.(http.Flusher)

	created := time.Now().Unix()
	chatID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())

	// 逐块读取并以 SSE 格式写出
	for {
		chunk, recvErr := reader.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			if p.log != nil {
				p.log.With(logger.String("model", modelName), logger.Err(recvErr)).Error("AI 网关流式读取失败")
			}
			break
		}

		// 构造 Chat Completions 流式响应片段。
		streamResp := map[string]any{
			"id":      chatID,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   modelName,
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"role":    "assistant",
						"content": chunk.Delta,
					},
					"finish_reason": nilIfEmpty(chunk.FinishReason),
				},
			},
		}

		data, _ := json.Marshal(streamResp)
		fmt.Fprintf(w, "data: %s\n\n", data)
		if canFlush {
			flusher.Flush()
		}
	}

	// 发送流结束标志
	fmt.Fprint(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}

	// 8. 流结束后记录计费
	if p.billing != nil && keyID != "" {
		if finalResp := reader.Response(); finalResp != nil {
			if berr := p.billing.Record(r.Context(), keyID, finalResp.ModelID, finalResp.Usage); berr != nil {
				if p.log != nil {
					p.log.With(logger.Err(berr)).Error("AI 网关计费记录失败")
				}
			}
		}
	}
}

// handleListModels 处理 GET /v1/models 请求，返回所有已注册模型列表.
func (p *Gateway) handleListModels(w http.ResponseWriter, _ *http.Request) {
	modelNames := p.listModels()
	created := time.Now().Unix()

	infos := make([]modelInfo, 0, len(modelNames))
	for _, name := range modelNames {
		infos = append(infos, modelInfo{
			ID:      name,
			Object:  "model",
			Created: created,
			OwnedBy: "gateway",
		})
	}

	resp := modelsResponse{
		Object: "list",
		Data:   infos,
	}

	_ = response.WriteSuccess(w, resp)
}

// convertMessages 将 Chat Completions 消息列表转换为 llm.Message 列表.
func convertMessages(reqs []messageReq) []llm.Message {
	msgs := make([]llm.Message, 0, len(reqs))
	for _, m := range reqs {
		msgs = append(msgs, llm.Message{
			Role:    llm.Role(m.Role),
			Content: m.Content,
		})
	}
	return msgs
}

// nilIfEmpty 若字符串为空则返回 nil，用于 JSON 输出 finish_reason 为 null.
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
