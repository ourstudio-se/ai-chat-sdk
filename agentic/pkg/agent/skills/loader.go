package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFromDir loads all skill YAML files from a directory
func LoadFromDir(dir string) (*Registry, error) {
	registry := NewRegistry()

	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("globbing skills dir: %w", err)
	}

	ymlFiles, err := filepath.Glob(filepath.Join(dir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("globbing skills dir: %w", err)
	}
	files = append(files, ymlFiles...)

	for _, file := range files {
		if err := loadFile(registry, file); err != nil {
			return nil, fmt.Errorf("loading %s: %w", file, err)
		}
	}

	return registry, nil
}

func loadFile(registry *Registry, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Try parsing as SkillFile (with variants)
	var skillFile SkillFile
	if err := yaml.Unmarshal(data, &skillFile); err == nil && skillFile.ID != "" {
		for i := range skillFile.Variants {
			skill := &skillFile.Variants[i]
			if skill.ID == "" {
				skill.ID = skillFile.ID
			}
			registry.Register(skill)
		}
		return nil
	}

	// Try parsing as single Skill
	var skill Skill
	if err := yaml.Unmarshal(data, &skill); err != nil {
		return fmt.Errorf("parsing yaml: %w", err)
	}

	if skill.ID == "" {
		// Use filename as ID
		skill.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	registry.Register(&skill)
	return nil
}

// LoadFromFS loads skills from an embedded filesystem
func LoadFromFS(fsys fs.FS, dir string) (*Registry, error) {
	registry := NewRegistry()

	err := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}

		var skillFile SkillFile
		if err := yaml.Unmarshal(data, &skillFile); err == nil && skillFile.ID != "" {
			for i := range skillFile.Variants {
				skill := &skillFile.Variants[i]
				if skill.ID == "" {
					skill.ID = skillFile.ID
				}
				registry.Register(skill)
			}
			return nil
		}

		var skill Skill
		if err := yaml.Unmarshal(data, &skill); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		if skill.ID == "" {
			skill.ID = strings.TrimSuffix(filepath.Base(path), ext)
		}
		registry.Register(&skill)
		return nil
	})

	return registry, err
}
