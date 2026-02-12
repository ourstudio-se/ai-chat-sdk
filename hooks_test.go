package aichat

import (
	"context"
	"testing"
)

func TestHookRegistry_RegisterAndGet(t *testing.T) {
	registry := NewHookRegistry()

	// Test preprocess hook registration
	preprocessCalled := false
	registry.RegisterPreprocess("skill1", func(ctx context.Context, req *PreprocessRequest) error {
		preprocessCalled = true
		return nil
	})

	hook, ok := registry.GetPreprocess("skill1")
	if !ok {
		t.Error("expected to find preprocess hook")
	}
	if hook == nil {
		t.Error("expected non-nil preprocess hook")
	}

	// Call the hook to verify it's the right one
	err := hook(context.Background(), &PreprocessRequest{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !preprocessCalled {
		t.Error("expected preprocess hook to be called")
	}

	// Test postprocess hook registration
	postprocessCalled := false
	registry.RegisterPostprocess("skill1", func(ctx context.Context, req *PostprocessRequest) error {
		postprocessCalled = true
		return nil
	})

	postHook, ok := registry.GetPostprocess("skill1")
	if !ok {
		t.Error("expected to find postprocess hook")
	}
	if postHook == nil {
		t.Error("expected non-nil postprocess hook")
	}

	err = postHook(context.Background(), &PostprocessRequest{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !postprocessCalled {
		t.Error("expected postprocess hook to be called")
	}
}

func TestHookRegistry_GetNonExistent(t *testing.T) {
	registry := NewHookRegistry()

	_, ok := registry.GetPreprocess("nonexistent")
	if ok {
		t.Error("expected not to find preprocess hook")
	}

	_, ok = registry.GetPostprocess("nonexistent")
	if ok {
		t.Error("expected not to find postprocess hook")
	}
}

func TestHookRegistry_FluentAPI(t *testing.T) {
	registry := NewHookRegistry().
		WithPreprocess("skill1", func(ctx context.Context, req *PreprocessRequest) error {
			return nil
		}).
		WithPostprocess("skill1", func(ctx context.Context, req *PostprocessRequest) error {
			return nil
		}).
		WithPreprocess("skill2", func(ctx context.Context, req *PreprocessRequest) error {
			return nil
		})

	if _, ok := registry.GetPreprocess("skill1"); !ok {
		t.Error("expected skill1 preprocess hook")
	}
	if _, ok := registry.GetPostprocess("skill1"); !ok {
		t.Error("expected skill1 postprocess hook")
	}
	if _, ok := registry.GetPreprocess("skill2"); !ok {
		t.Error("expected skill2 preprocess hook")
	}
}

func TestPreprocessRequest_DataModification(t *testing.T) {
	registry := NewHookRegistry()

	// Hook that modifies data
	registry.RegisterPreprocess("skill1", func(ctx context.Context, req *PreprocessRequest) error {
		// Modify data
		req.Data = map[string]any{"modified": true}
		req.Metadata["key"] = "value"
		return nil
	})

	hook, _ := registry.GetPreprocess("skill1")
	req := &PreprocessRequest{
		SkillID:  "skill1",
		Message:  "test message",
		Data:     map[string]any{"original": true},
		Metadata: make(map[string]any),
	}

	err := hook(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify data was modified
	data, ok := req.Data.(map[string]any)
	if !ok {
		t.Error("expected data to be map[string]any")
	}
	if _, ok := data["modified"]; !ok {
		t.Error("expected data to have 'modified' key")
	}

	// Verify metadata was set
	if req.Metadata["key"] != "value" {
		t.Error("expected metadata to have 'key' set to 'value'")
	}
}

func TestPostprocessRequest_ResponseModification(t *testing.T) {
	registry := NewHookRegistry()

	// Hook that modifies response
	registry.RegisterPostprocess("skill1", func(ctx context.Context, req *PostprocessRequest) error {
		req.Response["enriched"] = true
		req.Response["tokens"] = req.TokensUsed.TotalTokens
		return nil
	})

	hook, _ := registry.GetPostprocess("skill1")
	req := &PostprocessRequest{
		SkillID:  "skill1",
		Message:  "test message",
		Response: map[string]any{"answer": "original answer"},
		Metadata: make(map[string]any),
		Variant:  "test-variant",
		TokensUsed: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	err := hook(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify response was modified
	if req.Response["enriched"] != true {
		t.Error("expected response to have 'enriched' set to true")
	}
	if req.Response["tokens"] != 150 {
		t.Errorf("expected tokens to be 150, got %v", req.Response["tokens"])
	}
	// Verify original fields are preserved
	if req.Response["answer"] != "original answer" {
		t.Error("expected original answer to be preserved")
	}
}
