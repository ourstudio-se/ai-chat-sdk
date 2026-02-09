// pkg/agent/agent.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/conversation"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/llm"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/skills"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/tools"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/types"
)

// Re-export types for convenience
type (
	Message      = types.Message
	Role         = types.Role
	ToolCall     = types.ToolCall
	Conversation = types.Conversation
	Metadata     = types.Metadata
	ChatRequest  = types.ChatRequest
	ChatResponse = types.ChatResponse
)

const (
	RoleUser      = types.RoleUser
	RoleAssistant = types.RoleAssistant
	RoleTool      = types.RoleTool
)

var NewConversation = types.NewConversation

// Agent orchestrates conversations with an LLM
type Agent struct {
	provider      llm.Provider
	convStore     conversation.Store
	tools         *tools.Registry
	skills        *skills.Registry
	config        Config
	promptBuilder PromptBuilder
	abSelector    func(id string, variants []*skills.Skill) *skills.Skill
}

// PromptBuilder constructs system prompts from context
// Implement this for your domain-specific prompt logic
type PromptBuilder interface {
	Build(basePrompt string, skillsContent string, context map[string]any) string
}

// ToolAwarePromptBuilder builds prompts that highlight context fields matching tool parameters.
// This helps the LLM understand which context values to use with which tools.
type ToolAwarePromptBuilder struct {
	toolRegistry *tools.Registry

	// Optional: custom formatters for specific context keys
	// If nil, uses default formatting
	Formatters map[string]func(key string, value any) string

	// Optional: additional context to always include (even if not matching tool params)
	AlwaysInclude []string

	// Optional: custom section header (default: "## Current Context")
	SectionHeader string
}

// NewToolAwarePromptBuilder creates a prompt builder that uses tool parameter info
func NewToolAwarePromptBuilder(toolRegistry *tools.Registry) *ToolAwarePromptBuilder {
	return &ToolAwarePromptBuilder{
		toolRegistry:  toolRegistry,
		Formatters:    make(map[string]func(string, any) string),
		SectionHeader: "## Current Context",
	}
}

func (b *ToolAwarePromptBuilder) Build(basePrompt, skillsContent string, context map[string]any) string {
	prompt := basePrompt
	if prompt != "" {
		prompt += "\n\n"
	}
	prompt += skillsContent

	if len(context) == 0 {
		return prompt
	}

	// Get tool parameters to know which context fields are relevant
	toolParams := b.toolRegistry.Parameters()

	// Collect context fields that match tool parameters
	var relevantFields []string
	for key := range context {
		if _, isToolParam := toolParams[key]; isToolParam {
			relevantFields = append(relevantFields, key)
		}
	}

	// Add always-include fields
	for _, key := range b.AlwaysInclude {
		if _, exists := context[key]; exists {
			// Check if not already in relevantFields
			found := false
			for _, f := range relevantFields {
				if f == key {
					found = true
					break
				}
			}
			if !found {
				relevantFields = append(relevantFields, key)
			}
		}
	}

	if len(relevantFields) == 0 {
		return prompt
	}

	prompt += fmt.Sprintf("\n%s\n", b.SectionHeader)

	for _, key := range relevantFields {
		value := context[key]

		// Use custom formatter if available
		if formatter, ok := b.Formatters[key]; ok {
			prompt += formatter(key, value) + "\n"
			continue
		}

		// Default formatting based on tool param info
		if paramInfo, ok := toolParams[key]; ok {
			prompt += fmt.Sprintf("- **%s**: %v", key, value)
			if paramInfo.Description != "" {
				prompt += fmt.Sprintf(" _(used by %s)_", paramInfo.ToolName)
			}
			prompt += "\n"
		} else {
			prompt += fmt.Sprintf("- **%s**: %v\n", key, value)
		}
	}

	return prompt
}

// Option configures the agent
type Option func(*Agent)

// WithConfig sets the agent config
func WithConfig(cfg Config) Option {
	return func(a *Agent) { a.config = cfg }
}

// WithPromptBuilder sets a custom prompt builder
func WithPromptBuilder(pb PromptBuilder) Option {
	return func(a *Agent) { a.promptBuilder = pb }
}

// WithABSelector sets the A/B variant selector
func WithABSelector(sel func(id string, variants []*skills.Skill) *skills.Skill) Option {
	return func(a *Agent) { a.abSelector = sel }
}

