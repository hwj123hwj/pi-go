package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilterIntegration_ProjectLocalSkillsOnly(t *testing.T) {
	cwd, _ := os.Getwd()
	// Navigate to project root (test is in internal/skill/)
	projectRoot := filepath.Join(cwd, "..", "..")

	skillDirs := []string{}
	defaultSkillDir := filepath.Join(projectRoot, ".claude", "skills")
	if fi, err := os.Stat(defaultSkillDir); err == nil && fi.IsDir() {
		skillDirs = append(skillDirs, defaultSkillDir)
	}
	homeSkillDir := filepath.Join(os.Getenv("HOME"), ".claude", "skills")
	if fi, err := os.Stat(homeSkillDir); err == nil && fi.IsDir() {
		skillDirs = append(skillDirs, homeSkillDir)
	}

	if len(skillDirs) < 2 {
		t.Skip("Need both project-local and global skill dirs for this test")
	}

	result := LoadFromDirs(skillDirs...)
	t.Logf("Total loaded: %d skills", len(result.Skills))

	// Filter: only keep skills under project root
	var filtered []Skill
	seen := make(map[string]int)
	for _, s := range result.Skills {
		rel, err := filepath.Rel(projectRoot, s.FilePath)
		if err != nil || strings.HasPrefix(rel, "..") {
			t.Logf("Filtered out (outside workspace): %s -> %s", s.Name, s.FilePath)
			continue
		}
		if idx, exists := seen[s.Name]; exists {
			if len(s.FilePath) < len(filtered[idx].FilePath) {
				filtered[idx] = s
			}
			t.Logf("Deduplicated: %s", s.Name)
			continue
		}
		seen[s.Name] = len(filtered)
		filtered = append(filtered, s)
	}

	t.Logf("After filter: %d skills", len(filtered))
	for _, s := range filtered {
		t.Logf("  - %s: %s", s.Name, s.FilePath)
	}

	// Generate system prompt section
	prompt := FormatForSystemPrompt(filtered)

	// Verify no global paths
	home := os.Getenv("HOME")
	if strings.Contains(prompt, home+"/.claude") {
		t.Errorf("System prompt still contains global paths!\n%s", prompt)
	}

	// Verify skills are listed
	if !strings.Contains(prompt, "guizang-ppt-skill") {
		t.Error("Expected guizang-ppt-skill in prompt")
	}

	// Print the available_skills section for visual inspection
	start := strings.Index(prompt, "<available_skills>")
	end := strings.Index(prompt, "</available_skills>")
	if start >= 0 && end >= 0 {
		fmt.Println(prompt[start:end+len("</available_skills>")])
	}
}
