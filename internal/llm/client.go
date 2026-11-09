package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	APIKey     string
	BaseUrl    string
	HTTPClient *http.Client
}

func NewChatClient(apiKey, baseUrl string) *Client {
	c := &http.Client{Timeout: 120 * time.Second}
	return &Client{APIKey: apiKey, BaseUrl: baseUrl, HTTPClient: c}
}

// Call 以 chat completions 方式发送请求
func (c *Client) Call(ctx context.Context, req Request) (*Response, error) {
	if req.Messages == nil {
		return nil, fmt.Errorf("消息体为空")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("data 无法序列化, %e", err)
	}
	url := c.BaseUrl + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("新建 http 请求体失败, %e", err)
	}
	httpReq.Header.Add("Content-Type", "application/json")
	httpReq.Header.Add("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送 http 请求失败, %e", err)
	}

	defer resp.Body.Close()

	if body, err = io.ReadAll(resp.Body); err != nil {
		return nil, fmt.Errorf("获取 resp 失败, %e", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chat completions 返回 %s: %s, %e", resp.Status, snippet(body), err)
	}

	chatResp := &Response{}

	if err = json.Unmarshal(body, chatResp); err != nil {
		return nil, fmt.Errorf("反序列化失败, %e", err)
	}
	return chatResp, nil
}

// snippet 截取前 200 个rune
func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if r := []rune(s); len(r) > 200 {
		return string(r[:200]) + "..."
	}
	return s
}

// ptr 返回一个指针类型变量
func ptr[T any](v T) *T {
	return &v
}
