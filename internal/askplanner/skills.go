package askplanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Index holds the pre-scanned skill metadata for the system prompt.
type Index struct {
	SkillMD        string   // content of SKILL.md
	CoreFiles      []string // core reference .md filenames
	OncallFiles    []string // oncall experience filenames
	OncallPartial  []string // partial oncall filenames
	CustomerIssues int      // count of customer issue files
}

// BuildIndex scans the skills directory and builds an index.
func BuildIndex(skillsDir string) (*Index, error) {
	idx := &Index{}

	// Load SKILL.md from parent directory
	skillMDPath := filepath.Join(filepath.Dir(skillsDir), "SKILL.md")
	data, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}
	idx.SkillMD = string(data)

	// Core reference files
	idx.CoreFiles = listMD(skillsDir)

	// Oncall experiences
	oncallDir := filepath.Join(skillsDir, "optimizer-oncall-experiences-redacted")
	idx.OncallFiles = listMD(oncallDir)
	idx.OncallPartial = listMD(filepath.Join(oncallDir, "partial"))

	// Customer issues count
	issuesDir := filepath.Join(skillsDir, "tidb-customer-planner-issues")
	idx.CustomerIssues = len(listMD(issuesDir))

	return idx, nil
}

// SystemPromptSection returns the skills portion of the system prompt.
func (idx *Index) SystemPromptSection() string {
	var sb strings.Builder

	sb.WriteString(idx.SkillMD)
	sb.WriteString("\n\n---\n\n")

	sb.WriteString("## Available Skills Index\n\n")
	sb.WriteString("Use `list_skills` and `read_skill` tools to access these files.\n\n")

	sb.WriteString("### Core References\n")
	for _, f := range idx.CoreFiles {
		fmt.Fprintf(&sb, "- %s\n", f)
	}

	sb.WriteString("\n### Oncall Experiences\n")
	for _, f := range idx.OncallFiles {
		fmt.Fprintf(&sb, "- oncall/%s\n", f)
	}
	if len(idx.OncallPartial) > 0 {
		fmt.Fprintf(&sb, "- oncall/partial/ (%d partial cases)\n", len(idx.OncallPartial))
	}

	fmt.Fprintf(&sb, "\n### Customer Planner Issues\n")
	fmt.Fprintf(&sb, "- %d customer-reported planner issues (use list_skills with category='customer-issues' to browse, or search by keyword)\n", idx.CustomerIssues)

	return sb.String()
}

func listMD(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	return files
}
