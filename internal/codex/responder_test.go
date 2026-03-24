package codex

import (
	"strings"
	"testing"
)

func TestAppendAnswerWarning(t *testing.T) {
	got := appendAnswerWarning("final answer", "session history was not saved")
	if !strings.Contains(got, "final answer") || !strings.Contains(got, "Warning: session history was not saved") {
		t.Fatalf("appendAnswerWarning() = %q", got)
	}
}
