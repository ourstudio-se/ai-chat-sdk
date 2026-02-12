package aichat

import (
	"context"
)

// PreprocessHook is called before the LLM call.
// It can modify the request data or add additional context.
type PreprocessHook func(ctx context.Context, req *PreprocessRequest) error

// PostprocessHook is called after the LLM call.
// It can modify the response or perform additional actions.
type PostprocessHook func(ctx context.Context, req *PostprocessRequest) error

// PreprocessRequest contains data available during preprocessing.
type PreprocessRequest struct {
	// SkillID is the skill being executed.
	SkillID string

	// Message is the user's message.
	Message string

	// Data is the fetched data (can be modified by the hook).
	Data any

	// Context is the request context (can be modified by the hook).
	Context RequestContext

	// Metadata allows passing data between pre and post hooks.
	Metadata map[string]any
}

// PostprocessRequest contains data available during postprocessing.
type PostprocessRequest struct {
	// SkillID is the skill that was executed.
	SkillID string

	// Message is the user's message.
	Message string

	// Response is the LLM response (can be modified by the hook).
	Response map[string]any

	// Data is the original fetched data.
	Data any

	// Context is the request context.
	Context RequestContext

	// Metadata from preprocessing.
	Metadata map[string]any

	// Variant is the A/B test variant used.
	Variant string

	// TokensUsed is the token usage from the LLM call.
	TokensUsed TokenUsage
}

// HookRegistry manages pre/post processing hooks.
type HookRegistry struct {
	preprocess  map[string]PreprocessHook
	postprocess map[string]PostprocessHook
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		preprocess:  make(map[string]PreprocessHook),
		postprocess: make(map[string]PostprocessHook),
	}
}

// RegisterPreprocess registers a preprocessing hook for a skill.
func (r *HookRegistry) RegisterPreprocess(skillID string, hook PreprocessHook) {
	r.preprocess[skillID] = hook
}

// RegisterPostprocess registers a postprocessing hook for a skill.
func (r *HookRegistry) RegisterPostprocess(skillID string, hook PostprocessHook) {
	r.postprocess[skillID] = hook
}

// GetPreprocess returns the preprocessing hook for a skill.
func (r *HookRegistry) GetPreprocess(skillID string) (PreprocessHook, bool) {
	hook, ok := r.preprocess[skillID]
	return hook, ok
}

// GetPostprocess returns the postprocessing hook for a skill.
func (r *HookRegistry) GetPostprocess(skillID string) (PostprocessHook, bool) {
	hook, ok := r.postprocess[skillID]
	return hook, ok
}

// WithPreprocess is a fluent method to register a preprocessing hook.
func (r *HookRegistry) WithPreprocess(skillID string, hook PreprocessHook) *HookRegistry {
	r.RegisterPreprocess(skillID, hook)
	return r
}

// WithPostprocess is a fluent method to register a postprocessing hook.
func (r *HookRegistry) WithPostprocess(skillID string, hook PostprocessHook) *HookRegistry {
	r.RegisterPostprocess(skillID, hook)
	return r
}
