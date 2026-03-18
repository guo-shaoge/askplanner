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
	mustWriteFile(t, filepath.Join(docsDir, "sql-tuning-overview.md"), "---\ntitle: SQL Tuning Overview\nsummary: Learn about SQL tuning.\n---\n\n# SQL Tuning Overview\n\n## Understanding Queries\n\n## Optimization Process\n")

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	if !overlay.Available {
		t.Fatalf("expected overlay available, warning=%q", overlay.Warning)
	}
	if len(overlay.Docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(overlay.Docs))
	}
	if overlay.HeadingIndex == nil {
		t.Fatalf("expected heading index to be built")
	}

	section := overlay.SystemPromptSection()
	if !strings.Contains(section, "Official SQL Tuning Docs") {
		t.Fatalf("prompt section missing heading: %q", section)
	}
	if !strings.Contains(section, "search_docs") {
		t.Fatalf("prompt section missing search_docs guidance: %q", section)
	}
	// Check compact outline
	if !strings.Contains(section, "Understanding Queries | Optimization Process") {
		t.Fatalf("prompt section missing outline: %q", section)
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

func TestBuildHeadingIndex(t *testing.T) {
	docsDir := t.TempDir()
	content := "---\ntitle: SQL Plan Management\nsummary: Learn about SQL bindings.\n---\n\n# SQL Plan Management (SPM)\n\n## SQL binding\n\nSome text.\n\n### Create a binding\n\n### Delete a binding\n\n## Baseline capturing\n"
	mustWriteFile(t, filepath.Join(docsDir, "spm.md"), content)

	docs := []DocEntry{{Title: "SQL Plan Management", Path: "spm.md"}}
	idx := BuildHeadingIndex(docs, docsDir, nil)

	if idx == nil {
		t.Fatalf("expected non-nil index")
	}

	// Should have headings: # SPM, ## SQL binding, ### Create, ### Delete, ## Baseline
	headingCount := 0
	for _, e := range idx.Entries {
		if e.Level > 0 {
			headingCount++
		}
	}
	if headingCount != 5 {
		t.Fatalf("expected 5 headings, got %d", headingCount)
	}

	// Check outline (##-level only)
	outline := idx.Outline("spm.md")
	if len(outline) != 2 {
		t.Fatalf("expected 2 outline entries, got %d: %v", len(outline), outline)
	}
	if outline[0] != "SQL binding" || outline[1] != "Baseline capturing" {
		t.Fatalf("unexpected outline: %v", outline)
	}
}

func TestHeadingIndexSearch(t *testing.T) {
	docsDir := t.TempDir()
	content := "---\ntitle: SQL Plan Management\nsummary: Learn about SQL bindings.\n---\n\n# SQL Plan Management (SPM)\n\n## SQL binding\n\n### Create a binding\n\n### Cache bindings\n\n## Baseline capturing\n"
	mustWriteFile(t, filepath.Join(docsDir, "spm.md"), content)

	docs := []DocEntry{{Title: "SQL Plan Management", Path: "spm.md"}}
	idx := BuildHeadingIndex(docs, docsDir, nil)

	// Search for "binding" should find multiple matches
	results := idx.Search("binding", 10)
	if len(results) == 0 {
		t.Fatalf("expected matches for 'binding'")
	}
	for _, r := range results {
		if r.DocPath != "spm.md" {
			t.Fatalf("unexpected doc path: %s", r.DocPath)
		}
	}

	// Search for "cache binding" should rank "Cache bindings" highly
	results = idx.Search("cache binding", 10)
	found := false
	for _, r := range results {
		if strings.Contains(r.Heading, "Cache") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'Cache bindings' in results for 'cache binding'")
	}

	// Search for unrelated topic should return nothing
	results = idx.Search("completely unrelated xyzzy", 10)
	if len(results) != 0 {
		t.Fatalf("expected no matches, got %d", len(results))
	}
}

func TestHeadingIndexSearchCaseInsensitive(t *testing.T) {
	docsDir := t.TempDir()
	content := "# Overview\n\n## HASH JOIN Strategy\n\n## Index Selection\n"
	mustWriteFile(t, filepath.Join(docsDir, "doc.md"), content)

	docs := []DocEntry{{Title: "Optimizer Overview", Path: "doc.md"}}
	idx := BuildHeadingIndex(docs, docsDir, nil)

	results := idx.Search("hash join", 10)
	if len(results) == 0 {
		t.Fatalf("expected case-insensitive match for 'hash join'")
	}

	results = idx.Search("HASH JOIN", 10)
	if len(results) == 0 {
		t.Fatalf("expected case-insensitive match for 'HASH JOIN'")
	}
}

func TestHeadingIndexSearchStopWords(t *testing.T) {
	docsDir := t.TempDir()
	content := "# Doc\n\n## Statistics collection\n"
	mustWriteFile(t, filepath.Join(docsDir, "doc.md"), content)

	docs := []DocEntry{{Title: "Stats", Path: "doc.md"}}
	idx := BuildHeadingIndex(docs, docsDir, nil)

	// "how to do statistics" should filter stop words and match on "statistics"
	results := idx.Search("how to do statistics", 10)
	if len(results) == 0 {
		t.Fatalf("expected match after filtering stop words")
	}
}

func TestBuildHeadingIndexWithExtraDirs(t *testing.T) {
	docsDir := t.TempDir()

	// Create a manifest doc
	mustWriteFile(t, filepath.Join(docsDir, "overview.md"), "# Overview\n\n## Getting Started\n")

	// Create an extra dir with sql-statement docs
	mustWriteFile(t, filepath.Join(docsDir, "sql-statements", "sql-statement-admin.md"),
		"---\ntitle: ADMIN | TiDB SQL Statement Reference\nsummary: An overview of ADMIN.\n---\n\n# ADMIN\n\n## `ADMIN BINDINGS` related statement\n\n## `ADMIN CHECK` related statement\n")

	mustWriteFile(t, filepath.Join(docsDir, "sql-statements", "sql-statement-create-binding.md"),
		"---\ntitle: CREATE BINDING | TiDB SQL Statement Reference\nsummary: Create SQL bindings.\n---\n\n# CREATE BINDING\n\n## Synopsis\n\n## Examples\n")

	docs := []DocEntry{{Title: "Overview", Path: "overview.md"}}
	idx := BuildHeadingIndex(docs, docsDir, []string{"sql-statements"})

	// Should have entries from both manifest and extra dir
	hasManifest := false
	hasExtraDir := false
	for _, e := range idx.Entries {
		if e.DocPath == "overview.md" {
			hasManifest = true
		}
		if strings.HasPrefix(e.DocPath, "sql-statements/") {
			hasExtraDir = true
		}
	}
	if !hasManifest {
		t.Fatalf("expected manifest entries in index")
	}
	if !hasExtraDir {
		t.Fatalf("expected extra dir entries in index")
	}

	// Extra dir should NOT have outlines
	outline := idx.Outline("sql-statements/sql-statement-admin.md")
	if len(outline) != 0 {
		t.Fatalf("expected no outline for extra dir doc, got %v", outline)
	}

	// Manifest doc SHOULD have outlines
	outline = idx.Outline("overview.md")
	if len(outline) != 1 || outline[0] != "Getting Started" {
		t.Fatalf("expected outline for manifest doc, got %v", outline)
	}
}

func TestHeadingIndexSearchFindsExtraDirEntries(t *testing.T) {
	docsDir := t.TempDir()

	mustWriteFile(t, filepath.Join(docsDir, "spm.md"),
		"# SQL Plan Management\n\n## SQL binding\n\n### Create a binding\n")

	mustWriteFile(t, filepath.Join(docsDir, "sql-statements", "sql-statement-admin.md"),
		"---\ntitle: ADMIN | TiDB SQL Statement Reference\nsummary: An overview of ADMIN.\n---\n\n# ADMIN\n\n## `ADMIN BINDINGS` related statement\n")

	docs := []DocEntry{{Title: "SQL Plan Management", Path: "spm.md"}}
	idx := BuildHeadingIndex(docs, docsDir, []string{"sql-statements"})

	// Search for "binding" should find results from BOTH sources
	results := idx.Search("binding", 20)
	hasSPM := false
	hasAdmin := false
	for _, r := range results {
		if r.DocPath == "spm.md" {
			hasSPM = true
		}
		if r.DocPath == "sql-statements/sql-statement-admin.md" {
			hasAdmin = true
		}
	}
	if !hasSPM {
		t.Fatalf("expected spm.md in search results for 'binding'")
	}
	if !hasAdmin {
		t.Fatalf("expected sql-statement-admin.md in search results for 'binding'")
	}
}

func TestParseFrontmatterTitle(t *testing.T) {
	docsDir := t.TempDir()
	mustWriteFile(t, filepath.Join(docsDir, "sql-statements", "sql-statement-admin.md"),
		"---\ntitle: ADMIN | TiDB SQL Statement Reference\nsummary: overview\n---\n\n# ADMIN\n")

	docs := []DocEntry{}
	idx := BuildHeadingIndex(docs, docsDir, []string{"sql-statements"})

	// Title should be stripped of " | TiDB SQL Statement Reference" suffix
	for _, e := range idx.Entries {
		if e.Level == 0 && e.DocPath == "sql-statements/sql-statement-admin.md" {
			if e.DocTitle != "ADMIN" {
				t.Fatalf("expected title 'ADMIN', got %q", e.DocTitle)
			}
			return
		}
	}
	t.Fatalf("expected synthetic title entry for sql-statement-admin.md")
}

func TestSystemPromptSectionMentionsSQLStatements(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	overlayDir := filepath.Join(root, "overlay")

	mustMkdirAll(t, docsDir)
	mustMkdirAll(t, overlayDir)
	mustWriteFile(t, filepath.Join(overlayDir, "SKILL.md"), "# docs overlay")
	mustWriteFile(t, filepath.Join(overlayDir, "manifest.json"), `[{"title":"Overview","path":"overview.md"}]`)
	mustWriteFile(t, filepath.Join(docsDir, "overview.md"), "# Overview\n")

	overlay := LoadDocsOverlay(overlayDir, docsDir)
	section := overlay.SystemPromptSection()
	if !strings.Contains(section, "sql-statements") {
		t.Fatalf("prompt section should mention sql-statements: %q", section)
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
