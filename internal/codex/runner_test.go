package codex

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lab/askplanner/internal/usererr"
)

func TestClassifyRunErrorRateLimit(t *testing.T) {
	err := classifyRunError(context.Background(), errors.New("exit status 1"), "HTTP 429 Too Many Requests", "")
	if got := usererr.Message(err); !strings.Contains(got, "rate-limited") {
		t.Fatalf("user-facing message = %q", got)
	}
}

func TestClassifyRunErrorTimeout(t *testing.T) {
	err := classifyRunError(context.Background(), context.DeadlineExceeded, "", "")
	if got := usererr.Message(err); !strings.Contains(got, "timed out") {
		t.Fatalf("user-facing message = %q", got)
	}
}
