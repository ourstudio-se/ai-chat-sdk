package aichat

import (
	"context"
	"fmt"
	"log/slog"
)

// newFormatter creates a formatting function.
func newFormatter(chat ChatFn, logger *slog.Logger, customSystemPrompt string) FormatResponseFn {
	return func(ctx context.Context, req FormatRequest) (*FormatResponse, error) {
		if req.DetectedLanguage == "en" || req.Answer == "" {
			return &FormatResponse{
				FormattedAnswer: req.Answer,
				Language:        req.DetectedLanguage,
			}, nil
		}

		systemPrompt := buildFormatterSystemPrompt(req.DetectedLanguage, customSystemPrompt)

		userPrompt := fmt.Sprintf(`Original question: "%s"

Answer in English: "%s"

Please translate this answer to %s while maintaining:
- A friendly, helpful tone
- Technical accuracy
- Natural, conversational language
- Customer-friendly phrasing`,
			req.OriginalQuestion,
			req.Answer,
			req.DetectedLanguage,
		)

		translated, err := chat(ctx, systemPrompt, userPrompt, nil)
		if err != nil {
			logger.Warn("translation failed, using original answer", slog.String("error", err.Error()))
			return &FormatResponse{
				FormattedAnswer: req.Answer,
				Language:        req.DetectedLanguage,
			}, nil
		}

		logger.Debug("response formatted",
			slog.String("language", req.DetectedLanguage),
			slog.String("expert_type", string(req.ExpertType)),
			slog.Int("answer_length", len(translated)),
		)

		return &FormatResponse{
			FormattedAnswer: translated,
			Language:        req.DetectedLanguage,
		}, nil
	}
}

func buildFormatterSystemPrompt(language string, customSystemPrompt string) string {
	if customSystemPrompt != "" {
		return customSystemPrompt
	}

	if language == "sv" {
		return `Du är en hjälpsam assistent som översätter svar till kunder.

Din uppgift:
- Översätt det engelska svaret till naturlig svenska
- Behåll en vänlig, hjälpsam och informativ ton
- Använd vardagligt språk
- Behåll teknisk noggrannhet
- Var kort och koncis

Tone of voice:
- Confident but not arrogant
- Helpful and informative
- Human and approachable
- Focus on practical benefits`
	}

	return fmt.Sprintf(`You are a helpful assistant translating answers for customers.

Your task:
- Translate the English answer to natural %s
- Maintain a friendly, helpful, informative tone
- Use everyday language
- Preserve technical accuracy
- Keep it short and concise

Tone of voice:
- Confident but not arrogant
- Helpful and informative
- Human and approachable
- Focus on practical benefits`, language)
}
