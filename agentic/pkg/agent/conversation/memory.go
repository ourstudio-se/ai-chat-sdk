package conversation

import (
	"context"

	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/types"

	"sync"
	"time"
)

type MemoryStore struct {
	conversations map[string]*types.Conversation
	mu            sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		conversations: make(map[string]*types.Conversation),
	}
}

func (s *MemoryStore) Get(ctx context.Context, id string) (*types.Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, ok := s.conversations[id]
	if !ok {
		return nil, nil
	}
	return conv, nil
}

func (s *MemoryStore) Save(ctx context.Context, conv *types.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv.UpdatedAt = time.Now()
	s.conversations[conv.ID] = conv
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.conversations, id)
	return nil
}
