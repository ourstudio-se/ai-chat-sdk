// Package anthropic provides an Anthropic LLM provider implementation.
package anthropic

import (
	"context"
	"encoding/json"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/llm"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/tools"
)

// Provider implements llm.Provider for Anthropic's API.
type Provider struct {
	client anthropic.Client
}

// Config for the Anthropic provider.
type Config struct {
	APIKey  string
	BaseURL string
}

// New creates a new Anthropic provider with the given config.
func New(cfg Config) *Provider {
	var opts []option.RequestOption

	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	return &Provider{
		client: anthropic.NewClient(opts...),
	}
}

// NewFromEnv creates a provider using environment variables.
// Supports both direct Anthropic API and Azure Foundry.
func NewFromEnv() *Provider {
	cfg := Config{
		APIKey: os.Getenv("ANTHROPIC_API_KEY"),
	}

	// Azure Foundry support
	if os.Getenv("CLAUDE_CODE_USE_FOUNDRY") == "1" {
		if res := os.Getenv("ANTHROPIC_FOUNDRY_RESOURCE"); res != "" {
			cfg.BaseURL = "https://" + res + ".services.ai.azure.com/anthropic"
		}
		if url := os.Getenv("ANTHROPIC_FOUNDRY_BASE_URL"); url != "" {
			cfg.BaseURL = url
		}
		cfg.APIKey = os.Getenv("ANTHROPIC_FOUNDRY_API_KEY")
	}

	return New(cfg)
}

// Chat sends a chat request to Anthropic and returns the response.
func (p *Provider) Chat(ctx context.Context, req llm.Request) (*llm.Response, error) {
	// Build messages
	messages := toAnthropicMessages(req.Messages)

	// Build params
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  messages,
	}

	// Add system prompt if provided
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.System},
		}
	}

	// Add tools if provided
	if len(req.Tools) > 0 {
		params.Tools = toAnthropicTools(req.Tools)
	}

	// Make the request
	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, err
	}

	return fromAnthropicResponse(resp), nil
}

// toAnthropicMessages converts our messages to Anthropic format.
func toAnthropicMessages(msgs []llm.Message) []anthropic.MessageParam {
	var result []anthropic.MessageParam

	for _, msg := range msgs {
		switch msg.Role {
		case llm.RoleUser:
			if msg.ToolResult != nil {
				// Tool result message
				result = append(result, anthropic.NewUserMessage(
					anthropic.NewToolResultBlock(
						msg.ToolResult.ToolCallID,
						msg.ToolResult.Content,
						msg.ToolResult.IsError,
					),
				))
			} else {
				// Regular user message
				result = append(result, anthropic.NewUserMessage(
					anthropic.NewTextBlock(msg.Content),
				))
			}

		case llm.RoleAssistant:
			var content []anthropic.ContentBlockParamUnion

			// Add text content if present
			if msg.Content != "" {
				content = append(content, anthropic.NewTextBlock(msg.Content))
			}

			// Add tool use blocks
			for _, tc := range msg.ToolCalls {
				inputJSON, _ := json.Marshal(tc.Input)
				content = append(content, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    tc.ID,
						Name:  tc.Name,
						Input: json.RawMessage(inputJSON),
					},
				})
			}

			result = append(result, anthropic.NewAssistantMessage(content...))

		case llm.RoleTool:
			// Tool results are sent as user messages
			if msg.ToolResult != nil {
				result = append(result, anthropic.NewUserMessage(
					anthropic.NewToolResultBlock(
						msg.ToolResult.ToolCallID,
						msg.ToolResult.Content,
						msg.ToolResult.IsError,
					),
				))
			}
		}
	}

	return result
}

// toAnthropicTools converts our tool definitions to Anthropic format.
func toAnthropicTools(defs []tools.Definition) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, len(defs))

	for i, d := range defs {
		inputSchema := anthropic.ToolInputSchemaParam{
			Type: "object",
		}
		if props, ok := d.Parameters["properties"]; ok {
			inputSchema.Properties = props
		}

		result[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        d.Name,
				Description: anthropic.String(d.Description),
				InputSchema: inputSchema,
			},
		}
	}

	return result
}

// fromAnthropicResponse converts an Anthropic response to our format.
func fromAnthropicResponse(resp *anthropic.Message) *llm.Response {
	result := &llm.Response{
		Usage: llm.Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		},
	}

	// Map stop reason
	switch resp.StopReason {
	case "end_turn":
		result.StopReason = llm.StopReasonEnd
	case "tool_use":
		result.StopReason = llm.StopReasonToolUse
	case "max_tokens":
		result.StopReason = llm.StopReasonLength
	default:
		result.StopReason = llm.StopReasonEnd
	}

	// Extract content
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			input := make(map[string]any)
			_ = json.Unmarshal(block.Input, &input)
			result.ToolCalls = append(result.ToolCalls, llm.ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: input,
			})
		}
	}

	return result
}
