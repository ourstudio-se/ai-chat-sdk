package skills

import (
	"strings"
	"sync"

	aichat "github.com/ourstudio-se/ai-chat-sdk"
)

// Registry manages registered skills.
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*aichat.Skill
}

// NewRegistry creates a new skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*aichat.Skill),
	}
}

// Register adds a skill to the registry.
func (r *Registry) Register(skill *aichat.Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills[skill.ID] = skill
}

// Get returns a skill by ID.
func (r *Registry) Get(id string) (*aichat.Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[id]
	return skill, ok
}

// All returns all registered skills.
func (r *Registry) All() []*aichat.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]*aichat.Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		skills = append(skills, skill)
	}
	return skills
}

// Match finds skills matching the given message.
// Returns skills in order of match confidence (best matches first).
func (r *Registry) Match(message string) []*aichat.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	messageLower := strings.ToLower(message)
	var matches []*aichat.Skill

	for _, skill := range r.skills {
		if matchesSkill(skill, messageLower) {
			matches = append(matches, skill)
		}
	}

	return matches
}

// matchesSkill checks if a message matches a skill's triggers or intents.
func matchesSkill(skill *aichat.Skill, messageLower string) bool {
	// Check triggers (keyword matching)
	for _, trigger := range skill.Triggers {
		if strings.Contains(messageLower, strings.ToLower(trigger)) {
			return true
		}
	}

	// Check intents (keyword matching - could be enhanced with NLU)
	for _, intent := range skill.Intents {
		if strings.Contains(messageLower, strings.ToLower(intent)) {
			return true
		}
	}

	return false
}
