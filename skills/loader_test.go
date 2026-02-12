package skills

import (
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("parses valid skill YAML", func(t *testing.T) {
		yaml := `
id: test
name: Test Skill
triggers:
  - test
  - example
intents:
  - testing
tools:
  - get_data
instructions: |
  You are a test assistant.
  Be helpful.
examples:
  - user: "Hello"
    assistant: '{"answer": "Hi there!"}'
guardrails:
  - Be polite
  - Be accurate
output:
  type: object
  properties:
    answer:
      type: string
      description: The response
    count:
      type: integer
  required:
    - answer
variants:
  - variant: v1
    weight: 50
    instructions: Be friendly
  - variant: v2
    weight: 50
    instructions: Be professional
`
		skill, err := Parse([]byte(yaml))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if skill.ID != "test" {
			t.Errorf("expected ID 'test', got: %s", skill.ID)
		}
		if skill.Name != "Test Skill" {
			t.Errorf("expected name 'Test Skill', got: %s", skill.Name)
		}
		if len(skill.Triggers) != 2 {
			t.Errorf("expected 2 triggers, got: %d", len(skill.Triggers))
		}
		if len(skill.Intents) != 1 {
			t.Errorf("expected 1 intent, got: %d", len(skill.Intents))
		}
		if len(skill.Tools) != 1 {
			t.Errorf("expected 1 tool, got: %d", len(skill.Tools))
		}
		if len(skill.Examples) != 1 {
			t.Errorf("expected 1 example, got: %d", len(skill.Examples))
		}
		if len(skill.Guardrails) != 2 {
			t.Errorf("expected 2 guardrails, got: %d", len(skill.Guardrails))
		}
		if skill.Output == nil {
			t.Fatal("expected output schema to be set")
		}
		if skill.Output.Type != "object" {
			t.Errorf("expected output type 'object', got: %s", skill.Output.Type)
		}
		if len(skill.Output.Properties) != 2 {
			t.Errorf("expected 2 properties, got: %d", len(skill.Output.Properties))
		}
		if len(skill.Output.Required) != 1 {
			t.Errorf("expected 1 required field, got: %d", len(skill.Output.Required))
		}
		if len(skill.Variants) != 2 {
			t.Errorf("expected 2 variants, got: %d", len(skill.Variants))
		}
	})

	t.Run("returns error for missing ID", func(t *testing.T) {
		yaml := `
name: Test Skill
instructions: Test
`
		_, err := Parse([]byte(yaml))
		if err == nil {
			t.Fatal("expected error for missing ID")
		}
	})

	t.Run("parses nested output schema", func(t *testing.T) {
		yaml := `
id: nested
output:
  type: object
  properties:
    items:
      type: array
      items:
        type: object
        properties:
          code:
            type: string
          count:
            type: integer
    metadata:
      type: object
      properties:
        total:
          type: integer
`
		skill, err := Parse([]byte(yaml))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if skill.Output == nil {
			t.Fatal("expected output schema")
		}

		items := skill.Output.Properties["items"]
		if items.Type != "array" {
			t.Errorf("expected items type 'array', got: %s", items.Type)
		}
		if items.Items == nil {
			t.Fatal("expected items schema")
		}
		if items.Items.Type != "object" {
			t.Errorf("expected items.items type 'object', got: %s", items.Items.Type)
		}

		metadata := skill.Output.Properties["metadata"]
		if metadata.Type != "object" {
			t.Errorf("expected metadata type 'object', got: %s", metadata.Type)
		}
		if len(metadata.Properties) != 1 {
			t.Errorf("expected 1 metadata property, got: %d", len(metadata.Properties))
		}
	})

	t.Run("parses execution mode", func(t *testing.T) {
		yaml := `
id: agentic_skill
mode: agentic
instructions: Test
`
		skill, err := Parse([]byte(yaml))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if skill.Mode != "agentic" {
			t.Errorf("expected mode 'agentic', got: %s", skill.Mode)
		}
	})

	t.Run("parses context_in_prompt", func(t *testing.T) {
		yaml := `
id: contextual
context_in_prompt:
  - market
  - locale
  - userId
instructions: Test
`
		skill, err := Parse([]byte(yaml))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if len(skill.ContextInPrompt) != 3 {
			t.Errorf("expected 3 context keys, got: %d", len(skill.ContextInPrompt))
		}
	})
}

func TestRegistry(t *testing.T) {
	t.Run("registers and retrieves skills", func(t *testing.T) {
		registry := NewRegistry()

		yaml1 := `
id: skill1
name: Skill One
triggers: [one]
`
		skill1, _ := Parse([]byte(yaml1))
		registry.Register(skill1)

		yaml2 := `
id: skill2
name: Skill Two
triggers: [two]
`
		skill2, _ := Parse([]byte(yaml2))
		registry.Register(skill2)

		// Get by ID
		got, ok := registry.Get("skill1")
		if !ok {
			t.Fatal("expected to find skill1")
		}
		if got.Name != "Skill One" {
			t.Errorf("expected name 'Skill One', got: %s", got.Name)
		}

		// Get all
		all := registry.All()
		if len(all) != 2 {
			t.Errorf("expected 2 skills, got: %d", len(all))
		}
	})

	t.Run("matches skills by triggers", func(t *testing.T) {
		registry := NewRegistry()

		yaml := `
id: product
name: Product Skill
triggers: [product, price, buy]
intents: [shopping]
`
		skill, _ := Parse([]byte(yaml))
		registry.Register(skill)

		// Match by trigger
		matches := registry.Match("I want to buy a product")
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got: %d", len(matches))
		}
		if matches[0].ID != "product" {
			t.Errorf("expected product skill, got: %s", matches[0].ID)
		}

		// Match by intent
		matches = registry.Match("I need help shopping")
		if len(matches) != 1 {
			t.Errorf("expected 1 match for shopping intent")
		}

		// No match
		matches = registry.Match("hello world")
		if len(matches) != 0 {
			t.Errorf("expected no matches, got: %d", len(matches))
		}
	})
}
