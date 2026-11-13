package llm

import (
	"context"
	"fmt"
)

type Client interface {
	Call(ctx context.Context, req *Request) (*ChatResponse, error)
	Stream(ctx context.Context, req *Request) (<-chan *StreamEvent, error)
}

// ReadStream 读取流式消息并返回 Message
func ReadStream(messageChan <-chan *StreamEvent) (*Choice, error) {
	message := Message{Role: RoleAssistant}
	var finishReason *FinishReason
	firstContent := true

	// 用 map 按 index 累积 tool call 碎片
	toolCallMap := make(map[int]*ToolCall)
	maxIndex := -1

	for {
		se, ok := <-messageChan
		if !ok {
			break
		}
		if se.Err != nil {
			return nil, se.Err
		}
		if len(se.Chunk.Choices) == 0 {
			continue
		}

		choice := se.Chunk.Choices[0]
		delta := choice.Delta

		// 1. 文本增量
		if delta.Content != "" {
			message.Content += delta.Content
			if firstContent {
				firstContent = false
				fmt.Print("assistant: ")
			}
			fmt.Print(delta.Content)
		}

		// 2. tool call 碎片增量
		// 每个 tool 调用的第一个 chunk 携带 id、type 和 function, 后续 chunk 只携带 function 参数
		for _, tc := range delta.ToolCalls {
			if tc.Index > maxIndex {
				maxIndex = tc.Index
			}
			toolCall, exists := toolCallMap[tc.Index]
			if !exists {
				toolCall = &ToolCall{Type: "function"}
				toolCallMap[tc.Index] = toolCall
			}

			if tc.ID != "" {
				toolCall.ID = tc.ID
			}
			if tc.Type != "" {
				toolCall.Type = tc.Type
			}
			if tc.Function != nil {
				if tc.Function.Name != "" {
					toolCall.Function.Name = tc.Function.Name
				}
				toolCall.Function.Arguments += tc.Function.Arguments
			}
		}

		if choice.FinishReason != nil {
			finishReason = choice.FinishReason
			break
		}
	}

	if !firstContent {
		fmt.Println()
	}

	// 3. 按 index 顺序组装切片
	if len(toolCallMap) > 0 {
		message.ToolCalls = make([]ToolCall, 0, len(toolCallMap))
		for i := 0; i <= maxIndex; i++ {
			if acc, ok := toolCallMap[i]; ok {
				message.ToolCalls = append(message.ToolCalls, *acc)
			}
		}
	}

	if finishReason == nil {
		return nil, fmt.Errorf("stream ended without finish_reason: connection closed unexpectedly")
	}
	return &Choice{Message: message, FinishReason: *finishReason}, nil
}
