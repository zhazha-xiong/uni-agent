package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ChatClient struct {
	APIKey     string
	BaseUrl    string
	HTTPClient *http.Client
}

var _ Client = (*ChatClient)(nil)

func NewChatClient(apiKey, baseUrl string) *ChatClient {
	c := &http.Client{}
	// 确保 baseUrl 不含尾斜杠
	if strings.HasSuffix(baseUrl, "/") {
		baseUrl = baseUrl[:len(baseUrl)-1]
	}
	return &ChatClient{APIKey: apiKey, BaseUrl: baseUrl, HTTPClient: c}
}

// Stream 以 chat completions 流式方式发送请求
func (c *ChatClient) Stream(ctx context.Context, req *ChatRequest) (<-chan *StreamEvent, error) {
	if !req.Stream {
		return nil, fmt.Errorf("[StreamChat] Stream is not true")
	}
	respBody, err := c.doHttpRequest(ctx, req)
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
func (c *ChatClient) Call(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req.Stream {
		return nil, fmt.Errorf("[StreamChat] Stream is not false")
	}

	respBody, err := c.doHttpRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return processCallResp(respBody)
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
