package llm

import (
	"encoding/json"
	"fmt"
)

// Role 消息的发起角色
type Role string

// 消息角色
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 对话中的一条消息，四种角色共用此结构
type Message struct {
	Role Role `json:"role"`

	Content string `json:"content"`

	// ToolCalls 仅 assistant 消息使用
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID 仅 tool 消息使用，必须回填对应的 ToolCall.ID
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall 模型发起的一次工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // 恒为 "function"
	Function ToolCallFunc `json:"function"`
}

// ToolCallFunc 调用的函数名与参数
type ToolCallFunc struct {
	Name string `json:"name"`

	// Arguments JSON 字符串，不是 JSON 对象
	// 这里不解析：模型可能产出非法 JSON，解析与校验交给工具层
	Arguments string `json:"arguments"`
}

// ToolSpec 提供给模型的工具声明
type ToolSpec struct {
	Type     string      `json:"type"` // 恒为 "function"
	Function FunctionDef `json:"function"`
}

// FunctionDef 描述一个工具
type FunctionDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Parameters JSON Schema 原文，原样透传以免序列化时丢字段
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

// Request 调用 llm Chat/Stream 的请求体
type Request struct {
	Messages []Message  `json:"messages"`
	Tools    []ToolSpec `json:"tools,omitempty"`
}

// ReasoningEffort 控制思考强度，none 表示关闭思考
type ReasoningEffort string

// 思考强度取值
const (
	ReasoningNone ReasoningEffort = "none"
	ReasoningLow  ReasoningEffort = "low"
	ReasoningHigh ReasoningEffort = "high"
	ReasoningMax  ReasoningEffort = "max"
)

func ParseReasoningEffort(s string) (ReasoningEffort, error) {
	switch e := ReasoningEffort(s); e {
	case ReasoningNone, ReasoningLow, ReasoningHigh, ReasoningMax:
		return e, nil
	default:
		return "", fmt.Errorf("invalid reasoning effort %q", s)
	}
}

// ChatRequest POST /chat/completions 的请求体
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`

	// ReasoningEffort 留空时服务端按 high 处理，即默认开启思考
	ReasoningEffort ReasoningEffort `json:"reasoning_effort,omitempty"`

	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature 用指针区分未设置（nil）与明确设为 0
	Temperature *float64 `json:"temperature,omitempty"`

	Stream bool `json:"stream"`

	Tools []ToolSpec `json:"tools,omitempty"`
}

// ChatResponse 一次非流式回复
type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice 候选回复之一，取第 0 个即可
type Choice struct {
	Message      Message      `json:"message"`
	FinishReason FinishReason `json:"finish_reason"`
}

// FinishReason 模型停止生成的原因
type FinishReason string

// 停止原因
const (
	FinishStop      FinishReason = "stop"
	FinishLength    FinishReason = "length"
	FinishToolCalls FinishReason = "tool_calls"
)

// Usage token 用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamEvent 带错误的流式响应帧
type StreamEvent struct {
	Chunk *Chunk
	Err   error
}

// Chunk SSE 流式响应中的一帧
type Chunk struct {
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"` // 只出现在最后一帧
}

// ChunkChoice 分片中的一个候选
type ChunkChoice struct {
	Delta Delta `json:"delta"`

	// FinishReason 中途为 nil，最后一帧才有值
	FinishReason *FinishReason `json:"finish_reason"`
}

// Delta 本帧相对上一帧的增量
type Delta struct {
	Role      *Role           `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []ToolCallChunk `json:"tool_calls,omitempty"`
}

// ToolCallChunk 工具调用在流式下的分片
type ToolCallChunk struct {
	Index    int                `json:"index"` // 决定分片归并到第几个调用
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function *ToolCallFuncChunk `json:"function,omitempty"`
}

// ToolCallFuncChunk 分片形式的函数名与参数
type ToolCallFuncChunk struct {
	Name string `json:"name,omitempty"`

	// Arguments 片段，按到达顺序拼接后才是完整 JSON
	Arguments string `json:"arguments,omitempty"`
}
