package aichat

import (
	"context"
	"log/slog"
	"time"
)

// Config configures the SDK instance.
type Config struct {
	// LLMClient is the OpenAI client for LLM calls.
	// Required.
	LLMClient LLMClient

	// Skills is the registry of loaded skills.
	// Required.
	Skills SkillRegistry

	// Tools is the registry of available tools.
	// Required.
	Tools ToolRegistry

	// Experts are the expert implementations (for expert mode).
	// Optional - if not provided, skills work in pure agentic mode.
	Experts []*Expert

	// ExecutionMode is the default execution mode.
	// Defaults to ModeExpert.
	ExecutionMode ExecutionMode

	// Storage is the conversation storage backend.
	// Optional - defaults to in-memory storage.
	Storage ConversationStore

	// Logger is the structured logger.
	// Optional - defaults to slog.Default().
	Logger *slog.Logger

	// DefaultSkillID is the fallback skill when no triggers match.
	// Optional.
	DefaultSkillID string

	// MaxAgentTurns limits the number of turns in agentic mode.
	// Defaults to 10.
	MaxAgentTurns int

	// RequestTimeout is the maximum time for a chat request.
	// Defaults to 60 seconds.
	RequestTimeout time.Duration

	// AllowedOrigins for CORS in the HTTP server.
	// Defaults to allowing all origins.
	AllowedOrigins []string

	// Hooks is the registry for pre/post processing hooks.
	// Optional.
	Hooks *HookRegistry

	// Model is the default LLM model to use.
	// Defaults to "gpt-4o".
	Model string

	// Temperature is the default temperature for LLM calls.
	// Defaults to 0.7.
	Temperature float32
}

// withDefaults applies default values to the config.
func (c Config) withDefaults() Config {
	if c.ExecutionMode == "" {
		c.ExecutionMode = ModeExpert
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.MaxAgentTurns <= 0 {
		c.MaxAgentTurns = 10
	}
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = 60 * time.Second
	}
	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = []string{"*"}
	}
	if c.Model == "" {
		c.Model = "gpt-4o"
	}
	if c.Temperature == 0 {
		c.Temperature = 0.7
	}
	return c
}

// validate checks that required config fields are set.
func (c Config) validate() error {
	if c.LLMClient == nil {
		return NewConfigurationError("LLMClient is required", nil)
	}
	if c.Skills == nil {
		return NewConfigurationError("Skills registry is required", nil)
	}
	if c.Tools == nil {
		return NewConfigurationError("Tools registry is required", nil)
	}
	return nil
}

// LLMClient is the interface for LLM providers.
// This abstracts the OpenAI client to allow for testing and alternative providers.
type LLMClient interface {
	// ChatCompletion sends a chat completion request.
	ChatCompletion(ctx ChatCompletionContext) (ChatCompletionResult, error)

	// ChatCompletionStream sends a streaming chat completion request.
	ChatCompletionStream(ctx ChatCompletionContext) (ChatCompletionStream, error)
}

// ChatCompletionContext contains the context for a chat completion request.
type ChatCompletionContext struct {
	// Model is the model to use.
	Model string

	// Messages are the conversation messages.
	Messages []LLMMessage

	// Tools are the available tools for function calling.
	Tools []LLMTool

	// ResponseFormat specifies the expected response format.
	ResponseFormat *LLMResponseFormat

	// Temperature controls randomness.
	Temperature float32

	// MaxTokens limits the response length.
	MaxTokens int
}

// LLMMessage is a message in the conversation.
type LLMMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	Name       string        `json:"name,omitempty"`
	ToolCalls  []LLMToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

// LLMTool represents a tool available to the LLM.
type LLMTool struct {
	Type     string      `json:"type"`
	Function LLMFunction `json:"function"`
}

// LLMFunction describes a callable function.
type LLMFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// LLMToolCall represents an LLM's request to call a tool.
type LLMToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function LLMFunctionCall `json:"function"`
}

// LLMFunctionCall is the function details in a tool call.
type LLMFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// LLMResponseFormat specifies structured output format.
type LLMResponseFormat struct {
	Type       string         `json:"type"`
	JSONSchema *LLMJSONSchema `json:"json_schema,omitempty"`
}

