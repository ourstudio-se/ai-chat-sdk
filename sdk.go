package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// SDK is the main entry point for the unified AI chat SDK.
type SDK struct {
	config    Config
	router    *Router
	experts   map[string]*Expert
	hooks     *HookRegistry
	llm       LLMClient
	skills    SkillRegistry
	tools     ToolRegistry
	storage   ConversationStore
	logger    *slog.Logger
}

// New creates a new SDK instance with the given configuration.
func New(cfg Config) (*SDK, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Build expert map
	experts := make(map[string]*Expert)
	for _, expert := range cfg.Experts {
		experts[expert.SkillID] = expert
	}

	// Create router
	router := NewRouter(cfg.Skills, cfg.DefaultSkillID)

	// Use in-memory storage if none provided
	storage := cfg.Storage
	if storage == nil {
		storage = NewMemoryStore()
	}

	// Use empty hook registry if none provided
	hooks := cfg.Hooks
	if hooks == nil {
		hooks = NewHookRegistry()
	}

	return &SDK{
		config:  cfg,
		router:  router,
		experts: experts,
		hooks:   hooks,
		llm:     cfg.LLMClient,
		skills:  cfg.Skills,
		tools:   cfg.Tools,
		storage: storage,
		logger:  cfg.Logger,
	}, nil
}

// Chat processes a chat request and returns a typed response.
func (s *SDK) Chat(ctx context.Context, req ChatRequest) (ChatResult, error) {
	start := time.Now()

	// Create or retrieve conversation
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = NewConversationID()
	}

	// Route to skill
	skill, err := s.routeToSkill(req)
	if err != nil {
		return ChatResult{}, fmt.Errorf("routing failed: %w", err)
	}

	s.logger.Debug("routed to skill",
		"skillId", skill.ID,
		"message", req.Message,
	)

	// Determine execution mode
	mode := s.determineMode(req, skill)

	// Execute based on mode
	var result ChatResult
	switch mode {
	case ModeExpert:
		result, err = s.executeExpertMode(ctx, req, skill, conversationID)
	case ModeAgentic:
		result, err = s.executeAgenticMode(ctx, req, skill, conversationID)
	default:
		return ChatResult{}, NewConfigurationError(fmt.Sprintf("unknown execution mode: %s", mode), nil)
	}

	if err != nil {
		return ChatResult{}, err
	}

	result.Duration = time.Since(start)
	result.ConversationID = conversationID
	result.SkillID = skill.ID
	result.Mode = mode

	// Store messages
	if err := s.storeMessages(ctx, conversationID, req, result); err != nil {
		s.logger.Warn("failed to store messages", "error", err)
	}

	return result, nil
}

// ExecuteSkill runs a skill directly with provided data (for Expert handlers).
func (s *SDK) ExecuteSkill(ctx context.Context, skillID string, req SkillRequest) (*SkillResult, error) {
	skill, ok := s.skills.Get(skillID)
	if !ok {
		return nil, NewNotFoundError("skill", skillID)
	}

	// Select variant
	variant := s.selectVariant(skill, req.Variant, req.Context.String("userId", ""))

	// Build prompt
	prompt := s.buildPrompt(skill, variant, req.Message, req.Data, req.Context)

	// Get response format from schema
	responseFormat := s.buildResponseFormat(skill)

	// Call LLM
	llmResult, err := s.llm.ChatCompletion(ChatCompletionContext{
		Model:          s.config.Model,
		Messages:       prompt,
		ResponseFormat: responseFormat,
		Temperature:    s.config.Temperature,
	})
	if err != nil {
		return nil, NewLLMError("chat completion failed", err)
	}

	// Validate response against schema
	response := json.RawMessage(llmResult.Message.Content)
	if err := s.validateResponse(skill, response); err != nil {
		return nil, NewSchemaError("response validation failed", err)
	}

	return &SkillResult{
		Response:   response,
		Variant:    variant.Variant,
		TokensUsed: llmResult.Usage,
	}, nil
}

