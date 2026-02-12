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
	openAIClient := oai.NewClient(apiKey)
	llmClient := openai.New(openAIClient)

	// 2. Create and register tools (shared with agentic mode)
	toolRegistry := tools.NewRegistry()
	shared.RegisterTools(toolRegistry)

	// 3. Load skills from YAML files (shared with agentic mode)
	skillRegistry, err := skills.LoadDir("../skills")
	if err != nil {
		logger.Error("failed to load skills", "error", err)
		os.Exit(1)
	}

	// 4. Create SDK instance in EXPERT mode
	// In expert mode:
	// - Data is fetched deterministically (all sources for the skill)
	// - Single LLM call with pre-fetched data
	// - Predictable cost and latency
	// - Actions are suggested but not executed
	sdk, err := aichat.New(aichat.Config{
		LLMClient:      llmClient,
		Skills:         skillRegistry,
		Tools:          toolRegistry,
		ExecutionMode:  aichat.ModeExpert, // <-- Expert mode
		DefaultSkillID: "product",         // Fallback if no triggers match
		Logger:         logger,
		AllowedOrigins: []string{"*"},
	})
	if err != nil {
		logger.Error("failed to create SDK", "error", err)
		os.Exit(1)
	}

	// 5. Start HTTP server
	addr := ":3001"
	logger.Info("starting EXPERT mode server", "address", addr, "mode", "expert")
	logger.Info("try: curl -X POST http://localhost:3001/chat -H 'Content-Type: application/json' -d '{\"message\": \"Tell me about the Widget Pro\"}'")

	if err := http.ListenAndServe(addr, sdk.HTTPHandler()); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