// LLMJSONSchema defines a JSON schema for structured output.
type LLMJSONSchema struct {
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

// ChatCompletionResult is the result of a chat completion.
type ChatCompletionResult struct {
	// Message is the assistant's response.
	Message LLMMessage

	// FinishReason indicates why the response ended.
	FinishReason string

	// Usage contains token usage information.
	Usage TokenUsage
}

// ChatCompletionStream is a stream of chat completion chunks.
type ChatCompletionStream interface {
	// Recv receives the next chunk from the stream.
	Recv() (ChatCompletionChunk, error)

	// Close closes the stream.
	Close() error
}

// ChatCompletionChunk is a chunk in a streaming response.
type ChatCompletionChunk struct {
	// Delta contains the incremental content.
	Delta LLMMessageDelta

	// FinishReason indicates if/why the response ended.
	FinishReason string
}

// LLMMessageDelta is the incremental content in a stream chunk.
type LLMMessageDelta struct {
	Role      string        `json:"role,omitempty"`
	Content   string        `json:"content,omitempty"`
	ToolCalls []LLMToolCall `json:"tool_calls,omitempty"`
}

// SkillRegistry provides access to loaded skills.
type SkillRegistry interface {
	// Get returns a skill by ID.
	Get(id string) (*Skill, bool)

	// All returns all registered skills.
	All() []*Skill

	// Match finds skills matching the given message.
	Match(message string) []*Skill
}

// ToolRegistry provides access to registered tools.
type ToolRegistry interface {
	// GetSource returns a source tool by name.
	GetSource(name string) (*Source, bool)

	// GetAction returns an action tool by name.
	GetAction(name string) (*Action, bool)

	// AllSources returns all source tools.
	AllSources() []*Source

	// AllActions returns all action tools.
	AllActions() []*Action

	// GetForSkill returns tools available to a skill.
	GetForSkill(toolNames []string) (sources []*Source, actions []*Action, err error)
}

// ConversationStore persists conversations.
type ConversationStore interface {
	// GetConversation retrieves a conversation by ID.
	GetConversation(ctx ChatCompletionContext, id string) (*Conversation, error)

	// SaveConversation saves a conversation.
	SaveConversation(ctx ChatCompletionContext, conv *Conversation) error

	// AddMessage adds a message to a conversation.
	AddMessage(ctx ChatCompletionContext, conversationID string, msg Message) error

	// GetMessages retrieves messages for a conversation.
	GetMessages(ctx ChatCompletionContext, conversationID string, limit int) ([]Message, error)

	// SaveFeedback saves feedback for a message.
	SaveFeedback(ctx ChatCompletionContext, feedback Feedback) error
}

// Skill represents a skill definition loaded from YAML.
type Skill struct {
	// ID uniquely identifies the skill.
	ID string

	// Name is the human-readable skill name.
	Name string

	// Triggers are keywords that activate this skill.
	Triggers []string

	// Intents are semantic intents that activate this skill.
	Intents []string

	// Tools are the tool names available to this skill.
	Tools []string

	// Instructions are the system instructions for the LLM.
	Instructions string

	// Examples are few-shot examples for the LLM.
	Examples []SkillExample

	// Guardrails are rules the LLM should follow.
	Guardrails []string

	// Output defines the response schema.
	Output *OutputSchema

	// Variants are A/B testing variants.
	Variants []SkillVariant

	// Mode overrides the default execution mode for this skill.
	Mode ExecutionMode

	// ContextInPrompt lists context keys to include in the prompt.
	ContextInPrompt []string
}

// SkillExample is a few-shot example.
type SkillExample struct {
	User      string `yaml:"user"`
	Assistant string `yaml:"assistant"`
}

// SkillVariant is an A/B testing variant.
type SkillVariant struct {
	// Variant is the variant identifier.
	Variant string `yaml:"variant"`

	// Weight is the probability weight (0-100).
	Weight int `yaml:"weight"`

	// Instructions override the base instructions.
	Instructions string `yaml:"instructions"`
}

// OutputSchema defines the expected response structure.
type OutputSchema struct {
	Type       string                    `yaml:"type"`
	Properties map[string]PropertySchema `yaml:"properties"`
	Required   []string                  `yaml:"required"`
}

// PropertySchema defines a property in the output schema.
type PropertySchema struct {
	Type        string                    `yaml:"type"`
	Description string                    `yaml:"description,omitempty"`
	Nullable    bool                      `yaml:"nullable,omitempty"`
	Items       *PropertySchema           `yaml:"items,omitempty"`
	Properties  map[string]PropertySchema `yaml:"properties,omitempty"`
	Enum        []string                  `yaml:"enum,omitempty"`
}

// Source represents a read-only data source tool.
type Source struct {
	// Name identifies the tool.
	Name string

	// Description explains what the tool does.
	Description string

	// Params defines the tool's parameters.
	Params ParamDefinitions

	// Fetch is the function that retrieves data.
	Fetch FetchFn
}

// Action represents a tool that performs side effects.
type Action struct {
	// Name identifies the tool.
	Name string

	// Description explains what the tool does.
	Description string

	// Params defines the tool's parameters.
	Params ParamDefinitions

	// Execute is the function that performs the action.
	Execute ExecuteFn

	// RequiresConfirmation indicates if user must confirm before execution.
	RequiresConfirmation bool
}

// ParamDefinitions maps parameter names to their definitions.
type ParamDefinitions map[string]ParamDef

// ParamDef defines a tool parameter.
type ParamDef struct {
	// Type is the parameter type (string, int, bool, object, array, enum).
	Type string

	// Description explains the parameter.
	Description string

	// Required indicates if the parameter must be provided.
	Required bool

	// EnumValues are valid values for enum types.
	EnumValues []string

	// Default is the default value if not provided.
	Default any
}

// FetchFn is the signature for source tool functions.
type FetchFn func(ctx context.Context, params Input) (any, error)

// ExecuteFn is the signature for action tool functions.
type ExecuteFn func(ctx context.Context, params Input) (any, error)

// Input provides access to tool parameters.
type Input interface {
	// String returns a string parameter value.
	String(name string) string

	// StringOr returns a string parameter value with a default.
	StringOr(name, defaultValue string) string

	// Int returns an int parameter value.
	Int(name string) int

	// IntOr returns an int parameter value with a default.
	IntOr(name string, defaultValue int) int

	// Bool returns a bool parameter value.
	Bool(name string) bool

	// BoolOr returns a bool parameter value with a default.
	BoolOr(name string, defaultValue bool) bool

	// Object returns an object parameter value.
	Object(name string) map[string]any

	// Array returns an array parameter value.
	Array(name string) []any

	// Raw returns the raw parameter value.
	Raw(name string) any

	// Has checks if a parameter was provided.
	Has(name string) bool
}

// Expert defines an expert implementation that controls data fetching.
type Expert struct {
	// SkillID links this expert to a skill definition.
	SkillID string

	// Fetcher retrieves data for the skill.
	// If nil, the skill receives only the message (no data fetching).
	Fetcher ExpertFetcher

	// PostProcess transforms the skill result before returning.
	// If nil, the skill result is returned as-is.
	PostProcess ExpertPostProcessor
}

// ExpertFetcher is the function signature for expert data fetching.
type ExpertFetcher func(ctx context.Context, req Request, tools ToolExecutor) (any, error)

// ExpertPostProcessor is the function signature for post-processing.
type ExpertPostProcessor func(ctx context.Context, req Request, skillResult *SkillResult, fetchedData any) (*ExpertResult, error)

// Request contains the processed chat request passed to experts.
type Request struct {
	// Message is the user's question.
	Message string

	// EntityID is the entity identifier.
	EntityID string

	// Context is additional context.
	Context RequestContext

	// ConversationID is the conversation identifier.
	ConversationID string

	// ConversationHistory contains previous messages.
	ConversationHistory []Message
}

// ExpertResult is the result returned by an expert's post-processor.
type ExpertResult struct {
	// Answer is the response text (extracted from LLM response).
	Answer string

	// Details is additional structured data to return.
	Details any

	// SuggestedAction is an action the expert suggests.
	SuggestedAction *SuggestedAction
}