// ExecuteAction executes a confirmed action.
func (s *SDK) ExecuteAction(ctx context.Context, actionName string, params map[string]any) (any, error) {
	action, ok := s.tools.GetAction(actionName)
	if !ok {
		return nil, NewNotFoundError("action", actionName)
	}

	input := newToolInput(params, action.Params)
	return action.Execute(ctx, input)
}

// HTTPHandler returns an HTTP handler for the SDK.
func (s *SDK) HTTPHandler() http.Handler {
	return NewHTTPHandler(s)
}

// routeToSkill determines which skill should handle the request.
func (s *SDK) routeToSkill(req ChatRequest) (*Skill, error) {
	// Direct skill routing
	if req.SkillID != "" {
		skill, ok := s.skills.Get(req.SkillID)
		if !ok {
			return nil, NewNotFoundError("skill", req.SkillID)
		}
		return skill, nil
	}

	// Use router
	skill := s.router.Route(req.Message)
	if skill == nil {
		return nil, NewRoutingError("no skill matched", ErrNoSkillMatched)
	}

	return skill, nil
}

// determineMode returns the execution mode for this request.
func (s *SDK) determineMode(req ChatRequest, skill *Skill) ExecutionMode {
	// Request override
	if req.Mode != "" {
		return req.Mode
	}
	// Skill override
	if skill.Mode != "" {
		return skill.Mode
	}
	// Config default
	return s.config.ExecutionMode
}

// executeExpertMode runs the skill in expert mode.
func (s *SDK) executeExpertMode(ctx context.Context, req ChatRequest, skill *Skill, conversationID string) (ChatResult, error) {
	// Get expert for this skill
	expert := s.experts[skill.ID]

	// Build request
	expertReq := Request{
		Message:        req.Message,
		EntityID:       req.EntityID,
		Context:        req.Context,
		ConversationID: conversationID,
	}

	// Load conversation history
	history, err := s.loadConversationHistory(ctx, conversationID)
	if err != nil {
		s.logger.Warn("failed to load conversation history", "error", err)
	}
	expertReq.ConversationHistory = history

	// Fetch data if expert has a fetcher
	var fetchedData any
	var toolCalls []ToolCall
	if expert != nil && expert.Fetcher != nil {
		toolExec := &toolExecutorImpl{
			tools:     s.tools,
			toolCalls: &toolCalls,
		}
		fetchedData, err = expert.Fetcher(ctx, expertReq, toolExec)
		if err != nil {
			return ChatResult{}, NewToolError("expert fetcher", err)
		}
	}

	// Initialize metadata for hooks
	metadata := make(map[string]any)

	// Run preprocessing hook if registered
	if preprocessHook, ok := s.hooks.GetPreprocess(skill.ID); ok {
		preprocessReq := &PreprocessRequest{
			SkillID:  skill.ID,
			Message:  req.Message,
			Data:     fetchedData,
			Context:  req.Context,
			Metadata: metadata,
		}
		if err := preprocessHook(ctx, preprocessReq); err != nil {
			return ChatResult{}, fmt.Errorf("preprocess hook failed: %w", err)
		}
		// Update data from hook (may have been modified)
		fetchedData = preprocessReq.Data
	}

	// Execute skill
	skillResult, err := s.ExecuteSkill(ctx, skill.ID, SkillRequest{
		Message:        req.Message,
		Data:           fetchedData,
		Context:        req.Context,
		Variant:        req.Variant,
		ConversationID: conversationID,
	})
	if err != nil {
		return ChatResult{}, err
	}

	// Run postprocessing hook if registered
	if postprocessHook, ok := s.hooks.GetPostprocess(skill.ID); ok {
		// Parse response for hook
		var responseMap map[string]any
		if err := json.Unmarshal(skillResult.Response, &responseMap); err != nil {
			responseMap = make(map[string]any)
		}

		postprocessReq := &PostprocessRequest{
			SkillID:    skill.ID,
			Message:    req.Message,
			Response:   responseMap,
			Data:       fetchedData,
			Context:    req.Context,
			Metadata:   metadata,
			Variant:    skillResult.Variant,
			TokensUsed: skillResult.TokensUsed,
		}
		if err := postprocessHook(ctx, postprocessReq); err != nil {
			return ChatResult{}, fmt.Errorf("postprocess hook failed: %w", err)
		}
		// Update response from hook (may have been modified)
		modifiedResponse, err := json.Marshal(postprocessReq.Response)
		if err != nil {
			return ChatResult{}, fmt.Errorf("failed to marshal modified response: %w", err)
		}
		skillResult.Response = modifiedResponse
	}

	// Post-process if expert has a post-processor
	var response json.RawMessage
	var suggestedAction *SuggestedAction
	if expert != nil && expert.PostProcess != nil {
		expertResult, err := expert.PostProcess(ctx, expertReq, skillResult, fetchedData)
		if err != nil {
			return ChatResult{}, fmt.Errorf("expert post-processing failed: %w", err)
		}
		// Convert expert result to response
		response, err = json.Marshal(expertResult)
		if err != nil {
			return ChatResult{}, fmt.Errorf("failed to marshal expert result: %w", err)
		}
		suggestedAction = expertResult.SuggestedAction
	} else {
		response = skillResult.Response
	}

	return ChatResult{
		MessageID:       NewMessageID(),
		Variant:         skillResult.Variant,
		ToolCalls:       toolCalls,
		Response:        response,
		SuggestedAction: suggestedAction,
		TokensUsed:      skillResult.TokensUsed,
	}, nil
}

