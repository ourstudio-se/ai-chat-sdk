package tools

// Builder provides a fluent API for creating tools
type Builder struct {
	tool *Tool
}

// NewTool starts building a new tool
func NewTool(name string) *Builder {
	return &Builder{
		tool: &Tool{
			Name:       name,
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}},
		},
	}
}

// Description sets the tool description
func (b *Builder) Description(desc string) *Builder {
	b.tool.Description = desc
	return b
}

// StringParam adds a string parameter
func (b *Builder) StringParam(name, description string, required bool) *Builder {
	props := b.tool.Parameters["properties"].(map[string]any)
	props[name] = map[string]any{"type": "string", "description": description}
	if required {
		req := b.tool.Parameters["required"].([]string)
		b.tool.Parameters["required"] = append(req, name)
	}
	return b
}

// IntParam adds an integer parameter
func (b *Builder) IntParam(name, description string, required bool) *Builder {
	props := b.tool.Parameters["properties"].(map[string]any)
	props[name] = map[string]any{"type": "integer", "description": description}
	if required {
		req := b.tool.Parameters["required"].([]string)
		b.tool.Parameters["required"] = append(req, name)
	}
	return b
}

// EnumParam adds an enum parameter
func (b *Builder) EnumParam(name, description string, values []string, required bool) *Builder {
	props := b.tool.Parameters["properties"].(map[string]any)
	props[name] = map[string]any{"type": "string", "description": description, "enum": values}
	if required {
		req := b.tool.Parameters["required"].([]string)
		b.tool.Parameters["required"] = append(req, name)
	}
	return b
}

// ArrayParam adds an array of strings parameter
func (b *Builder) ArrayParam(name, description string, required bool) *Builder {
	props := b.tool.Parameters["properties"].(map[string]any)
	props[name] = map[string]any{
		"type":        "array",
		"description": description,
		"items":       map[string]any{"type": "string"},
	}
	if required {
		req := b.tool.Parameters["required"].([]string)
		b.tool.Parameters["required"] = append(req, name)
	}
	return b
}

// BoolParam adds a boolean parameter
func (b *Builder) BoolParam(name, description string, required bool) *Builder {
	props := b.tool.Parameters["properties"].(map[string]any)
	props[name] = map[string]any{"type": "boolean", "description": description}
	if required {
		req := b.tool.Parameters["required"].([]string)
		b.tool.Parameters["required"] = append(req, name)
	}
	return b
}

// Handler sets the tool handler
func (b *Builder) Handler(h Handler) *Builder {
	b.tool.Handler = h
	return b
}

// Build returns the completed tool
func (b *Builder) Build() *Tool {
	return b.tool
}
