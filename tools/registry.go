package tools

import (
	"context"
	"fmt"
	"sync"

	aichat "github.com/ourstudio-se/ai-chat-sdk"
)

// Registry manages registered tools.
type Registry struct {
	mu      sync.RWMutex
	sources map[string]*aichat.Source
	actions map[string]*aichat.Action
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		sources: make(map[string]*aichat.Source),
		actions: make(map[string]*aichat.Action),
	}
}

// RegisterSource adds a source tool to the registry.
func (r *Registry) RegisterSource(name string, src Source) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sources[name] = &aichat.Source{
		Name:        name,
		Description: src.Description,
		Params:      toParamDefinitions(src.Params),
		Fetch:       toFetchFn(src.Fetch),
	}
}

// RegisterAction adds an action tool to the registry.
func (r *Registry) RegisterAction(name string, act Action) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.actions[name] = &aichat.Action{
		Name:                 name,
		Description:          act.Description,
		Params:               toParamDefinitions(act.Params),
		Execute:              toExecuteFn(act.Execute),
		RequiresConfirmation: act.RequiresConfirmation,
	}
}

// GetSource returns a source tool by name.
func (r *Registry) GetSource(name string) (*aichat.Source, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	src, ok := r.sources[name]
	return src, ok
}

// GetAction returns an action tool by name.
func (r *Registry) GetAction(name string) (*aichat.Action, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	act, ok := r.actions[name]
	return act, ok
}

// AllSources returns all registered source tools.
func (r *Registry) AllSources() []*aichat.Source {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sources := make([]*aichat.Source, 0, len(r.sources))
	for _, src := range r.sources {
		sources = append(sources, src)
	}
	return sources
}

// AllActions returns all registered action tools.
func (r *Registry) AllActions() []*aichat.Action {
	r.mu.RLock()
	defer r.mu.RUnlock()

	actions := make([]*aichat.Action, 0, len(r.actions))
	for _, act := range r.actions {
		actions = append(actions, act)
	}
	return actions
}

// GetForSkill returns tools available to a skill by name.
func (r *Registry) GetForSkill(toolNames []string) (sources []*aichat.Source, actions []*aichat.Action, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range toolNames {
		if src, ok := r.sources[name]; ok {
			sources = append(sources, src)
			continue
		}
		if act, ok := r.actions[name]; ok {
			actions = append(actions, act)
			continue
		}
		return nil, nil, fmt.Errorf("%w: %s", aichat.ErrToolNotFound, name)
	}

	return sources, actions, nil
}

// Source defines a read-only data source tool (user-facing API).
type Source struct {
	Description string
	Params      Params
	Fetch       func(ctx context.Context, p Input) (any, error)
}

// Action defines a tool that performs side effects (user-facing API).
type Action struct {
	Description          string
	Params               Params
	Execute              func(ctx context.Context, p Input) (any, error)
	RequiresConfirmation bool
}

// Params is a map of parameter definitions.
type Params map[string]ParamDef

// ParamDef defines a tool parameter.
type ParamDef struct {
	description string
	paramType   string
	required    bool
	enumValues  []string
	defaultVal  any
}

// String creates a string parameter definition.
func String(description string, required bool) ParamDef {
	return ParamDef{
		description: description,
		paramType:   "string",
		required:    required,
	}
}

// StringWithDefault creates a string parameter with a default value.
func StringWithDefault(description string, defaultValue string) ParamDef {
	return ParamDef{
		description: description,
		paramType:   "string",
		required:    false,
		defaultVal:  defaultValue,
	}
}

// Int creates an integer parameter definition.
func Int(description string, required bool) ParamDef {
	return ParamDef{
		description: description,
		paramType:   "integer",
		required:    required,
	}
}

// IntWithDefault creates an integer parameter with a default value.
func IntWithDefault(description string, defaultValue int) ParamDef {
	return ParamDef{
		description: description,
		paramType:   "integer",
		required:    false,
		defaultVal:  defaultValue,
	}
}

// Bool creates a boolean parameter definition.
func Bool(description string, required bool) ParamDef {
	return ParamDef{
		description: description,
		paramType:   "boolean",
		required:    required,
	}
}

// BoolWithDefault creates a boolean parameter with a default value.
func BoolWithDefault(description string, defaultValue bool) ParamDef {
	return ParamDef{
		description: description,
		paramType:   "boolean",
		required:    false,
		defaultVal:  defaultValue,
	}
}

