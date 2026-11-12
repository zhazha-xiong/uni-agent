package llm

import "context"

type Client interface {
	Call(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	Stream(ctx context.Context, req *ChatRequest) (<-chan *StreamEvent, error)
}
