// Package adk adapts Google ADK agents for servex.
package adk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	"github.com/Tsukikage7/servex/v2/llm"
)

var (
	// ErrNilAgent 表示传入的 ADK agent 为空.
	ErrNilAgent = errors.New("llm/adk: agent is nil")
	// ErrNilModel 表示传入的 servex ChatModel 为空.
	ErrNilModel = errors.New("llm/adk: model is nil")
)

// Config 是创建 ADK agent wrapper 的最小配置.
type Config struct {
	Name        string
	Description string
}

// Agent 包装 Google ADK agent，避免业务代码直接依赖创建细节.
type Agent struct {
	agent agent.Agent
}

// NewAgent 创建一个基础 ADK agent 并包装为 servex adapter.
func NewAgent(cfg Config) (*Agent, error) {
	raw, err := agent.New(agent.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
	})
	if err != nil {
		return nil, err
	}
	return WrapAgent(raw)
}

// LLMAgentConfig 是创建 ADK LLMAgent 的 servex 入口配置.
type LLMAgentConfig struct {
	Name                  string
	Description           string
	Instruction           string
	GlobalInstruction     string
	Model                 llm.ChatModel
	ModelName             string
	GenerateContentConfig *genai.GenerateContentConfig
	SubAgents             []agent.Agent
	Tools                 []tool.Tool
	Toolsets              []tool.Toolset
}

// NewLLMAgent 创建使用 servex ChatModel 的 ADK LLMAgent.
func NewLLMAgent(cfg LLMAgentConfig) (*Agent, error) {
	modelName := cfg.ModelName
	if modelName == "" {
		modelName = cfg.Name
	}
	model, err := AsModel(modelName, cfg.Model)
	if err != nil {
		return nil, err
	}
	raw, err := llmagent.New(llmagent.Config{
		Name:                  cfg.Name,
		Description:           cfg.Description,
		SubAgents:             cfg.SubAgents,
		Model:                 model,
		Instruction:           cfg.Instruction,
		GlobalInstruction:     cfg.GlobalInstruction,
		GenerateContentConfig: cfg.GenerateContentConfig,
		Tools:                 cfg.Tools,
		Toolsets:              cfg.Toolsets,
	})
	if err != nil {
		return nil, err
	}
	return WrapAgent(raw)
}

// WrapAgent 包装已有 ADK agent.
func WrapAgent(adkAgent agent.Agent) (*Agent, error) {
	if adkAgent == nil {
		return nil, ErrNilAgent
	}
	return &Agent{agent: adkAgent}, nil
}

// Agent 返回底层 Google ADK agent.
func (a *Agent) Agent() agent.Agent {
	if a == nil {
		return nil
	}
	return a.agent
}

// Name 返回 agent 名称.
func (a *Agent) Name() string {
	return a.agent.Name()
}

// Description 返回 agent 描述.
func (a *Agent) Description() string {
	return a.agent.Description()
}

// RunnerConfig 是创建 ADK Runner 的 servex 入口配置.
type RunnerConfig struct {
	AppName           string
	Agent             *Agent
	RawAgent          agent.Agent
	SessionService    session.Service
	ArtifactService   artifact.Service
	MemoryService     memory.Service
	Plugins           []*plugin.Plugin
	AutoCreateSession bool
}

// NewRunner 创建 ADK Runner，优先使用 Agent wrapper，必要时可传 RawAgent.
func NewRunner(cfg RunnerConfig) (*runner.Runner, error) {
	raw := cfg.RawAgent
	if cfg.Agent != nil {
		raw = cfg.Agent.Agent()
	}
	return runner.New(runner.Config{
		AppName:           cfg.AppName,
		Agent:             raw,
		SessionService:    cfg.SessionService,
		ArtifactService:   cfg.ArtifactService,
		MemoryService:     cfg.MemoryService,
		PluginConfig:      runner.PluginConfig{Plugins: cfg.Plugins},
		AutoCreateSession: cfg.AutoCreateSession,
	})
}

// AsModel 将 servex ChatModel 适配为 ADK model.LLM.
func AsModel(name string, chatModel llm.ChatModel) (model.LLM, error) {
	if chatModel == nil {
		return nil, ErrNilModel
	}
	return &servexModel{name: name, model: chatModel}, nil
}

type servexModel struct {
	name  string
	model llm.ChatModel
}

func (m *servexModel) Name() string {
	return m.name
}

func (m *servexModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if req == nil {
			yield(nil, errors.New("llm/adk: request is nil"))
			return
		}
		messages, err := fromADKContents(req.Contents)
		if err != nil {
			yield(nil, err)
			return
		}
		opts, err := fromADKRequestOptions(req)
		if err != nil {
			yield(nil, err)
			return
		}
		if !stream {
			resp, err := m.model.Generate(ctx, messages, opts...)
			if err != nil {
				yield(nil, err)
				return
			}
			converted, err := toADKResponse(resp, false)
			yield(converted, err)
			return
		}

		reader, err := m.model.Stream(ctx, messages, opts...)
		if err != nil {
			yield(nil, err)
			return
		}
		defer reader.Close()
		for {
			chunk, err := reader.Recv()
			if errors.Is(err, io.EOF) {
				if resp := reader.Response(); resp != nil {
					converted, err := toADKResponse(resp, false)
					yield(converted, err)
				}
				return
			}
			if err != nil {
				yield(nil, err)
				return
			}
			converted, err := toADKResponse(&llm.ChatResponse{
				Message:      llm.AssistantMessage(chunk.Delta),
				FinishReason: chunk.FinishReason,
			}, true)
			if !yield(converted, err) {
				return
			}
		}
	}
}

