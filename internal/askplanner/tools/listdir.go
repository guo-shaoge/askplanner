package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"lab/askplanner/internal/askplanner/util"
)

// ListDirTool lists directory contents within the sandbox.
type ListDirTool struct {
	sandbox *util.Sandbox
}

func NewListDirTool(sandbox *util.Sandbox) *ListDirTool {
	return &ListDirTool{sandbox: sandbox}
}

func (t *ListDirTool) Name() string { return "list_dir" }

func (t *ListDirTool) Description() string {
	return "List the contents of a directory. Shows files and subdirectories with [dir] or [file] markers. Useful for navigating the TiDB codebase structure."
}

func (t *ListDirTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"path"},
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path to list (relative to project root or absolute)",
			},
		},
	}
}

func (t *ListDirTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
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

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "", fmt.Errorf("read directory: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Directory: %s (%d entries)\n", args.Path, len(entries))
	for _, e := range entries {
		if e.Name() == ".git" || e.Name() == "bazel-bin" || e.Name() == "bazel-out" || e.Name() == "bazel-testlogs" {
			continue
		}
		marker := "[file]"
		if e.IsDir() {
			marker = "[dir] "
		}
		fmt.Fprintf(&sb, "  %s %s\n", marker, e.Name())
	}

	return sb.String(), nil
}
