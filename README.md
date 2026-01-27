# AI Chat SDK

A flexible, generic AI chat SDK for building conversational interfaces with expert routing, multilingual support, and pluggable storage.

## Installation

```bash
go get github.com/ourstudio-se/ai-chat-sdk
```

## Quick Start

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"

    aichat "github.com/ourstudio-se/ai-chat-sdk"
)

func main() {
    sdk, _ := aichat.New(aichat.Config{
        OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
        Experts: map[aichat.ExpertType]aichat.Expert{
            "general": {
                Name:        "General Expert",
                Description: "General questions",
                Handler: func(ctx context.Context, req aichat.ExpertRequest) (*aichat.ExpertResult, error) {
                    return &aichat.ExpertResult{Answer: "Hello! How can I help?"}, nil
                },
            },
        },
        DefaultExpert: "general",
    })

    http.ListenAndServe(":3001", sdk.HTTPHandler())
}
```

---

## Wiring Up the SDK

### Step 1: Define Your Experts

Experts combine metadata (for routing) with handlers (for processing) in a single place:

```go
experts := map[aichat.ExpertType]aichat.Expert{
    "product": {
        Name:        "Product Expert",
        Description: "Questions about product features, specifications, pricing, and availability",
        Handler:     handleProductQuestion,
    },
    "support": {
        Name:        "Support Expert",
        Description: "Questions about troubleshooting, returns, warranties, and customer service",
        Handler:     handleSupportQuestion,
    },
    "sales": {
        Name:        "Sales Expert",
        Description: "Questions about purchasing, discounts, bulk orders, and partnerships",
        Handler:     handleSalesQuestion,
    },
}
```

The `Description` field is critical - it's used by the LLM to decide which expert should handle each question.

### Step 2: Implement Expert Handlers

Each expert handler processes questions and returns answers. **Experts are responsible for resolving their own entity data** using the provided `EntityID`:

```go
// Your data store (database, API, etc.)
var productDB = map[string]Product{
    "product-123": {Name: "Widget Pro", Price: 299.99, Features: []string{"Wireless", "Waterproof"}},
    "product-456": {Name: "Gadget Mini", Price: 49.99, Features: []string{"Compact", "Portable"}},
}

func handleProductQuestion(ctx context.Context, req aichat.ExpertRequest) (*aichat.ExpertResult, error) {
    // req.Message contains the user's question (translated to English)
    // req.EntityID contains the entity identifier (e.g., product ID)
    // req.RoutingReasoning contains why this expert was chosen

    // Expert resolves its own entity data
    product, found := productDB[req.EntityID]
    if !found {
        return &aichat.ExpertResult{
            Answer: "I couldn't find that product. Could you provide a valid product ID?",
        }, nil
    }

    // Generate response using the resolved data
    answer := fmt.Sprintf(
        "The %s is priced at $%.2f. Key features: %v. What else would you like to know?",
        product.Name, product.Price, product.Features,
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
    // Support expert can also look up entity data if needed
    if req.EntityID != "" {
        if product, found := productDB[req.EntityID]; found {
            return &aichat.ExpertResult{
                Answer: fmt.Sprintf("I'd be happy to help with your %s! What issue are you experiencing?", product.Name),
            }, nil
        }
    }

    return &aichat.ExpertResult{
        Answer: "I'd be happy to help! Please describe your issue.",
    }, nil
}
```

### Step 3: Configure Storage (Optional)

By default, the SDK uses in-memory storage (conversations lost on restart). For production, use file-based or custom storage:

**File-based storage:**
```go
store, err := aichat.NewFileStore("./data/conversations", logger)
if err != nil {
    log.Fatal(err)
}
```

**Custom storage (e.g., database):**
```go
store := aichat.ConversationStore{
    Create: func(ctx context.Context, entityID string) (*aichat.Conversation, error) {
        conv := &aichat.Conversation{
            ID:        uuid.New().String(),
            CreatedAt: time.Now(),
            EntityID:  entityID,
            Messages:  []aichat.Message{},
        }
        err := db.InsertConversation(ctx, conv)
        return conv, err
    },
    Get: func(ctx context.Context, id string) (*aichat.Conversation, error) {
        return db.GetConversation(ctx, id)
    },
    AddMessage: func(ctx context.Context, id string, msg aichat.Message) error {
        return db.AddMessage(ctx, id, msg)
    },
    Save: func(ctx context.Context, conversation *aichat.Conversation) error {
        return db.UpdateConversation(ctx, conversation)
    },
}
```

### Step 4: Add Domain Glossary (Optional)

For accurate translations in your domain, provide a glossary:

```go
glossary := map[string]aichat.GlossaryTerms{
    "shopping cart": {
        English: "shopping cart",
        Swedish: "kundvagn",
        German:  "Warenkorb",
        French:  "panier",
    },
    "checkout": {
        English: "checkout",
        Swedish: "kassa",
        German:  "Kasse",
        French:  "caisse",
    },
}
```

### Step 5: Create the SDK

Put it all together:

```go
sdk, err := aichat.New(aichat.Config{
    // Required
    OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),

    // Expert definitions with handlers
    Experts: experts,

    // Fallback when routing fails
    DefaultExpert:    "support",
    DefaultReasoning: "Routing to support for general assistance",

    // Optional: Custom storage
    Storage: store,

    // Optional: Domain glossary
    Glossary: glossary,

    // Optional: Custom logger
    Logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })),

    // Optional: CORS origins
    AllowedOrigins: []string{"https://myapp.com", "http://localhost:3000"},
})
if err != nil {
    log.Fatal(err)
}
```

### Step 6: Start the Server

**Option A: Use the built-in HTTP handler**
```go
http.ListenAndServe(":3001", sdk.HTTPHandler())
```

**Option B: Mount on your existing router**
```go
router := chi.NewRouter()
router.Mount("/api/chat", sdk.HTTPHandler())
http.ListenAndServe(":8080", router)
```

**Option C: Use the ProcessChat function directly**
```go
processFn := sdk.ProcessChat()

