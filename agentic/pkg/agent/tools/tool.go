package tools

import "context"

// Tool defines a capability the agent can use
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
	Handler     Handler        `json:"-"`
}

// Handler executes a tool and returns the result
type Handler func(ctx context.Context, input map[string]any) (any, error)

// Definition returns the tool definition without the handler (for LLM)
type Definition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToDefinition converts a Tool to a Definition
func (t *Tool) ToDefinition() Definition {
	return Definition{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  t.Parameters,
	}
}
