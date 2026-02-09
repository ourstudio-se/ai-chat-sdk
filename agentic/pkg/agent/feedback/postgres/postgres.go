// pkg/agent/feedback/postgres/postgres.go
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/feedback"
)

// Store implements feedback.Store with PostgreSQL
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

// New creates a new PostgreSQL feedback store
func New(pool *pgxpool.Pool, opts ...Option) *Store {
	s := &Store{
		pool:      pool,
		tableName: "feedback",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Store) Save(ctx context.Context, fb *feedback.Feedback) error {
	var snapshotJSON []byte
	var err error

	if fb.Snapshot != nil {
		snapshotJSON, err = json.Marshal(fb.Snapshot)
		if err != nil {
			return fmt.Errorf("marshaling snapshot: %w", err)
		}
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (id, session_id, message_id, rating, comment, snapshot, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (message_id) DO UPDATE SET
			rating = EXCLUDED.rating,
			comment = EXCLUDED.comment,
			snapshot = EXCLUDED.snapshot
	`, s.tableName)

	_, err = s.pool.Exec(ctx, query,
		fb.ID, fb.SessionID, fb.MessageID, fb.Rating, fb.Comment, snapshotJSON, fb.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("saving feedback: %w", err)
	}

	return nil
}

func (s *Store) GetByMessage(ctx context.Context, messageID string) (*feedback.Feedback, error) {
	query := fmt.Sprintf(`
		SELECT id, session_id, message_id, rating, comment, snapshot, created_at
		FROM %s
		WHERE message_id = $1
	`, s.tableName)

	row := s.pool.QueryRow(ctx, query, messageID)

	var fb feedback.Feedback
	var snapshotJSON []byte

	err := row.Scan(
		&fb.ID,
		&fb.SessionID,
		&fb.MessageID,
		&fb.Rating,
		&fb.Comment,
		&snapshotJSON,
		&fb.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning feedback: %w", err)
	}

	if snapshotJSON != nil {
		var snapshot feedback.Snapshot
		if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
			return nil, fmt.Errorf("unmarshaling snapshot: %w", err)
		}
		fb.Snapshot = &snapshot
	}

	return &fb, nil
}

func (s *Store) GetBySession(ctx context.Context, sessionID string) ([]*feedback.Feedback, error) {
	query := fmt.Sprintf(`
		SELECT id, session_id, message_id, rating, comment, snapshot, created_at
		FROM %s
		WHERE session_id = $1
		ORDER BY created_at DESC
	`, s.tableName)

	rows, err := s.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("querying feedback: %w", err)
	}
	defer rows.Close()

	var result []*feedback.Feedback
	for rows.Next() {
		var fb feedback.Feedback
		var snapshotJSON []byte

		if err := rows.Scan(
			&fb.ID,
			&fb.SessionID,
			&fb.MessageID,
			&fb.Rating,
			&fb.Comment,
			&snapshotJSON,
			&fb.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if snapshotJSON != nil {
			var snapshot feedback.Snapshot
			json.Unmarshal(snapshotJSON, &snapshot)
			fb.Snapshot = &snapshot
		}

		result = append(result, &fb)
	}

	return result, nil
}

// Migration returns the SQL to create the feedback table
func Migration(tableName string) string {
	if tableName == "" {
		tableName = "feedback"
	}
	return fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			message_id TEXT NOT NULL UNIQUE,
			rating TEXT NOT NULL,
			comment TEXT,
			snapshot JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_%s_session_id ON %s (session_id);
		CREATE INDEX IF NOT EXISTS idx_%s_rating ON %s (rating);
		CREATE INDEX IF NOT EXISTS idx_%s_created_at ON %s (created_at DESC);

		-- View for A/B testing analysis
		CREATE OR REPLACE VIEW %s_by_variant AS
		SELECT
			snapshot->>'skillVariant' AS skill_variant,
			rating,
			COUNT(*) AS count,
			COUNT(*) FILTER (WHERE rating = 'positive') AS positive_count,
			COUNT(*) FILTER (WHERE rating = 'negative') AS negative_count
		FROM %s
		WHERE snapshot->>'skillVariant' IS NOT NULL
		GROUP BY snapshot->>'skillVariant', rating;
	`, tableName, tableName, tableName, tableName, tableName, tableName, tableName, tableName, tableName)
}
