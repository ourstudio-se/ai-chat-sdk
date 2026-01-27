package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	aichat "github.com/ourstudio-se/ai-chat-sdk"
)

// Simulated product database
var productDB = map[string]Product{
	"product-123": {
		Name:     "Widget Pro X1000",
		Category: "Electronics",
		Price:    299.99,
		Features: []string{"Wireless", "Waterproof", "Long battery life"},
	},
	"product-456": {
		Name:     "Gadget Mini",
		Category: "Accessories",
		Price:    49.99,
		Features: []string{"Compact", "Portable", "USB-C charging"},
	},
}

type Product struct {
	Name     string
	Category string
	Price    float64
	Features []string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	sdk, err := aichat.New(aichat.Config{
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		Logger:       logger,

		// Define experts with their metadata and handlers in one place
		Experts: map[aichat.ExpertType]aichat.Expert{
			"product": {
				Name:        "Product Expert",
				Description: "Questions about product features, specifications, and capabilities",
				Handler:     handleProductQuestion,
			},
			"support": {
				Name:        "Support Expert",
				Description: "Questions about troubleshooting, issues, and general help",
				Handler:     handleSupportQuestion,
			},
		},

		DefaultExpert:    "support",
		DefaultReasoning: "Routing to support for general assistance",

		Glossary: map[string]aichat.GlossaryTerms{
			"battery life": {
				English: "battery life",
				Swedish: "batteritid",
				German:  "Akkulaufzeit",
			},
		},

		AllowedOrigins: []string{"*"},
	})
	if err != nil {
		logger.Error("failed to create SDK", "error", err)
		os.Exit(1)
	}

	addr := ":3001"
	logger.Info("starting server", "address", addr)
	if err := http.ListenAndServe(addr, sdk.HTTPHandler()); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func handleProductQuestion(ctx context.Context, req aichat.ExpertRequest) (*aichat.ExpertResult, error) {
	// Expert resolves its own entity data using the EntityID
	product, found := productDB[req.EntityID]
	if !found {
		return &aichat.ExpertResult{
			Answer: "I couldn't find information about that product. Could you provide a valid product ID?",
		}, nil
	}

	answer := fmt.Sprintf(
		"The %s is a great choice! It's in the %s category, priced at $%.2f. "+
			"Key features include: %v. Is there anything specific you'd like to know?",
		product.Name, product.Category, product.Price, product.Features,
	)

	return &aichat.ExpertResult{
		Answer: answer,
		Details: map[string]any{
			"productId": req.EntityID,
			"product":   product,
			"source":    "product_database",
		},
	}, nil
}

func handleSupportQuestion(ctx context.Context, req aichat.ExpertRequest) (*aichat.ExpertResult, error) {
	// Support expert might also look up entity data if needed
	if req.EntityID != "" {
		if product, found := productDB[req.EntityID]; found {
			return &aichat.ExpertResult{
				Answer: fmt.Sprintf(
					"I'd be happy to help you with your %s! Could you please describe the issue you're experiencing?",
					product.Name,
				),
			}, nil
		}
	}

	return &aichat.ExpertResult{
		Answer: "I'd be happy to help you with that! Could you please provide more details about the issue you're experiencing?",
	}, nil
}
