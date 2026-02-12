package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	aichat "github.com/ourstudio-se/ai-chat-sdk"
	oai "github.com/sashabaranov/go-openai"
)

// Client wraps the OpenAI API client.
type Client struct {
	client *oai.Client
}

// NewClient creates a new OpenAI client.
func New(client *oai.Client) *Client {
	return &Client{
		client: client,
	}
}

// ChatCompletion sends a chat completion request.
func (c *Client) ChatCompletion(ctx aichat.ChatCompletionContext) (aichat.ChatCompletionResult, error) {
	req := c.buildRequest(ctx)

	resp, err := c.client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		// Log request details for debugging
		reqJSON, _ := json.MarshalIndent(req, "", "  ")
		return aichat.ChatCompletionResult{}, fmt.Errorf("openai chat completion failed: %w\nRequest: %s", err, string(reqJSON))
	}

	if len(resp.Choices) == 0 {
		return aichat.ChatCompletionResult{}, fmt.Errorf("openai returned no choices")
	}

	choice := resp.Choices[0]
	message := c.convertMessage(choice.Message)

	return aichat.ChatCompletionResult{
		Message:      message,
		FinishReason: string(choice.FinishReason),
		Usage: aichat.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// ChatCompletionStream sends a streaming chat completion request.
func (c *Client) ChatCompletionStream(ctx aichat.ChatCompletionContext) (aichat.ChatCompletionStream, error) {
	req := c.buildRequest(ctx)
	req.Stream = true

	stream, err := c.client.CreateChatCompletionStream(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("openai stream failed: %w", err)
	}

	return &streamWrapper{stream: stream}, nil
}

func (c *Client) buildRequest(ctx aichat.ChatCompletionContext) oai.ChatCompletionRequest {
	messages := make([]oai.ChatCompletionMessage, 0, len(ctx.Messages))
	for _, msg := range ctx.Messages {
		messages = append(messages, c.convertToOpenAIMessage(msg))
	}

	req := oai.ChatCompletionRequest{
		Model:       ctx.Model,
		Messages:    messages,
		Temperature: ctx.Temperature,
		MaxTokens:   ctx.MaxTokens,
	}

	// Add tools if provided
	if len(ctx.Tools) > 0 {
		tools := make([]oai.Tool, 0, len(ctx.Tools))
		for _, tool := range ctx.Tools {
			tools = append(tools, oai.Tool{
				Type: oai.ToolTypeFunction,
				Function: &oai.FunctionDefinition{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			})
		}
		req.Tools = tools
	}

	// Add response format if provided
	if ctx.ResponseFormat != nil {
		switch ctx.ResponseFormat.Type {
		case "json_object":
			req.ResponseFormat = &oai.ChatCompletionResponseFormat{
				Type: oai.ChatCompletionResponseFormatTypeJSONObject,
			}
		case "json_schema":
			if ctx.ResponseFormat.JSONSchema != nil {
				req.ResponseFormat = &oai.ChatCompletionResponseFormat{
					Type: oai.ChatCompletionResponseFormatTypeJSONSchema,
					JSONSchema: &oai.ChatCompletionResponseFormatJSONSchema{
						Name:   ctx.ResponseFormat.JSONSchema.Name,
						Schema: json.RawMessage(mustMarshal(ctx.ResponseFormat.JSONSchema.Schema)),
						Strict: ctx.ResponseFormat.JSONSchema.Strict,
					},
				}
			}
		}
	}

	return req
}

func (c *Client) convertToOpenAIMessage(msg aichat.LLMMessage) oai.ChatCompletionMessage {
	oaiMsg := oai.ChatCompletionMessage{
		Role:    msg.Role,
		Content: msg.Content,
		Name:    msg.Name,
	}

	// Handle tool call ID for tool responses
	if msg.ToolCallID != "" {
		oaiMsg.ToolCallID = msg.ToolCallID
	}

	// Convert tool calls
	if len(msg.ToolCalls) > 0 {
		oaiMsg.ToolCalls = make([]oai.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			oaiMsg.ToolCalls = append(oaiMsg.ToolCalls, oai.ToolCall{
				ID:   tc.ID,
				Type: oai.ToolType(tc.Type),
				Function: oai.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	return oaiMsg
}

func (c *Client) convertMessage(msg oai.ChatCompletionMessage) aichat.LLMMessage {
	llmMsg := aichat.LLMMessage{
		Role:       msg.Role,
		Content:    msg.Content,
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
	}

	// Convert tool calls
	if len(msg.ToolCalls) > 0 {
		llmMsg.ToolCalls = make([]aichat.LLMToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			llmMsg.ToolCalls = append(llmMsg.ToolCalls, aichat.LLMToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
				Function: aichat.LLMFunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	return llmMsg
}

// streamWrapper wraps the OpenAI stream.
type streamWrapper struct {
	stream *oai.ChatCompletionStream
}

// Recv receives the next chunk from the stream.
func (s *streamWrapper) Recv() (aichat.ChatCompletionChunk, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		if err == io.EOF {
			return aichat.ChatCompletionChunk{}, io.EOF
		}
		return aichat.ChatCompletionChunk{}, fmt.Errorf("stream recv failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return aichat.ChatCompletionChunk{}, nil
	}

	choice := resp.Choices[0]
	delta := aichat.LLMMessageDelta{
		Role:    choice.Delta.Role,
		Content: choice.Delta.Content,
	}

	// Convert tool calls in delta
	if len(choice.Delta.ToolCalls) > 0 {
		delta.ToolCalls = make([]aichat.LLMToolCall, 0, len(choice.Delta.ToolCalls))
		for _, tc := range choice.Delta.ToolCalls {
			delta.ToolCalls = append(delta.ToolCalls, aichat.LLMToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
				Function: aichat.LLMFunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	return aichat.ChatCompletionChunk{
		Delta:        delta,
		FinishReason: string(choice.FinishReason),
	}, nil
}

// Close closes the stream.
func (s *streamWrapper) Close() error {
	s.stream.Close()
	return nil
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
