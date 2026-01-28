package aichat

import (
	"log/slog"
	"time"
)

// Config holds SDK configuration.
type Config struct {
	// OpenAIAPIKey is the API key for OpenAI.
	OpenAIAPIKey string

	// Logger is the structured logger to use. If nil, a default logger is used.
	Logger *slog.Logger

	// Experts defines the available experts with their metadata and handlers.
	// Each expert is responsible for resolving any entity data it needs using req.EntityID.
	Experts map[ExpertType]Expert

	// DefaultExpert is the fallback expert type when routing fails.
	DefaultExpert ExpertType

	// DefaultReasoning is the default reasoning when falling back to default expert.
	DefaultReasoning string

	// RouterSystemPromptTemplate is the template for the router's system prompt (optional).
	// Use {{EXPERTS}} placeholder for expert definitions and {{CONTEXT}} for entity context.
	RouterSystemPromptTemplate string

	// Storage is the conversation store (optional, defaults to in-memory).
	Storage ConversationStore

	// FormatterSystemPrompt is a custom system prompt for the formatter (optional).
	FormatterSystemPrompt string

	// TranslatorSystemPrompt is a custom system prompt for the translator (optional).
	TranslatorSystemPrompt string

	// Glossary contains domain-specific term translations (optional).
	Glossary map[string]GlossaryTerms

	// AllowedOrigins for CORS. Must be explicitly configured unless DevMode is enabled.
	AllowedOrigins []string

	// DevMode enables permissive settings for development (e.g., allows all CORS origins).
	// IMPORTANT: Do not enable in production.
	DevMode bool

	// RequestTimeout is the maximum duration for a request (defaults to 30s).
	RequestTimeout time.Duration

	// MaxRequestBodySize is the maximum size of a request body in bytes (defaults to 1MB).
	MaxRequestBodySize int64

	// MaxMessageLength is the maximum length of a message in characters (defaults to 10000).
	MaxMessageLength int
}

// GlossaryTerms contains translations for a term in different languages.
type GlossaryTerms struct {
	English   string
	Swedish   string
	German    string
	Norwegian string
	Danish    string
	French    string
}

// DefaultRouterSystemPromptTemplate is the default template for the router.
const DefaultRouterSystemPromptTemplate = `You are a router that classifies questions.

{{CONTEXT}}

Your job is to determine which expert should respond:
{{EXPERTS}}

Respond ONLY with JSON in this format:
{"expert": "<expert_type>", "reasoning": "brief explanation"}

IMPORTANT: The "reasoning" field should be in the same language as the user's question.`

// applyDefaults fills in default values for the config.
func (c *Config) applyDefaults() {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	if c.RouterSystemPromptTemplate == "" {
		c.RouterSystemPromptTemplate = DefaultRouterSystemPromptTemplate
	}

	if len(c.AllowedOrigins) == 0 && c.DevMode {
		c.AllowedOrigins = []string{"*"}
	}

	if c.RequestTimeout == 0 {
		c.RequestTimeout = 30 * time.Second
	}

	if c.MaxRequestBodySize == 0 {
		c.MaxRequestBodySize = 1 << 20 // 1MB
	}

	if c.MaxMessageLength == 0 {
		c.MaxMessageLength = 10000
	}
}
