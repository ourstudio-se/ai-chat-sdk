package server

import (
	"net/http"

	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent"
	"github.com/ourstudio-se/ai-chat-sdk/agentic/pkg/agent/feedback"
)

// Server is an HTTP server for the agent
type Server struct {
	router        *chi.Mux
	agent         *agent.Agent
	feedbackStore feedback.Store // Optional
}

// Config for the server
type Config struct {
	CORSOrigins []string
}

// Option configures the server
type Option func(*Server)

// WithFeedback enables feedback collection
func WithFeedback(store feedback.Store) Option {
	return func(s *Server) {
		s.feedbackStore = store
	}
}

// New creates a new HTTP server
func New(ag *agent.Agent, cfg Config, opts ...Option) *Server {
	s := &Server{
		router: chi.NewRouter(),
		agent:  ag,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.setupMiddleware(cfg)
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware(cfg Config) {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))

	origins := cfg.CORSOrigins
	if len(origins) == 0 {
		origins = []string{"*"}
	}

	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

func (s *Server) setupRoutes() {
	s.router.Get("/health", s.healthHandler)

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Post("/chat", s.chatHandler)
		r.Get("/conversations/{id}", s.getConversationHandler)
		r.Delete("/conversations/{id}", s.deleteConversationHandler)

		// Feedback routes (only if enabled)
		if s.feedbackStore != nil {
			r.Post("/feedback", s.submitFeedbackHandler)
			r.Get("/conversations/{id}/feedback", s.getSessionFeedbackHandler)
		}
	})
}

// Handler returns the HTTP handler
func (s *Server) Handler() http.Handler {
	return s.router
}

// ListenAndServe starts the server
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.router)
}
