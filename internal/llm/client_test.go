package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

// TestMain 只在本地有 .env 时加载它，找不到不算错
func TestMain(m *testing.M) {
	loadDotEnv()
	os.Exit(m.Run())
}

func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		p := filepath.Join(dir, ".env")
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

// ---------- Call ----------

func TestCall_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{
			"id": "chatcmpl-1",
			"choices": [{
				"message": {"role": "assistant", "content": "你好"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5}
		}`)
	}))
	defer srv.Close()

	c := NewChatClient("sk-test", srv.URL)
	resp, err := c.Call(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("期望成功，实际报错: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices 数量 = %d, 期望 1", len(resp.Choices))
	}
	if got := resp.Choices[0].Message.Content; got != "你好" {
		t.Errorf("content = %q, 期望 你好", got)
	}
	if got := resp.Choices[0].FinishReason; got != FinishStop {
		t.Errorf("finish_reason = %q, 期望 %q", got, FinishStop)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 5 {
		t.Errorf("usage 解析错误: %+v", resp.Usage)
	}
}

func TestCall_RequestShape(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotAuth   string
		gotCT     string
		gotBody   struct {
			Model           string    `json:"model"`
			Messages        []Message `json:"messages"`
			Stream          bool      `json:"stream"`
			ReasoningEffort string    `json:"reasoning_effort"`
			MaxTokens       int       `json:"max_tokens"`
		}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	c := NewChatClient("sk-test", srv.URL+"/") // 故意带尾斜杠
	_, err := c.Call(context.Background(), &ChatRequest{
		Model:           "deepseek-flash",
		Messages:        []Message{{Role: RoleUser, Content: "hi"}},
		ReasoningEffort: ReasoningNone,
		MaxTokens:       100,
	})
	if err != nil {
		t.Fatalf("期望成功，实际报错: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, 期望 POST", gotMethod)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, 期望 /chat/completions（检查尾斜杠处理）", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(gotCT, "application/json") {
		t.Errorf("Content-Type = %q", gotCT)
	}
	if gotBody.Model != "deepseek-flash" {
		t.Errorf("model = %q", gotBody.Model)
	}
	if gotBody.Stream {
		t.Error("Call 发出的请求 stream 应为 false")
	}
	if gotBody.ReasoningEffort != string(ReasoningNone) {
		t.Errorf("reasoning_effort = %q, 期望 %q", gotBody.ReasoningEffort, ReasoningNone)
	}
	if gotBody.MaxTokens != 100 {
		t.Errorf("max_tokens = %d, 期望 100", gotBody.MaxTokens)
	}
}

func TestCall_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	c := NewChatClient("sk-bad", srv.URL)
	resp, err := c.Call(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("401 应返回错误")
	}
	if resp != nil {
		t.Error("出错时应返回 nil 响应")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("错误信息应包含状态码，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("错误信息应包含响应体，实际: %v", err)
	}
}

func TestCall_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[]}`)
	}))
	defer srv.Close()

	c := NewChatClient("sk-test", srv.URL)
	_, err := c.Call(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err == nil || !strings.Contains(err.Error(), "候选结果为空") {
		t.Fatalf("空 choices 应报错，实际: %v", err)
	}
}

func TestCall_EmptyMessages(t *testing.T) {
	c := NewChatClient("sk-test", "http://127.0.0.1:0")
	_, err := c.Call(context.Background(), &ChatRequest{Model: "deepseek-flash"})
	if err == nil || !strings.Contains(err.Error(), "消息体为空") {
		t.Fatalf("空消息应报错，实际: %v", err)
	}
}

func TestCall_RejectsStreamRequest(t *testing.T) {
	c := NewChatClient("sk-test", "http://127.0.0.1:0")
	_, err := c.Call(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Stream:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "Stream") {
		t.Fatalf("Call 收到 stream 请求应报错，实际: %v", err)
	}
}

// ---------- Stream ----------

// newSSEServer 按帧返回 SSE，每帧自动 flush
func newSSEServer(t *testing.T, frames []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter 不支持 Flush")
			return
		}
		for _, f := range frames {
			fmt.Fprint(w, f)
			flusher.Flush()
		}
	}))
}

func collect(t *testing.T, ch <-chan *StreamEvent) []*StreamEvent {
	t.Helper()
	var got []*StreamEvent
	for ev := range ch {
		got = append(got, ev)
	}
	return got
}

func newStreamClient(t *testing.T, frames []string) *ChatClient {
	t.Helper()
	srv := newSSEServer(t, frames)
	t.Cleanup(srv.Close)
	return NewChatClient("sk-test", srv.URL)
}

func TestStream_Success(t *testing.T) {
	c := newStreamClient(t, []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"你\"},\"finish_reason\":null}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"好\"},\"finish_reason\":null}]}\n\n",
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n",
		"data: [DONE]\n\n",
	})
	ch, err := c.Stream(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("期望成功，实际报错: %v", err)
	}

	events := collect(t, ch)
	if len(events) != 3 {
		t.Fatalf("事件数 = %d, 期望 3", len(events))
	}

	var content strings.Builder
	for _, ev := range events {
		if ev.Err != nil {
			t.Fatalf("不应有错误事件: %v", ev.Err)
		}
		for _, c := range ev.Chunk.Choices {
			content.WriteString(c.Delta.Content)
		}
	}
	if content.String() != "你好" {
		t.Errorf("拼接内容 = %q, 期望 你好", content.String())
	}

	last := events[len(events)-1].Chunk
	if last.Choices[0].FinishReason == nil || *last.Choices[0].FinishReason != FinishStop {
		t.Errorf("最后一帧 finish_reason 应为 stop")
	}
	if last.Usage == nil || last.Usage.TotalTokens != 3 {
		t.Errorf("最后一帧 usage 解析错误: %+v", last.Usage)
	}
}

