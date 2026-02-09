// Package types contains shared types with no internal dependencies.
package types

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	ID        string     `json:"id"`
	Role      Role       `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type ToolCall struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Input  map[string]any `json:"input"`
	Output any            `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type Metadata struct {
	UserID       string            `json:"userId,omitempty"`
	SkillVariant string            `json:"skillVariant,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
}

type Conversation struct {
	ID        string         `json:"id"`
	Messages  []Message      `json:"messages"`
	Context   map[string]any `json:"context,omitempty"`
	Metadata  Metadata       `json:"metadata"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

func NewConversation() *Conversation {
	now := time.Now()
	return &Conversation{
		ID:        uuid.NewString(),
		Messages:  []Message{},
		Context:   make(map[string]any),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (c *Conversation) AddUserMessage(content string) *Message {
	msg := Message{
		ID:        uuid.NewString(),
		Role:      RoleUser,
		Content:   content,
		CreatedAt: time.Now(),
	}
	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now()
	return &msg
}

func (c *Conversation) AddAssistantMessage(content string, toolCalls []ToolCall) *Message {
	msg := Message{
		ID:        uuid.NewString(),
		Role:      RoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
		CreatedAt: time.Now(),
	}
	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now()
	return &msg
}

func (c *Conversation) GetMessage(id string) *Message {
	for i := range c.Messages {
		if c.Messages[i].ID == id {
			return &c.Messages[i]
		}
	}
	return nil
}

type ChatRequest struct {
	SessionID string         `json:"sessionId,omitempty"`
	Message   string         `json:"message"`
	Context   map[string]any `json:"context,omitempty"`
	UserID    string         `json:"userId,omitempty"`
}

type ChatResponse struct {
	SessionID  string     `json:"sessionId"`
	MessageID  string     `json:"messageId"`
	Response   string     `json:"response"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
	SkillsUsed []string   `json:"skillsUsed,omitempty"`
}
