package aichat

import (
	"strings"
)

// Router routes incoming messages to appropriate skills.
type Router struct {
	skills         SkillRegistry
	defaultSkillID string
}

// NewRouter creates a new router.
func NewRouter(skills SkillRegistry, defaultSkillID string) *Router {
	return &Router{
		skills:         skills,
		defaultSkillID: defaultSkillID,
	}
}

// Route finds the best skill for a message.
func (r *Router) Route(message string) *Skill {
	// Find matching skills
	matches := r.skills.Match(message)

	if len(matches) > 0 {
		// Return the first match (could be enhanced with scoring)
		return matches[0]
	}

	// Fall back to default skill
	if r.defaultSkillID != "" {
		if skill, ok := r.skills.Get(r.defaultSkillID); ok {
			return skill
		}
	}

	return nil
}

// RouteWithScore finds the best skill and returns a confidence score.
func (r *Router) RouteWithScore(message string) (*Skill, float64) {
	matches := r.skills.Match(message)

	if len(matches) == 0 {
		if r.defaultSkillID != "" {
			if skill, ok := r.skills.Get(r.defaultSkillID); ok {
				return skill, 0.1 // Low confidence for default
			}
		}
		return nil, 0
	}

	// Calculate score based on trigger matches
	messageLower := strings.ToLower(message)
	bestSkill := matches[0]
	bestScore := calculateScore(bestSkill, messageLower)

	for _, skill := range matches[1:] {
		score := calculateScore(skill, messageLower)
		if score > bestScore {
			bestSkill = skill
			bestScore = score
		}
	}

	return bestSkill, bestScore
}

// calculateScore calculates a match score for a skill.
func calculateScore(skill *Skill, messageLower string) float64 {
	var score float64
	words := strings.Fields(messageLower)
	totalWords := float64(len(words))
	if totalWords == 0 {
		return 0
	}

	// Count trigger matches
	triggerMatches := 0
	for _, trigger := range skill.Triggers {
		triggerLower := strings.ToLower(trigger)
		for _, word := range words {
			if word == triggerLower || strings.Contains(word, triggerLower) {
				triggerMatches++
			}
		}
	}

	// Count intent matches
	intentMatches := 0
	for _, intent := range skill.Intents {
		intentLower := strings.ToLower(intent)
		for _, word := range words {
			if word == intentLower || strings.Contains(word, intentLower) {
				intentMatches++
			}
		}
	}

	// Weight triggers higher than intents
	score = (float64(triggerMatches)*2 + float64(intentMatches)) / totalWords

	// Normalize to 0-1 range
	if score > 1 {
		score = 1
	}

	return score
}