// executeAgenticMode runs the skill in agentic mode.
func (s *SDK) executeAgenticMode(ctx context.Context, req ChatRequest, skill *Skill, conversationID string) (ChatResult, error) {
	// Select variant
	variant := s.selectVariant(skill, req.Variant, req.Context.String("userId", ""))

	// Get tools for this skill
	sources, actions, err := s.tools.GetForSkill(skill.Tools)
	if err != nil {
		return ChatResult{}, err
	}

	// Initialize metadata for hooks
	metadata := make(map[string]any)
	var fetchedData any // Accumulated data from tool calls

	// Run preprocessing hook if registered (before agent loop starts)
	if preprocessHook, ok := s.hooks.GetPreprocess(skill.ID); ok {
		preprocessReq := &PreprocessRequest{
			SkillID:  skill.ID,
			Message:  req.Message,
			Data:     nil, // No data yet in agentic mode
			Context:  req.Context,
			Metadata: metadata,
		}
		if err := preprocessHook(ctx, preprocessReq); err != nil {
			return ChatResult{}, fmt.Errorf("preprocess hook failed: %w", err)
		}
		// Hooks can inject initial data for agentic mode
		fetchedData = preprocessReq.Data
	}

	// Build LLM tools
	llmTools := s.buildLLMTools(sources, actions)

	// Build initial messages
	messages := s.buildAgentMessages(skill, variant, req)

	// If preprocess hook injected data, add it to context
	if fetchedData != nil {
		dataJSON, _ := json.MarshalIndent(fetchedData, "", "  ")
		messages = append(messages[:len(messages)-1], // Insert before user message
			LLMMessage{
				Role:    "system",
				Content: "Initial data:\n" + string(dataJSON),
			},
			messages[len(messages)-1], // User message
		)
	}

	// Agent loop
	var toolCalls []ToolCall
	var totalUsage TokenUsage
	for turn := 0; turn < s.config.MaxAgentTurns; turn++ {
		// Call LLM
		llmResult, err := s.llm.ChatCompletion(ChatCompletionContext{
			Model:          s.config.Model,
			Messages:       messages,
			Tools:          llmTools,
			ResponseFormat: s.buildResponseFormat(skill),
			Temperature:    s.config.Temperature,
		})
		if err != nil {
			return ChatResult{}, NewLLMError("chat completion failed", err)
		}

		totalUsage.PromptTokens += llmResult.Usage.PromptTokens
		totalUsage.CompletionTokens += llmResult.Usage.CompletionTokens
		totalUsage.TotalTokens += llmResult.Usage.TotalTokens

		// Check if LLM wants to call tools
		if len(llmResult.Message.ToolCalls) > 0 {
			// Execute tool calls
			messages = append(messages, llmResult.Message)
			for _, toolCall := range llmResult.Message.ToolCalls {
				result, tc, err := s.executeToolCall(ctx, toolCall, sources, actions)
				if err != nil {
					// Handle action confirmation requirement
					if err == ErrActionRequiresConfirmation {
						return ChatResult{
							MessageID:  NewMessageID(),
							Variant:    variant.Variant,
							ToolCalls:  toolCalls,
							TokensUsed: totalUsage,
							SuggestedAction: &SuggestedAction{
								Tool:   toolCall.Function.Name,
								Params: tc.Params,
								Reason: "Action requires user confirmation",
							},
						}, nil
					}
					return ChatResult{}, err
				}
				toolCalls = append(toolCalls, tc)
				messages = append(messages, LLMMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: toolCall.ID,
				})
			}
			continue
		}

		// LLM produced a final response
		response := json.RawMessage(llmResult.Message.Content)
		if err := s.validateResponse(skill, response); err != nil {
			return ChatResult{}, NewSchemaError("response validation failed", err)
		}

		// Run postprocessing hook if registered
		if postprocessHook, ok := s.hooks.GetPostprocess(skill.ID); ok {
			// Parse response for hook
			var responseMap map[string]any
			if err := json.Unmarshal(response, &responseMap); err != nil {
				responseMap = make(map[string]any)
			}

			postprocessReq := &PostprocessRequest{
				SkillID:    skill.ID,
				Message:    req.Message,
				Response:   responseMap,
				Data:       fetchedData,
				Context:    req.Context,
				Metadata:   metadata,
				Variant:    variant.Variant,
				TokensUsed: totalUsage,
			}
			if err := postprocessHook(ctx, postprocessReq); err != nil {
				return ChatResult{}, fmt.Errorf("postprocess hook failed: %w", err)
			}
			// Update response from hook (may have been modified)
			modifiedResponse, err := json.Marshal(postprocessReq.Response)
			if err != nil {
				return ChatResult{}, fmt.Errorf("failed to marshal modified response: %w", err)
			}
			response = modifiedResponse
		}

		return ChatResult{
			MessageID:  NewMessageID(),
			Variant:    variant.Variant,
			ToolCalls:  toolCalls,
			Response:   response,
			TokensUsed: totalUsage,
		}, nil
	}

	return ChatResult{}, NewSDKError(ErrCodeInternal, "max agent turns exceeded", ErrMaxTurnsExceeded)
}

