package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// HTTPHandler returns an HTTP handler for the SDK.
type httpHandler struct {
	sdk            *SDK
	allowedOrigins []string
}

// NewHTTPHandler creates a new HTTP handler.
func NewHTTPHandler(sdk *SDK) http.Handler {
	h := &httpHandler{
		sdk:            sdk,
		allowedOrigins: sdk.config.AllowedOrigins,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /chat", h.handleChat)
	mux.HandleFunc("POST /chat/stream", h.handleChatStream)
	mux.HandleFunc("POST /feedback", h.handleFeedback)
	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("GET /skills", h.handleListSkills)

	return h.withMiddleware(mux)
}

// withMiddleware wraps the handler with middleware.
func (h *httpHandler) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID))
		w.Header().Set("X-Request-ID", requestID)

		// CORS
		origin := r.Header.Get("Origin")
		if h.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}

		// Handle preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *httpHandler) isAllowedOrigin(origin string) bool {
	for _, allowed := range h.allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

type requestIDKey struct{}

// ChatHTTPRequest is the HTTP request body for chat.
type ChatHTTPRequest struct {
	Message        string         `json:"message"`
	ConversationID string         `json:"conversationId,omitempty"`
	EntityID       string         `json:"entityId,omitempty"`
	Context        RequestContext `json:"context,omitempty"`
	Mode           string         `json:"mode,omitempty"`
	SkillID        string         `json:"skillId,omitempty"`
	Variant        string         `json:"variant,omitempty"`
}

// ChatHTTPResponse is the HTTP response body for chat.
type ChatHTTPResponse struct {
	ConversationID  string           `json:"conversationId"`
	MessageID       string           `json:"messageId"`
	SkillID         string           `json:"skillId"`
	Variant         string           `json:"variant,omitempty"`
	Mode            string           `json:"mode"`
	ToolsCalled     []ToolCall       `json:"toolsCalled,omitempty"`
	Response        json.RawMessage  `json:"response"`
	SuggestedAction *SuggestedAction `json:"suggestedAction,omitempty"`
	DurationMs      int64            `json:"durationMs"`
}

// FeedbackHTTPRequest is the HTTP request body for feedback.
type FeedbackHTTPRequest struct {
	MessageID string `json:"messageId"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment,omitempty"`
}

