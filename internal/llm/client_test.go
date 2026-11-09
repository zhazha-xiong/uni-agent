package llm

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func TestClient_Call(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Errorf(".env 文件未找到")
		return
	}

	apiKey := os.Getenv("APIKEY")
	baseUrl := os.Getenv("BASEURL")

	if len(apiKey) == 0 || len(baseUrl) == 0 {
		t.Errorf("apikey 和 baseUrl 未读取")
		return
	}

	c := NewChatClient(apiKey, baseUrl)
	m := Message{Role: RoleUser, Content: "你好，Deepseek!"}
	req := Request{Model: "deepseek-flash",
		Messages:        []Message{m},
		MaxTokens:       2000,
		Temperature:     ptr(0.0),
		Stream:          false,
		ReasoningEffort: ReasoningNone,
	}
	ctx := context.Background()
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
