package codex

import (
	"context"
	"errors"
	"os/exec"
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

func TestShowcaseClassifyRunErrorMessages(t *testing.T) {
	testCases := []struct {
		name   string
		ctx    context.Context
		runErr error
		stderr string
		stdout string
		want   string
	}{
		{
			name:   "timeout",
			ctx:    context.Background(),
			runErr: context.DeadlineExceeded,
			want:   "The request timed out while waiting for Codex. Please retry.",
		},
		{
			name:   "rate_limit",
			ctx:    context.Background(),
			runErr: errors.New("exit status 1"),
			stderr: "HTTP 429 Too Many Requests",
			want:   "Codex is rate-limited right now. Please retry in a moment.",
		},
		{
			name:   "network",
			ctx:    context.Background(),
			runErr: errors.New("exit status 1"),
			stderr: "dial tcp 10.0.0.1:443: connect: connection refused",
			want:   "Codex could not be reached because of a network problem. Please retry.",
		},
		{
			name:   "missing_bin",
			ctx:    context.Background(),
			runErr: exec.ErrNotFound,
			want:   "Codex CLI is not available on this host. Check `CODEX_BIN` and the Codex installation.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyRunError(tc.ctx, tc.runErr, tc.stderr, tc.stdout)
			got := usererr.Message(err)
			if got != tc.want {
				t.Fatalf("user-facing message = %q, want %q", got, tc.want)
			}
			t.Logf("%s => %s", tc.name, got)
		})
	}
}
