package aichat

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status string `json:"status"`
}

// newHealthHandler returns a handler for health check requests.
func newHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
	}
}

// newChatHandler returns a handler for POST /chat requests.
func newChatHandler(processChat ProcessChatFn, maxMessageLength int, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Parse request
		var httpReq HTTPChatRequest
		if err := json.NewDecoder(r.Body).Decode(&httpReq); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// 2. Validate
		if httpReq.Message == "" {
			respondError(w, http.StatusBadRequest, "Message cannot be empty")
			return
		}

		if len(httpReq.Message) > maxMessageLength {
			respondError(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("Message exceeds maximum length of %d characters", maxMessageLength))
			return
		}

		// 3. Convert to service request
		serviceReq := ChatRequest{
			Message:        httpReq.Message,
			ConversationID: stringValue(httpReq.ConversationID),
			EntityID:       stringValue(httpReq.EntityID),
		}

		// 4. Call service (business logic)
		result, err := processChat(r.Context(), serviceReq)
		if err != nil {
			logger.Error("failed to process chat message", "error", err)
			respondError(w, http.StatusInternalServerError, "An error occurred while processing your message")
			return
		}

		// 5. Build HTTP response
		response := buildChatResponse(result, httpReq.Message)
		respondJSON(w, http.StatusOK, response)
	}
}

// newChatStreamHandler returns a handler for POST /chat/stream requests with SSE.
func newChatStreamHandler(processChat ProcessChatFn, maxMessageLength int, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Parse request
		var httpReq HTTPChatRequest
		if err := json.NewDecoder(r.Body).Decode(&httpReq); err != nil {
			sendStreamEvent(w, errorStreamEvent("Invalid request body"), logger)
			return
		}

		// 2. Validate
		if httpReq.Message == "" {
			sendStreamEvent(w, errorStreamEvent("Message cannot be empty"), logger)
			return
		}

		if len(httpReq.Message) > maxMessageLength {
			sendStreamEvent(w, errorStreamEvent(
				fmt.Sprintf("Message exceeds maximum length of %d characters", maxMessageLength)), logger)
			return
		}

		// 3. Set SSE headers
		setSSEHeaders(w)

		// 4. Convert to service request
		serviceReq := ChatRequest{
			Message:        httpReq.Message,
			ConversationID: stringValue(httpReq.ConversationID),
			EntityID:       stringValue(httpReq.EntityID),
		}

		// 5. Send "thinking" event
		sendStreamEvent(w, StreamEvent{
			Type: EventThinking,
		}, logger)
		flush(w)

		// 6. Call service (business logic)
		result, err := processChat(r.Context(), serviceReq)
		if err != nil {
			logger.Error("failed to process chat message", "error", err)
			sendStreamEvent(w, errorStreamEvent("An error occurred while processing your message"), logger)
			return
		}

		// 7. Send "done" event
		sendStreamEvent(w, buildDoneStreamEvent(result), logger)
	}
}

func buildChatResponse(result *ChatResult, message string) HTTPChatResponse {
	return HTTPChatResponse{
		ConversationID: result.ConversationID,
		Expert:         result.ExpertResult.ExpertType,
		ExpertName:     result.ExpertResult.ExpertName,
		Message:        message,
		Reasoning:      result.ExpertResult.Reasoning,
		Response:       result.ExpertResult.Answer,
	}
}

func buildDoneStreamEvent(result *ChatResult) StreamEvent {
	expertType := result.ExpertResult.ExpertType
	return StreamEvent{
		Type:           EventDone,
		ConversationID: &result.ConversationID,
		Expert:         &expertType,
		ExpertName:     &result.ExpertResult.ExpertName,
		Content:        &result.ExpertResult.Answer,
	}
}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func sendStreamEvent(w http.ResponseWriter, event StreamEvent, logger *slog.Logger) {
	data, err := json.Marshal(event)
	if err != nil {
		logger.Error("failed to marshal stream event", "error", err)
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"content\":\"Internal serialization error\"}\n\n")
		flush(w)
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flush(w)
}

func errorStreamEvent(message string) StreamEvent {
	return StreamEvent{
		Type:    EventError,
		Content: stringPtr(message),
	}
}

func flush(w http.ResponseWriter) {
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringPtr(s string) *string {
	return &s
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// newHTTPRouter creates and configures the Chi router with all middleware and routes.
func newHTTPRouter(
	allowedOrigins []string,
	requestTimeout time.Duration,
	maxRequestBodySize int64,
	logger *slog.Logger,
	healthHandler http.HandlerFunc,
	chatHandler http.HandlerFunc,
	chatStreamHandler http.HandlerFunc,
) *chi.Mux {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(requestIDMiddleware)
	r.Use(recoveryMiddleware(logger))
	r.Use(loggingMiddleware(logger))
	r.Use(chimiddleware.RealIP)
	r.Use(timeoutMiddleware(requestTimeout))
	r.Use(bodySizeLimitMiddleware(maxRequestBodySize))

	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	}))

	// Routes
	r.Get("/health", healthHandler)
	r.Post("/chat", chatHandler)
	r.Post("/chat/stream", chatStreamHandler)

	return r
}
