package aichat

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// NewChatService creates the main chat processing function.
func NewChatService(
	translate TranslateFn,
	formatResponse FormatResponseFn,
	dispatchQuestion DispatchQuestionFn,
	store ConversationStore,
	logger *slog.Logger,
) ProcessChatFn {
	return func(ctx context.Context, req ChatRequest) (*ChatResult, error) {
		// 1. Translate message to English for consistent processing
		translation, err := translate(ctx, req.Message)
		if err != nil {
			return nil, fmt.Errorf("translation failed: %w", err)
		}

		logger.Debug("message translated",
			"original", req.Message,
			"translated", translation.TranslatedMessage,
			"detected_language", translation.DetectedLanguage,
			"confidence", translation.Confidence,
		)

		// 2. Get or create conversation
		conversation, err := getOrCreateConversation(ctx, req, store)
		if err != nil {
			return nil, err
		}

		// 3. Store user message (original language)
		if err := storeUserMessage(ctx, store, conversation.ID, req.Message); err != nil {
			return nil, err
		}

		// 4. Route and process with expert (using English translation)
		// Expert is responsible for resolving any entity data it needs
		expertReq := ExpertRequest{
			Message:  translation.TranslatedMessage,
			EntityID: conversation.EntityID,
		}

		expertResult, err := dispatchQuestion(ctx, expertReq)
		if err != nil {
			return nil, err
		}

		// 5. Format response in user's language
		formattedResponse, err := formatResponse(ctx, FormatRequest{
			ExpertType:         expertResult.ExpertType,
			Answer:             expertResult.Answer,
			OriginalQuestion:   req.Message,
			TranslatedQuestion: translation.TranslatedMessage,
			DetectedLanguage:   translation.DetectedLanguage,
		})
		if err != nil {
			logger.Warn("formatting failed, using fallback answer", "error", err)
			// Fallback to expert's answer if formatting fails
			formattedResponse = &FormatResponse{
				FormattedAnswer: expertResult.Answer,
				Language:        translation.DetectedLanguage,
			}
		}

		// Update expert result with formatted answer
		expertResult.Answer = formattedResponse.FormattedAnswer

		// 6. Store assistant message
		if err := storeAssistantMessage(ctx, store, conversation.ID, expertResult); err != nil {
			logger.Warn("failed to store assistant message", "error", err)
			// Don't fail - response is already generated
		}

		return &ChatResult{
			ConversationID: conversation.ID,
			ExpertResult:   expertResult,
		}, nil
	}
}

func getOrCreateConversation(
	ctx context.Context,
	req ChatRequest,
	store ConversationStore,
) (*Conversation, error) {
	if req.ConversationID != "" {
		// Existing conversation
		conv, err := store.Get(ctx, req.ConversationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get conversation: %w", err)
		}
		return conv, nil
	}

	// New conversation
	conv, err := store.Create(ctx, req.EntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return conv, nil
}

func storeUserMessage(ctx context.Context, store ConversationStore, conversationID, message string) error {
	msg := Message{
		Role:      RoleUser,
		Content:   message,
		Timestamp: time.Now(),
	}
	return store.AddMessage(ctx, conversationID, msg)
}

func storeAssistantMessage(ctx context.Context, store ConversationStore, conversationID string, result *ExpertResult) error {
	msg := Message{
		Role:      RoleAssistant,
		Content:   result.Answer,
		Timestamp: time.Now(),
		Expert:    &result.ExpertName,
	}
	return store.AddMessage(ctx, conversationID, msg)
}
