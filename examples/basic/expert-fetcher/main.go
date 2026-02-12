package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	aichat "github.com/ourstudio-se/ai-chat-sdk"
	"github.com/ourstudio-se/ai-chat-sdk/examples/basic/shared"
	"github.com/ourstudio-se/ai-chat-sdk/llm/openai"
	"github.com/ourstudio-se/ai-chat-sdk/skills"
	"github.com/ourstudio-se/ai-chat-sdk/tools"
	oai "github.com/sashabaranov/go-openai"
)

// SlimProduct is a token-optimized product representation.
// Only contains essential fields for the LLM to reason about.
type SlimProduct struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	InStock  bool    `json:"in_stock"`
	Quantity int     `json:"quantity,omitempty"`
}

// FullProductDetails contains complete product information for the response.
type FullProductDetails struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Features    []string `json:"features"`
	InStock     bool     `json:"in_stock"`
	Quantity    int      `json:"quantity"`
}

// SkillResponse matches the product skill's output schema.
type SkillResponse struct {
	Answer       string   `json:"answer"`
	ProductCodes []string `json:"product_codes,omitempty"`
	InStock      *bool    `json:"in_stock,omitempty"`
}

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

	// 2. Create and register tools (shared with other modes)
	toolRegistry := tools.NewRegistry()
	shared.RegisterTools(toolRegistry)

	// 3. Load skills from YAML files (shared with other modes)
	skillRegistry, err := skills.LoadDir("../skills")
	if err != nil {
		logger.Error("failed to load skills", "error", err)
		os.Exit(1)
	}

	// 4. Create hooks for token optimization
	hooks := aichat.NewHookRegistry()

	// Preprocess hook: Filters and optimizes data before LLM call
	hooks.RegisterPreprocess("product", func(ctx context.Context, req *aichat.PreprocessRequest) error {
		logger.Debug("preprocess hook called",
			"skillId", req.SkillID,
			"message", req.Message,
		)

		// Store original data in metadata for postprocess
		req.Metadata["original_data"] = req.Data

		// Example: Filter to only stock-related data if question is about stock
		if containsAny(strings.ToLower(req.Message), "stock", "available", "inventory", "quantity") {
			req.Metadata["focus"] = "inventory"
			logger.Debug("preprocess: focusing on inventory data")
		}

		return nil
	})

	// Postprocess hook: Enriches response with full data
	hooks.RegisterPostprocess("product", func(ctx context.Context, req *aichat.PostprocessRequest) error {
		logger.Debug("postprocess hook called",
			"skillId", req.SkillID,
			"variant", req.Variant,
			"tokens", req.TokensUsed.TotalTokens,
		)

		// Add metadata about processing
		req.Response["processed_by_hook"] = true
		req.Response["variant_used"] = req.Variant
		req.Response["tokens_used"] = req.TokensUsed.TotalTokens

		return nil
	})

	// 5. Create expert with custom fetcher
	productExpert := &aichat.Expert{
		SkillID: "product",

		// Fetcher: YOU control what data to fetch
		// This demonstrates:
		// - Conditional data fetching based on question
		// - Token optimization by converting to slim types
		// - Combining data from multiple sources
		Fetcher: func(ctx context.Context, req aichat.Request, toolExec aichat.ToolExecutor) (any, error) {
			logger.Debug("expert fetcher called",
				"message", req.Message,
				"entityId", req.EntityID,
			)

			// Determine which products to fetch based on message
			var productCodes []string

			// Check if specific product is mentioned
			message := strings.ToLower(req.Message)
			if strings.Contains(message, "widget") {
				productCodes = append(productCodes, "WIDGET-PRO")
			}
			if strings.Contains(message, "gadget") {
				productCodes = append(productCodes, "GADGET-MINI")
			}

			// If no specific product mentioned, fetch all
			if len(productCodes) == 0 {
				productCodes = []string{"WIDGET-PRO", "GADGET-MINI"}
			}

			// Fetch products and convert to slim format
			var slimProducts []SlimProduct
			for _, code := range productCodes {
				// Fetch product details
				productResult, err := toolExec.Execute(ctx, "get_product", map[string]any{
					"code": code,
				})
				if err != nil {
					logger.Warn("failed to fetch product", "code", code, "error", err)
					continue
				}

				// Fetch inventory
				inventoryResult, err := toolExec.Execute(ctx, "get_inventory", map[string]any{
					"product_code": code,
				})
				if err != nil {
					logger.Warn("failed to fetch inventory", "code", code, "error", err)
					continue
				}

				// Convert to JSON and back to access fields
				productJSON, _ := json.Marshal(productResult)
				var product shared.Product
				json.Unmarshal(productJSON, &product)

				inventoryJSON, _ := json.Marshal(inventoryResult)
				var inventory shared.Inventory
				json.Unmarshal(inventoryJSON, &inventory)

				// Create slim version for token optimization
				slimProducts = append(slimProducts, SlimProduct{
					Code:     product.Code,
					Name:     product.Name,
					Price:    product.Price,
					InStock:  inventory.InStock,
					Quantity: inventory.Quantity,
				})
			}

			logger.Debug("fetched products",
				"count", len(slimProducts),
				"codes", productCodes,
			)

			return slimProducts, nil
		},

		// PostProcess: Enrich the LLM response with full details
		PostProcess: func(ctx context.Context, req aichat.Request, skillResult *aichat.SkillResult, fetchedData any) (*aichat.ExpertResult, error) {
			logger.Debug("expert post-process called",
				"variant", skillResult.Variant,
				"tokens", skillResult.TokensUsed.TotalTokens,
			)

			// Parse the LLM response
			var response SkillResponse
			if err := json.Unmarshal(skillResult.Response, &response); err != nil {
				logger.Warn("failed to parse skill response", "error", err)
				return &aichat.ExpertResult{
					Answer: string(skillResult.Response),
				}, nil
			}

			// If product codes were returned, enrich with full details
			var fullDetails []FullProductDetails
			if len(response.ProductCodes) > 0 {
				slimProducts, _ := fetchedData.([]SlimProduct)
				for _, code := range response.ProductCodes {
					for _, slim := range slimProducts {
						if slim.Code == code {
							// Get full product from DB (in real app, you'd cache this)
							fullDetails = append(fullDetails, FullProductDetails{
								Code:        slim.Code,
								Name:        slim.Name,
								Description: getProductDescription(slim.Code),
								Price:       slim.Price,
								Features:    getProductFeatures(slim.Code),
								InStock:     slim.InStock,
								Quantity:    slim.Quantity,
							})
						}
					}
				}
			}

			return &aichat.ExpertResult{
				Answer:  response.Answer,
				Details: fullDetails,
			}, nil
		},
	}

	// 6. Create SDK instance with expert and hooks
	sdk, err := aichat.New(aichat.Config{
		LLMClient:      llmClient,
		Skills:         skillRegistry,
		Tools:          toolRegistry,
		ExecutionMode:  aichat.ModeExpert,
		DefaultSkillID: "product",
		Logger:         logger,
		MaxAgentTurns:  10,
		AllowedOrigins: []string{"*"},
		Hooks:          hooks,
		Experts:        []*aichat.Expert{productExpert},
	})
	if err != nil {
		logger.Error("failed to create SDK", "error", err)
		os.Exit(1)
	}

	// 7. Start HTTP server
	addr := ":3003"
	logger.Info("starting EXPERT with FETCHER mode server", "address", addr)
	logger.Info("this example demonstrates:")
	logger.Info("  - Expert.Fetcher for custom data fetching")
	logger.Info("  - Expert.PostProcess for response enrichment")
	logger.Info("  - Hooks for pre/post processing")
	logger.Info("try: curl -X POST http://localhost:3003/chat -H 'Content-Type: application/json' -d '{\"message\": \"Is the Widget Pro in stock?\"}'")

	if err := http.ListenAndServe(addr, sdk.HTTPHandler()); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// Helper functions

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func getProductDescription(code string) string {
	descriptions := map[string]string{
		"WIDGET-PRO":  "Our flagship widget with advanced features",
		"GADGET-MINI": "Compact and portable gadget",
	}
	return descriptions[code]
}

func getProductFeatures(code string) []string {
	features := map[string][]string{
		"WIDGET-PRO":  {"Wireless", "Waterproof", "Long battery"},
		"GADGET-MINI": {"Compact", "USB-C", "Lightweight"},
	}
	return features[code]
}
