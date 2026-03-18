package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DocEntry describes one curated official docs page.
type DocEntry struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

// DocHeading represents one heading extracted from a doc file.
type DocHeading struct {
	DocPath  string // relative path from manifest
	DocTitle string // from manifest
	Summary  string // from YAML frontmatter summary
	Heading  string // heading text (empty for title/summary-only entries)
	Level    int    // heading level 1-4 (0 for synthetic title/summary entry)
	Line     int    // 1-based line number
}

// HeadingIndex is the pre-built heading index for all curated docs.
type HeadingIndex struct {
	Entries []DocHeading
	// outlines maps doc path -> list of ##-level heading texts (for prompt)
	outlines map[string][]string
}

// HeadingMatch is a search result from the heading index.
type HeadingMatch struct {
	DocPath  string
	DocTitle string
	Heading  string
	Level    int
	Line     int
	Score    int
}

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "in": true,
	"of": true, "and": true, "for": true, "how": true, "is": true,
	"by": true, "with": true, "using": true, "that": true, "does": true,
	"do": true, "can": true, "what": true, "it": true, "on": true,
	"or": true, "be": true, "i": true, "my": true, "me": true,
}

// BuildHeadingIndex parses manifest docs and optional extra directories,
// building a searchable heading index. Extra dirs (e.g., "sql-statements")
// are relative to docsDir and are indexed for search but not for prompt outlines.
func BuildHeadingIndex(docs []DocEntry, docsDir string, extraDirs []string) *HeadingIndex {
	idx := &HeadingIndex{
		outlines: make(map[string][]string),
	}

	// Index manifest docs (with outline tracking for system prompt).
	for _, doc := range docs {
		idx.indexFile(docsDir, doc.Path, doc.Title, true)
	}

	// Index extra directories (search only, no outlines).
	for _, dir := range extraDirs {
		pattern := filepath.Join(docsDir, dir, "*.md")
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, absPath := range files {
			relPath := filepath.Join(dir, filepath.Base(absPath))
			title := parseFrontmatterTitle(absPath)
			idx.indexFile(docsDir, relPath, title, false)
		}
	}

	return idx
}

// indexFile parses a single doc file and adds its headings to the index.
// If trackOutline is true, ##-level headings are stored for system prompt rendering.
func (idx *HeadingIndex) indexFile(docsDir, relPath, docTitle string, trackOutline bool) {
	absPath := filepath.Join(docsDir, relPath)
	f, err := os.Open(absPath)
	if err != nil {
		return
	}
	defer f.Close()

	var summary string
	var outlineHeadings []string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	inFrontmatter := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Parse YAML frontmatter
		if lineNum == 1 && strings.TrimSpace(line) == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			if strings.TrimSpace(line) == "---" {
				inFrontmatter = false
				continue
			}
			if val, ok := strings.CutPrefix(line, "summary:"); ok {
				summary = strings.TrimSpace(val)
				summary = strings.Trim(summary, "'\"")
			}
			continue
		}

		// Parse markdown headings
		if !strings.HasPrefix(line, "#") {
			continue
		}
		level := 0
		for _, ch := range line {
			if ch == '#' {
				level++
			} else {
				break
			}
		}
		if level < 1 || level > 4 {
			continue
		}
		text := strings.TrimSpace(line[level:])
		if text == "" {
			continue
		}

		idx.Entries = append(idx.Entries, DocHeading{
			DocPath:  relPath,
			DocTitle: docTitle,
			Summary:  summary,
			Heading:  text,
			Level:    level,
			Line:     lineNum,
		})

		if trackOutline && level == 2 {
			outlineHeadings = append(outlineHeadings, text)
		}
	}

	// Add a synthetic entry for the doc title + summary (for keyword matching)
	idx.Entries = append(idx.Entries, DocHeading{
		DocPath:  relPath,
		DocTitle: docTitle,
		Summary:  summary,
		Heading:  docTitle,
		Level:    0,
		Line:     1,
	})

	if trackOutline {
		idx.outlines[relPath] = outlineHeadings
	}
}

// parseFrontmatterTitle extracts the title from a doc's YAML frontmatter.
// Returns the filename (without extension) if parsing fails.
func parseFrontmatterTitle(absPath string) string {
	fallback := strings.TrimSuffix(filepath.Base(absPath), ".md")

	f, err := os.Open(absPath)
	if err != nil {
		return fallback
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	inFrontmatter := false

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if lineNum == 1 && strings.TrimSpace(line) == "---" {
			inFrontmatter = true
			continue
		}
		if !inFrontmatter {
			break
		}
		if strings.TrimSpace(line) == "---" {
			break
		}
		if val, ok := strings.CutPrefix(line, "title:"); ok {
			title := strings.TrimSpace(val)
			title = strings.Trim(title, "'\"")
			// Strip common suffix from sql-statement docs
			title, _ = strings.CutSuffix(title, " | TiDB SQL Statement Reference")
			if title != "" {
				return title
			}
		}
	}

	return fallback
}

