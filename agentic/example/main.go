package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"time"

	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/conversation"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/feedback"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/llm/anthropic"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/server"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/skills"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/tools"
)

func main() {
	// 1. LLM Provider
	llmProvider := anthropic.NewFromEnv()

	// 2. Conversation store (in-memory for this example)
	convStore := conversation.NewMemoryStore()

	// 3. Tools - now in separate file (tools.go)
	toolRegistry := tools.NewRegistry()
	registerTools(toolRegistry) // Defined in tools.go

	// 4. Skills
	skillRegistry, err := skills.LoadFromDir("./skills")
	if err != nil {
		log.Fatalf("Failed to load skills: %v", err)
	}

	// Set fallback
	if fallback := skillRegistry.GetVariant("assistant", "default"); fallback != nil {
		skillRegistry.SetFallback(fallback)
	}

	// 5. A/B selector
	abSelector := skills.NewABSelector().Weighted()

	// 6. Create agent
	ag := agent.New(
		llmProvider,
		convStore,
		toolRegistry,
		skillRegistry,
		agent.WithConfig(agent.DefaultConfig().
			WithModel("claude-sonnet-4-5").
			WithBasePrompt("You are a helpful assistant. Use the available tools when you need real data like weather, time, or external information.")),
		agent.WithABSelector(abSelector),
	)

	// 7. Optional: feedback store
	feedbackStore := feedback.NewMemoryStore()

	// 8. Run test or start server
	if os.Getenv("RUN_TEST") == "1" {
		runTest(ag)
		return
	}

	// HTTP Server
	srv := server.New(ag, server.Config{}, server.WithFeedback(feedbackStore))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting server on :%s\n", port)
	fmt.Println("\nEndpoints:")
	fmt.Println("  POST /api/v1/chat")
	fmt.Println("  GET  /api/v1/conversations/{id}")
	fmt.Println("  POST /api/v1/feedback")
	fmt.Println("  GET  /health")
	fmt.Println("\nExample curl:")
	fmt.Println(`  curl -X POST http://localhost:` + port + `/api/v1/chat \`)
	fmt.Println(`    -H "Content-Type: application/json" \`)
	fmt.Println(`    -d '{"message": "What is the weather in Paris?"}'`)

	if err := srv.ListenAndServe(":" + port); err != nil {
		log.Fatal(err)
	}
}

func runTest(ag *agent.Agent) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("=== Running Test Conversation ===")

	// Test 1: Weather (real API call)
	fmt.Println("Test 1: Weather query")
	resp, err := ag.Chat(ctx, agent.ChatRequest{
		Message: "What's the weather like in Tokyo right now?",
	})
	if err != nil {
		log.Fatalf("Chat error: %v", err)
	}
	printResponse(resp)

	// Test 2: Follow-up in same session
	fmt.Println("\nTest 2: Follow-up question (same session)")
	resp2, err := ag.Chat(ctx, agent.ChatRequest{
		SessionID: resp.SessionID,
		Message:   "What about the time there?",
	})
	if err != nil {
		log.Fatalf("Chat error: %v", err)
	}
	printResponse(resp2)

	// Test 3: New conversation with different topic
	fmt.Println("\nTest 3: Joke request")
	resp3, err := ag.Chat(ctx, agent.ChatRequest{
		Message: "Tell me a joke!",
	})
	if err != nil {
		log.Fatalf("Chat error: %v", err)
	}
	printResponse(resp3)

	fmt.Println("\n=== Tests Complete ===")
}

func printResponse(resp *agent.ChatResponse) {
	fmt.Printf("Session:    %s\n", resp.SessionID)
	fmt.Printf("Message ID: %s\n", resp.MessageID)
	fmt.Printf("Skills:     %v\n", resp.SkillsUsed)
	if len(resp.ToolCalls) > 0 {
		fmt.Printf("Tools used: ")
		for _, tc := range resp.ToolCalls {
			fmt.Printf("%s ", tc.Name)
		}
		fmt.Println()
	}
	fmt.Printf("Response:   %s\n", resp.Response)
}
