package codex

import (
	"strings"
	"testing"
)

func TestBuildInitialPromptIncludesAttachmentContext(t *testing.T) {
	req := Request{
		UserMessage:  "Analyze this bundle",
		RuntimeNotes: []string{"Attachment bundle directory: .askplanner/lark:thread:oc_1/om_1"},
		Attachments: []Attachment{{
			Kind:         "plan_replayer_zip",
			PublicID:     "lark:thread:oc_1/om_1/extracted",
			OriginalName: "trace.zip",
			SavedPath:    ".askplanner/lark:thread:oc_1/om_1/raw/trace.zip",
			ExtractedDir: ".askplanner/lark:thread:oc_1/om_1/extracted/trace",
		}},
	}

	prompt := BuildInitialPrompt("base prompt", "", req)
	for _, needle := range []string{
		"## Message Context",
		"public_id=lark:thread:oc_1/om_1/extracted",
		"trace.zip",
		".askplanner/lark:thread:oc_1/om_1/raw/trace.zip",
		".askplanner/lark:thread:oc_1/om_1/extracted/trace",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("expected prompt to contain %q", needle)
		}
	}
}
