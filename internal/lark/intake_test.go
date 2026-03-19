package lark

import (
	"strings"
	"testing"
)

func TestDefaultUserMessageKeepsExplicitQuestion(t *testing.T) {
	got := defaultUserMessage("  analyze this plan  ", []AttachmentRef{{Kind: AttachmentKindFile}})
	if got != "analyze this plan" {
		t.Fatalf("unexpected explicit user message: %q", got)
	}
}

func TestDefaultUserMessageForAttachmentOnlyRequestsAcknowledgement(t *testing.T) {
	got := defaultUserMessage("", []AttachmentRef{{Kind: AttachmentKindFile}})
	for _, want := range []string{
		"can see the attached file or files",
		"send a more specific question",
		"Do not analyze the files yet",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected synthesized message to contain %q, got %q", want, got)
		}
	}
}
