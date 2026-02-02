package aichat

import (
	"net/http"

	openai "github.com/sashabaranov/go-openai"
)

const (
	// OpenRouterBaseURL is the base URL for OpenRouter API.
	OpenRouterBaseURL = "https://openrouter.ai/api/v1"
)

// OpenRouterConfig holds configuration for creating an OpenRouter client.
type OpenRouterConfig struct {
	// APIKey is your OpenRouter API key (required).
	APIKey string

	// SiteURL is your site URL for OpenRouter rankings (optional but recommended).
	// This helps OpenRouter track which apps are using which models.
	SiteURL string

	// SiteName is your site/app name for OpenRouter rankings (optional but recommended).
	SiteName string
}

// NewOpenRouterClient creates an OpenAI-compatible client configured for OpenRouter.
// OpenRouter provides access to various LLM models through an OpenAI-compatible API.
func NewOpenRouterClient(cfg OpenRouterConfig) *openai.Client {
	config := openai.DefaultConfig(cfg.APIKey)
	config.BaseURL = OpenRouterBaseURL

	// Add custom headers if provided
	if cfg.SiteURL != "" || cfg.SiteName != "" {
		config.HTTPClient = &http.Client{
			Transport: &openRouterTransport{
				base:     http.DefaultTransport,
				siteURL:  cfg.SiteURL,
				siteName: cfg.SiteName,
			},
		}
	}

	return openai.NewClientWithConfig(config)
}

// openRouterTransport adds OpenRouter-specific headers to requests.
type openRouterTransport struct {
	base     http.RoundTripper
	siteURL  string
	siteName string
}

func (t *openRouterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid mutating the original
	req2 := req.Clone(req.Context())

	if t.siteURL != "" {
		req2.Header.Set("HTTP-Referer", t.siteURL)
	}
	if t.siteName != "" {
		req2.Header.Set("X-Title", t.siteName)
	}

	return t.base.RoundTrip(req2)
}

// OpenRouterModels provides model name mappings for popular OpenRouter models.
// Use these with Config.ModelMap to route to specific models.
var OpenRouterModels = struct {
	// Claude models
	Claude35Sonnet string
	Claude35Haiku  string
	Claude3Opus    string

	// GPT models
	GPT4o     string
	GPT4oMini string
	GPT4Turbo string

	// Open source models
	Llama31405B string
	Llama3170B  string
	Llama318B   string
	Mixtral8x7B string
	Mistral7B   string

	// Google models
	Gemini15Pro   string
	Gemini15Flash string
}{
	// Claude models
	Claude35Sonnet: "anthropic/claude-3.5-sonnet",
	Claude35Haiku:  "anthropic/claude-3.5-haiku",
	Claude3Opus:    "anthropic/claude-3-opus",

	// GPT models
	GPT4o:     "openai/gpt-4o",
	GPT4oMini: "openai/gpt-4o-mini",
	GPT4Turbo: "openai/gpt-4-turbo",

	// Open source models
	Llama31405B: "meta-llama/llama-3.1-405b-instruct",
	Llama3170B:  "meta-llama/llama-3.1-70b-instruct",
	Llama318B:   "meta-llama/llama-3.1-8b-instruct",
	Mixtral8x7B: "mistralai/mixtral-8x7b-instruct",
	Mistral7B:   "mistralai/mistral-7b-instruct",

	// Google models
	Gemini15Pro:   "google/gemini-pro-1.5",
	Gemini15Flash: "google/gemini-flash-1.5",
}

// DefaultOpenRouterModelMap returns a model map suitable for OpenRouter
// using Claude 3.5 models for high-quality responses.
func DefaultOpenRouterModelMap() map[ModelTier]string {
	return map[ModelTier]string{
		ModelNano:      OpenRouterModels.Claude35Haiku,
		ModelMini:      OpenRouterModels.Claude35Haiku,
		ModelStandard:  OpenRouterModels.Claude35Sonnet,
		ModelReasoning: OpenRouterModels.Claude35Sonnet,
	}
}

// GPTOpenRouterModelMap returns a model map for OpenRouter using OpenAI GPT models.
func GPTOpenRouterModelMap() map[ModelTier]string {
	return map[ModelTier]string{
		ModelNano:      OpenRouterModels.GPT4oMini,
		ModelMini:      OpenRouterModels.GPT4oMini,
		ModelStandard:  OpenRouterModels.GPT4o,
		ModelReasoning: OpenRouterModels.GPT4o,
	}
}
