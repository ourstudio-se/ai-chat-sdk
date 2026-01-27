package aichat

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

// newRouter creates a routing function that determines which expert should handle a question.
func newRouter(
	chatJSON ChatJSONFn,
	experts map[ExpertType]Expert,
	systemPromptTemplate string,
	defaultExpert ExpertType,
	defaultReasoning string,
	logger *slog.Logger,
) RouteQuestionFn {
	return func(ctx context.Context, message string, entityID string) (*RouteResult, error) {
		expertsStr := buildExpertsDefinition(experts)

		systemPrompt := systemPromptTemplate
		systemPrompt = strings.ReplaceAll(systemPrompt, "{{EXPERTS}}", expertsStr)

		// Include entity ID in context if available
		contextStr := "No additional context available."
		if entityID != "" {
			contextStr = fmt.Sprintf("Entity ID: %s", entityID)
		}
		systemPrompt = strings.ReplaceAll(systemPrompt, "{{CONTEXT}}", contextStr)

		var result struct {
			Expert    string `json:"expert"`
			Reasoning string `json:"reasoning"`
		}

		opts := &ChatJSONOptions{
			Model:       ModelMini,
			Temperature: 0.3,
		}
		if err := chatJSON(ctx, systemPrompt, message, opts, &result); err != nil {
			// Fallback to default expert on routing failure
			if defaultExpert != "" {
				logger.Warn("routing failed, using default expert",
					slog.String("error", err.Error()),
					slog.String("default_expert", string(defaultExpert)),
				)
				return &RouteResult{
					Expert:     defaultExpert,
					ExpertName: getExpertName(experts, defaultExpert),
					Reasoning:  defaultReasoning,
				}, nil
			}
			return nil, fmt.Errorf("failed to route question: %w", err)
		}

		expertType := ExpertType(result.Expert)
		expertName := getExpertName(experts, expertType)

		logger.Debug("routed question to expert",
			slog.String("expert_type", string(expertType)),
			slog.String("expert_name", expertName),
			slog.String("reasoning", result.Reasoning),
		)

		return &RouteResult{
			Expert:     expertType,
			ExpertName: expertName,
			Reasoning:  result.Reasoning,
		}, nil
	}
}

func buildExpertsDefinition(experts map[ExpertType]Expert) string {
	if len(experts) == 0 {
		return "No experts defined."
	}

	// Sort keys for deterministic output
	keys := make([]ExpertType, 0, len(experts))
	for k := range experts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return string(keys[i]) < string(keys[j])
	})

	var sb strings.Builder
	for _, expertType := range keys {
		expert := experts[expertType]
		sb.WriteString(fmt.Sprintf("- \"%s\" - %s: %s\n", expertType, expert.Name, expert.Description))
	}
	return sb.String()
}

func getExpertName(experts map[ExpertType]Expert, expertType ExpertType) string {
	if expert, exists := experts[expertType]; exists {
		return expert.Name
	}
	return string(expertType)
}
