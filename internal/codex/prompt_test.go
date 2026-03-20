package codex

import (
	"strings"
	"testing"
	"time"
)

func TestBuildInitialPromptIncludesAttachmentContext(t *testing.T) {
	prompt := BuildInitialPrompt("base prompt", "older summary", "analyze the file", AttachmentContext{
		RootDir: "/tmp/user-a",
		Items: []AttachmentItem{
			{
				Name:         "report.sql",
				Type:         "file",
				SavedAt:      time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
				OriginalName: "report.sql",
			},
			{
				Name:         "image_20260320_090000_om1.png",
				Type:         "image",
				SavedAt:      time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC),
				OriginalName: "",
			},
		},
	})

	wantSnippets := []string{
		"The current user's uploaded-file library is stored under: /tmp/user-a",
		"If you cannot tell which file the user means, do not guess.",
		"report.sql [file] saved_at=2026-03-20T10:00:00Z",
		"image_20260320_090000_om1.png [image] saved_at=2026-03-20T09:00:00Z",
	}
	for _, snippet := range wantSnippets {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("prompt missing %q:\n%s", snippet, prompt)
		}
	}
}

func TestBuildResumePromptHandlesEmptyAttachmentLibrary(t *testing.T) {
	prompt := BuildResumePrompt("what next", AttachmentContext{
		RootDir: "/tmp/user-a",
	})

	if !strings.Contains(prompt, "Current top-level entries: none.") {
		t.Fatalf("prompt missing empty-library marker:\n%s", prompt)
	}
}
