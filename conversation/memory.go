package conversation

import (
	"sync"
	"time"

	aichat "github.com/ourstudio-se/ai-chat-sdk"
)

// MemoryStore is an in-memory conversation store.
type MemoryStore struct {
	mu            sync.RWMutex
	conversations map[string]*aichat.Conversation
	feedback      map[string][]aichat.Feedback
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		conversations: make(map[string]*aichat.Conversation),
		feedback:      make(map[string][]aichat.Feedback),
	}
}

// GetConversation retrieves a conversation by ID.
func (s *MemoryStore) GetConversation(ctx aichat.ChatCompletionContext, id string) (*aichat.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, ok := s.conversations[id]
	if !ok {
		return nil, aichat.ErrConversationNotFound
	}
	return conv, nil
}

// SaveConversation saves a conversation.
func (s *MemoryStore) SaveConversation(ctx aichat.ChatCompletionContext, conv *aichat.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conversations[conv.ID] = conv
	return nil
}

// AddMessage adds a message to a conversation.
func (s *MemoryStore) AddMessage(ctx aichat.ChatCompletionContext, conversationID string, msg aichat.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[conversationID]
	if !ok {
		// Create new conversation
		conv = &aichat.Conversation{
			ID:        conversationID,
			Messages:  make([]aichat.Message, 0),
			CreatedAt: time.Now(),
		}
		s.conversations[conversationID] = conv
	}

	conv.Messages = append(conv.Messages, msg)
	conv.UpdatedAt = time.Now()

	return nil
}

// GetMessages retrieves messages for a conversation.
func (s *MemoryStore) GetMessages(ctx aichat.ChatCompletionContext, conversationID string, limit int) ([]aichat.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, ok := s.conversations[conversationID]
	if !ok {
		return nil, nil
	}

	messages := conv.Messages
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	return messages, nil
}

// SaveFeedback saves feedback for a message.
func (s *MemoryStore) SaveFeedback(ctx aichat.ChatCompletionContext, fb aichat.Feedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.feedback[fb.MessageID] = append(s.feedback[fb.MessageID], fb)
	return nil
}

// GetFeedback retrieves feedback for a message.
func (s *MemoryStore) GetFeedback(messageID string) []aichat.Feedback {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.feedback[messageID]
}