// selectVariant chooses an A/B test variant for the skill.
func (s *SDK) selectVariant(skill *Skill, requestedVariant, userID string) SkillVariant {
	// Use requested variant if specified
	if requestedVariant != "" {
		for _, v := range skill.Variants {
			if v.Variant == requestedVariant {
				return v
			}
		}
	}

	// If no variants defined, return empty variant
	if len(skill.Variants) == 0 {
		return SkillVariant{Instructions: skill.Instructions}
	}

	// Weighted random selection (deterministic based on userID if provided)
	totalWeight := 0
	for _, v := range skill.Variants {
		totalWeight += v.Weight
	}

	seed := time.Now().UnixNano()
	if userID != "" {
		// Simple hash for sticky assignment
		for _, c := range userID {
			seed += int64(c)
		}
	}

	target := int(seed % int64(totalWeight))
	cumulative := 0
	for _, v := range skill.Variants {
		cumulative += v.Weight
		if target < cumulative {
			return v
		}
	}

	return skill.Variants[0]
}

// buildPrompt constructs the LLM prompt.
func (s *SDK) buildPrompt(skill *Skill, variant SkillVariant, message string, data any, reqCtx RequestContext) []LLMMessage {
	var messages []LLMMessage

	// System message with instructions
	instructions := variant.Instructions
	if instructions == "" {
		instructions = skill.Instructions
	}

	// Add guardrails
	if len(skill.Guardrails) > 0 {
		instructions += "\n\nRules:\n"
		for _, g := range skill.Guardrails {
			instructions += "- " + g + "\n"
		}
	}

	// Add context values if specified
	if len(skill.ContextInPrompt) > 0 && reqCtx != nil {
		instructions += "\n\nContext:\n"
		for _, key := range skill.ContextInPrompt {
			if val := reqCtx.String(key, ""); val != "" {
				instructions += fmt.Sprintf("- %s: %s\n", key, val)
			}
		}
	}

	// Add output format instructions
	if skill.Output != nil {
		instructions += "\n\nYou MUST respond with valid JSON matching this schema:\n"
		schemaJSON, _ := json.MarshalIndent(outputSchemaToJSONSchema(skill.Output), "", "  ")
		instructions += string(schemaJSON)
	}

	messages = append(messages, LLMMessage{
		Role:    "system",
		Content: instructions,
	})

	// Add examples as user/assistant pairs
	for _, ex := range skill.Examples {
		messages = append(messages,
			LLMMessage{Role: "user", Content: ex.User},
			LLMMessage{Role: "assistant", Content: ex.Assistant},
		)
	}

	// Add data context if provided
	if data != nil {
		dataJSON, _ := json.MarshalIndent(data, "", "  ")
		messages = append(messages, LLMMessage{
			Role:    "system",
			Content: "Available data:\n" + string(dataJSON),
		})
	}

	// Add user message
	messages = append(messages, LLMMessage{
		Role:    "user",
		Content: message,
	})

	return messages
}

