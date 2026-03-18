package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DocEntry describes one curated official docs page.
type DocEntry struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

// DocsOverlay holds the askplanner-local official docs prompt overlay.
type DocsOverlay struct {
	SkillMD   string
	Docs      []DocEntry
	Available bool
	Warning   string
}

// LoadDocsOverlay loads the local prompt overlay and validates all manifest entries.
func LoadDocsOverlay(overlayDir, docsDir string) *DocsOverlay {
	overlay := &DocsOverlay{}

	if overlayDir == "" {
		overlay.Warning = "official docs overlay disabled: DOCS_OVERLAY_DIR is empty"
		return overlay
	}
	if docsDir == "" {
		overlay.Warning = "official docs overlay disabled: TIDB_DOCS_DIR is empty"
		return overlay
	}

	skillPath := filepath.Join(overlayDir, "SKILL.md")
	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: read %s: %v", skillPath, err)
		return overlay
	}
	if strings.TrimSpace(string(skillData)) == "" {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: %s is empty", skillPath)
		return overlay
	}

	manifestPath := filepath.Join(overlayDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: read %s: %v", manifestPath, err)
		return overlay
	}

	var docs []DocEntry
	if err := json.Unmarshal(manifestData, &docs); err != nil {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: parse %s: %v", manifestPath, err)
		return overlay
	}
	if len(docs) == 0 {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: %s has no entries", manifestPath)
		return overlay
	}

	absDocsDir, err := filepath.Abs(docsDir)
	if err != nil {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: resolve docs dir %s: %v", docsDir, err)
		return overlay
	}

	for i, doc := range docs {
		doc.Title = strings.TrimSpace(doc.Title)
		doc.Path = filepath.Clean(strings.TrimSpace(doc.Path))

		if doc.Title == "" {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: manifest entry %d has empty title", i)
			return overlay
		}
		if doc.Path == "." || doc.Path == "" {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: manifest entry %q has empty path", doc.Title)
			return overlay
		}
		if filepath.IsAbs(doc.Path) {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: manifest path %q must be relative", doc.Path)
			return overlay
		}

		resolved, err := resolveDocPath(absDocsDir, doc.Path)
		if err != nil {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: %v", err)
			return overlay
		}

		info, err := os.Stat(resolved)
		if err != nil {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: stat %s: %v", doc.Path, err)
			return overlay
		}
		if info.IsDir() {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: %s is a directory", doc.Path)
			return overlay
		}

		docs[i] = doc
	}

	overlay.SkillMD = string(skillData)
	overlay.Docs = docs
	overlay.Available = true
	return overlay
}

// SystemPromptSection renders the docs overlay portion of the system prompt.
func (o *DocsOverlay) SystemPromptSection() string {
	if o == nil || !o.Available {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Official SQL Tuning Docs\n\n")
	sb.WriteString(o.SkillMD)
	sb.WriteString("\n\n### Curated Official Pages\n")
	sb.WriteString("Use `search_docs` to find the right official page, then `read_file` with the returned path for exact details.\n\n")
	for _, doc := range o.Docs {
		fmt.Fprintf(&sb, "- %s (%s)\n", doc.Title, doc.Path)
	}
	return sb.String()
}

func resolveDocPath(docsDir, relPath string) (string, error) {
	joined := filepath.Join(docsDir, relPath)
	resolved, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", relPath, err)
	}

	rel, err := filepath.Rel(docsDir, resolved)
	if err != nil {
		return "", fmt.Errorf("relativize %s: %w", relPath, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("manifest path %q escapes docs root", relPath)
	}

	return resolved, nil
}
