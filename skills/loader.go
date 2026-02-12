package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	aichat "github.com/ourstudio-se/ai-chat-sdk"
	"gopkg.in/yaml.v3"
)

// skillYAML is the YAML structure for skill definitions.
type skillYAML struct {
	ID              string        `yaml:"id"`
	Name            string        `yaml:"name"`
	Triggers        []string      `yaml:"triggers"`
	Intents         []string      `yaml:"intents"`
	Tools           []string      `yaml:"tools"`
	Instructions    string        `yaml:"instructions"`
	Examples        []exampleYAML `yaml:"examples"`
	Guardrails      []string      `yaml:"guardrails"`
	Output          *outputYAML   `yaml:"output"`
	Variants        []variantYAML `yaml:"variants"`
	Mode            string        `yaml:"mode"`
	ContextInPrompt []string      `yaml:"context_in_prompt"`
}

type exampleYAML struct {
	User      string `yaml:"user"`
	Assistant string `yaml:"assistant"`
}

type variantYAML struct {
	Variant      string `yaml:"variant"`
	Weight       int    `yaml:"weight"`
	Instructions string `yaml:"instructions"`
}

type outputYAML struct {
	Type       string                  `yaml:"type"`
	Properties map[string]propertyYAML `yaml:"properties"`
	Required   []string                `yaml:"required"`
}

type propertyYAML struct {
	Type        string                  `yaml:"type"`
	Description string                  `yaml:"description,omitempty"`
	Nullable    bool                    `yaml:"nullable,omitempty"`
	Items       *propertyYAML           `yaml:"items,omitempty"`
	Properties  map[string]propertyYAML `yaml:"properties,omitempty"`
	Enum        []string                `yaml:"enum,omitempty"`
}

// LoadDir loads all skill definitions from a directory.
func LoadDir(dir string) (*Registry, error) {
	registry := NewRegistry()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		skill, err := LoadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load skill %s: %w", name, err)
		}

		registry.Register(skill)
	}

	return registry, nil
}

// LoadFile loads a single skill definition from a file.
func LoadFile(path string) (*aichat.Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return Parse(data)
}

// Parse parses skill YAML content into a Skill.
func Parse(data []byte) (*aichat.Skill, error) {
	var raw skillYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if raw.ID == "" {
		return nil, fmt.Errorf("skill ID is required")
	}

	skill := &aichat.Skill{
		ID:              raw.ID,
		Name:            raw.Name,
		Triggers:        raw.Triggers,
		Intents:         raw.Intents,
		Tools:           raw.Tools,
		Instructions:    raw.Instructions,
		Guardrails:      raw.Guardrails,
		ContextInPrompt: raw.ContextInPrompt,
	}

	// Convert examples
	for _, ex := range raw.Examples {
		skill.Examples = append(skill.Examples, aichat.SkillExample{
			User:      ex.User,
			Assistant: ex.Assistant,
		})
	}

	// Convert variants
	for _, v := range raw.Variants {
		skill.Variants = append(skill.Variants, aichat.SkillVariant{
			Variant:      v.Variant,
			Weight:       v.Weight,
			Instructions: v.Instructions,
		})
	}

	// Convert output schema
	if raw.Output != nil {
		skill.Output = convertOutputSchema(raw.Output)
	}

	// Convert mode
	if raw.Mode != "" {
		skill.Mode = aichat.ExecutionMode(raw.Mode)
	}

	return skill, nil
}

// convertOutputSchema converts YAML output schema to aichat.OutputSchema.
func convertOutputSchema(raw *outputYAML) *aichat.OutputSchema {
	schema := &aichat.OutputSchema{
		Type:       raw.Type,
		Required:   raw.Required,
		Properties: make(map[string]aichat.PropertySchema),
	}

	for name, prop := range raw.Properties {
		schema.Properties[name] = convertPropertySchema(prop)
	}

	return schema
}

// convertPropertySchema converts YAML property schema to aichat.PropertySchema.
func convertPropertySchema(raw propertyYAML) aichat.PropertySchema {
	prop := aichat.PropertySchema{
		Type:        raw.Type,
		Description: raw.Description,
		Nullable:    raw.Nullable,
		Enum:        raw.Enum,
	}

	if raw.Items != nil {
		items := convertPropertySchema(*raw.Items)
		prop.Items = &items
	}

	if len(raw.Properties) > 0 {
		prop.Properties = make(map[string]aichat.PropertySchema)
		for name, p := range raw.Properties {
			prop.Properties[name] = convertPropertySchema(p)
		}
	}

	return prop
}
