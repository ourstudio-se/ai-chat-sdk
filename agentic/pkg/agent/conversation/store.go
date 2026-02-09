package conversation

import (
	"context"

	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/types"
)

// Store defines conversation persistence.
type Store interface {
	Get(ctx context.Context, id string) (*types.Conversation, error)
	Save(ctx context.Context, conv *types.Conversation) error
	Delete(ctx context.Context, id string) error
}

// Filter for listing conversations
type Filter struct {
	UserID    string
	Tags      []string
	Limit     int
	Offset    int
	OrderBy   string // "created_at", "updated_at"
	OrderDesc bool
}

// DefaultFilter returns a default filter
func DefaultFilter() Filter {
	return Filter{
		Limit:     20,
		Offset:    0,
		OrderBy:   "updated_at",
		OrderDesc: true,
	}
}
