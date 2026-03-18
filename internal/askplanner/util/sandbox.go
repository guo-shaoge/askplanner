package util

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Sandbox validates that file paths stay within allowed directory roots.
type Sandbox struct {
	projectRoot  string
	allowedRoots []string // relative to projectRoot
}

// NewSandbox creates a sandbox with the given allowed root paths (relative to projectRoot).
func NewSandbox(projectRoot string, allowedRoots []string) *Sandbox {
	abs := make([]string, len(allowedRoots))
	for i, r := range allowedRoots {
		abs[i] = filepath.Join(projectRoot, r)
	}
	return &Sandbox{
		projectRoot:  projectRoot,
		allowedRoots: abs,
	}
}

// Resolve validates and resolves a path. The input can be relative (to projectRoot) or absolute.
// Returns the absolute path if it falls within an allowed root.
func (s *Sandbox) Resolve(path string) (string, error) {
	cleaned := filepath.Clean(path)

	// Make absolute
	if !filepath.IsAbs(cleaned) {
		cleaned = filepath.Join(s.projectRoot, cleaned)
	}

	// Resolve symlinks
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		// If file doesn't exist yet, use the cleaned path
		resolved = cleaned
	}

	for _, root := range s.allowedRoots {
		if strings.HasPrefix(resolved, root) {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("path %q is outside allowed directories", path)
}