result, err := processFn(ctx, aichat.ChatRequest{
    Message:  "What are the product features?",
    EntityID: "product-123",
})
```

---

## Complete Example

```go
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
```

---

## HTTP API

### POST /chat

Send a chat message and receive a response.

**Request:**
```json
{
    "message": "What features does this product have?",
    "entityId": "product-123",
    "conversationId": "optional-existing-conversation-id"
}
```

**Response:**
```json
{
    "conversationId": "550e8400-e29b-41d4-a716-446655440000",
    "expert": "product",
    "expertName": "Product Expert",
    "message": "What features does this product have?",
    "reasoning": "User is asking about product features",
    "response": "The Widget Pro has the following features..."
}
```

### POST /chat/stream

Same as `/chat` but returns Server-Sent Events for real-time streaming.

**Events:**
```
data: {"type": "thinking"}
data: {"type": "done", "conversationId": "...", "expert": "product", "content": "..."}
```

**Error event:**
```
data: {"type": "error", "content": "Error message"}
```

### GET /health

Health check endpoint.

**Response:**
```json
{"status": "ok"}
```

---

## Advanced Configuration

### Custom Router Prompt

Override the default routing prompt:

```go
RouterSystemPromptTemplate: `You are a router for an e-commerce chatbot.

{{CONTEXT}}

Available experts:
{{EXPERTS}}

Respond with JSON: {"expert": "<type>", "reasoning": "<why>"}`,
```

### Custom Translator Prompt

```go
TranslatorSystemPrompt: `You are a translation expert for an e-commerce platform.
Preserve product names, brand names, and technical specifications exactly.
Return JSON: {"translatedMessage": "...", "detectedLanguage": "...", "confidence": 0.95}`,
```

### Custom Formatter Prompt

```go
FormatterSystemPrompt: `You are a friendly customer service assistant.
Translate the response while maintaining a helpful, professional tone.
Keep technical terms accurate but explain them simply.`,
```

---

## Testing

```bash
# Health check
curl http://localhost:3001/health

# Send a message
curl -X POST http://localhost:3001/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What features does this have?", "entityId": "product-123"}'

# Continue conversation
curl -X POST http://localhost:3001/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "How much does it cost?", "conversationId": "<id-from-previous-response>"}'
```

---

## Architecture

```
User Message
     │
     ▼
┌─────────────┐
│  Translator │  ← Detects language, translates to English
└─────────────┘
     │
     ▼
┌─────────────┐
│   Router    │  ← Routes to appropriate expert based on content
└─────────────┘
     │
     ▼
┌─────────────┐
│   Expert    │  ← Your handler resolves entity data and processes question
└─────────────┘
     │
     ▼
┌─────────────┐
│  Formatter  │  ← Translates response back to user's language
└─────────────┘
     │
     ▼
User Response
```

---

## Why Experts Resolve Their Own Data

The SDK delegates entity resolution to experts rather than using a centralized resolver. This design provides:

1. **Flexibility**: Different experts may need different data about the same entity
2. **Performance**: Experts only fetch what they need
3. **Type Safety**: Each expert knows exactly what data structure it expects
4. **Separation of Concerns**: Data fetching logic stays with the domain expert

For example, a Product Expert might need full product specifications, while a Support Expert only needs the product name to personalize responses.

---

## License

MIT
