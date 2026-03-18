package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultDocSearchLimit = 10
	maxDocSearchLimit     = 20
)

// SearchDocsTool searches curated official TiDB docs pages.
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
	return "Search curated official TiDB documentation for SQL tuning features, syntax, optimizer hints, variables, and best practices. Use this before read_file when you need the right official docs page."
}

func (t *SearchDocsTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"pattern"},
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Search pattern (basic regex supported)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of matching lines to return. Default: 10, maximum: 20.",
			},
		},
	}
}

func (t *SearchDocsTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if t.overlay == nil || !t.overlay.Available {
		return "", fmt.Errorf("official docs overlay is not available")
	}

	var args struct {
		Pattern string `json:"pattern"`
		Limit   int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return "", fmt.Errorf("pattern is required")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = defaultDocSearchLimit
	}
	if limit > maxDocSearchLimit {
		limit = maxDocSearchLimit
	}

	files := make([]string, 0, len(t.overlay.Docs))
	titleByPath := make(map[string]string, len(t.overlay.Docs))
	for _, doc := range t.overlay.Docs {
		abs := filepath.Join(t.docsDir, doc.Path)
		files = append(files, abs)
		titleByPath[abs] = doc.Title
	}

	output, err := runScopedSearch(ctx, args.Pattern, files)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(output)) == "" {
		return "No matches found.", nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > limit {
		lines = lines[:limit]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Official docs search: %q (%d results)\n", args.Pattern, len(lines))
	for _, line := range lines {
		path, lineNo, text, ok := parseSearchLine(line)
		if !ok {
			continue
		}

		title := titleByPath[path]
		relPath, err := filepath.Rel(t.projectRoot, path)
		if err != nil {
			relPath = path
		}

		fmt.Fprintf(&sb, "- %s [%s:%d] %s\n", title, relPath, lineNo, strings.TrimSpace(text))
	}
	if len(lines) == limit {
		sb.WriteString("... (results truncated)\n")
	}

	return sb.String(), nil
}

func runScopedSearch(ctx context.Context, pattern string, files []string) ([]byte, error) {
	if rgPath, err := exec.LookPath("rg"); err == nil {
		args := []string{"-n", "--no-heading", pattern}
		args = append(args, files...)
		cmd := exec.CommandContext(ctx, rgPath, args...)
		output, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return nil, nil
			}
			if len(output) == 0 {
				return nil, fmt.Errorf("search docs failed: %w", err)
			}
		}
		return output, nil
	}

	args := []string{"-nH", pattern}
	args = append(args, files...)
	cmd := exec.CommandContext(ctx, "grep", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		if len(output) == 0 {
			return nil, fmt.Errorf("search docs failed: %w", err)
		}
	}
	return output, nil
}

func parseSearchLine(line string) (string, int, string, bool) {
	parts := strings.SplitN(line, ":", 3)
	if len(parts) != 3 {
		return "", 0, "", false
	}
	lineNo, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, "", false
	}
	return parts[0], lineNo, parts[2], true
}