// buildAgentMessages constructs messages for agentic mode.
func (s *SDK) buildAgentMessages(skill *Skill, variant SkillVariant, req ChatRequest) []LLMMessage {
	var messages []LLMMessage

	// System message
	instructions := variant.Instructions
	if instructions == "" {
		instructions = skill.Instructions
	}

	// Add guardrails
	if len(skill.Guardrails) > 0 {
		instructions += "\n\nRules:\n"
		for _, g := range skill.Guardrails {
			instructions += "- " + g + "\n"
		}
	}

	messages = append(messages, LLMMessage{
		Role:    "system",
		Content: instructions,
	})

	// Add examples
	for _, ex := range skill.Examples {
		messages = append(messages,
			LLMMessage{Role: "user", Content: ex.User},
			LLMMessage{Role: "assistant", Content: ex.Assistant},
		)
	}

	// Add user message
	messages = append(messages, LLMMessage{
		Role:    "user",
		Content: req.Message,
	})

	return messages
}

// buildResponseFormat creates the response format for structured output.
func (s *SDK) buildResponseFormat(skill *Skill) *LLMResponseFormat {
	if skill.Output == nil {
		return nil
	}

	// Use simple json_object format for broader compatibility
	// The output schema is still used for validation
	return &LLMResponseFormat{
		Type: "json_object",
	}
}

// buildLLMTools converts tools to LLM function format.
func (s *SDK) buildLLMTools(sources []*Source, actions []*Action) []LLMTool {
	var tools []LLMTool

	for _, src := range sources {
		tools = append(tools, LLMTool{
			Type: "function",
			Function: LLMFunction{
				Name:        src.Name,
				Description: src.Description,
				Parameters:  paramDefsToJSONSchema(src.Params),
			},
		})
	}

	for _, act := range actions {
		tools = append(tools, LLMTool{
			Type: "function",
			Function: LLMFunction{
				Name:        act.Name,
				Description: act.Description,
				Parameters:  paramDefsToJSONSchema(act.Params),
			},
		})
	}

	return tools
}