func fromADKContents(contents []*genai.Content) ([]llm.Message, error) {
	out := make([]llm.Message, 0, len(contents))
	for _, content := range contents {
		if content == nil {
			continue
		}
		msg, err := fromADKContent(content)
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, nil
}

func fromADKContent(content *genai.Content) (llm.Message, error) {
	msg := llm.Message{Role: fromADKRole(content.Role)}
	var text strings.Builder
	for _, part := range content.Parts {
		if part == nil {
			continue
		}
		if part.Text != "" {
			text.WriteString(part.Text)
		}
		if part.FunctionCall != nil {
			args, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return llm.Message{}, fmt.Errorf("llm/adk: marshal function call %q args: %w", part.FunctionCall.Name, err)
			}
			call := llm.ToolCall{ID: part.FunctionCall.ID}
			call.Function.Name = part.FunctionCall.Name
			call.Function.Arguments = string(args)
			msg.Role = llm.RoleAssistant
			msg.ToolCalls = append(msg.ToolCalls, call)
		}
		if part.FunctionResponse != nil {
			payload, err := json.Marshal(part.FunctionResponse.Response)
			if err != nil {
				return llm.Message{}, fmt.Errorf("llm/adk: marshal function response %q: %w", part.FunctionResponse.Name, err)
			}
			msg.Role = llm.RoleTool
			msg.ToolCallID = part.FunctionResponse.ID
			text.Write(payload)
		}
	}
	msg.Content = text.String()
	return msg, nil
}

func fromADKRole(role string) llm.Role {
	switch role {
	case string(genai.RoleModel):
		return llm.RoleAssistant
	default:
		return llm.RoleUser
	}
}

func fromADKRequestOptions(req *model.LLMRequest) ([]llm.CallOption, error) {
	if req == nil {
		return nil, nil
	}
	out := make([]llm.CallOption, 0, 6)
	if req.Model != "" {
		out = append(out, llm.WithModel(req.Model))
	}
	if req.Config != nil {
		cfg := req.Config
		if cfg.Temperature != nil {
			out = append(out, llm.WithTemperature(float64(*cfg.Temperature)))
		}
		if cfg.MaxOutputTokens > 0 {
			out = append(out, llm.WithMaxTokens(int(cfg.MaxOutputTokens)))
		}
		if cfg.TopP != nil {
			out = append(out, llm.WithTopP(float64(*cfg.TopP)))
		}
		if len(cfg.StopSequences) > 0 {
			out = append(out, llm.WithStop(cfg.StopSequences...))
		}
		tools, err := fromADKTools(cfg.Tools)
		if err != nil {
			return nil, err
		}
		if len(tools) > 0 {
			out = append(out, llm.WithTools(tools...))
		}
	}
	return out, nil
}

func fromADKTools(tools []*genai.Tool) ([]llm.Tool, error) {
	var out []llm.Tool
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		for _, decl := range tool.FunctionDeclarations {
			if decl == nil {
				continue
			}
			converted := llm.Tool{Function: llm.FunctionDef{
				Name:        decl.Name,
				Description: decl.Description,
			}}
			var err error
			if decl.ParametersJsonSchema != nil {
				converted.Function.Parameters, err = json.Marshal(decl.ParametersJsonSchema)
			} else if decl.Parameters != nil {
				converted.Function.Parameters, err = json.Marshal(decl.Parameters)
			}
			if err != nil {
				return nil, fmt.Errorf("llm/adk: marshal tool %q schema: %w", decl.Name, err)
			}
			out = append(out, converted)
		}
	}
	return out, nil
}

func toADKResponse(resp *llm.ChatResponse, partial bool) (*model.LLMResponse, error) {
	if resp == nil {
		return nil, nil
	}
	content, err := toADKContent(resp.Message)
	if err != nil {
		return nil, err
	}
	return &model.LLMResponse{
		Content:       content,
		UsageMetadata: toADKUsage(resp.Usage),
		ModelVersion:  resp.ModelID,
		Partial:       partial,
		TurnComplete:  !partial,
		FinishReason:  genai.FinishReason(resp.FinishReason),
	}, nil
}

func toADKContent(msg llm.Message) (*genai.Content, error) {
	content := &genai.Content{Role: string(genai.RoleModel)}
	if msg.Content != "" {
		content.Parts = append(content.Parts, &genai.Part{Text: msg.Content})
	}
	for _, call := range msg.ToolCalls {
		var args map[string]any
		if call.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("llm/adk: parse tool call %q arguments: %w", call.Function.Name, err)
			}
		}
		content.Parts = append(content.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   call.ID,
				Name: call.Function.Name,
				Args: args,
			},
		})
	}
	return content, nil
}

func toADKUsage(usage llm.Usage) *genai.GenerateContentResponseUsageMetadata {
	return &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     int32(usage.PromptTokens),
		CandidatesTokenCount: int32(usage.CompletionTokens),
		TotalTokenCount:      int32(usage.TotalTokens),
	}
}