// Search finds headings matching the given query keywords.
func (idx *HeadingIndex) Search(query string, limit int) []HeadingMatch {
	if idx == nil || len(idx.Entries) == 0 {
		return nil
	}

	keywords := tokenizeQuery(query)
	if len(keywords) == 0 {
		return nil
	}

	type scored struct {
		entry DocHeading
		score int
	}
	var matches []scored

	for _, entry := range idx.Entries {
		target := strings.ToLower(entry.Heading + " " + entry.DocTitle + " " + entry.Summary)
		score := 0
		for _, kw := range keywords {
			if strings.Contains(target, kw) {
				score++
			}
		}
		if score == 0 {
			continue
		}
		// Bonus for title/summary match (level 0) or high-level headings
		if entry.Level <= 1 {
			score += 1
		}
		matches = append(matches, scored{entry, score})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		// Stable order: by doc path, then line number
		if matches[i].entry.DocPath != matches[j].entry.DocPath {
			return matches[i].entry.DocPath < matches[j].entry.DocPath
		}
		return matches[i].entry.Line < matches[j].entry.Line
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	result := make([]HeadingMatch, len(matches))
	for i, m := range matches {
		result[i] = HeadingMatch{
			DocPath:  m.entry.DocPath,
			DocTitle: m.entry.DocTitle,
			Heading:  m.entry.Heading,
			Level:    m.entry.Level,
			Line:     m.entry.Line,
			Score:    m.score,
		}
	}
	return result
}

// Outline returns the ##-level headings for a doc path (for system prompt).
func (idx *HeadingIndex) Outline(docPath string) []string {
	if idx == nil {
		return nil
	}
	return idx.outlines[docPath]
}

func tokenizeQuery(query string) []string {
	words := strings.Fields(strings.ToLower(query))
	var keywords []string
	for _, w := range words {
		// Strip punctuation
		w = strings.Trim(w, ".,;:!?\"'()[]{}*")
		if w == "" || stopWords[w] {
			continue
		}
		keywords = append(keywords, w)
	}
	return keywords
}

// DocsOverlay holds the askplanner-local official docs prompt overlay.
type DocsOverlay struct {
	SkillMD      string
	Docs         []DocEntry
	HeadingIndex *HeadingIndex
	Available    bool
	Warning      string
}

// LoadDocsOverlay loads the local prompt overlay and validates all manifest entries.
func LoadDocsOverlay(overlayDir, docsDir string) *DocsOverlay {
	overlay := &DocsOverlay{}

	if overlayDir == "" {
		overlay.Warning = "official docs overlay disabled: DOCS_OVERLAY_DIR is empty"
		return overlay
	}
	if docsDir == "" {
		overlay.Warning = "official docs overlay disabled: TIDB_DOCS_DIR is empty"
		return overlay
	}

	skillPath := filepath.Join(overlayDir, "SKILL.md")
	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: read %s: %v", skillPath, err)
		return overlay
	}
	if strings.TrimSpace(string(skillData)) == "" {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: %s is empty", skillPath)
		return overlay
	}

	manifestPath := filepath.Join(overlayDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: read %s: %v", manifestPath, err)
		return overlay
	}

	var docs []DocEntry
	if err := json.Unmarshal(manifestData, &docs); err != nil {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: parse %s: %v", manifestPath, err)
		return overlay
	}
	if len(docs) == 0 {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: %s has no entries", manifestPath)
		return overlay
	}

	absDocsDir, err := filepath.Abs(docsDir)
	if err != nil {
		overlay.Warning = fmt.Sprintf("official docs overlay disabled: resolve docs dir %s: %v", docsDir, err)
		return overlay
	}

	for i, doc := range docs {
		doc.Title = strings.TrimSpace(doc.Title)
		doc.Path = filepath.Clean(strings.TrimSpace(doc.Path))

		if doc.Title == "" {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: manifest entry %d has empty title", i)
			return overlay
		}
		if doc.Path == "." || doc.Path == "" {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: manifest entry %q has empty path", doc.Title)
			return overlay
		}
		if filepath.IsAbs(doc.Path) {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: manifest path %q must be relative", doc.Path)
			return overlay
		}

		resolved, err := resolveDocPath(absDocsDir, doc.Path)
		if err != nil {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: %v", err)
			return overlay
		}

		info, err := os.Stat(resolved)
		if err != nil {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: stat %s: %v", doc.Path, err)
			return overlay
		}
		if info.IsDir() {
			overlay.Warning = fmt.Sprintf("official docs overlay disabled: %s is a directory", doc.Path)
			return overlay
		}

		docs[i] = doc
	}

	overlay.SkillMD = string(skillData)
	overlay.Docs = docs
	overlay.HeadingIndex = BuildHeadingIndex(docs, absDocsDir, []string{"sql-statements"})
	overlay.Available = true
	return overlay
}

// SystemPromptSection renders the docs overlay portion of the system prompt.
func (o *DocsOverlay) SystemPromptSection() string {
	if o == nil || !o.Available {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Official SQL Tuning Docs\n\n")
	sb.WriteString(o.SkillMD)
	sb.WriteString("\n\n### Curated Official Pages\n")
	sb.WriteString("Browse the outlines below to find the right doc. Use `search_docs` with keywords to find specific subsections, then `read_file` with the returned path and line offset.\n\n")
	for _, doc := range o.Docs {
		outline := o.HeadingIndex.Outline(doc.Path)
		if len(outline) > 0 {
			fmt.Fprintf(&sb, "- %s (%s): %s\n", doc.Title, doc.Path, strings.Join(outline, " | "))
		} else {
			fmt.Fprintf(&sb, "- %s (%s)\n", doc.Title, doc.Path)
		}
	}
	sb.WriteString("\nSQL statement reference docs (`sql-statements/`) are also searchable via `search_docs`.\n")
	return sb.String()
}

func resolveDocPath(docsDir, relPath string) (string, error) {
	joined := filepath.Join(docsDir, relPath)
	resolved, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", relPath, err)
	}

	rel, err := filepath.Rel(docsDir, resolved)
	if err != nil {
		return "", fmt.Errorf("relativize %s: %w", relPath, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("manifest path %q escapes docs root", relPath)
	}

	return resolved, nil
}
