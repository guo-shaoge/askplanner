package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDocsOverlaySuccess(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	overlayDir := filepath.Join(root, "overlay")

	mustMkdirAll(t, docsDir)
	mustMkdirAll(t, overlayDir)
	mustWriteFile(t, filepath.Join(overlayDir, "SKILL.md"), "# docs overlay")
	mustWriteFile(t, filepath.Join(overlayDir, "manifest.json"), `[{"title":"SQL Tuning Overview","path":"sql-tuning-overview.md"}]`)
	mustWriteFile(t, filepath.Join(docsDir, "sql-tuning-overview.md"), "overview")

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	if !overlay.Available {
		t.Fatalf("expected overlay available, warning=%q", overlay.Warning)
	}
	if len(overlay.Docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(overlay.Docs))
	}
	section := overlay.SystemPromptSection()
	if !strings.Contains(section, "Official SQL Tuning Docs") {
		t.Fatalf("prompt section missing heading: %q", section)
	}
	if !strings.Contains(section, "search_docs") {
		t.Fatalf("prompt section missing search_docs guidance: %q", section)
	}
}

func TestLoadDocsOverlayMissingDocDisablesOverlay(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	overlayDir := filepath.Join(root, "overlay")

	mustMkdirAll(t, docsDir)
	mustMkdirAll(t, overlayDir)
	mustWriteFile(t, filepath.Join(overlayDir, "SKILL.md"), "# docs overlay")
	mustWriteFile(t, filepath.Join(overlayDir, "manifest.json"), `[{"title":"Missing","path":"missing.md"}]`)

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	if overlay.Available {
		t.Fatalf("expected overlay unavailable")
	}
	if !strings.Contains(overlay.Warning, "missing.md") {
		t.Fatalf("unexpected warning: %q", overlay.Warning)
	}
}

func TestLoadDocsOverlayRejectsEscapingPath(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	overlayDir := filepath.Join(root, "overlay")

	mustMkdirAll(t, docsDir)
	mustMkdirAll(t, overlayDir)
	mustWriteFile(t, filepath.Join(overlayDir, "SKILL.md"), "# docs overlay")
	mustWriteFile(t, filepath.Join(overlayDir, "manifest.json"), `[{"title":"Escape","path":"../secret.md"}]`)

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	if overlay.Available {
		t.Fatalf("expected overlay unavailable")
	}
	if !strings.Contains(overlay.Warning, "escapes docs root") {
		t.Fatalf("unexpected warning: %q", overlay.Warning)
	}
}

func TestLoadDocsOverlayMalformedManifest(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	overlayDir := filepath.Join(root, "overlay")

	mustMkdirAll(t, docsDir)
	mustMkdirAll(t, overlayDir)
	mustWriteFile(t, filepath.Join(overlayDir, "SKILL.md"), "# docs overlay")
	mustWriteFile(t, filepath.Join(overlayDir, "manifest.json"), `{`)

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	if overlay.Available {
		t.Fatalf("expected overlay unavailable")
	}
	if !strings.Contains(overlay.Warning, "parse") {
		t.Fatalf("unexpected warning: %q", overlay.Warning)
	}
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
