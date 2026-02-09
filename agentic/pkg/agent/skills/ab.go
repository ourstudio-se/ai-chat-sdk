package skills

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"strings"
	"time"
)

// ABSelector provides A/B test variant selection strategies
type ABSelector struct {
	rng *rand.Rand
}

// NewABSelector creates a new A/B selector
func NewABSelector() *ABSelector {
	return &ABSelector{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Random selects a random variant
func (s *ABSelector) Random() func(id string, variants []*Skill) *Skill {
	return func(id string, variants []*Skill) *Skill {
		if len(variants) == 0 {
			return nil
		}
		return variants[s.rng.Intn(len(variants))]
	}
}

// Weighted selects a variant based on weights
func (s *ABSelector) Weighted() func(id string, variants []*Skill) *Skill {
	return func(id string, variants []*Skill) *Skill {
		if len(variants) == 0 {
			return nil
		}

		totalWeight := 0
		for _, v := range variants {
			w := v.Weight
			if w <= 0 {
				w = 1
			}
			totalWeight += w
		}

		r := s.rng.Intn(totalWeight)
		cumulative := 0
		for _, v := range variants {
			w := v.Weight
			if w <= 0 {
				w = 1
			}
			cumulative += w
			if r < cumulative {
				return v
			}
		}
		return variants[0]
	}
}

// Sticky selects a consistent variant based on user/session ID (for consistent UX)
func (s *ABSelector) Sticky(identifier string) func(id string, variants []*Skill) *Skill {
	return func(id string, variants []*Skill) *Skill {
		if len(variants) == 0 {
			return nil
		}

		// Hash identifier + skill ID for deterministic selection
		h := sha256.New()
		h.Write([]byte(identifier))
		h.Write([]byte(id))
		sum := h.Sum(nil)
		idx := binary.BigEndian.Uint64(sum[:8]) % uint64(len(variants))

		return variants[idx]
	}
}

// Fixed always selects a specific variant
func (s *ABSelector) Fixed(variant string) func(id string, variants []*Skill) *Skill {
	return func(id string, variants []*Skill) *Skill {
		for _, v := range variants {
			if v.Variant == variant {
				return v
			}
		}
		// Fallback to first
		if len(variants) > 0 {
			return variants[0]
		}
		return nil
	}
}

// VariantInfo returns which variant was selected (for tracking)
func VariantInfo(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}
	// Return variant of first skill (or combination)
	variants := make([]string, 0, len(skills))
	for _, s := range skills {
		v := s.Variant
		if v == "" {
			v = "default"
		}
		variants = append(variants, s.ID+":"+v)
	}
	return strings.Join(variants, ",")
}
