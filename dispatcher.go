package aichat

import (
	"context"
	"fmt"
	"log/slog"
)

// NewDispatcher creates a dispatcher function that routes and processes questions.
func NewDispatcher(
	routeQuestion RouteQuestionFn,
	experts map[ExpertType]Expert,
	defaultExpert ExpertType,
	logger *slog.Logger,
) DispatchQuestionFn {
	return func(ctx context.Context, req ExpertRequest) (*ExpertResult, error) {
		// 1. Route to expert
		routeResult, err := routeQuestion(ctx, req.Message, req.EntityID)
		if err != nil {
			return nil, fmt.Errorf("failed to route question: %w", err)
		}

		logger.Debug("question routed",
			"expert_type", string(routeResult.Expert),
			"expert_name", routeResult.ExpertName,
		)

		// 2. Get expert implementation
		expert, exists := experts[routeResult.Expert]
		if !exists {
			// Try default expert
			if defaultExpert != "" {
				expert, exists = experts[defaultExpert]
			}

			if !exists {
				logger.Warn("expert not found, returning routing reasoning",
					"expert_type", string(routeResult.Expert),
				)
				// Fallback: return routing reasoning
				return &ExpertResult{
					ExpertType: routeResult.Expert,
					ExpertName: routeResult.ExpertName,
					Answer:     routeResult.Reasoning,
					Reasoning:  routeResult.Reasoning,
				}, nil
			}
		}

		// 3. Process with expert
		req.RoutingReasoning = routeResult.Reasoning
		result, err := expert.Handler(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("expert processing failed: %w", err)
		}

		// 4. Enrich with routing metadata
		result.ExpertType = routeResult.Expert
		result.ExpertName = routeResult.ExpertName
		result.Reasoning = routeResult.Reasoning

		return result, nil
	}
}

// NewDispatcherStreaming creates a streaming dispatcher that sends events during processing.
func NewDispatcherStreaming(
	routeQuestion RouteQuestionFn,
	experts map[ExpertType]Expert,
	defaultExpert ExpertType,
	logger *slog.Logger,
) DispatchQuestionStreamFn {
	return func(ctx context.Context, req ExpertRequest, stream StreamCallback) (*ExpertResult, error) {
		// 1. Route to expert
		routeResult, err := routeQuestion(ctx, req.Message, req.EntityID)
		if err != nil {
			return nil, fmt.Errorf("failed to route question: %w", err)
		}

		logger.Debug("question routed",
			"expert_type", string(routeResult.Expert),
			"expert_name", routeResult.ExpertName,
		)

		// Send routing event
		expertType := routeResult.Expert
		stream(StreamEvent{
			Type:       EventRouting,
			Expert:     &expertType,
			ExpertName: &routeResult.ExpertName,
		})

		// 2. Get expert implementation
		expert, exists := experts[routeResult.Expert]
		if !exists {
			if defaultExpert != "" {
				expert, exists = experts[defaultExpert]
			}

			if !exists {
				logger.Warn("expert not found, returning routing reasoning",
					"expert_type", string(routeResult.Expert),
				)
				return &ExpertResult{
					ExpertType: routeResult.Expert,
					ExpertName: routeResult.ExpertName,
					Answer:     routeResult.Reasoning,
					Reasoning:  routeResult.Reasoning,
				}, nil
			}
		}

		// Send processing event
		stream(StreamEvent{
			Type:       EventProcessing,
			Expert:     &expertType,
			ExpertName: &routeResult.ExpertName,
		})

		// 3. Process with expert (use streaming handler if available)
		req.RoutingReasoning = routeResult.Reasoning

		var result *ExpertResult
		if expert.StreamHandler != nil {
			result, err = expert.StreamHandler(ctx, req, stream)
		} else {
			// Fallback to non-streaming handler
			result, err = expert.Handler(ctx, req)
			// Send the full content as a single chunk
			if err == nil && result != nil {
				stream(StreamEvent{
					Type:    EventContent,
					Content: &result.Answer,
				})
			}
		}

		if err != nil {
			return nil, fmt.Errorf("expert processing failed: %w", err)
		}

		// 4. Enrich with routing metadata
		result.ExpertType = routeResult.Expert
		result.ExpertName = routeResult.ExpertName
		result.Reasoning = routeResult.Reasoning

		return result, nil
	}
}