// New creates a new agent
func New(
	provider llm.Provider,
	convStore conversation.Store,
	toolRegistry *tools.Registry,
	skillRegistry *skills.Registry,
	opts ...Option,
) *Agent {
	a := &Agent{
		provider:      provider,
		convStore:     convStore,
		tools:         toolRegistry,
		skills:        skillRegistry,
		config:        DefaultConfig(),
		promptBuilder: NewToolAwarePromptBuilder(toolRegistry),
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Chat handles a chat request
func (a *Agent) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Load or create conversation
	var conv *types.Conversation
	var err error

	if req.SessionID != "" {
		conv, err = a.convStore.Get(ctx, req.SessionID)
		if err != nil {
			return nil, fmt.Errorf("loading conversation: %w", err)
		}
	}

	if conv == nil {
		conv = NewConversation()
	}

	// Update context if provided
	if req.Context != nil {
		for k, v := range req.Context {
			conv.Context[k] = v
		}
	}

	// Set user ID in metadata
	if req.UserID != "" {
		conv.Metadata.UserID = req.UserID
	}

	// Add user message
	conv.AddUserMessage(req.Message)

	// Select skills based on message
	selectedSkills := a.skills.Select(req.Message, a.abSelector)

	// Track variant for analytics
	variantInfo := skills.VariantInfo(selectedSkills)
	conv.Metadata.SkillVariant = variantInfo

	// Build system prompt
	skillsContent := skills.FormatForPrompt(selectedSkills)
	systemPrompt := a.promptBuilder.Build(a.config.BasePrompt, skillsContent, conv.Context)

	// Run agent loop
	response, toolCalls, err := a.runLoop(ctx, systemPrompt, conv.Messages)
	if err != nil {
		return nil, fmt.Errorf("agent loop: %w", err)
	}

	// Add assistant message
	msg := conv.AddAssistantMessage(response, toolCalls)

	// Save conversation
	if err := a.convStore.Save(ctx, conv); err != nil {
		return nil, fmt.Errorf("saving conversation: %w", err)
	}

	// Extract skill IDs
	skillIDs := make([]string, len(selectedSkills))
	for i, s := range selectedSkills {
		skillIDs[i] = s.ID
	}

	return &ChatResponse{
		SessionID:  conv.ID,
		MessageID:  msg.ID,
		Response:   response,
		ToolCalls:  toolCalls,
		SkillsUsed: skillIDs,
	}, nil
}

func (a *Agent) runLoop(ctx context.Context, systemPrompt string, messages []Message) (string, []ToolCall, error) {
	var allToolCalls []ToolCall

	// Convert to LLM messages
	llmMessages := a.toLLMMessages(messages)

	for turn := 0; turn < a.config.MaxTurns; turn++ {
		resp, err := a.provider.Chat(ctx, llm.Request{
			Model:       a.config.Model,
			System:      systemPrompt,
			Messages:    llmMessages,
			Tools:       a.tools.Definitions(),
			MaxTokens:   a.config.MaxTokens,
			Temperature: a.config.Temperature,
		})
		if err != nil {
			return "", nil, err
		}

		// Done - return response
		if resp.StopReason == llm.StopReasonEnd {
			return resp.Content, allToolCalls, nil
		}

		// Handle tool use
		if resp.StopReason == llm.StopReasonToolUse {
			// Add assistant message with tool calls
			assistantMsg := llm.Message{
				Role:      llm.RoleAssistant,
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			}
			llmMessages = append(llmMessages, assistantMsg)

			// Execute tools
			for _, tc := range resp.ToolCalls {
				output, err := a.tools.Execute(ctx, tc.Name, tc.Input)

				toolCall := ToolCall{
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				}

				var resultContent string
				if err != nil {
					toolCall.Error = err.Error()
					resultContent = fmt.Sprintf("Error: %v", err)
				} else {
					toolCall.Output = output
					b, _ := json.Marshal(output)
					resultContent = string(b)
				}

				allToolCalls = append(allToolCalls, toolCall)

				// Add tool result
				llmMessages = append(llmMessages, llm.Message{
					Role: llm.RoleTool,
					ToolResult: &llm.ToolResult{
						ToolCallID: tc.ID,
						Content:    resultContent,
						IsError:    err != nil,
					},
				})
			}
		}
	}

	return "I wasn't able to complete your request in the allowed steps.", allToolCalls, nil
}

func (a *Agent) toLLMMessages(messages []Message) []llm.Message {
	result := make([]llm.Message, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			result = append(result, llm.Message{
				Role:    llm.RoleUser,
				Content: msg.Content,
			})
		case RoleAssistant:
			llmTC := make([]llm.ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				llmTC[i] = llm.ToolCall{ID: tc.ID, Name: tc.Name, Input: tc.Input}
			}
			result = append(result, llm.Message{
				Role:      llm.RoleAssistant,
				Content:   msg.Content,
				ToolCalls: llmTC,
			})
		}
	}

	return result
}

// GetConversation returns a conversation by ID
func (a *Agent) GetConversation(ctx context.Context, id string) (*types.Conversation, error) {
	return a.convStore.Get(ctx, id)
}
