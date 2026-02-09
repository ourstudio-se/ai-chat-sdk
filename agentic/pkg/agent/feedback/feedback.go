package feedback

import (
	"time"

	"github.com/google/uuid"
)

type Rating string

const (
	RatingPositive Rating = "positive"
	RatingNegative Rating = "negative"
)

type Feedback struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	MessageID string    `json:"messageId"`
	Rating    Rating    `json:"rating"`
	Comment   *string   `json:"comment,omitempty"`
	Snapshot  *Snapshot `json:"snapshot,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Snapshot struct {
	UserMessage   string         `json:"userMessage"`
	AgentResponse string         `json:"agentResponse"`
	Context       map[string]any `json:"context,omitempty"`
	SkillVariant  string         `json:"skillVariant,omitempty"`
}

func New(sessionID, messageID string, rating Rating) *Feedback {
	return &Feedback{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		MessageID: messageID,
		Rating:    rating,
		CreatedAt: time.Now(),
	}
}

func (f *Feedback) WithComment(comment string) *Feedback {
	f.Comment = &comment
	return f
}

func (f *Feedback) WithSnapshot(s *Snapshot) *Feedback {
	f.Snapshot = s
	return f
}