// executeToolCall executes a single tool call from the LLM.
func (s *SDK) executeToolCall(ctx context.Context, call LLMToolCall, sources []*Source, actions []*Action) (string, ToolCall, error) {
	start := time.Now()
	toolName := call.Function.Name

	// Parse arguments
	var params map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &params); err != nil {
		return "", ToolCall{}, NewToolError(toolName, fmt.Errorf("invalid arguments: %w", err))
	}

	tc := ToolCall{
		Name:   toolName,
		Params: params,
	}

	// Try sources first
	for _, src := range sources {
		if src.Name == toolName {
			input := newToolInput(params, src.Params)
			result, err := src.Fetch(ctx, input)
			tc.Duration = time.Since(start)
			if err != nil {
				return "", tc, NewToolError(toolName, err)
			}
			resultJSON, _ := json.Marshal(result)
			return string(resultJSON), tc, nil
		}
	}

	// Try actions
	for _, act := range actions {
		if act.Name == toolName {
			if act.RequiresConfirmation {
				tc.Duration = time.Since(start)
				return "", tc, ErrActionRequiresConfirmation
			}
			input := newToolInput(params, act.Params)
			result, err := act.Execute(ctx, input)
			tc.Duration = time.Since(start)
			if err != nil {
				return "", tc, NewToolError(toolName, err)
			}
			resultJSON, _ := json.Marshal(result)
			return string(resultJSON), tc, nil
		}
	}

	return "", tc, NewNotFoundError("tool", toolName)
}

// validateResponse checks if the response matches the schema.
func (s *SDK) validateResponse(skill *Skill, response json.RawMessage) error {
	if skill.Output == nil {
		return nil
	}

	// Basic validation: ensure it's valid JSON that can unmarshal to a map
	var data map[string]any
	if err := json.Unmarshal(response, &data); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}

	// Check required fields
	for _, required := range skill.Output.Required {
		if _, ok := data[required]; !ok {
			return fmt.Errorf("missing required field: %s", required)
		}
	}

	return nil
}

// loadConversationHistory retrieves previous messages.
func (s *SDK) loadConversationHistory(ctx context.Context, conversationID string) ([]Message, error) {
	if conversationID == "" {
		return nil, nil
	}
	return s.storage.GetMessages(ChatCompletionContext{}, conversationID, 20)
}

// storeMessages saves messages to storage.
func (s *SDK) storeMessages(ctx context.Context, conversationID string, req ChatRequest, result ChatResult) error {
	// Store user message
	userMsg := Message{
		ID:             NewMessageID(),
		ConversationID: conversationID,
		Role:           "user",
		Content:        req.Message,
		CreatedAt:      time.Now(),
	}
	if err := s.storage.AddMessage(ChatCompletionContext{}, conversationID, userMsg); err != nil {
		return err
	}

	// Store assistant message
	assistantMsg := Message{
		ID:             result.MessageID,
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        string(result.Response),
		SkillID:        result.SkillID,
		Variant:        result.Variant,
		CreatedAt:      time.Now(),
	}
	return s.storage.AddMessage(ChatCompletionContext{}, conversationID, assistantMsg)
}

// toolExecutorImpl implements ToolExecutor for expert fetchers.
type toolExecutorImpl struct {
	tools     ToolRegistry
	toolCalls *[]ToolCall
}

func (t *toolExecutorImpl) Execute(ctx context.Context, toolName string, params map[string]any) (any, error) {
	start := time.Now()

	source, ok := t.tools.GetSource(toolName)
	if !ok {
		return nil, NewNotFoundError("tool", toolName)
	}

	input := newToolInput(params, source.Params)
	result, err := source.Fetch(ctx, input)

	*t.toolCalls = append(*t.toolCalls, ToolCall{
		Name:     toolName,
		Params:   params,
		Duration: time.Since(start),
	})

	return result, err
}

