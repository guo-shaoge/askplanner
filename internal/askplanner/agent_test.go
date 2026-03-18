package askplanner

import (
	"strings"
	"testing"

	"lab/askplanner/internal/askplanner/llmprovider"
)

func TestSummarizeToolCalls(t *testing.T) {
	toolCalls := []llmprovider.ToolCall{
		{Function: llmprovider.ToolCallFunction{Name: "read_skill"}},
		{Function: llmprovider.ToolCallFunction{Name: "search_code"}},
		{Function: llmprovider.ToolCallFunction{Name: "read_skill"}},
	}

	got := summarizeToolCalls(toolCalls)
	want := "call read_skill x2, search_code"
	if got != want {
		t.Fatalf("unexpected tool summary: got %q want %q", got, want)
	}
}

func TestSummarizeToolCallsFinalAnswer(t *testing.T) {
	if got := summarizeToolCalls(nil); got != "return final answer" {
		t.Fatalf("unexpected final step summary: %q", got)
	}
}

func TestCompactSnippet(t *testing.T) {
	got := compactSnippet("  first line\n second\tline  ", 80)
	if got != "first line second line" {
		t.Fatalf("unexpected compact snippet: %q", got)
	}
}

func TestCompactSnippetTruncatesRunes(t *testing.T) {
	got := compactSnippet("调优 调优 调优 调优", 8)
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis, got %q", got)
	}
	if got == "调优 调优 调优 调优" {
		t.Fatalf("expected truncation, got %q", got)
	}
}
