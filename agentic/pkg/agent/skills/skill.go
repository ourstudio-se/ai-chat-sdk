package skills

// Skill defines agent behavior for specific intents
type Skill struct {
	ID           string    `yaml:"id" json:"id"`
	Name         string    `yaml:"name" json:"name"`
	Description  string    `yaml:"description" json:"description"`
	Intents      []string  `yaml:"intents" json:"intents"`
	Triggers     []string  `yaml:"triggers" json:"triggers"`
	Instructions string    `yaml:"instructions" json:"instructions"`
	Examples     []Example `yaml:"examples" json:"examples"`
	Guardrails   []string  `yaml:"guardrails" json:"guardrails,omitempty"`
	Tools        []string  `yaml:"tools" json:"tools,omitempty"`

	// A/B testing
	Variant string `yaml:"variant" json:"variant,omitempty"`
	Weight  int    `yaml:"weight" json:"weight,omitempty"`
}

// Example shows the skill in action
type Example struct {
	User      string `yaml:"user" json:"user"`
	Assistant string `yaml:"assistant" json:"assistant"`
}

// SkillFile represents a YAML file with skill variants
type SkillFile struct {
	ID       string  `yaml:"id"`
	Variants []Skill `yaml:"variants"`
}
