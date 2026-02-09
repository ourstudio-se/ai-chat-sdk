// pkg/agent/conversation/postgres/postgres.go
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/conversation"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/types"
)

// Store implements conversation.Store with PostgreSQL
type Store struct {
	pool      *pgxpool.Pool
	tableName string
}

// Option configures the store
type Option func(*Store)

// WithTableName sets a custom table name
func WithTableName(name string) Option {
	return func(s *Store) {
		s.tableName = name
	}
}

// New creates a new PostgreSQL conversation store
func New(pool *pgxpool.Pool, opts ...Option) *Store {
	s := &Store{
		pool:      pool,
		tableName: "conversations",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Store) Get(ctx context.Context, id string) (*types.Conversation, error) {
	query := fmt.Sprintf(`
		SELECT id, messages, context, metadata, created_at, updated_at
		FROM %s
		WHERE id = $1
	`, s.tableName)

	row := s.pool.QueryRow(ctx, query, id)

	var conv types.Conversation
	var messagesJSON, contextJSON, metadataJSON []byte

	err := row.Scan(
		&conv.ID,
		&messagesJSON,
		&contextJSON,
		&metadataJSON,
		&conv.CreatedAt,
		&conv.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning conversation: %w", err)
	}

	if err := json.Unmarshal(messagesJSON, &conv.Messages); err != nil {
		return nil, fmt.Errorf("unmarshaling messages: %w", err)
	}
	if contextJSON != nil {
		if err := json.Unmarshal(contextJSON, &conv.Context); err != nil {
			return nil, fmt.Errorf("unmarshaling context: %w", err)
		}
	}
	if err := json.Unmarshal(metadataJSON, &conv.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	return &conv, nil
}

func (s *Store) Save(ctx context.Context, conv *types.Conversation) error {
	messagesJSON, err := json.Marshal(conv.Messages)
	if err != nil {
		return fmt.Errorf("marshaling messages: %w", err)
	}
	contextJSON, err := json.Marshal(conv.Context)
	if err != nil {
		return fmt.Errorf("marshaling context: %w", err)
	}
	metadataJSON, err := json.Marshal(conv.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	conv.UpdatedAt = time.Now()

	query := fmt.Sprintf(`
		INSERT INTO %s (id, messages, context, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			messages = EXCLUDED.messages,
			context = EXCLUDED.context,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`, s.tableName)

	_, err = s.pool.Exec(ctx, query,
		conv.ID, messagesJSON, contextJSON, metadataJSON, conv.CreatedAt, conv.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving conversation: %w", err)
	}

	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, s.tableName)
	_, err := s.pool.Exec(ctx, query, id)
	return err
}

func (s *Store) List(ctx context.Context, filter conversation.Filter) ([]*types.Conversation, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("metadata->>'userId' = $%d", argIdx))
		args = append(args, filter.UserID)
		argIdx++
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Build ORDER BY
	orderBy := "updated_at"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	orderDir := "DESC"
	if !filter.OrderDesc {
		orderDir = "ASC"
	}

	query := fmt.Sprintf(`
		SELECT id, messages, context, metadata, created_at, updated_at
		FROM %s
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, s.tableName, whereClause, orderBy, orderDir, argIdx, argIdx+1)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying conversations: %w", err)
	}
	defer rows.Close()

	var conversations []*types.Conversation
	for rows.Next() {
		var conv types.Conversation
		var messagesJSON, contextJSON, metadataJSON []byte

		if err := rows.Scan(
			&conv.ID, &messagesJSON, &contextJSON, &metadataJSON, &conv.CreatedAt, &conv.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		json.Unmarshal(messagesJSON, &conv.Messages)
		json.Unmarshal(contextJSON, &conv.Context)
		json.Unmarshal(metadataJSON, &conv.Metadata)

		conversations = append(conversations, &conv)
	}

	return conversations, nil
}

// Migration returns the SQL to create the conversations table
func Migration(tableName string) string {
	if tableName == "" {
		tableName = "conversations"
	}
	return fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			messages JSONB NOT NULL DEFAULT '[]',
			context JSONB,
			metadata JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_%s_user_id ON %s ((metadata->>'userId'));
		CREATE INDEX IF NOT EXISTS idx_%s_updated_at ON %s (updated_at DESC);
	`, tableName, tableName, tableName, tableName, tableName)
}
