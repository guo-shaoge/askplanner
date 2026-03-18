package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	defaultDocSearchLimit = 10
	maxDocSearchLimit     = 20
)

// SearchDocsTool searches curated official TiDB docs by keyword matching
// against document headings, titles, and summaries.
type SearchDocsTool struct {
	projectRoot string
	docsDir     string
	overlay     *DocsOverlay
}

func NewSearchDocsTool(projectRoot, docsDir string, overlay *DocsOverlay) *SearchDocsTool {
	return &SearchDocsTool{
		projectRoot: projectRoot,
		docsDir:     docsDir,
		overlay:     overlay,
	}
}

func (t *SearchDocsTool) Name() string { return "search_docs" }

func (t *SearchDocsTool) Description() string {
	return "Search official TiDB SQL tuning docs by keywords. Matches doc titles, summaries, and section headings. Returns matching sections with file paths and line numbers for use with read_file."
}

func (t *SearchDocsTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"query"},
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Keywords or topic to search for in official docs headings and titles (e.g. 'reload binding', 'statistics health', 'hash join hint')",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results. Default: 10, maximum: 20.",
			},
		},
	}
}

func (t *SearchDocsTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if t.overlay == nil || !t.overlay.Available {
		return "", fmt.Errorf("official docs overlay is not available")
	}

	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return "", fmt.Errorf("query is required")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = defaultDocSearchLimit
	}
	if limit > maxDocSearchLimit {
		limit = maxDocSearchLimit
	}

	matches := t.overlay.HeadingIndex.Search(args.Query, limit)
	if len(matches) == 0 {
		return "No matches found.", nil
	}

	// Group results by doc for cleaner output.
	type docGroup struct {
		title   string
		path    string
		entries []HeadingMatch
	}
	var groups []docGroup
	groupIdx := make(map[string]int) // path -> index in groups

	for _, m := range matches {
		if idx, ok := groupIdx[m.DocPath]; ok {
			groups[idx].entries = append(groups[idx].entries, m)
		} else {
			groupIdx[m.DocPath] = len(groups)
			groups = append(groups, docGroup{
				title:   m.DocTitle,
				path:    m.DocPath,
				entries: []HeadingMatch{m},
			})
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Docs search: %q (%d results)\n\n", args.Query, len(matches))
	for i, g := range groups {
		fmt.Fprintf(&sb, "%d. %s [%s]\n", i+1, g.title, g.path)
		for _, e := range g.entries {
			levelMark := strings.Repeat("#", e.Level)
			if e.Level == 0 {
				levelMark = "title"
			}
			fmt.Fprintf(&sb, "   - L%d: %s (%s)\n", e.Line, e.Heading, levelMark)
		}
	}
	sb.WriteString("\nUse read_file(path=\"contrib/tidb-docs/<path>\", offset=<line>) to read the section.\n")

	return sb.String(), nil
}
