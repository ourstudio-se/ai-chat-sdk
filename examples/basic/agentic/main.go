package main

import (
	"log/slog"
	"net/http"
	"os"

	aichat "github.com/ourstudio-se/ai-chat-sdk"
	"github.com/ourstudio-se/ai-chat-sdk/examples/basic/shared"
	"github.com/ourstudio-se/ai-chat-sdk/llm/openai"
	"github.com/ourstudio-se/ai-chat-sdk/skills"
	"github.com/ourstudio-se/ai-chat-sdk/tools"

	oai "github.com/sashabaranov/go-openai"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// 1. Create OpenAI client
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		logger.Error("OPENAI_API_KEY environment variable is required")
		os.Exit(1)
	}
	openaiClient := oai.NewClient(apiKey)
	llmClient := openai.New(openaiClient)

	// 2. Create and register tools (shared with expert mode)
	toolRegistry := tools.NewRegistry()
	shared.RegisterTools(toolRegistry)

	// 3. Load skills from YAML files (shared with expert mode)
	skillRegistry, err := skills.LoadDir("../skills")
	if err != nil {
		logger.Error("failed to load skills", "error", err)
		os.Exit(1)
	}

	// 4. Create SDK instance in AGENTIC mode
	// In agentic mode:
	// - LLM decides which tools to call via function calling
	// - Multiple LLM calls possible as the agent reasons
	// - More flexible, handles complex questions
	// - Actions can be executed (with confirmation if required)
	// - Variable cost and latency
	sdk, err := aichat.New(aichat.Config{
		LLMClient:      llmClient,
		Skills:         skillRegistry,
		Tools:          toolRegistry,
		ExecutionMode:  aichat.ModeAgentic, // <-- Agentic mode
		DefaultSkillID: "product",          // Fallback if no triggers match
		Logger:         logger,
		MaxAgentTurns:  10, // Limit agent reasoning turns
		AllowedOrigins: []string{"*"},
	})
	if err != nil {
		logger.Error("failed to create SDK", "error", err)
		os.Exit(1)
	}

	// 5. Start HTTP server
	addr := ":3002"
	logger.Info("starting AGENTIC mode server", "address", addr, "mode", "agentic")
	logger.Info("try: curl -X POST http://localhost:3002/chat -H 'Content-Type: application/json' -d '{\"message\": \"Is the Widget Pro in stock?\"}'")

	if err := http.ListenAndServe(addr, sdk.HTTPHandler()); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
