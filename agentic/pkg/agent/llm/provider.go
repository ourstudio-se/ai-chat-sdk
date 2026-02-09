package llm

import (
	"context"

	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/tools"
)

// Provider defines the interface for LLM providers
type Provider interface {
	Chat(ctx context.Context, req Request) (*Response, error)
}

// Request is a provider-agnostic chat request
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Tools       []tools.Definition
	MaxTokens   int
	Temperature float64
}

// Message is a provider-agnostic message
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolResult *ToolResult
}

// Role is the message role
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall represents a tool invocation request
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResult represents a tool execution result
type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// Response is a provider-agnostic response
type Response struct {
	Content    string
	ToolCalls  []ToolCall
	StopReason StopReason
	Usage      Usage
}

// StopReason indicates why the model stopped
type StopReason string

const (
	StopReasonEnd     StopReason = "end"
	StopReasonToolUse StopReason = "tool_use"
	StopReasonLength  StopReason = "length"
)

// Usage contains token usage information
type Usage struct {
	InputTokens  int
	OutputTokens int
}
