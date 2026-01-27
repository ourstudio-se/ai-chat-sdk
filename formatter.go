package aichat

import (
	"context"
	"fmt"
	"log/slog"
)

// newFormatter creates a formatting function.
func newFormatter(chat ChatFn, logger *slog.Logger, customSystemPrompt string, glossary map[string]GlossaryTerms) FormatResponseFn {
	return func(ctx context.Context, req FormatRequest) (*FormatResponse, error) {
		if req.DetectedLanguage == "en" || req.Answer == "" {
			return &FormatResponse{
				FormattedAnswer: req.Answer,
				Language:        req.DetectedLanguage,
			}, nil
		}

		glossaryPrompt := formatGlossaryForPrompt(glossary, req.DetectedLanguage)
		systemPrompt := buildFormatterSystemPrompt(req.DetectedLanguage, glossaryPrompt, customSystemPrompt)

		userPrompt := fmt.Sprintf(`Original question: "%s"

Answer in English: "%s"

Please translate this answer to %s while maintaining:
- A friendly, helpful tone
- Technical accuracy
- Natural, conversational language
- Customer-friendly phrasing
- IMPORTANT: Use the domain-specific terminology provided in the system prompt`,
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
			slog.Bool("has_glossary", len(glossary) > 0),
			slog.Int("answer_length", len(translated)),
		)

		return &FormatResponse{
			FormattedAnswer: translated,
			Language:        req.DetectedLanguage,
		}, nil
	}
}

func buildFormatterSystemPrompt(language string, glossaryPrompt string, customSystemPrompt string) string {
	if customSystemPrompt != "" {
		return customSystemPrompt + glossaryPrompt
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
- Focus on practical benefits` + glossaryPrompt
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
- Focus on practical benefits`, language) + glossaryPrompt
}

func formatGlossaryForPrompt(glossary map[string]GlossaryTerms, targetLanguage string) string {
	if glossary == nil || len(glossary) == 0 {
		return ""
	}

	result := "\n\nDomain-specific terminology (use these translations):\n"
	for term, translations := range glossary {
		translation := getTranslationForLanguage(translations, targetLanguage)
		if translation != "" && translation != translations.English {
			result += "- " + term + " → " + translation + "\n"
		}
	}

	return result
}

func getTranslationForLanguage(terms GlossaryTerms, languageCode string) string {
	switch languageCode {
	case "sv":
		return terms.Swedish
	case "de":
		return terms.German
	case "no", "nb", "nn":
		return terms.Norwegian
	case "da":
		return terms.Danish
	case "fr":
		return terms.French
	default:
		return terms.English
	}
}
