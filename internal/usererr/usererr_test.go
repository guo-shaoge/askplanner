package usererr

import (
	"errors"
	"strings"
	"testing"
)

func TestMessageReturnsTypedUserMessage(t *testing.T) {
	err := Wrap(KindTimeout, "Timed out while waiting for Codex.", errors.New("context deadline exceeded"))
	if got := Message(err); got != "Timed out while waiting for Codex." {
		t.Fatalf("Message() = %q", got)
	}
}

func TestOrDefaultFallsBackForUntypedErrors(t *testing.T) {
	got := OrDefault(errors.New("boom"), "Something failed.")
	if got != "Something failed." {
		t.Fatalf("OrDefault() = %q", got)
	}
}

func TestWrapLocalStoragePermissionDenied(t *testing.T) {
	err := WrapLocalStorage("fallback", errors.New("permission denied"))
	if got := Message(err); !strings.Contains(got, "filesystem permission problem") {
		t.Fatalf("Message() = %q", got)
	}
}

func TestWrapLocalStorageDiskFull(t *testing.T) {
	err := WrapLocalStorage("fallback", errors.New("no space left on device"))
	if got := Message(err); !strings.Contains(got, "disk is full") {
		t.Fatalf("Message() = %q", got)
	}
}
