package askplanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const defaultLineLimit = 200

// ReadFileTool reads a file within the sandbox.
type ReadFileTool struct {
	sandbox *Sandbox
}

func NewReadFileTool(sandbox *Sandbox) *ReadFileTool {
	return &ReadFileTool{sandbox: sandbox}
}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string {
	return "Read file content by path. Use this to read TiDB source code or any file within the allowed directories. Paths can be relative to the project root (e.g. 'contrib/tidb/pkg/planner/optimize.go') or absolute."
}

func (t *ReadFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"required": []string{"path"},
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path to read (relative to project root or absolute)",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Line number to start reading from (1-based, default 1)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of lines to read (default 200)",
			},
		},
	}
}

func (t *ReadFileTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := t.sandbox.Resolve(args.Path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	offset := args.Offset
	if offset < 1 {
		offset = 1
	}
	limit := args.Limit
	if limit <= 0 {
		limit = defaultLineLimit
	}

	startIdx := offset - 1
	if startIdx >= len(lines) {
		return fmt.Sprintf("File has %d lines, offset %d is past end of file.", len(lines), offset), nil
	}
	endIdx := startIdx + limit
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "File: %s (%d lines total, showing lines %d-%d)\n", args.Path, len(lines), offset, offset+endIdx-startIdx-1)
	for i := startIdx; i < endIdx; i++ {
		fmt.Fprintf(&sb, "%4d | %s\n", i+1, lines[i])
	}
	if endIdx < len(lines) {
		fmt.Fprintf(&sb, "... (%d more lines)\n", len(lines)-endIdx)
	}

	return sb.String(), nil
}
