package aichat

import (
	"errors"
	"log/slog"
	"net/http"
)

// SDK is the main AI Chat SDK instance.
type SDK struct {
	config      *Config
	logger      *slog.Logger
	processChat ProcessChatFn
	httpHandler http.Handler
}

// New creates a new AI Chat SDK instance.
func New(config Config) (*SDK, error) {
	config.applyDefaults()

	if config.OpenAIClient == nil {
		return nil, errors.New("OpenAIClient is required")
	}

	if len(config.Experts) == 0 {
		return nil, errors.New("at least one expert must be configured")
	}

	if len(config.AllowedOrigins) == 0 {
		return nil, errors.New("AllowedOrigins must be configured (or enable DevMode)")
	}

	logger := config.Logger

	// Wrap OpenAI client with internal API
	openaiClient := newInternalOpenAIClient(config.OpenAIClient, logger)

	// Create translator
	translateFn := newTranslator(openaiClient.ChatJSON, logger, config.TranslatorSystemPrompt)

	// Create router
	routeQuestionFn := newRouter(
		openaiClient.ChatJSON,
		config.Experts,
		config.RouterSystemPromptTemplate,
		config.DefaultExpert,
		config.DefaultReasoning,
		logger,
	)

	// Create formatter
	formatResponseFn := newFormatter(openaiClient.Chat, logger, config.FormatterSystemPrompt, config.Glossary)

	// Create dispatcher
	dispatchQuestionFn := NewDispatcher(
		routeQuestionFn,
		config.Experts,
		config.DefaultExpert,
		logger,
	)

	// Create storage (default to in-memory)
	store := config.Storage
	if store.Create == nil {
		store = NewMemoryStore(logger)
	}

	// Create chat service
	processChatFn := NewChatService(
		translateFn,
		formatResponseFn,
		dispatchQuestionFn,
		store,
		logger,
	)

	// Create HTTP handlers
	healthHandler := newHealthHandler()
	chatHandler := newChatHandler(processChatFn, config.MaxMessageLength, logger)
	chatStreamHandler := newChatStreamHandler(processChatFn, config.MaxMessageLength, logger)

	// Create HTTP router
	httpHandler := newHTTPRouter(
		config.AllowedOrigins,
		config.RequestTimeout,
		config.MaxRequestBodySize,
		logger,
		healthHandler,
		chatHandler,
		chatStreamHandler,
	)

	return &SDK{
		config:      &config,
		logger:      logger,
		processChat: processChatFn,
		httpHandler: httpHandler,
	}, nil
}

// ProcessChat returns the chat processing function for direct use (without HTTP).
func (s *SDK) ProcessChat() ProcessChatFn {
	return s.processChat
}

// HTTPHandler returns the HTTP handler for the SDK.
func (s *SDK) HTTPHandler() http.Handler {
	return s.httpHandler
}