// Object creates an object parameter definition.
func Object(description string, required bool) ParamDef {
	return ParamDef{
		description: description,
		paramType:   "object",
		required:    required,
	}
}

// Array creates an array parameter definition.
func Array(description string, required bool) ParamDef {
	return ParamDef{
		description: description,
		paramType:   "array",
		required:    required,
	}
}

// Enum creates an enum parameter definition.
func Enum(description string, values []string, required bool) ParamDef {
	return ParamDef{
		description: description,
		paramType:   "string",
		required:    required,
		enumValues:  values,
	}
}

// Input provides type-safe access to tool parameters.
type Input struct {
	params    map[string]any
	paramDefs Params
}

// NewInput creates an Input from raw parameters.
func NewInput(params map[string]any, defs Params) Input {
	return Input{params: params, paramDefs: defs}
}

// String returns a string parameter value.
func (i Input) String(name string) string {
	return i.StringOr(name, "")
}

// StringOr returns a string parameter value with a default.
func (i Input) StringOr(name, defaultValue string) string {
	if v, ok := i.params[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if def, ok := i.paramDefs[name]; ok && def.defaultVal != nil {
		if s, ok := def.defaultVal.(string); ok {
			return s
		}
	}
	return defaultValue
}

// Int returns an int parameter value.
func (i Input) Int(name string) int {
	return i.IntOr(name, 0)
}

// IntOr returns an int parameter value with a default.
func (i Input) IntOr(name string, defaultValue int) int {
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
	if def, ok := i.paramDefs[name]; ok && def.defaultVal != nil {
		if n, ok := def.defaultVal.(int); ok {
			return n
		}
	}
	return defaultValue
}

// Bool returns a bool parameter value.
func (i Input) Bool(name string) bool {
	return i.BoolOr(name, false)
}

// BoolOr returns a bool parameter value with a default.
func (i Input) BoolOr(name string, defaultValue bool) bool {
	if v, ok := i.params[name]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	if def, ok := i.paramDefs[name]; ok && def.defaultVal != nil {
		if b, ok := def.defaultVal.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// Object returns an object parameter value.
func (i Input) Object(name string) map[string]any {
	if v, ok := i.params[name]; ok {
		if obj, ok := v.(map[string]any); ok {
			return obj
		}
	}
	return nil
}

// Array returns an array parameter value.
func (i Input) Array(name string) []any {
	if v, ok := i.params[name]; ok {
		if arr, ok := v.([]any); ok {
			return arr
		}
	}
	return nil
}

// Raw returns the raw parameter value.
func (i Input) Raw(name string) any {
	return i.params[name]
}

// Has checks if a parameter was provided.
func (i Input) Has(name string) bool {
	_, ok := i.params[name]
	return ok
}

// toParamDefinitions converts user-facing Params to aichat.ParamDefinitions.
func toParamDefinitions(params Params) aichat.ParamDefinitions {
	defs := make(aichat.ParamDefinitions)
	for name, p := range params {
		defs[name] = aichat.ParamDef{
			Type:        p.paramType,
			Description: p.description,
			Required:    p.required,
			EnumValues:  p.enumValues,
			Default:     p.defaultVal,
		}
	}
	return defs
}

// toFetchFn converts user-facing fetch function to aichat.FetchFn.
func toFetchFn(fn func(ctx context.Context, p Input) (any, error)) aichat.FetchFn {
	return func(ctx context.Context, params aichat.Input) (any, error) {
		// Convert aichat.Input to tools.Input
		input := Input{
			params:    extractParams(params),
			paramDefs: nil,
		}
		return fn(ctx, input)
	}
}

// toExecuteFn converts user-facing execute function to aichat.ExecuteFn.
func toExecuteFn(fn func(ctx context.Context, p Input) (any, error)) aichat.ExecuteFn {
	return func(ctx context.Context, params aichat.Input) (any, error) {
		input := Input{
			params:    extractParams(params),
			paramDefs: nil,
		}
		return fn(ctx, input)
	}
}

// extractParams extracts raw params from aichat.Input.
func extractParams(input aichat.Input) map[string]any {
	// This is a workaround - in practice the Input interface
	// should provide a way to get all params
	return nil
}