// ErrorResponse is the HTTP error response body.
type ErrorResponse struct {
	Error   string         `json:"error"`
	Code    string         `json:"code,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (h *httpHandler) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", ErrCodeValidation, nil)
		return
	}

	if req.Message == "" {
		h.writeError(w, http.StatusBadRequest, "message is required", ErrCodeValidation, nil)
		return
	}

	chatReq := ChatRequest{
		Message:        req.Message,
		ConversationID: req.ConversationID,
		EntityID:       req.EntityID,
		Context:        req.Context,
		Mode:           ExecutionMode(req.Mode),
		SkillID:        req.SkillID,
		Variant:        req.Variant,
	}

	result, err := h.sdk.Chat(r.Context(), chatReq)
	if err != nil {
		h.handleError(w, err)
		return
	}

	resp := ChatHTTPResponse{
		ConversationID:  result.ConversationID,
		MessageID:       result.MessageID,
		SkillID:         result.SkillID,
		Variant:         result.Variant,
		Mode:            string(result.Mode),
		ToolsCalled:     result.ToolCalls,
		Response:        result.Response,
		SuggestedAction: result.SuggestedAction,
		DurationMs:      result.Duration.Milliseconds(),
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *httpHandler) handleChatStream(w http.ResponseWriter, r *http.Request) {
	var req ChatHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", ErrCodeValidation, nil)
		return
	}

	if req.Message == "" {
		h.writeError(w, http.StatusBadRequest, "message is required", ErrCodeValidation, nil)
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "streaming not supported", ErrCodeInternal, nil)
		return
	}

	// For now, do non-streaming and send result as single event
	// Full streaming implementation would require changes to the LLM interface
	chatReq := ChatRequest{
		Message:        req.Message,
		ConversationID: req.ConversationID,
		EntityID:       req.EntityID,
		Context:        req.Context,
		Mode:           ExecutionMode(req.Mode),
		SkillID:        req.SkillID,
		Variant:        req.Variant,
	}

	// Send routing event
	h.writeSSE(w, flusher, "routing", map[string]any{
		"type": "routing",
	})

	// Send thinking event
	h.writeSSE(w, flusher, "thinking", map[string]any{
		"type": "thinking",
	})

	result, err := h.sdk.Chat(r.Context(), chatReq)
	if err != nil {
		h.writeSSE(w, flusher, "error", map[string]any{
			"type":  "error",
			"error": err.Error(),
		})
		return
	}

	// Send done event with full response
	h.writeSSE(w, flusher, "done", map[string]any{
		"type":            "done",
		"conversationId":  result.ConversationID,
		"messageId":       result.MessageID,
		"skillId":         result.SkillID,
		"variant":         result.Variant,
		"mode":            result.Mode,
		"toolsCalled":     result.ToolCalls,
		"response":        result.Response,
		"suggestedAction": result.SuggestedAction,
		"durationMs":      result.Duration.Milliseconds(),
	})
}

func (h *httpHandler) handleFeedback(w http.ResponseWriter, r *http.Request) {
	var req FeedbackHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", ErrCodeValidation, nil)
		return
	}

	if req.MessageID == "" {
		h.writeError(w, http.StatusBadRequest, "messageId is required", ErrCodeValidation, nil)
		return
	}

	feedback := Feedback{
		MessageID: req.MessageID,
		Rating:    req.Rating,
		Comment:   req.Comment,
		CreatedAt: time.Now(),
	}

	if err := h.sdk.storage.SaveFeedback(ChatCompletionContext{}, feedback); err != nil {
		h.handleError(w, err)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *httpHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (h *httpHandler) handleListSkills(w http.ResponseWriter, r *http.Request) {
	skills := h.sdk.skills.All()

	type SkillInfo struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Triggers []string `json:"triggers"`
		Intents  []string `json:"intents"`
	}

	var skillInfos []SkillInfo
	for _, skill := range skills {
		skillInfos = append(skillInfos, SkillInfo{
			ID:       skill.ID,
			Name:     skill.Name,
			Triggers: skill.Triggers,
			Intents:  skill.Intents,
		})
	}

	h.writeJSON(w, http.StatusOK, skillInfos)
}

func (h *httpHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *httpHandler) writeError(w http.ResponseWriter, status int, message, code string, details map[string]any) {
	resp := ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	}
	h.writeJSON(w, status, resp)
}

func (h *httpHandler) handleError(w http.ResponseWriter, err error) {
	if sdkErr, ok := err.(*SDKError); ok {
		status := h.errorCodeToStatus(sdkErr.Code)
		h.writeError(w, status, sdkErr.Message, sdkErr.Code, sdkErr.Details)
		return
	}

	h.writeError(w, http.StatusInternalServerError, err.Error(), ErrCodeInternal, nil)
}

func (h *httpHandler) errorCodeToStatus(code string) int {
	switch code {
	case ErrCodeValidation:
		return http.StatusBadRequest
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeRouting:
		return http.StatusBadRequest
	case ErrCodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func (h *httpHandler) writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	dataBytes, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", dataBytes)
	flusher.Flush()
}

// StreamingChatResponse allows reading streaming responses.
type StreamingChatResponse struct {
	reader io.ReadCloser
}

// ReadEvent reads the next SSE event from the stream.
func (s *StreamingChatResponse) ReadEvent() (string, json.RawMessage, error) {
	// Implementation would parse SSE format
	return "", nil, io.EOF
}

// Close closes the streaming response.
func (s *StreamingChatResponse) Close() error {
	return s.reader.Close()
}

// HTTPClient provides a client for the SDK's HTTP API.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPClient creates a new HTTP client.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Chat sends a chat request.
func (c *HTTPClient) Chat(ctx context.Context, req ChatHTTPRequest) (*ChatHTTPResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("request failed: %s", errResp.Error)
	}

	var result ChatHTTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ChatStream sends a streaming chat request.
func (c *HTTPClient) ChatStream(ctx context.Context, req ChatHTTPRequest) (*StreamingChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/stream", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	return &StreamingChatResponse{reader: resp.Body}, nil
}

// SendFeedback sends feedback for a message.
func (c *HTTPClient) SendFeedback(ctx context.Context, messageID string, rating int, comment string) error {
	req := FeedbackHTTPRequest{
		MessageID: messageID,
		Rating:    rating,
		Comment:   comment,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/feedback", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("request failed: %s", errResp.Error)
	}

	return nil
}
