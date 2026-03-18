package llmprovider

import "context"

// Message represents a chat message in the conversation.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the function name and serialized arguments.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition describes a tool available to the model.
type ToolDefinition struct {
	Type     string          `json:"type"`
	Function ToolDefFunction `json:"function"`
}

// ToolDefFunction holds the function schema for a tool definition.
type ToolDefFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// CompletionRequest is sent to the LLM provider.
type CompletionRequest struct {
	Messages    []Message
	Tools       []ToolDefinition
	Temperature float64
	Model       string // optional override
}

// CompletionResponse is returned by the LLM provider.
type CompletionResponse struct {
	Message      Message
	FinishReason string
	Usage        Usage
}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Provider is the interface for any LLM backend.
type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	Name() string
}
