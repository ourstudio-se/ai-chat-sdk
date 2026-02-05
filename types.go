package aichat

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ExpertType identifies an expert category.
type ExpertType string

// ModelTier represents the tier of OpenAI model to use.
type ModelTier string

const (
	ModelNano      ModelTier = "nano"
	ModelMini      ModelTier = "mini"
	ModelStandard  ModelTier = "standard"
	ModelReasoning ModelTier = "reasoning"
)

// ChatOptions contains optional parameters for chat completions.
type ChatOptions struct {
	Model       ModelTier
	Temperature float32
	MaxTokens   int
}

// ChatJSONOptions contains optional parameters for JSON chat completions.
type ChatJSONOptions struct {
	Model       ModelTier
	Temperature float32
	MaxTokens   int
}

// ChatFn performs a chat completion and returns the response string.
type ChatFn func(ctx context.Context, systemPrompt, userMessage string, opts *ChatOptions) (string, error)

// ChatJSONFn performs a chat completion with JSON mode and unmarshals into result.
type ChatJSONFn func(ctx context.Context, systemPrompt, userMessage string, opts *ChatJSONOptions, result any) error

// ChatStreamFn performs a streaming chat completion and calls the callback for each token.
type ChatStreamFn func(ctx context.Context, systemPrompt, userMessage string, opts *ChatOptions, onToken func(token string)) (string, error)

// TranslationResult contains the result of a translation.
type TranslationResult struct {
	TranslatedMessage string  `json:"translatedMessage"`
	DetectedLanguage  string  `json:"detectedLanguage"`
	Confidence        float64 `json:"confidence"`
}

// TranslateFn translates a message and detects its language.
type TranslateFn func(ctx context.Context, message string) (*TranslationResult, error)

// RouteResult contains the routing decision.
type RouteResult struct {
	Expert     ExpertType
	ExpertName string
	Reasoning  string
}

// RouteQuestionFn routes a question to the appropriate expert.
type RouteQuestionFn func(ctx context.Context, message string, entityID string) (*RouteResult, error)

// ExpertRequest is passed to expert handlers.
// Experts are responsible for resolving any entity data they need using EntityID.
type ExpertRequest struct {
	Message          string
	EntityID         string
	RoutingReasoning string
	Data             any // Structured data passed from the request
}

// ExpertResult is returned by expert handlers.
type ExpertResult struct {
	ExpertType ExpertType `json:"expertType"`
	ExpertName string     `json:"expertName"`
	Answer     string     `json:"answer"`
	Reasoning  string     `json:"reasoning,omitempty"`
	Details    any        `json:"details,omitempty"`
}

// GetDetails extracts the Details field from an ExpertResult as the specified type T.
// This provides type-safe access to expert-specific details that consumers define.
//
// Example:
//
//	type ProductDetails struct {
//	    ProductID string  `json:"productId"`
//	    Product   Product `json:"product"`
//	}
//
//	details, err := aichat.GetDetails[ProductDetails](result.ExpertResult)
//	if err == nil {
//	    fmt.Println(details.Product.Name)  // Full type safety!
//	}
func GetDetails[T any](result *ExpertResult) (T, error) {
	var zero T
	if result == nil {
		return zero, errors.New("expert result is nil")
	}
	if result.Details == nil {
		return zero, errors.New("details is nil")
	}
	details, ok := result.Details.(T)
	if !ok {
		return zero, fmt.Errorf("details type mismatch: expected %T, got %T", zero, result.Details)
	}
	return details, nil
}

// HandleQuestionFn handles an expert question.
type HandleQuestionFn func(ctx context.Context, req ExpertRequest) (*ExpertResult, error)

// HandleQuestionStreamFn handles an expert question with streaming support.
// The stream callback should be called with EventContent events for each token/chunk.
type HandleQuestionStreamFn func(ctx context.Context, req ExpertRequest, stream StreamCallback) (*ExpertResult, error)

// Expert combines expert metadata with its handler.
type Expert struct {
	// Name is the display name of the expert.
	Name string

	// Description is used by the LLM to determine when to route to this expert.
	Description string

	// Handler processes questions for this expert.
	Handler HandleQuestionFn

	// StreamHandler processes questions with streaming support.
	// If nil, Handler will be used and content sent in one chunk.
	StreamHandler HandleQuestionStreamFn
}

// FormatRequest represents a formatting request.
type FormatRequest struct {
	ExpertType         ExpertType
	Answer             string
	OriginalQuestion   string
	TranslatedQuestion string
	DetectedLanguage   string
}

// FormatResponse represents a formatted response.
type FormatResponse struct {
	FormattedAnswer string
	Language        string
}

// FormatResponseFn formats an expert answer for the user.
type FormatResponseFn func(ctx context.Context, req FormatRequest) (*FormatResponse, error)

// ChatRequest represents an incoming chat message.
type ChatRequest struct {
	ConversationID string `json:"conversationId,omitempty"`
	Message        string `json:"message"`
	EntityID       string `json:"entityId,omitempty"`
	Data           any    `json:"data,omitempty"` // Structured data for experts
}

