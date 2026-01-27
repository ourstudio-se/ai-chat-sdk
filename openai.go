package aichat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	openai "github.com/sashabaranov/go-openai"
)

// openaiClient is an internal struct of functions for OpenAI API access.
type openaiClient struct {
	Chat     ChatFn
	ChatJSON ChatJSONFn
}

// modelMap maps model tiers to actual OpenAI model names.
var modelMap = map[ModelTier]string{
	ModelNano:      "gpt-4o-mini",
	ModelMini:      "gpt-4o-mini",
	ModelStandard:  "gpt-4o",
	ModelReasoning: "gpt-4o",
}

// getModelName returns the actual OpenAI model name for a given tier.
func getModelName(tier ModelTier) string {
	if name, ok := modelMap[tier]; ok {
		return name
	}
	return modelMap[ModelMini]
}

// defaultChatOptions returns ChatOptions with sensible defaults.
func defaultChatOptions() ChatOptions {
	return ChatOptions{
		Model:       ModelMini,
		Temperature: 0.7,
		MaxTokens:   0,
	}
}

// defaultChatJSONOptions returns ChatJSONOptions with sensible defaults.
func defaultChatJSONOptions() ChatJSONOptions {
	return ChatJSONOptions{
		Model:       ModelMini,
		Temperature: 0.3,
		MaxTokens:   0,
	}
}

// newOpenAIClient creates a new OpenAI client with function-based API.
func newOpenAIClient(apiKey string, logger *slog.Logger) (*openaiClient, error) {
	if apiKey == "" {
		return nil, errors.New("OpenAI API key is required")
	}

	client := openai.NewClient(apiKey)

	return &openaiClient{
		Chat:     newChatFn(client, logger),
		ChatJSON: newChatJSONFn(client, logger),
	}, nil
}

func newChatFn(client *openai.Client, logger *slog.Logger) ChatFn {
	return func(ctx context.Context, systemPrompt, userMessage string, opts *ChatOptions) (string, error) {
		if opts == nil {
			defaultOpts := defaultChatOptions()
			opts = &defaultOpts
		}

		modelName := getModelName(opts.Model)

		logger.Debug("creating chat completion",
			slog.String("model", modelName),
			slog.Float64("temperature", float64(opts.Temperature)),
			slog.Int("user_message_len", len(userMessage)),
		)

		req := openai.ChatCompletionRequest{
			Model: modelName,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userMessage,
				},
			},
			Temperature: opts.Temperature,
		}

		if opts.MaxTokens > 0 {
			req.MaxTokens = opts.MaxTokens
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", fmt.Errorf("OpenAI API error: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", errors.New("no response from OpenAI")
		}

		content := resp.Choices[0].Message.Content
		if content == "" {
			return "", errors.New("empty response from OpenAI")
		}

		logger.Debug("chat completion successful",
			slog.String("model", modelName),
			slog.Int("response_len", len(content)),
			slog.Int("prompt_tokens", resp.Usage.PromptTokens),
			slog.Int("completion_tokens", resp.Usage.CompletionTokens),
		)

		return content, nil
	}
}

func newChatJSONFn(client *openai.Client, logger *slog.Logger) ChatJSONFn {
	return func(ctx context.Context, systemPrompt, userMessage string, opts *ChatJSONOptions, result any) error {
		if opts == nil {
			defaultOpts := defaultChatJSONOptions()
			opts = &defaultOpts
		}

		modelName := getModelName(opts.Model)

		logger.Debug("creating JSON chat completion",
			slog.String("model", modelName),
			slog.Float64("temperature", float64(opts.Temperature)),
			slog.Int("user_message_len", len(userMessage)),
		)

		req := openai.ChatCompletionRequest{
			Model: modelName,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userMessage,
				},
			},
			Temperature: opts.Temperature,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		}

		if opts.MaxTokens > 0 {
			req.MaxTokens = opts.MaxTokens
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return fmt.Errorf("OpenAI API error: %w", err)
		}

		if len(resp.Choices) == 0 {
			return errors.New("no response from OpenAI")
		}

		content := resp.Choices[0].Message.Content
		if content == "" {
			return errors.New("empty response from OpenAI")
		}

		if err := json.Unmarshal([]byte(content), result); err != nil {
			return fmt.Errorf("failed to parse OpenAI JSON response: %w (content: %s)", err, content)
		}

		logger.Debug("JSON chat completion successful",
			slog.String("model", modelName),
			slog.Int("response_len", len(content)),
			slog.Int("prompt_tokens", resp.Usage.PromptTokens),
			slog.Int("completion_tokens", resp.Usage.CompletionTokens),
		)

		return nil
	}
}
