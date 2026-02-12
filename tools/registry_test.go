package tools

import (
	"context"
	"testing"
)

func TestRegistry(t *testing.T) {
	t.Run("registers and retrieves sources", func(t *testing.T) {
		registry := NewRegistry()

		registry.RegisterSource("get_product", Source{
			Description: "Get product details",
			Params: Params{
				"id": String("Product ID", true),
			},
			Fetch: func(ctx context.Context, p Input) (any, error) {
				return map[string]string{"id": p.String("id")}, nil
			},
		})

		source, ok := registry.GetSource("get_product")
		if !ok {
			t.Fatal("expected to find source")
		}
		if source.Description != "Get product details" {
			t.Errorf("expected description 'Get product details', got: %s", source.Description)
		}
	})

	t.Run("registers and retrieves actions", func(t *testing.T) {
		registry := NewRegistry()

		registry.RegisterAction("create_ticket", Action{
			Description:          "Create a support ticket",
			RequiresConfirmation: true,
			Params: Params{
				"subject": String("Ticket subject", true),
			},
			Execute: func(ctx context.Context, p Input) (any, error) {
				return map[string]string{"id": "TKT-123"}, nil
			},
		})

		action, ok := registry.GetAction("create_ticket")
		if !ok {
			t.Fatal("expected to find action")
		}
		if !action.RequiresConfirmation {
			t.Error("expected RequiresConfirmation to be true")
		}
	})

	t.Run("returns all sources and actions", func(t *testing.T) {
		registry := NewRegistry()

		registry.RegisterSource("source1", Source{Description: "Source 1"})
		registry.RegisterSource("source2", Source{Description: "Source 2"})
		registry.RegisterAction("action1", Action{Description: "Action 1"})

		sources := registry.AllSources()
		if len(sources) != 2 {
			t.Errorf("expected 2 sources, got: %d", len(sources))
		}

		actions := registry.AllActions()
		if len(actions) != 1 {
			t.Errorf("expected 1 action, got: %d", len(actions))
		}
	})

	t.Run("gets tools for skill", func(t *testing.T) {
		registry := NewRegistry()

		registry.RegisterSource("get_product", Source{Description: "Get product"})
		registry.RegisterSource("get_inventory", Source{Description: "Get inventory"})
		registry.RegisterAction("create_order", Action{Description: "Create order"})

		sources, actions, err := registry.GetForSkill([]string{"get_product", "create_order"})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if len(sources) != 1 {
			t.Errorf("expected 1 source, got: %d", len(sources))
		}
		if len(actions) != 1 {
			t.Errorf("expected 1 action, got: %d", len(actions))
		}
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		registry := NewRegistry()

		_, _, err := registry.GetForSkill([]string{"unknown_tool"})
		if err == nil {
			t.Fatal("expected error for unknown tool")
		}
	})
}

func TestParamDef(t *testing.T) {
	t.Run("creates string parameter", func(t *testing.T) {
		param := String("Test description", true)
		if param.paramType != "string" {
			t.Errorf("expected type 'string', got: %s", param.paramType)
		}
		if !param.required {
			t.Error("expected required to be true")
		}
		if param.description != "Test description" {
			t.Errorf("expected description 'Test description', got: %s", param.description)
		}
	})

	t.Run("creates string with default", func(t *testing.T) {
		param := StringWithDefault("Optional param", "default_value")
		if param.required {
			t.Error("expected required to be false")
		}
		if param.defaultVal != "default_value" {
			t.Errorf("expected default 'default_value', got: %v", param.defaultVal)
		}
	})

	t.Run("creates int parameter", func(t *testing.T) {
		param := Int("Count", true)
		if param.paramType != "integer" {
			t.Errorf("expected type 'integer', got: %s", param.paramType)
		}
	})

	t.Run("creates bool parameter", func(t *testing.T) {
		param := Bool("Enabled", false)
		if param.paramType != "boolean" {
			t.Errorf("expected type 'boolean', got: %s", param.paramType)
		}
	})

	t.Run("creates enum parameter", func(t *testing.T) {
		param := Enum("Priority", []string{"low", "medium", "high"}, true)
		if param.paramType != "string" {
			t.Errorf("expected type 'string', got: %s", param.paramType)
		}
		if len(param.enumValues) != 3 {
			t.Errorf("expected 3 enum values, got: %d", len(param.enumValues))
		}
	})

	t.Run("creates object parameter", func(t *testing.T) {
		param := Object("Data", true)
		if param.paramType != "object" {
			t.Errorf("expected type 'object', got: %s", param.paramType)
		}
	})

	t.Run("creates array parameter", func(t *testing.T) {
		param := Array("Items", false)
		if param.paramType != "array" {
			t.Errorf("expected type 'array', got: %s", param.paramType)
		}
	})
}

func TestInput(t *testing.T) {
	t.Run("extracts string values", func(t *testing.T) {
		input := NewInput(
			map[string]any{"name": "John", "city": "Stockholm"},
			Params{"name": String("Name", true)},
		)

		if input.String("name") != "John" {
			t.Errorf("expected 'John', got: %s", input.String("name"))
		}
		if input.StringOr("missing", "default") != "default" {
			t.Error("expected default value")
		}
	})

	t.Run("extracts int values", func(t *testing.T) {
		input := NewInput(
			map[string]any{"count": 42, "float": 3.14},
			nil,
		)

		if input.Int("count") != 42 {
			t.Errorf("expected 42, got: %d", input.Int("count"))
		}
		if input.Int("float") != 3 {
			t.Errorf("expected 3 (truncated), got: %d", input.Int("float"))
		}
		if input.IntOr("missing", 99) != 99 {
			t.Error("expected default value")
		}
	})

	t.Run("extracts bool values", func(t *testing.T) {
		input := NewInput(
			map[string]any{"enabled": true},
			nil,
		)

		if !input.Bool("enabled") {
			t.Error("expected true")
		}
		if input.BoolOr("missing", true) != true {
			t.Error("expected default value")
		}
	})

	t.Run("extracts object values", func(t *testing.T) {
		input := NewInput(
			map[string]any{"data": map[string]any{"key": "value"}},
			nil,
		)

		obj := input.Object("data")
		if obj == nil {
			t.Fatal("expected object")
		}
		if obj["key"] != "value" {
			t.Errorf("expected key 'value', got: %v", obj["key"])
		}
	})

	t.Run("extracts array values", func(t *testing.T) {
		input := NewInput(
			map[string]any{"items": []any{"a", "b", "c"}},
			nil,
		)

		arr := input.Array("items")
		if len(arr) != 3 {
			t.Errorf("expected 3 items, got: %d", len(arr))
		}
	})

	t.Run("checks if parameter exists", func(t *testing.T) {
		input := NewInput(
			map[string]any{"present": "value"},
			nil,
		)

		if !input.Has("present") {
			t.Error("expected present to exist")
		}
		if input.Has("missing") {
			t.Error("expected missing to not exist")
		}
	})

	t.Run("returns raw value", func(t *testing.T) {
		input := NewInput(
			map[string]any{"custom": []int{1, 2, 3}},
			nil,
		)

		raw := input.Raw("custom")
		if raw == nil {
			t.Fatal("expected raw value")
		}
	})
}
