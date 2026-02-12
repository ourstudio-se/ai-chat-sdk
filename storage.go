package aichat

import (
	"sync"
	"time"
)

// memoryStore is an in-memory conversation store.
type memoryStore struct {
	mu            sync.RWMutex
	conversations map[string]*Conversation
	feedback      map[string][]Feedback
}

// NewMemoryStore creates a new in-memory conversation store.
func NewMemoryStore() ConversationStore {
	return &memoryStore{
		conversations: make(map[string]*Conversation),
		feedback:      make(map[string][]Feedback),
	}
}

// GetConversation retrieves a conversation by ID.
func (s *memoryStore) GetConversation(ctx ChatCompletionContext, id string) (*Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, ok := s.conversations[id]
	if !ok {
		return nil, ErrConversationNotFound
	}
	return conv, nil
}

// SaveConversation saves a conversation.
func (s *memoryStore) SaveConversation(ctx ChatCompletionContext, conv *Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conversations[conv.ID] = conv
	return nil
}

// AddMessage adds a message to a conversation.
func (s *memoryStore) AddMessage(ctx ChatCompletionContext, conversationID string, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[conversationID]
	if !ok {
		// Create new conversation
		conv = &Conversation{
			ID:        conversationID,
			Messages:  make([]Message, 0),
			CreatedAt: time.Now(),
		}
		s.conversations[conversationID] = conv
	}

	conv.Messages = append(conv.Messages, msg)
	conv.UpdatedAt = time.Now()

	return nil
}

// GetMessages retrieves messages for a conversation.
func (s *memoryStore) GetMessages(ctx ChatCompletionContext, conversationID string, limit int) ([]Message, error) {
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
func (s *memoryStore) SaveFeedback(ctx ChatCompletionContext, fb Feedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.feedback[fb.MessageID] = append(s.feedback[fb.MessageID], fb)
	return nil
}
