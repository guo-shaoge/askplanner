package askplanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListSkillsTool lists available skill files.
type ListSkillsTool struct {
	skillsDir string
}

func NewListSkillsTool(skillsDir string) *ListSkillsTool {
	return &ListSkillsTool{skillsDir: skillsDir}
}

func (t *ListSkillsTool) Name() string { return "list_skills" }

func (t *ListSkillsTool) Description() string {
	return "List available TiDB optimizer skill/reference files. Filter by category: 'core' (main references), 'oncall' (oncall experiences), 'customer-issues' (customer planner issues). Without a category, lists all categories with file counts."
}

func (t *ListSkillsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"category": map[string]any{
				"type":        "string",
				"description": "Category filter: 'core', 'oncall', or 'customer-issues'. Empty = show all categories.",
				"enum":        []string{"core", "oncall", "customer-issues"},
			},
		},
	}
}

func (t *ListSkillsTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Category string `json:"category"`
	}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}

	var sb strings.Builder

	switch args.Category {
	case "core", "":
		files := listMDFiles(t.skillsDir)
		if args.Category == "" {
			fmt.Fprintf(&sb, "CORE REFERENCES (%d files):\n", len(files))
		}
		for _, f := range files {
			fmt.Fprintf(&sb, "  - %s\n", f)
		}

	default:
		// handled below
	}

	oncallDir := filepath.Join(t.skillsDir, "optimizer-oncall-experiences-redacted")
	if args.Category == "oncall" || args.Category == "" {
		files := listMDFiles(oncallDir)
		partial := listMDFiles(filepath.Join(oncallDir, "partial"))
		if args.Category == "" {
			fmt.Fprintf(&sb, "\nONCALL EXPERIENCES (%d files, %d partial):\n", len(files), len(partial))
		}
		for _, f := range files {
			fmt.Fprintf(&sb, "  - oncall/%s\n", f)
		}
		if len(partial) > 0 {
			fmt.Fprintf(&sb, "  Partial (%d files):\n", len(partial))
			for _, f := range partial {
				fmt.Fprintf(&sb, "  - oncall/partial/%s\n", f)
			}
		}
	}

	issuesDir := filepath.Join(t.skillsDir, "tidb-customer-planner-issues")
	if args.Category == "customer-issues" || args.Category == "" {
		files := listMDFiles(issuesDir)
		if args.Category == "" {
			fmt.Fprintf(&sb, "\nCUSTOMER PLANNER ISSUES (%d files):\n", len(files))
			// Only show first 20 for overview
			limit := 20
			if len(files) < limit {
				limit = len(files)
			}
			for _, f := range files[:limit] {
				fmt.Fprintf(&sb, "  - issues/%s\n", f)
			}
			if len(files) > 20 {
				fmt.Fprintf(&sb, "  ... and %d more (use category='customer-issues' to see all)\n", len(files)-20)
			}
		} else {
			for _, f := range files {
				fmt.Fprintf(&sb, "  - issues/%s\n", f)
			}
		}
	}

	return sb.String(), nil
}

// ReadSkillTool reads a specific skill file.
type ReadSkillTool struct {
	skillsDir string
}

func NewReadSkillTool(skillsDir string) *ReadSkillTool {
	return &ReadSkillTool{skillsDir: skillsDir}
}

func (t *ReadSkillTool) Name() string { return "read_skill" }

func (t *ReadSkillTool) Description() string {
	return "Read the content of a specific skill/reference file. Use the name from list_skills output. Prefix with 'oncall/' for oncall experiences, 'oncall/partial/' for partial oncall cases, or 'issues/' for customer issues."
}

func (t *ReadSkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"required": []string{"name"},
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Skill file name, e.g. 'join-strategies.md', 'oncall/stale-stats-after-bulk-data-change.md', or 'issues/issue-12345-description.md'",
			},
		},
	}
}

func (t *ReadSkillTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	// Resolve the name to a file path
	var filePath string
	name := filepath.Clean(args.Name)

	if strings.HasPrefix(name, "oncall/") {
		filePath = filepath.Join(t.skillsDir, "optimizer-oncall-experiences-redacted", strings.TrimPrefix(name, "oncall/"))
	} else if strings.HasPrefix(name, "issues/") {
		filePath = filepath.Join(t.skillsDir, "tidb-customer-planner-issues", strings.TrimPrefix(name, "issues/"))
	} else {
		filePath = filepath.Join(t.skillsDir, name)
	}

	// Ensure the resolved path stays within the skills directory
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	absSkills, err := filepath.Abs(t.skillsDir)
	if err != nil {
		return "", fmt.Errorf("resolve skills dir: %w", err)
	}
	if !strings.HasPrefix(absPath, absSkills) {
		return "", fmt.Errorf("path %q is outside skills directory", args.Name)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read skill file: %w", err)
	}

	return fmt.Sprintf("Skill: %s\n\n%s", args.Name, string(data)), nil
}

// listMDFiles returns .md filenames in a directory (non-recursive).
func listMDFiles(dir string) []string {
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
