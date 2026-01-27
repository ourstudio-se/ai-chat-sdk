package aichat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// DefaultTranslatorSystemPrompt is the default system prompt for translation.
const DefaultTranslatorSystemPrompt = `You are a translation expert specialized in technical queries.

Your task:
1. Detect the language of the user's message
2. Translate it to English (if not already English)
3. Preserve technical terms, brand names, model names, and specifications

Return ONLY valid JSON in this exact format:
{
  "translatedMessage": "the English translation",
  "detectedLanguage": "two-letter ISO 639-1 code (sv, en, de, fr, no, da, etc.)",
  "confidence": 0.95
}

IMPORTANT RULES:
- Keep brand names unchanged
- Keep model names unchanged
- Preserve numbers and units exactly: "2023", "750 kg", "1200 mm"
- Translate only the natural language parts
- If already English, return it unchanged with detectedLanguage: "en"
- Be precise with technical terminology`

// newTranslator creates a translation function.
func newTranslator(chatJSON ChatJSONFn, logger *slog.Logger, customSystemPrompt string) TranslateFn {
	systemPrompt := customSystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultTranslatorSystemPrompt
	}

	return func(ctx context.Context, message string) (*TranslationResult, error) {
		if isLikelyEnglish(message) {
			logger.Debug("message appears to be English, skipping translation")
			return &TranslationResult{
				TranslatedMessage: message,
				DetectedLanguage:  "en",
				Confidence:        0.95,
			}, nil
		}

		var response TranslationResult
		if err := chatJSON(ctx, systemPrompt, message, nil, &response); err != nil {
			return nil, fmt.Errorf("translation API call failed: %w", err)
		}

		if response.TranslatedMessage == "" {
			return nil, fmt.Errorf("translation returned empty message")
		}
		if response.DetectedLanguage == "" {
			logger.Warn("no language detected, defaulting to 'en'")
			response.DetectedLanguage = "en"
		}
		if response.Confidence == 0 {
			response.Confidence = 0.5
		}

		logger.Debug("message translated",
			slog.String("original", message),
			slog.String("translated", response.TranslatedMessage),
			slog.String("language", response.DetectedLanguage),
			slog.Float64("confidence", response.Confidence),
		)

		return &response, nil
	}
}

func isLikelyEnglish(text string) bool {
	lowerText := strings.ToLower(text)

	swedishWords := []string{
		"hur", "många", "får", "plats", "kan", "jag", "min", "mitt",
		"bilen", "bagaget", "släp", "dragkrok", "färg", "vad", "vilken",
		"är", "har", "med", "och", "för", "från", "till", "på", "i",
	}

	germanWords := []string{
		"wie", "viele", "kann", "ich", "mein", "auto", "kofferraum",
		"anhänger", "farbe", "welche", "ist", "hat", "und", "für",
	}

	nordicWords := []string{
		"hvor", "mange", "bil", "bagasje", "tilhenger", "farge",
	}

	nonEnglishCount := 0

	for _, word := range swedishWords {
		if strings.Contains(lowerText, " "+word+" ") ||
			strings.HasPrefix(lowerText, word+" ") ||
			strings.HasSuffix(lowerText, " "+word) {
			nonEnglishCount++
		}
	}

	for _, word := range germanWords {
		if strings.Contains(lowerText, " "+word+" ") ||
			strings.HasPrefix(lowerText, word+" ") ||
			strings.HasSuffix(lowerText, " "+word) {
			nonEnglishCount++
		}
	}

	for _, word := range nordicWords {
		if strings.Contains(lowerText, " "+word+" ") ||
			strings.HasPrefix(lowerText, word+" ") ||
			strings.HasSuffix(lowerText, " "+word) {
			nonEnglishCount++
		}
	}

	return nonEnglishCount < 2
}
