package skills

import (
	"fmt"
	"strings"
	"sync"
)

// Registry manages skills and their variants
type Registry struct {
	skills   map[string][]*Skill // ID -> variants
	fallback *Skill
	mu       sync.RWMutex
}

// NewRegistry creates a new skill registry
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string][]*Skill),
	}
}

// Register adds a skill to the registry
func (r *Registry) Register(skill *Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.ID] = append(r.skills[skill.ID], skill)
}

// SetFallback sets the fallback skill used when no skills match
func (r *Registry) SetFallback(skill *Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = skill
}

// Count returns the number of unique skill IDs registered
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.skills)
}

// Get returns all variants of a skill
func (r *Registry) Get(id string) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skills[id]
}

// GetVariant returns a specific variant of a skill
func (r *Registry) GetVariant(id, variant string) *Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	variants := r.skills[id]
	for _, s := range variants {
		if s.Variant == variant {
			return s
		}
	}

	// Return first variant if requested variant not found
	if len(variants) > 0 {
		return variants[0]
	}
	return nil
}

// All returns all skills (one per ID, using provided variant selector)
func (r *Registry) All(variantSelector func(id string, variants []*Skill) *Skill) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Skill
	for id, variants := range r.skills {
		if len(variants) > 0 {
			if variantSelector != nil {
				result = append(result, variantSelector(id, variants))
			} else {
				result = append(result, variants[0])
			}
		}
	}
	return result
}

// Select returns skills matching the user message
func (r *Registry) Select(userMessage string, variantSelector func(id string, variants []*Skill) *Skill) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	msgLower := strings.ToLower(userMessage)
	var matched []*Skill
	seen := make(map[string]bool)

	for id, variants := range r.skills {
		if seen[id] || len(variants) == 0 {
			continue
		}

		// Check first variant for matching (they all share intents/triggers)
		skill := variants[0]

		// Check triggers
		for _, trigger := range skill.Triggers {
			if strings.Contains(msgLower, strings.ToLower(trigger)) {
				var selected *Skill
				if variantSelector != nil {
					selected = variantSelector(id, variants)
				} else {
					selected = variants[0]
				}
				matched = append(matched, selected)
				seen[id] = true
				break
			}
		}

		if seen[id] {
			continue
		}

		// Check intents
		for _, intent := range skill.Intents {
			if strings.Contains(msgLower, strings.ToLower(intent)) {
				var selected *Skill
				if variantSelector != nil {
					selected = variantSelector(id, variants)
				} else {
					selected = variants[0]
				}
				matched = append(matched, selected)
				seen[id] = true
				break
			}
		}
	}

	// Limit to 3 skills
	if len(matched) > 3 {
		matched = matched[:3]
	}

	// Use fallback if nothing matched
	if len(matched) == 0 && r.fallback != nil {
		matched = append(matched, r.fallback)
	}

	return matched
}

// FormatForPrompt renders skills into system prompt content
func FormatForPrompt(skills []*Skill) string {
	var sb strings.Builder

	for _, skill := range skills {
		sb.WriteString(fmt.Sprintf("## %s\n", skill.Name))
		sb.WriteString(skill.Instructions)
		sb.WriteString("\n\n")

		if len(skill.Examples) > 0 {
			sb.WriteString("### Examples\n")
			for _, ex := range skill.Examples {
				sb.WriteString(fmt.Sprintf("User: %s\nAssistant: %s\n\n", ex.User, ex.Assistant))
			}
		}

		if len(skill.Guardrails) > 0 {
			sb.WriteString("### Avoid\n")
			for _, g := range skill.Guardrails {
				sb.WriteString(fmt.Sprintf("- %s\n", g))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
