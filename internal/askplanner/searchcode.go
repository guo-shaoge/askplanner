package askplanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const maxSearchResults = 30

// SearchCodeTool searches for patterns in code files using grep.
type SearchCodeTool struct {
	sandbox     *Sandbox
	defaultPath string // relative to project root
}

func NewSearchCodeTool(sandbox *Sandbox, defaultPath string) *SearchCodeTool {
	return &SearchCodeTool{sandbox: sandbox, defaultPath: defaultPath}
}

func (t *SearchCodeTool) Name() string { return "search_code" }

func (t *SearchCodeTool) Description() string {
	return "Search for a pattern in source code files using grep. Returns matching lines with file paths and line numbers. Useful for finding function definitions, variable usage, or specific code patterns in the TiDB codebase."
}

func (t *SearchCodeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"required": []string{"pattern"},
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Search pattern (basic regex supported)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to search in (relative to project root). Default: contrib/tidb/pkg/planner/",
			},
			"file_pattern": map[string]any{
				"type":        "string",
				"description": "File glob pattern to filter (e.g. '*.go'). Default: '*.go'",
			},
		},
	}
}

func (t *SearchCodeTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Pattern     string `json:"pattern"`
		Path        string `json:"path"`
		FilePattern string `json:"file_pattern"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	searchPath := args.Path
	if searchPath == "" {
		searchPath = t.defaultPath
	}

	resolved, err := t.sandbox.Resolve(searchPath)
	if err != nil {
		return "", err
	}

	filePattern := args.FilePattern
	if filePattern == "" {
		filePattern = "*.go"
	}

	// Try rg first, fall back to grep
	var cmd *exec.Cmd
	if rgPath, err := exec.LookPath("rg"); err == nil {
		cmd = exec.CommandContext(ctx, rgPath,
			"-n", "--no-heading",
			"--glob", filePattern,
			"--max-count", "3",
			"-m", fmt.Sprintf("%d", maxSearchResults),
			args.Pattern, resolved,
		)
	} else {
		cmd = exec.CommandContext(ctx, "grep",
			"-rn",
			"--include", filePattern,
			args.Pattern, resolved,
		)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "No matches found.", nil
		}
		// If output is non-empty, grep succeeded but had partial results
		if len(output) > 0 {
			// Continue with the output
		} else {
			return "", fmt.Errorf("search failed: %w", err)
		}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > maxSearchResults {
		lines = lines[:maxSearchResults]
	}

	// Make paths relative to project root for readability
	var sb strings.Builder
	fmt.Fprintf(&sb, "Search: %q in %s (%d results)\n", args.Pattern, searchPath, len(lines))
	for _, line := range lines {
		// Strip the resolved absolute path prefix to show relative paths
		line = strings.TrimPrefix(line, resolved+"/")
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	if len(lines) == maxSearchResults {
		sb.WriteString("... (results truncated)\n")
	}

	return sb.String(), nil
}
