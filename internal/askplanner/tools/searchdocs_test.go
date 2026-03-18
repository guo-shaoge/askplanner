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
	mustWriteFile(t, filepath.Join(docsDir, "join-reorder.md"), "# Join Reorder\n\n## Join Order Algorithm\n\n## Greedy Algorithm\n")
	mustWriteFile(t, filepath.Join(docsDir, "statistics.md"), "# Statistics\n\n## Collect Statistics\n\n## Automatic Update\n")

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	tool := NewSearchDocsTool(root, docsDir, overlay)

	out, err := tool.Execute(context.Background(), `{"query":"join order"}`)
	if err != nil {
		t.Fatalf("search docs: %v", err)
	}
	if !strings.Contains(out, "Join Reorder") {
		t.Fatalf("missing title in output: %q", out)
	}
	if !strings.Contains(out, "join-reorder.md") {
		t.Fatalf("missing path in output: %q", out)
	}
}

func TestSearchDocsToolUnavailableOverlay(t *testing.T) {
	tool := NewSearchDocsTool("/tmp/project", "/tmp/project/contrib/tidb-docs", &DocsOverlay{})
	_, err := tool.Execute(context.Background(), `{"query":"stats"}`)
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
	mustWriteFile(t, filepath.Join(docsDir, "statistics.md"), "# Statistics\n\n## Collect Statistics\n")

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	tool := NewSearchDocsTool(root, docsDir, overlay)

	out, err := tool.Execute(context.Background(), `{"query":"completely unrelated xyzzy"}`)
	if err != nil {
		t.Fatalf("search docs: %v", err)
	}
	if out != "No matches found." {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestSearchDocsToolMultiKeywordScoring(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "contrib", "tidb-docs")
	overlayDir := filepath.Join(root, "overlay")

	mustWriteFile(t, filepath.Join(overlayDir, "SKILL.md"), "# docs overlay")
	mustWriteFile(t, filepath.Join(overlayDir, "manifest.json"), `[{"title":"SQL Plan Management","path":"spm.md"}]`)
	mustWriteFile(t, filepath.Join(docsDir, "spm.md"), "# SQL Plan Management\n\n## SQL binding\n\n### Create a binding\n\n### Cache bindings\n\n## Baseline capturing\n")

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	tool := NewSearchDocsTool(root, docsDir, overlay)

	out, err := tool.Execute(context.Background(), `{"query":"cache binding"}`)
	if err != nil {
		t.Fatalf("search docs: %v", err)
	}
	if !strings.Contains(out, "Cache bindings") {
		t.Fatalf("expected 'Cache bindings' in output: %q", out)
	}
}

func TestSearchDocsToolCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "contrib", "tidb-docs")
	overlayDir := filepath.Join(root, "overlay")

	mustWriteFile(t, filepath.Join(overlayDir, "SKILL.md"), "# docs overlay")
	mustWriteFile(t, filepath.Join(overlayDir, "manifest.json"), `[{"title":"Optimizer Hints","path":"hints.md"}]`)
	mustWriteFile(t, filepath.Join(docsDir, "hints.md"), "# Optimizer Hints\n\n## HASH_JOIN Hint\n\n## USE_INDEX Hint\n")

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	tool := NewSearchDocsTool(root, docsDir, overlay)

	out, err := tool.Execute(context.Background(), `{"query":"hash_join"}`)
	if err != nil {
		t.Fatalf("search docs: %v", err)
	}
	if !strings.Contains(out, "HASH_JOIN") {
		t.Fatalf("expected case-insensitive match: %q", out)
	}
}
