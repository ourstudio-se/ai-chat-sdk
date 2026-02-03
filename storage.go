package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NewMemoryStore creates a new in-memory conversation store.
// This is useful for development and testing, but conversations are lost on restart.
func NewMemoryStore(logger *slog.Logger) ConversationStore {
	var mu sync.RWMutex
	conversations := make(map[string]*Conversation)

	logger.Info("initialized in-memory store")

	return ConversationStore{
		Create: func(ctx context.Context, entityID string) (*Conversation, error) {
			mu.Lock()
			defer mu.Unlock()

			conversation := &Conversation{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				EntityID:  entityID,
				Messages:  []Message{},
			}

			conversations[conversation.ID] = conversation

			logger.Debug("created conversation",
				slog.String("conversation_id", conversation.ID),
				slog.String("entity_id", entityID),
			)

			return conversation, nil
		},

		Get: func(ctx context.Context, id string) (*Conversation, error) {
			mu.RLock()
			defer mu.RUnlock()

			conversation, exists := conversations[id]
			if !exists {
				return nil, ErrConversationNotFound
			}

			// Return a deep copy to prevent concurrent modification
			result := *conversation
			result.Messages = make([]Message, len(conversation.Messages))
			for i := range conversation.Messages {
				msg := conversation.Messages[i]
				if msg.Expert != nil {
					expertCopy := *msg.Expert
					msg.Expert = &expertCopy
				}
				if msg.Feedback != nil {
					feedbackCopy := *msg.Feedback
					msg.Feedback = &feedbackCopy
				}
				result.Messages[i] = msg
			}

			logger.Debug("retrieved conversation",
				slog.String("conversation_id", id),
				slog.Int("message_count", len(result.Messages)),
			)

			return &result, nil
		},

		AddMessage: func(ctx context.Context, id string, msg Message) error {
			mu.Lock()
			defer mu.Unlock()

			conversation, exists := conversations[id]
			if !exists {
				return ErrConversationNotFound
			}

			AddMessage(conversation, msg)

			logger.Debug("added message to conversation",
				slog.String("conversation_id", id),
				slog.String("role", string(msg.Role)),
				slog.Int("total_messages", len(conversation.Messages)),
			)

			return nil
		},

		Save: func(ctx context.Context, conversation *Conversation) error {
			mu.Lock()
			defer mu.Unlock()

			conversations[conversation.ID] = conversation
			return nil
		},

		UpdateFeedback: func(ctx context.Context, conversationID, messageID string, feedback MessageFeedback) error {
			mu.Lock()
			defer mu.Unlock()

			conversation, exists := conversations[conversationID]
			if !exists {
				return ErrConversationNotFound
			}

			for i := range conversation.Messages {
				if conversation.Messages[i].ID == messageID {
					conversation.Messages[i].Feedback = &feedback
					return nil
				}
			}
			return ErrMessageNotFound
		},
	}
}

// NewFileStore creates a new file-based conversation store.
func NewFileStore(dataDir string, logger *slog.Logger) (ConversationStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return ConversationStore{}, fmt.Errorf("failed to create conversations directory: %w", err)
	}

	logger.Info("initialized file store", slog.String("directory", dataDir))

	var mu sync.RWMutex

	getFilePath := func(id string) string {
		return filepath.Join(dataDir, fmt.Sprintf("%s.json", id))
	}

	saveUnlocked := func(conversation *Conversation) error {
		path := getFilePath(conversation.ID)

		data, err := json.MarshalIndent(conversation, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal conversation: %w", err)
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("failed to write conversation file: %w", err)
		}

		return nil
	}

	getUnlocked := func(id string) (*Conversation, error) {
		path := getFilePath(id)

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, ErrConversationNotFound
			}
			return nil, fmt.Errorf("failed to read conversation file: %w", err)
		}

		var conversation Conversation
		if err := json.Unmarshal(data, &conversation); err != nil {
			return nil, fmt.Errorf("failed to unmarshal conversation: %w", err)
		}

		return &conversation, nil
	}

	return ConversationStore{
		Create: func(ctx context.Context, entityID string) (*Conversation, error) {
			mu.Lock()
			defer mu.Unlock()

			conversation := &Conversation{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				EntityID:  entityID,
				Messages:  []Message{},
			}

			if err := saveUnlocked(conversation); err != nil {
				return nil, fmt.Errorf("failed to save new conversation: %w", err)
			}

			logger.Debug("created conversation",
				slog.String("conversation_id", conversation.ID),
				slog.String("entity_id", entityID),
			)

			return conversation, nil
		},

		Get: func(ctx context.Context, id string) (*Conversation, error) {
			mu.RLock()
			defer mu.RUnlock()

			conversation, err := getUnlocked(id)
			if err != nil {
				return nil, err
			}

			logger.Debug("retrieved conversation",
				slog.String("conversation_id", id),
				slog.Int("message_count", len(conversation.Messages)),
			)

			return conversation, nil
		},

		AddMessage: func(ctx context.Context, id string, msg Message) error {
			mu.Lock()
			defer mu.Unlock()

			conversation, err := getUnlocked(id)
			if err != nil {
				return err
			}

			AddMessage(conversation, msg)

			if err := saveUnlocked(conversation); err != nil {
				return fmt.Errorf("failed to save conversation after adding message: %w", err)
			}

			logger.Debug("added message to conversation",
				slog.String("conversation_id", id),
				slog.String("role", string(msg.Role)),
				slog.Int("total_messages", len(conversation.Messages)),
			)

			return nil
		},

		Save: func(ctx context.Context, conversation *Conversation) error {
			mu.Lock()
			defer mu.Unlock()

			return saveUnlocked(conversation)
		},

		UpdateFeedback: func(ctx context.Context, conversationID, messageID string, feedback MessageFeedback) error {
			mu.Lock()
			defer mu.Unlock()

			conversation, err := getUnlocked(conversationID)
			if err != nil {
				return err
			}

			for i := range conversation.Messages {
				if conversation.Messages[i].ID == messageID {
					conversation.Messages[i].Feedback = &feedback
					return saveUnlocked(conversation)
				}
			}
			return ErrMessageNotFound
		},
	}, nil
}