func TestStream_IgnoresNonDataLines(t *testing.T) {
	c := newStreamClient(t, []string{
		": keep-alive\n\n",
		"event: ping\n\n",
		"\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\n",
		"data: [DONE]\n\n",
	})
	ch, err := c.Stream(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("期望成功，实际报错: %v", err)
	}

	events := collect(t, ch)
	if len(events) != 1 {
		t.Fatalf("事件数 = %d, 期望 1（非 data 行应被忽略）", len(events))
	}
	if events[0].Err != nil {
		t.Fatalf("不应有错误事件: %v", events[0].Err)
	}
}

func TestStream_MissingDoneReturnsError(t *testing.T) {
	c := newStreamClient(t, []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"半截\"},\"finish_reason\":null}]}\n\n",
	})
	ch, err := c.Stream(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("期望成功，实际报错: %v", err)
	}

	events := collect(t, ch)
	if len(events) == 0 {
		t.Fatal("应至少收到一个事件")
	}
	last := events[len(events)-1]
	if last.Err == nil || !strings.Contains(last.Err.Error(), "DONE") {
		t.Fatalf("缺少 [DONE] 时应返回错误事件，实际: %v", last.Err)
	}
}

func TestStream_BadJSONReturnsError(t *testing.T) {
	c := newStreamClient(t, []string{
		"data: {not json}\n\n",
	})
	ch, err := c.Stream(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("期望成功，实际报错: %v", err)
	}

	events := collect(t, ch)
	if len(events) != 1 || events[0].Err == nil {
		t.Fatalf("非法 JSON 应产生一个错误事件，实际: %+v", events)
	}
}

func TestStream_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer srv.Close()

	c := NewChatClient("sk-test", srv.URL)
	ch, err := c.Stream(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Stream:   true,
	})
	if err == nil {
		t.Fatal("429 应同步返回错误")
	}
	if ch != nil {
		t.Error("出错时 channel 应为 nil")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("错误信息应包含状态码，实际: %v", err)
	}
}

func TestStream_RejectsNonStreamRequest(t *testing.T) {
	c := NewChatClient("sk-test", "http://127.0.0.1:0")
	ch, err := c.Stream(context.Background(), &ChatRequest{
		Model:    "deepseek-flash",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("Stream 收到非流式请求应报错")
	}
	if ch != nil {
		t.Error("出错时 channel 应为 nil")
	}
}

// ---------- 真实 API 冒烟测试（默认跳过） ----------

func realClient(t *testing.T) *ChatClient {
	t.Helper()
	if os.Getenv("LLM_INTEGRATION") == "" {
		t.Skip("跳过真实 API 测试：设置 LLM_INTEGRATION=1 才运行")
	}
	key := os.Getenv("LLM_APIKEY")
	baseURL := os.Getenv("LLM_BASEURL")
	if key == "" || baseURL == "" {
		t.Skip("未设置 LLM_APIKEY / LLM_BASEURL，跳过真实 API 测试")
	}
	return NewChatClient(key, baseURL)
}

func TestRealAPI_Call(t *testing.T) {
	c := realClient(t)
	resp, err := c.Call(context.Background(), &ChatRequest{
		Model:           "deepseek-flash",
		Messages:        []Message{{Role: RoleUser, Content: "你好，Deepseek!"}},
		MaxTokens:       200,
		ReasoningEffort: ReasoningNone,
	})
	if err != nil {
		t.Fatalf("真实 API 调用失败: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("候选答案为空")
	}
	ch := resp.Choices[0]
	if ch.FinishReason != FinishStop {
		t.Errorf("非自然停止: %s", ch.FinishReason)
	}
	if ch.Message.Role != RoleAssistant {
		t.Errorf("响应角色 = %q, 期望 assistant", ch.Message.Role)
	}
	if ch.Message.Content == "" {
		t.Error("响应内容为空")
	}
	if resp.Usage == nil || resp.Usage.TotalTokens == 0 {
		t.Errorf("usage 异常: %+v", resp.Usage)
	}
	t.Logf("回复: %s", ch.Message.Content)
}

func TestRealAPI_Stream(t *testing.T) {
	c := realClient(t)
	ch, err := c.Stream(context.Background(), &ChatRequest{
		Model:           "deepseek-flash",
		Messages:        []Message{{Role: RoleUser, Content: "你好，Deepseek!"}},
		MaxTokens:       200,
		ReasoningEffort: ReasoningNone,
		Stream:          true,
	})
	if err != nil {
		t.Fatalf("真实 API 流式请求失败: %v", err)
	}

	var content strings.Builder
	var sawStop bool
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("流式返回错误: %v", ev.Err)
		}
		for _, c := range ev.Chunk.Choices {
			content.WriteString(c.Delta.Content)
			if c.FinishReason != nil && *c.FinishReason == FinishStop {
				sawStop = true
			}
		}
	}
	if !sawStop {
		t.Error("没有收到 finish_reason=stop")
	}
	if content.Len() == 0 {
		t.Error("流式拼接内容为空")
	}
	t.Logf("回复: %s", content.String())
}
