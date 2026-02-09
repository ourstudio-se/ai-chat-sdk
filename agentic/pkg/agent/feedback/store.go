package feedback

import (
	"context"
	"sync"
)

type Store interface {
	Save(ctx context.Context, fb *Feedback) error
	GetByMessage(ctx context.Context, messageID string) (*Feedback, error)
	GetBySession(ctx context.Context, sessionID string) ([]*Feedback, error)
}

// MemoryStore is an in-memory implementation.
type MemoryStore struct {
	items map[string]*Feedback
	mu    sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]*Feedback)}
}

func (s *MemoryStore) Save(ctx context.Context, fb *Feedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[fb.MessageID] = fb
	return nil
}

func (s *MemoryStore) GetByMessage(ctx context.Context, messageID string) (*Feedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.items[messageID], nil
}

func (s *MemoryStore) GetBySession(ctx context.Context, sessionID string) ([]*Feedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Feedback
	for _, fb := range s.items {
		if fb.SessionID == sessionID {
			result = append(result, fb)
		}
	}
	return result, nil
}
