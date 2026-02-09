package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/feedback"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/types"
)

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// ChatRequest is the API request for chat
type ChatRequest struct {
	SessionID string         `json:"sessionId,omitempty"`
	Message   string         `json:"message"`
	Context   map[string]any `json:"context,omitempty"`
	UserID    string         `json:"userId,omitempty"`
}

func (s *Server) chatHandler(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	resp, err := s.agent.Chat(r.Context(), types.ChatRequest{
		SessionID: req.SessionID,
		Message:   req.Message,
		Context:   req.Context,
		UserID:    req.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) getConversationHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	conv, err := s.agent.GetConversation(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conv == nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	writeJSON(w, http.StatusOK, conv)
}

func (s *Server) deleteConversationHandler(w http.ResponseWriter, r *http.Request) {
	// Implementation depends on whether you want to support deletion
	writeError(w, http.StatusNotImplemented, "not implemented")
}

// FeedbackRequest is the API request for submitting feedback
type FeedbackRequest struct {
	SessionID string          `json:"sessionId"`
	MessageID string          `json:"messageId"`
	Rating    feedback.Rating `json:"rating"`
	Comment   *string         `json:"comment,omitempty"`
}

func (s *Server) submitFeedbackHandler(w http.ResponseWriter, r *http.Request) {
	var req FeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SessionID == "" || req.MessageID == "" {
		writeError(w, http.StatusBadRequest, "sessionId and messageId are required")
		return
	}

	if req.Rating != feedback.RatingPositive && req.Rating != feedback.RatingNegative {
		writeError(w, http.StatusBadRequest, "rating must be 'positive' or 'negative'")
		return
	}

	// Load conversation to capture snapshot
	conv, err := s.agent.GetConversation(r.Context(), req.SessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conv == nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	// Find the message and create snapshot
	var snapshot *feedback.Snapshot
	msg := conv.GetMessage(req.MessageID)
	if msg != nil && msg.Role == types.RoleAssistant {
		// Find preceding user message
		var userMsg string
		for i := len(conv.Messages) - 1; i >= 0; i-- {
			if conv.Messages[i].ID == req.MessageID {
				// Look for user message before this
				for j := i - 1; j >= 0; j-- {
					if conv.Messages[j].Role == types.RoleUser {
						userMsg = conv.Messages[j].Content
						break
					}
				}
				break
			}
		}

		snapshot = &feedback.Snapshot{
			UserMessage:   userMsg,
			AgentResponse: msg.Content,
			Context:       conv.Context,
			SkillVariant:  conv.Metadata.SkillVariant,
		}
	}

	fb := feedback.New(req.SessionID, req.MessageID, req.Rating)
	if req.Comment != nil {
		fb.WithComment(*req.Comment)
	}
	fb.WithSnapshot(snapshot)

	if err := s.feedbackStore.Save(r.Context(), fb); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, fb)
}

func (s *Server) getSessionFeedbackHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")

	items, err := s.feedbackStore.GetBySession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
