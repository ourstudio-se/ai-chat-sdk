package aichat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ExecutionMode determines how the SDK processes requests.
type ExecutionMode string

const (
	// ModeExpert uses deterministic data fetching with a single LLM call.
	// The Expert's Fetcher function controls what data is fetched.
	ModeExpert ExecutionMode = "expert"

	// ModeAgentic lets the LLM decide which tools to call via function calling.
	// Multiple LLM calls may occur as the agent reasons about what data to fetch.
	ModeAgentic ExecutionMode = "agentic"
)

// ChatRequest represents an incoming chat message.
type ChatRequest struct {
	// Message is the user's question or input.
	Message string `json:"message"`

	// ConversationID links this message to an existing conversation.
	// If empty, a new conversation is created.
	ConversationID string `json:"conversationId,omitempty"`

	// EntityID is an optional identifier for the entity being discussed
	// (e.g., product ID, user ID). Used for data fetching.
	EntityID string `json:"entityId,omitempty"`

	// Context contains additional context from the app layer
	// (e.g., market, locale, productId, userId).
	Context RequestContext `json:"context,omitempty"`

	// Mode overrides the SDK's default execution mode for this request.
	Mode ExecutionMode `json:"mode,omitempty"`

	// SkillID forces routing to a specific skill (bypasses router).
	SkillID string `json:"skillId,omitempty"`

	// Variant forces a specific A/B test variant.
	Variant string `json:"variant,omitempty"`
}

// RequestContext holds contextual information from the app layer.
type RequestContext map[string]any

// String returns a string value from context with a default.
func (c RequestContext) String(key, defaultValue string) string {
	if c == nil {
		return defaultValue
	}
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

// Int returns an int value from context with a default.
func (c RequestContext) Int(key string, defaultValue int) int {
	if c == nil {
		return defaultValue
	}
	if v, ok := c[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return defaultValue
}

// Bool returns a bool value from context with a default.
func (c RequestContext) Bool(key string, defaultValue bool) bool {
	if c == nil {
		return defaultValue
	}
	if v, ok := c[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// ChatResult is the response from processing a chat request.
type ChatResult struct {
	// ConversationID identifies the conversation.
	ConversationID string `json:"conversationId"`

	// MessageID uniquely identifies this response message.
	MessageID string `json:"messageId"`

	// SkillID is the skill that handled the request.
	SkillID string `json:"skillId"`

	// Variant is the A/B test variant used.
	Variant string `json:"variant,omitempty"`

	// Mode is the execution mode used.
	Mode ExecutionMode `json:"mode"`

	// ToolCalls lists the tools that were called during processing.
	ToolCalls []ToolCall `json:"toolsCalled,omitempty"`

	// Response is the typed JSON response matching the skill's output schema.
	Response json.RawMessage `json:"response"`

	// SuggestedAction is an optional action suggested by the LLM
	// (expert mode only - not executed, just suggested).
	SuggestedAction *SuggestedAction `json:"suggestedAction,omitempty"`

	// TokensUsed is the total tokens consumed.
	TokensUsed TokenUsage `json:"tokensUsed,omitempty"`

	// Duration is how long processing took.
	Duration time.Duration `json:"duration,omitempty"`
}

// ToolCall represents a tool invocation during processing.
type ToolCall struct {
	// Name is the tool that was called.
	Name string `json:"name"`

	// Params are the parameters passed to the tool.
	Params map[string]any `json:"params,omitempty"`

	// Duration is how long the tool call took.
	Duration time.Duration `json:"duration,omitempty"`
}

// SuggestedAction represents an action the LLM wants to perform.
// In expert mode, actions are suggested but not executed.
type SuggestedAction struct {
	// Tool is the action tool to execute.
	Tool string `json:"tool"`

	// Params are the parameters for the action.
	Params map[string]any `json:"params"`

	// Reason explains why this action is suggested.
	Reason string `json:"reason,omitempty"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	// PromptTokens is the tokens used in the prompt.
	PromptTokens int `json:"promptTokens"`

	// CompletionTokens is the tokens in the response.
	CompletionTokens int `json:"completionTokens"`

	// TotalTokens is the sum of prompt and completion tokens.
	TotalTokens int `json:"totalTokens"`
}

// SkillRequest is used when executing a skill directly (e.g., from an Expert).
type SkillRequest struct {
	// Message is the user's question.
	Message string

	// Data is custom data to include in the prompt (provided by Expert.Fetcher).
	Data any

	// Context is additional context from the request.
	Context RequestContext

	// Variant forces a specific A/B test variant.
	Variant string

	// ConversationID for conversation history context.
	ConversationID string
}

// SkillResult is the result of executing a skill.
type SkillResult struct {
	// Response is the typed JSON response.
	Response json.RawMessage

	// Variant is the variant that was used.
	Variant string

	// TokensUsed tracks token consumption.
	TokensUsed TokenUsage
}

// Message represents a message in a conversation.
type Message struct {
	// ID uniquely identifies the message.
	ID string `json:"id"`

	// ConversationID links the message to a conversation.
	ConversationID string `json:"conversationId"`

	// Role is "user" or "assistant".
	Role string `json:"role"`

	// Content is the message content.
	Content string `json:"content"`

	// SkillID is the skill that generated assistant messages.
	SkillID string `json:"skillId,omitempty"`

	// Variant is the A/B test variant used.
	Variant string `json:"variant,omitempty"`

	// Metadata holds additional message data.
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is when the message was created.
	CreatedAt time.Time `json:"createdAt"`
}

// Conversation represents a chat conversation.
type Conversation struct {
	// ID uniquely identifies the conversation.
	ID string `json:"id"`

	// EntityID is the entity being discussed.
	EntityID string `json:"entityId,omitempty"`

	// Context is conversation-level context.
	Context RequestContext `json:"context,omitempty"`

	// Messages are the messages in the conversation.
	Messages []Message `json:"messages"`

	// CreatedAt is when the conversation started.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the conversation was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// Feedback represents user feedback on a message.
type Feedback struct {
	// MessageID is the message being rated.
	MessageID string `json:"messageId"`

	// Rating is the feedback score (e.g., 1-5 or thumbs up/down).
	Rating int `json:"rating"`

	// Comment is optional feedback text.
	Comment string `json:"comment,omitempty"`

	// CreatedAt is when feedback was submitted.
	CreatedAt time.Time `json:"createdAt"`
}

// NewConversationID generates a new conversation ID.
func NewConversationID() string {
	return uuid.New().String()
}

// NewMessageID generates a new message ID.
func NewMessageID() string {
	return uuid.New().String()
}

// GetResponse extracts a typed response from a ChatResult.
// It unmarshals the JSON response into the provided type T.
func GetResponse[T any](result ChatResult) (T, error) {
	var response T
	if err := json.Unmarshal(result.Response, &response); err != nil {
		return response, NewSDKError(ErrCodeValidation, "failed to unmarshal response", err)
	}
	return response, nil
}

// GetSkillResponse extracts a typed response from a SkillResult.
func GetSkillResponse[T any](result SkillResult) (T, error) {
	var response T
	if err := json.Unmarshal(result.Response, &response); err != nil {
		return response, NewSDKError(ErrCodeValidation, "failed to unmarshal response", err)
	}
	return response, nil
}

// ToolExecutor allows Expert fetchers to execute registered tools.
type ToolExecutor interface {
	// Execute runs a tool by name with the given parameters.
	Execute(ctx context.Context, toolName string, params map[string]any) (any, error)
}
