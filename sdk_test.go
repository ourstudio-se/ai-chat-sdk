package aichat

import (
	"context"
	"encoding/json"
	"testing"
)

// mockLLMClient is a mock LLM client for testing.
type mockLLMClient struct {
	response    string
	toolCalls   []LLMToolCall
	shouldError bool
}

func (m *mockLLMClient) ChatCompletion(ctx ChatCompletionContext) (ChatCompletionResult, error) {
	if m.shouldError {
		return ChatCompletionResult{}, NewLLMError("mock error", nil)
	}

	return ChatCompletionResult{
		Message: LLMMessage{
			Role:      "assistant",
			Content:   m.response,
			ToolCalls: m.toolCalls,
		},
		FinishReason: "stop",
		Usage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}

func (m *mockLLMClient) ChatCompletionStream(ctx ChatCompletionContext) (ChatCompletionStream, error) {
	return nil, NewLLMError("not implemented", nil)
}

// mockSkillRegistry is a mock skill registry for testing.
type mockSkillRegistry struct {
	skills map[string]*Skill
}

func (m *mockSkillRegistry) Get(id string) (*Skill, bool) {
	skill, ok := m.skills[id]
	return skill, ok
}

func (m *mockSkillRegistry) All() []*Skill {
	var skills []*Skill
	for _, skill := range m.skills {
		skills = append(skills, skill)
	}
	return skills
}

func (m *mockSkillRegistry) Match(message string) []*Skill {
	// Simple match: return first skill
	for _, skill := range m.skills {
		return []*Skill{skill}
	}
	return nil
}

// mockToolRegistry is a mock tool registry for testing.
type mockToolRegistry struct {
	sources map[string]*Source
	actions map[string]*Action
}

func (m *mockToolRegistry) GetSource(name string) (*Source, bool) {
	src, ok := m.sources[name]
	return src, ok
}

func (m *mockToolRegistry) GetAction(name string) (*Action, bool) {
	act, ok := m.actions[name]
	return act, ok
}

func (m *mockToolRegistry) AllSources() []*Source {
	var sources []*Source
	for _, src := range m.sources {
		sources = append(sources, src)
	}
	return sources
}

func (m *mockToolRegistry) AllActions() []*Action {
	var actions []*Action
	for _, act := range m.actions {
		actions = append(actions, act)
	}
	return actions
}

func (m *mockToolRegistry) GetForSkill(toolNames []string) ([]*Source, []*Action, error) {
	var sources []*Source
	var actions []*Action
	for _, name := range toolNames {
		if src, ok := m.sources[name]; ok {
			sources = append(sources, src)
		}
		if act, ok := m.actions[name]; ok {
			actions = append(actions, act)
		}
	}
	return sources, actions, nil
}

func TestNewSDK(t *testing.T) {
	t.Run("creates SDK with valid config", func(t *testing.T) {
		cfg := Config{
			LLMClient: &mockLLMClient{},
			Skills:    &mockSkillRegistry{skills: map[string]*Skill{}},
			Tools:     &mockToolRegistry{},
		}

		sdk, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if sdk == nil {
			t.Fatal("expected SDK to be created")
		}
	})

	t.Run("returns error without LLM client", func(t *testing.T) {
		cfg := Config{
			Skills: &mockSkillRegistry{},
			Tools:  &mockToolRegistry{},
		}

		_, err := New(cfg)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("applies default values", func(t *testing.T) {
		cfg := Config{
			LLMClient: &mockLLMClient{},
			Skills:    &mockSkillRegistry{skills: map[string]*Skill{}},
			Tools:     &mockToolRegistry{},
		}

		sdk, err := New(cfg)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if sdk.config.ExecutionMode != ModeExpert {
			t.Errorf("expected default mode to be expert, got: %s", sdk.config.ExecutionMode)
		}
		if sdk.config.MaxAgentTurns != 10 {
			t.Errorf("expected max agent turns to be 10, got: %d", sdk.config.MaxAgentTurns)
		}
		if sdk.config.Model != "gpt-4o" {
			t.Errorf("expected default model to be gpt-4o, got: %s", sdk.config.Model)
		}
	})
}

func TestChat(t *testing.T) {
	t.Run("routes to skill and returns response", func(t *testing.T) {
		responseJSON := `{"answer": "Test response"}`

		skills := &mockSkillRegistry{
			skills: map[string]*Skill{
				"test": {
					ID:           "test",
					Name:         "Test Skill",
					Triggers:     []string{"test"},
					Instructions: "You are a test assistant",
					Output: &OutputSchema{
						Type: "object",
						Properties: map[string]PropertySchema{
							"answer": {Type: "string"},
						},
						Required: []string{"answer"},
					},
				},
			},
		}

		sdk, err := New(Config{
			LLMClient: &mockLLMClient{response: responseJSON},
			Skills:    skills,
			Tools:     &mockToolRegistry{},
		})
		if err != nil {
			t.Fatalf("failed to create SDK: %v", err)
		}

		result, err := sdk.Chat(context.Background(), ChatRequest{
			Message: "test message",
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if result.SkillID != "test" {
			t.Errorf("expected skill ID to be 'test', got: %s", result.SkillID)
		}
		if result.ConversationID == "" {
			t.Error("expected conversation ID to be set")
		}
		if result.MessageID == "" {
			t.Error("expected message ID to be set")
		}
	})

	t.Run("uses specified skill ID", func(t *testing.T) {
		responseJSON := `{"answer": "Test response"}`

		skills := &mockSkillRegistry{
			skills: map[string]*Skill{
				"specific": {
					ID:           "specific",
					Name:         "Specific Skill",
					Instructions: "You are specific",
					Output: &OutputSchema{
						Type: "object",
						Properties: map[string]PropertySchema{
							"answer": {Type: "string"},
						},
						Required: []string{"answer"},
					},
				},
			},
		}

		sdk, err := New(Config{
			LLMClient: &mockLLMClient{response: responseJSON},
			Skills:    skills,
			Tools:     &mockToolRegistry{},
		})
		if err != nil {
			t.Fatalf("failed to create SDK: %v", err)
		}

		result, err := sdk.Chat(context.Background(), ChatRequest{
			Message: "any message",
			SkillID: "specific",
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if result.SkillID != "specific" {
			t.Errorf("expected skill ID to be 'specific', got: %s", result.SkillID)
		}
	})

	t.Run("returns error for unknown skill", func(t *testing.T) {
		skills := &mockSkillRegistry{
			skills: map[string]*Skill{},
		}

		sdk, err := New(Config{
			LLMClient: &mockLLMClient{},
			Skills:    skills,
			Tools:     &mockToolRegistry{},
		})
		if err != nil {
			t.Fatalf("failed to create SDK: %v", err)
		}

		_, err = sdk.Chat(context.Background(), ChatRequest{
			Message: "any message",
			SkillID: "unknown",
		})
		if err == nil {
			t.Fatal("expected error for unknown skill")
		}
	})
}

func TestExecuteSkill(t *testing.T) {
	t.Run("executes skill and returns typed response", func(t *testing.T) {
		responseJSON := `{"answer": "Skill response", "count": 42}`

		skills := &mockSkillRegistry{
			skills: map[string]*Skill{
				"test": {
					ID:           "test",
					Name:         "Test Skill",
					Instructions: "Test instructions",
					Output: &OutputSchema{
						Type: "object",
						Properties: map[string]PropertySchema{
							"answer": {Type: "string"},
							"count":  {Type: "integer"},
						},
						Required: []string{"answer"},
					},
				},
			},
		}

		sdk, err := New(Config{
			LLMClient: &mockLLMClient{response: responseJSON},
			Skills:    skills,
			Tools:     &mockToolRegistry{},
		})
		if err != nil {
			t.Fatalf("failed to create SDK: %v", err)
		}

		result, err := sdk.ExecuteSkill(context.Background(), "test", SkillRequest{
			Message: "test message",
			Data:    map[string]any{"key": "value"},
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Extract typed response
		var resp struct {
			Answer string `json:"answer"`
			Count  int    `json:"count"`
		}
		if err := json.Unmarshal(result.Response, &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Answer != "Skill response" {
			t.Errorf("expected answer 'Skill response', got: %s", resp.Answer)
		}
		if resp.Count != 42 {
			t.Errorf("expected count 42, got: %d", resp.Count)
		}
	})
}

func TestVariantSelection(t *testing.T) {
	t.Run("uses requested variant", func(t *testing.T) {
		responseJSON := `{"answer": "Response"}`

		skills := &mockSkillRegistry{
			skills: map[string]*Skill{
				"test": {
					ID:           "test",
					Instructions: "Default instructions",
					Variants: []SkillVariant{
						{Variant: "v1", Weight: 50, Instructions: "V1 instructions"},
						{Variant: "v2", Weight: 50, Instructions: "V2 instructions"},
					},
					Output: &OutputSchema{
						Type: "object",
						Properties: map[string]PropertySchema{
							"answer": {Type: "string"},
						},
						Required: []string{"answer"},
					},
				},
			},
		}

		sdk, err := New(Config{
			LLMClient: &mockLLMClient{response: responseJSON},
			Skills:    skills,
			Tools:     &mockToolRegistry{},
		})
		if err != nil {
			t.Fatalf("failed to create SDK: %v", err)
		}

		result, err := sdk.Chat(context.Background(), ChatRequest{
			Message: "test",
			SkillID: "test",
			Variant: "v2",
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if result.Variant != "v2" {
			t.Errorf("expected variant 'v2', got: %s", result.Variant)
		}
	})
}

func TestRequestContext(t *testing.T) {
	t.Run("extracts string values", func(t *testing.T) {
		ctx := RequestContext{
			"market": "SE",
			"locale": "sv-SE",
		}

		if ctx.String("market", "") != "SE" {
			t.Error("expected market to be 'SE'")
		}
		if ctx.String("missing", "default") != "default" {
			t.Error("expected default value for missing key")
		}
	})

	t.Run("extracts int values", func(t *testing.T) {
		ctx := RequestContext{
			"count":   42,
			"float":   3.14,
			"int64":   int64(100),
		}

		if ctx.Int("count", 0) != 42 {
			t.Error("expected count to be 42")
		}
		if ctx.Int("float", 0) != 3 {
			t.Error("expected float to be truncated to 3")
		}
		if ctx.Int("missing", 99) != 99 {
			t.Error("expected default value for missing key")
		}
	})

	t.Run("extracts bool values", func(t *testing.T) {
		ctx := RequestContext{
			"enabled": true,
		}

		if !ctx.Bool("enabled", false) {
			t.Error("expected enabled to be true")
		}
		if ctx.Bool("missing", true) != true {
			t.Error("expected default value for missing key")
		}
	})
}

func TestGetResponse(t *testing.T) {
	t.Run("extracts typed response", func(t *testing.T) {
		type TestResponse struct {
			Answer string `json:"answer"`
			Count  int    `json:"count"`
		}

		result := ChatResult{
			Response: json.RawMessage(`{"answer": "test", "count": 5}`),
		}

		resp, err := GetResponse[TestResponse](result)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if resp.Answer != "test" {
			t.Errorf("expected answer 'test', got: %s", resp.Answer)
		}
		if resp.Count != 5 {
			t.Errorf("expected count 5, got: %d", resp.Count)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		type TestResponse struct {
			Answer string `json:"answer"`
		}

		result := ChatResult{
			Response: json.RawMessage(`not json`),
		}

		_, err := GetResponse[TestResponse](result)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestRouter(t *testing.T) {
	t.Run("routes based on triggers", func(t *testing.T) {
		skills := &mockSkillRegistry{
			skills: map[string]*Skill{
				"product": {
					ID:       "product",
					Triggers: []string{"product", "price"},
				},
				"support": {
					ID:       "support",
					Triggers: []string{"help", "issue"},
				},
			},
		}

		router := NewRouter(skills, "")

		// Test product match
		skill := router.Route("Tell me about the product")
		if skill == nil {
			t.Fatal("expected skill to be found")
		}
	})

	t.Run("uses default skill when no match", func(t *testing.T) {
		skills := &mockSkillRegistry{
			skills: map[string]*Skill{
				"default": {
					ID: "default",
				},
			},
		}

		router := NewRouter(skills, "default")

		skill := router.Route("random message with no triggers")
		if skill == nil {
			t.Fatal("expected default skill")
		}
		if skill.ID != "default" {
			t.Errorf("expected default skill, got: %s", skill.ID)
		}
	})
}

func TestMemoryStore(t *testing.T) {
	t.Run("stores and retrieves messages", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := ChatCompletionContext{}
		convID := "conv-123"

		// Add messages
		msg1 := Message{ID: "msg-1", ConversationID: convID, Role: "user", Content: "Hello"}
		msg2 := Message{ID: "msg-2", ConversationID: convID, Role: "assistant", Content: "Hi there!"}

		if err := store.AddMessage(ctx, convID, msg1); err != nil {
			t.Fatalf("failed to add message: %v", err)
		}
		if err := store.AddMessage(ctx, convID, msg2); err != nil {
			t.Fatalf("failed to add message: %v", err)
		}

		// Retrieve messages
		messages, err := store.GetMessages(ctx, convID, 10)
		if err != nil {
			t.Fatalf("failed to get messages: %v", err)
		}

		if len(messages) != 2 {
			t.Errorf("expected 2 messages, got: %d", len(messages))
		}
	})

	t.Run("limits returned messages", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := ChatCompletionContext{}
		convID := "conv-456"

		// Add 5 messages
		for i := 0; i < 5; i++ {
			msg := Message{ID: string(rune('a' + i)), ConversationID: convID}
			store.AddMessage(ctx, convID, msg)
		}

		// Request only 2
		messages, err := store.GetMessages(ctx, convID, 2)
		if err != nil {
			t.Fatalf("failed to get messages: %v", err)
		}

		if len(messages) != 2 {
			t.Errorf("expected 2 messages, got: %d", len(messages))
		}
	})

	t.Run("stores and retrieves feedback", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := ChatCompletionContext{}

		fb := Feedback{MessageID: "msg-1", Rating: 5, Comment: "Great!"}
		if err := store.SaveFeedback(ctx, fb); err != nil {
			t.Fatalf("failed to save feedback: %v", err)
		}
	})
}
