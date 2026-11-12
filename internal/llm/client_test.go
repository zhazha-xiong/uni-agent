package llm

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func TestClient_Call(t *testing.T) {
	req, c, err := getReq()
	if err != nil {
		t.Error(err)
		return
	}
	ctx := context.Background()
	req.Stream = false
	resp, err := c.Call(ctx, req)
	if err != nil {
		t.Error(err)
		return
	}
	if len(resp.Choices) == 0 {
		t.Errorf("请求候选答案为空")
		return
	}
	ch := resp.Choices[0]

	u := resp.Usage
	if u != nil && (u.PromptTokens == 0 || u.CompletionTokens == 0) {
		t.Errorf("请求 token 为 0")
		return
	}
	if u != nil {
		log.Printf("总 Token 消耗: %d, 用户提示词开销: %d, llm 回复开销: %d", u.TotalTokens, u.PromptTokens, u.CompletionTokens)
	}
	if ch.FinishReason != FinishStop {
		t.Errorf("请求非自然停止，结束原因为：%s", ch.FinishReason)
		return
	}
	if ch.Message.Role != RoleAssistant {
		t.Errorf("响应结果并非来自 LLM")
		return
	}
	if len(ch.Message.Content) == 0 {
		t.Errorf("响应结果为空")
		return
	}
	log.Print(ch.Message.Content)
}

func TestClient_Stream(t *testing.T) {
	req, c, err := getReq()
	if err != nil {
		t.Error(err)
		return
	}
	ctx := context.Background()
	req.Stream = true
	messageChan, err := c.Stream(ctx, req)
	done := false
	for {
		se, ok := <-messageChan
		if !ok {
			break
		}
		if se.Err != nil {
			t.Error(se)
			return
		}
		resp := se.Chunk
		if len(resp.Choices) == 0 {
			t.Errorf("请求候选答案为空")
			return
		}
		ch := resp.Choices[0]
		// 流式请求非最后一帧结束理由均为 null
		if ch.FinishReason != nil && *ch.FinishReason == FinishStop {
			done = true
			u := resp.Usage
			if u != nil && (u.PromptTokens == 0 || u.CompletionTokens == 0) {
				t.Errorf("请求 token 为 0")
				return
			}
			if u != nil {
				log.Printf("\n总 Token 消耗: %d, 用户提示词开销: %d, llm 回复开销: %d\n", u.TotalTokens, u.PromptTokens, u.CompletionTokens)
			}
			break
		}
		if ch.Delta.Role != nil && *ch.Delta.Role != RoleAssistant {
			t.Errorf("响应结果并非来自 LLM")
			return
		}
		if ch.Delta.Role == nil && len(ch.Delta.Content) == 0 {
			t.Errorf("响应结果为空")
			return
		}
		log.Print(ch.Delta.Content)
	}
	if !done {
		t.Errorf("流式响应非正常终止")
		return
	}
}

// ptr 返回一个指针类型变量
func ptr[T any](v T) *T {
	return &v
}

// getReq 获取 llm chat 请求体
func getReq() (*ChatRequest, *Client, error) {
	err := godotenv.Load("../../.env")
	if err != nil {
		return nil, nil, fmt.Errorf(".env 文件未找到")
	}

	apiKey := os.Getenv("APIKEY")
	baseUrl := os.Getenv("BASEURL")

	if len(apiKey) == 0 || len(baseUrl) == 0 {
		return nil, nil, fmt.Errorf("apikey 和 baseUrl 未读取")
	}

	c := NewChatClient(apiKey, baseUrl)
	m := Message{Role: RoleUser, Content: "你好，Deepseek!"}
	req := ChatRequest{Model: "deepseek-flash",
		Messages:        []Message{m},
		MaxTokens:       2000,
		Temperature:     ptr(0.0),
		ReasoningEffort: ReasoningNone,
	}

	return &req, c, nil
}
