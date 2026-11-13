package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"uni-agent/internal/llm"
	"uni-agent/internal/tool"
)

type Loop struct {
	client   llm.Client
	toolSet  map[string]tool.Tool
	tools    []llm.ToolSpec
	messages []llm.Message
	stdin    *bufio.Reader
}

func NewLoop(c llm.Client, set map[string]tool.Tool) (*Loop, error) {
	if c == nil {
		return nil, fmt.Errorf("llm client is nil")
	}
	if set == nil {
		return nil, fmt.Errorf("tool set is nil")
	}
	// 注入系统提示词
	l := &Loop{client: c, toolSet: set, messages: NewSystemMessages(), stdin: bufio.NewReader(os.Stdin)}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		spec, err := toolToSpec(set[name])
		if err != nil {
			return nil, err
		}
		l.tools = append(l.tools, *spec)
	}
	return l, nil
}

// Run 运行 Agent 循环
func (l *Loop) Run(ctx context.Context) error {
	waitInput := true
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 等待用户输入:
			if waitInput {
				fmt.Print("user: ")
				line, err := l.stdin.ReadString('\n')
				if err != nil {
					return fmt.Errorf("读取输入失败: %w", err)
				}
				input := strings.TrimSpace(line)
				if input == "" {
					continue
				}
				l.messages = append(l.messages, llm.Message{Role: llm.RoleUser, Content: input})
			}

			// 调用大模型 API
			resp, err := l.client.Stream(ctx, &llm.Request{Messages: l.messages, Tools: l.tools})
			if err != nil {
				return fmt.Errorf("agent loop 意外终止，错误信息: %w ", err)
			}
			choice, err := llm.ReadStream(resp)
			if err != nil {
				return fmt.Errorf("agent loop 意外终止，错误信息: %w ", err)
			}

			l.messages = append(l.messages, choice.Message)
			switch choice.FinishReason {
			case llm.FinishStop:
				waitInput = true
			case llm.FinishToolCalls:
				// 处理工具调用
				l.handlerToolCall(ctx, choice.Message.ToolCalls)
				waitInput = false
			case llm.FinishLength:
				return fmt.Errorf("tool call truncated by max_tokens: arguments may be incomplete")
			default:
				return fmt.Errorf("未知 finish_reason: %q", choice.FinishReason)
			}
		}
	}
}

// toolToSpec 根据 tool 组装 toolSpec
func toolToSpec(tool tool.Tool) (*llm.ToolSpec, error) {
	toolSpec := llm.ToolSpec{Type: "function"}
	params, err := json.Marshal(tool.Params)
	if err != nil {
		return nil, fmt.Errorf("序列化失败, 请检查 Tool Params: %w", err)
	}
	toolSpec.Function = llm.FunctionDef{Name: tool.Name, Description: tool.Description, Parameters: params}
	return &toolSpec, nil
}

// handlerToolCall 处理工具调用
func (l *Loop) handlerToolCall(ctx context.Context, toolCallList []llm.ToolCall) {
	for _, toolCall := range toolCallList {
		name := toolCall.Function.Name
		id := toolCall.ID
		params := toolCall.Function.Arguments
		t, ok := l.toolSet[name]
		if !ok {
			content := fmt.Sprintf("tool call 发生错误: name: %s 不是合法的工具名，请检查", name)
			l.messages = append(l.messages, llm.Message{Role: llm.RoleTool, Content: content, ToolCallID: id})
			continue
		}
		resp, err := t.Handler(ctx, json.RawMessage(params))
		if err != nil {
			content := fmt.Sprintf("tool call 发生错误: name: %s, reason: %v", name, err)
			l.messages = append(l.messages, llm.Message{Role: llm.RoleTool, Content: content, ToolCallID: id})
			continue
		}
		l.messages = append(l.messages, llm.Message{Role: llm.RoleTool, Content: resp, ToolCallID: id})
	}
}