// ChatResult is the processed chat result.
type ChatResult struct {
	ConversationID string        `json:"conversationId"`
	MessageID      string        `json:"messageId"`
	ExpertResult   *ExpertResult `json:"expertResult"`
}

// ProcessChatFn processes a complete chat request.
type ProcessChatFn func(ctx context.Context, req ChatRequest) (*ChatResult, error)

// StreamCallback is called to send streaming events to the client.
type StreamCallback func(event StreamEvent)

// ProcessChatStreamFn processes a chat request with streaming support.
type ProcessChatStreamFn func(ctx context.Context, req ChatRequest, stream StreamCallback) (*ChatResult, error)

// DispatchQuestionFn routes and processes a question with the appropriate expert.
type DispatchQuestionFn func(ctx context.Context, req ExpertRequest) (*ExpertResult, error)

// DispatchQuestionStreamFn routes and processes a question with streaming support.
type DispatchQuestionStreamFn func(ctx context.Context, req ExpertRequest, stream StreamCallback) (*ExpertResult, error)

// MessageRole represents the role of a message sender.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// FeedbackType represents the type of feedback.
type FeedbackType string

const (
	FeedbackThumbsUp   FeedbackType = "thumbs_up"
	FeedbackThumbsDown FeedbackType = "thumbs_down"
)

// MessageFeedback represents user feedback on a message.
type MessageFeedback struct {
	Type      FeedbackType `json:"type"`
	Comment   string       `json:"comment,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// Message represents a single message in a conversation.
type Message struct {
	ID        string           `json:"id"`
	Role      MessageRole      `json:"role"`
	Content   string           `json:"content"`
	Timestamp time.Time        `json:"timestamp"`
	Expert    *string          `json:"expert,omitempty"`
	Data      any              `json:"data,omitempty"`
	Feedback  *MessageFeedback `json:"feedback,omitempty"`
}

// Conversation represents a conversation between a user and the assistant.
type Conversation struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	EntityID  string    `json:"entityId,omitempty"`
	Messages  []Message `json:"messages"`
}

// AddMessage appends a message to the conversation.
func AddMessage(c *Conversation, msg Message) {
	c.Messages = append(c.Messages, msg)
}

// ConversationStore is a struct of functions for conversation persistence.
type ConversationStore struct {
	Create         func(ctx context.Context, entityID string) (*Conversation, error)
	Get            func(ctx context.Context, id string) (*Conversation, error)
	AddMessage     func(ctx context.Context, id string, msg Message) error
	Save           func(ctx context.Context, conversation *Conversation) error
	UpdateFeedback func(ctx context.Context, conversationID, messageID string, feedback MessageFeedback) error
}

// StreamEventType represents the type of server-sent event.
type StreamEventType string

const (
	EventThinking    StreamEventType = "thinking"
	EventTranslating StreamEventType = "translating"
	EventRouting     StreamEventType = "routing"
	EventProcessing  StreamEventType = "processing"
	EventContent     StreamEventType = "content"
	EventDone        StreamEventType = "done"
	EventError       StreamEventType = "error"
)

// StreamEvent represents a server-sent event for streaming responses.
type StreamEvent struct {
	Type           StreamEventType `json:"type"`
	ConversationID *string         `json:"conversationId,omitempty"`
	Expert         *ExpertType     `json:"expert,omitempty"`
	ExpertName     *string         `json:"expertName,omitempty"`
	Content        *string         `json:"content,omitempty"`
	MessageID      *string         `json:"messageId,omitempty"`
	Data           any             `json:"data,omitempty"` // Structured data from expert
}

// HTTPChatRequest represents the HTTP request body for chat endpoints.
type HTTPChatRequest struct {
	Message        string  `json:"message"`
	ConversationID *string `json:"conversationId,omitempty"`
	EntityID       *string `json:"entityId,omitempty"`
	Data           any     `json:"data,omitempty"` // Structured data for experts
}

// HTTPChatResponse represents the HTTP response body for chat endpoints.
type HTTPChatResponse struct {
	ConversationID string     `json:"conversationId"`
	MessageID      string     `json:"messageId"`
	Expert         ExpertType `json:"expert"`
	ExpertName     string     `json:"expertName"`
	Message        string     `json:"message"`
	Reasoning      string     `json:"reasoning"`
	Response       string     `json:"response"`
	Data           any        `json:"data,omitempty"` // Structured data from expert
}

// HTTPFeedbackRequest represents the HTTP request body for feedback endpoints.
type HTTPFeedbackRequest struct {
	ConversationID string       `json:"conversationId"`
	MessageID      string       `json:"messageId"`
	Type           FeedbackType `json:"type"`
	Comment        string       `json:"comment,omitempty"`
}

// HTTPFeedbackResponse represents the HTTP response body for feedback endpoints.
type HTTPFeedbackResponse struct {
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
}
