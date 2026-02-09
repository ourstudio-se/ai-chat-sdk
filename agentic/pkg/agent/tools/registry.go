package tools

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages available tools
type Registry struct {
	tools map[string]*Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

// Get returns a tool by name
func (r *Registry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns all tool definitions (for LLM)
func (r *Registry) Definitions() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]Definition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.ToDefinition())
	}
	return defs
}

// Execute runs a tool by name with the given input
func (r *Registry) Execute(ctx context.Context, name string, input map[string]any) (any, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	return tool.Handler(ctx, input)
}

// Names returns all registered tool names
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ParamInfo describes a tool parameter
type ParamInfo struct {
	Name        string
	Description string
	Required    bool
	ToolName    string // Which tool uses this parameter
}

// Parameters returns all unique parameter names across all tools with their info
func (r *Registry) Parameters() map[string]ParamInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	params := make(map[string]ParamInfo)
	for _, tool := range r.tools {
		props, ok := tool.Parameters["properties"].(map[string]any)
		if !ok {
			continue
		}
		required := make(map[string]bool)
		if reqList, ok := tool.Parameters["required"].([]string); ok {
			for _, r := range reqList {
				required[r] = true
			}
		}

		for paramName, paramSchema := range props {
			schema, ok := paramSchema.(map[string]any)
			if !ok {
				continue
			}
			desc, _ := schema["description"].(string)

			// Only add if not already present, or if this one is required
			existing, exists := params[paramName]
			if !exists || (required[paramName] && !existing.Required) {
				params[paramName] = ParamInfo{
					Name:        paramName,
					Description: desc,
					Required:    required[paramName],
					ToolName:    tool.Name,
				}
			}
		}
	}
	return params
}
