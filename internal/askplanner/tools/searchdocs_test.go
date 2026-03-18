package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchDocsToolExecute(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "contrib", "tidb-docs")
	overlayDir := filepath.Join(root, "overlay")

	mustWriteFile(t, filepath.Join(overlayDir, "SKILL.md"), "# docs overlay")
	mustWriteFile(t, filepath.Join(overlayDir, "manifest.json"), `[{"title":"Join Reorder","path":"join-reorder.md"},{"title":"Statistics","path":"statistics.md"}]`)
	mustWriteFile(t, filepath.Join(docsDir, "join-reorder.md"), "Join reorder controls the join order.\n")
	mustWriteFile(t, filepath.Join(docsDir, "statistics.md"), "Statistics drive cardinality estimation.\n")

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	tool := NewSearchDocsTool(root, docsDir, overlay)

	out, err := tool.Execute(context.Background(), `{"pattern":"join order"}`)
	if err != nil {
		t.Fatalf("search docs: %v", err)
	}
	if !strings.Contains(out, "Join Reorder") {
		t.Fatalf("missing title in output: %q", out)
	}
	if !strings.Contains(out, "contrib/tidb-docs/join-reorder.md") {
		t.Fatalf("missing relative path in output: %q", out)
	}
}

func TestSearchDocsToolUnavailableOverlay(t *testing.T) {
	tool := NewSearchDocsTool("/tmp/project", "/tmp/project/contrib/tidb-docs", &DocsOverlay{})
	_, err := tool.Execute(context.Background(), `{"pattern":"stats"}`)
	if err == nil {
		t.Fatalf("expected unavailable overlay error")
	}
}

func TestSearchDocsToolNoMatches(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "contrib", "tidb-docs")
	overlayDir := filepath.Join(root, "overlay")

	mustWriteFile(t, filepath.Join(overlayDir, "SKILL.md"), "# docs overlay")
	mustWriteFile(t, filepath.Join(overlayDir, "manifest.json"), `[{"title":"Statistics","path":"statistics.md"}]`)
	mustWriteFile(t, filepath.Join(docsDir, "statistics.md"), "Statistics drive cardinality estimation.\n")

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	tool := NewSearchDocsTool(root, docsDir, overlay)

	out, err := tool.Execute(context.Background(), `{"pattern":"missing phrase"}`)
	if err != nil {
		t.Fatalf("search docs: %v", err)
	}
	if out != "No matches found." {
		t.Fatalf("unexpected output: %q", out)
	}
}
