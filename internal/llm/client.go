package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type ChatClient struct {
	APIKey      string
	BaseUrl     string
	Model       string
	Effort      ReasoningEffort
	MaxTokens   int
	Temperature *float64
	HTTPClient  *http.Client
}

var _ Client = (*ChatClient)(nil)

func NewChatClient(apiKey, baseUrl, model, reason, maxTokens, temperature string) (*ChatClient, error) {
	// 确保 baseUrl 不含尾斜杠
	if strings.HasSuffix(baseUrl, "/") {
		baseUrl = baseUrl[:len(baseUrl)-1]
	}
	c := ChatClient{APIKey: apiKey, BaseUrl: baseUrl, Model: model, HTTPClient: &http.Client{}}
	if reason != "" {
		effort, err := ParseReasoningEffort(reason)
		if err != nil {
			return nil, fmt.Errorf("创建 llm client 失败，思考强度非法")
		}
		c.Effort = effort
	}
	if maxTokens != "" {
		tokens, err := strconv.Atoi(maxTokens)
		if err != nil {
			return nil, fmt.Errorf("创建 llm client 失败，MAX TOKENS 非法")
		}
		if tokens < 1 || tokens > 393216 {
			return nil, fmt.Errorf("创建 llm client 失败，MAX TOKENS 非法")
		}
		c.MaxTokens = tokens
	}
	if temperature != "" {
		t, err := strconv.ParseFloat(temperature, 64)
		if err != nil {
			return nil, fmt.Errorf("创建 llm client 失败，temperature 非法")
		}
		if t < 0 || t > 2 {
			return nil, fmt.Errorf("创建 llm client 失败，temperature 非法")
		}
		c.Temperature = ptr(t)
	}
	return &c, nil
}

// Stream 以 chat completions 流式方式发送请求
func (c *ChatClient) Stream(ctx context.Context, req *Request) (<-chan *StreamEvent, error) {
	chatReq, err := c.buildChatRequest(req, true)
	if err != nil {
		return nil, err
	}
	respBody, err := c.doHttpRequest(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	messageChan := make(chan *StreamEvent)
	go func() {
		processStreamResp(ctx, respBody, messageChan)
	}()
	return messageChan, nil
}

// Call 以 chat completions 非流式方式发送请求
func (c *ChatClient) Call(ctx context.Context, req *Request) (*ChatResponse, error) {
	chatReq, err := c.buildChatRequest(req, false)
	if err != nil {
		return nil, err
	}
	respBody, err := c.doHttpRequest(ctx, chatReq)
	if err != nil {
		return nil, err
	}
	return processCallResp(respBody)
}

// buildChatRequest 组装 chatRequest
func (c *ChatClient) buildChatRequest(req *Request, stream bool) (*ChatRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("请求体为空")
	}
	chatReq := &ChatRequest{Model: c.Model,
		Messages:        req.Messages,
		ReasoningEffort: c.Effort,
		MaxTokens:       c.MaxTokens,
		Temperature:     c.Temperature,
		Stream:          stream,
		Tools:           req.Tools}
	return chatReq, nil
}

// doHttpRequest 发送 HTTP 请求
func (c *ChatClient) doHttpRequest(ctx context.Context, req *ChatRequest) (io.ReadCloser, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("消息体为空")
	}
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("data 无法序列化, %w", err)
	}
	url := c.BaseUrl + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("新建 http 请求体失败, %w", err)
	}
	httpReq.Header.Add("Content-Type", "application/json")
	httpReq.Header.Add("Authorization", "Bearer "+c.APIKey)
	if req.Stream {
		httpReq.Header.Add("Accept", "text/event-stream")
	}
	// 调用方需要主动关闭
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送 http 请求失败, %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat completions 返回 %s: %s", resp.Status, body)
	}

	return resp.Body, nil
}

// processCallResp 处理非流式响应
func processCallResp(respBody io.ReadCloser) (*ChatResponse, error) {
	defer respBody.Close()
	resp, err := io.ReadAll(respBody)
	if err != nil {
		return nil, fmt.Errorf("获取 resp 失败, %w", err)
	}

	chatResp := &ChatResponse{}

	if err := json.Unmarshal(resp, chatResp); err != nil {
		return nil, fmt.Errorf("反序列化失败, %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("响应候选结果为空")
	}
	return chatResp, nil
}

// processStreamResp 处理流式响应
func processStreamResp(ctx context.Context, respBody io.ReadCloser, messageChan chan *StreamEvent) {
	defer close(messageChan)
	defer respBody.Close()

	br := bufio.NewReader(respBody)
	for {
		var se StreamEvent
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				// 流式响应终止
				if data == "[DONE]" {
					return
				}

				var chunk Chunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					se.Err = err
					_ = sendStreamEvent(ctx, messageChan, se)
					return
				}
				se.Chunk = &chunk
				_ = sendStreamEvent(ctx, messageChan, se)
			}

		}
		if err == io.EOF {
			se.Err = fmt.Errorf("流在收到 [DONE] 之前结束了")
			_ = sendStreamEvent(ctx, messageChan, se)
			return
		}
		if err != nil {
			se.Err = err
			_ = sendStreamEvent(ctx, messageChan, se)
			return
		}
	}
}

// sendStreamEvent 发送 StreamEvent 帧
func sendStreamEvent(ctx context.Context, messageChan chan<- *StreamEvent, se StreamEvent) error {
	select {
	case messageChan <- &se:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func ptr[T any](v T) *T {
	return &v
}
