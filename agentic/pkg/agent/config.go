package agent

import "time"

// Config holds agent configuration
type Config struct {
	// LLM settings
	Model       string  `yaml:"model" json:"model"`
	MaxTokens   int     `yaml:"maxTokens" json:"maxTokens"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	MaxTurns    int     `yaml:"maxTurns" json:"maxTurns"`

	// System prompt components
	BasePrompt string `yaml:"basePrompt" json:"basePrompt"`

	// Timeouts
	RequestTimeout time.Duration `yaml:"requestTimeout" json:"requestTimeout"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Model:          "claude-sonnet-4-5",
		MaxTokens:      4096,
		Temperature:    0.7,
		MaxTurns:       10,
		RequestTimeout: 60 * time.Second,
		BasePrompt:     "",
	}
}

// WithModel sets the model
func (c Config) WithModel(model string) Config {
	c.Model = model
	return c
}

// WithMaxTokens sets max tokens
func (c Config) WithMaxTokens(tokens int) Config {
	c.MaxTokens = tokens
	return c
}

// WithMaxTurns sets max turns for agent loop
func (c Config) WithMaxTurns(turns int) Config {
	c.MaxTurns = turns
	return c
}

// WithBasePrompt sets the base system prompt
func (c Config) WithBasePrompt(prompt string) Config {
	c.BasePrompt = prompt
	return c
}