// Helper functions

// outputSchemaToJSONSchema converts OutputSchema to JSON Schema format.
func outputSchemaToJSONSchema(schema *OutputSchema) map[string]any {
	properties := make(map[string]any)
	for name, prop := range schema.Properties {
		properties[name] = propertySchemaToJSONSchema(prop)
	}

	result := map[string]any{
		"type":       schema.Type,
		"properties": properties,
	}
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}
	result["additionalProperties"] = false

	return result
}

// propertySchemaToJSONSchema converts PropertySchema to JSON Schema format.
func propertySchemaToJSONSchema(prop PropertySchema) map[string]any {
	result := map[string]any{
		"type": prop.Type,
	}
	if prop.Description != "" {
		result["description"] = prop.Description
	}
	if prop.Items != nil {
		result["items"] = propertySchemaToJSONSchema(*prop.Items)
	}
	if len(prop.Properties) > 0 {
		props := make(map[string]any)
		for name, p := range prop.Properties {
			props[name] = propertySchemaToJSONSchema(p)
		}
		result["properties"] = props
		result["additionalProperties"] = false
	}
	if len(prop.Enum) > 0 {
		result["enum"] = prop.Enum
	}
	return result
}

// paramDefsToJSONSchema converts ParamDefinitions to JSON Schema format.
func paramDefsToJSONSchema(params ParamDefinitions) map[string]any {
	properties := make(map[string]any)
	var required []string

	for name, param := range params {
		prop := map[string]any{
			"type":        param.Type,
			"description": param.Description,
		}
		if len(param.EnumValues) > 0 {
			prop["enum"] = param.EnumValues
		}
		properties[name] = prop

		if param.Required {
			required = append(required, name)
		}
	}

	result := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		result["required"] = required
	}

	return result
}

// toolInput implements the Input interface.
type toolInput struct {
	params     map[string]any
	paramDefs  ParamDefinitions
}

func newToolInput(params map[string]any, defs ParamDefinitions) *toolInput {
	return &toolInput{params: params, paramDefs: defs}
}

func (i *toolInput) String(name string) string {
	return i.StringOr(name, "")
}

func (i *toolInput) StringOr(name, defaultValue string) string {
	if v, ok := i.params[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if def, ok := i.paramDefs[name]; ok && def.Default != nil {
		if s, ok := def.Default.(string); ok {
			return s
		}
	}
	return defaultValue
}

func (i *toolInput) Int(name string) int {
	return i.IntOr(name, 0)
}

func (i *toolInput) IntOr(name string, defaultValue int) int {
	if v, ok := i.params[name]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	if def, ok := i.paramDefs[name]; ok && def.Default != nil {
		if n, ok := def.Default.(int); ok {
			return n
		}
	}
	return defaultValue
}

func (i *toolInput) Bool(name string) bool {
	return i.BoolOr(name, false)
}

func (i *toolInput) BoolOr(name string, defaultValue bool) bool {
	if v, ok := i.params[name]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	if def, ok := i.paramDefs[name]; ok && def.Default != nil {
		if b, ok := def.Default.(bool); ok {
			return b
		}
	}
	return defaultValue
}

func (i *toolInput) Object(name string) map[string]any {
	if v, ok := i.params[name]; ok {
		if obj, ok := v.(map[string]any); ok {
			return obj
		}
	}
	return nil
}

func (i *toolInput) Array(name string) []any {
	if v, ok := i.params[name]; ok {
		if arr, ok := v.([]any); ok {
			return arr
		}
	}
	return nil
}

func (i *toolInput) Raw(name string) any {
	return i.params[name]
}

func (i *toolInput) Has(name string) bool {
	_, ok := i.params[name]
	return ok
}
