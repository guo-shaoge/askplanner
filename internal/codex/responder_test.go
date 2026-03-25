package codex

import (
	"path/filepath"
	"testing"
)

func TestResolveExistingConversationKeyPrefersFallbackWhenPreferredMissing(t *testing.T) {
	store, err := NewFileSessionStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Put(SessionRecord{ConversationKey: "legacy", SessionID: "session-1"}); err != nil {
		t.Fatalf("put legacy: %v", err)
	}

	responder := &Responder{store: store}
	got, legacy := responder.ResolveExistingConversationKey("preferred", "legacy")
	if got != "legacy" {
		t.Fatalf("conversation key = %q, want legacy", got)
	}
	if !legacy {
		t.Fatalf("expected fallback route")
	}
}

func TestResolveExistingConversationKeyPrefersPreferredWhenBothExist(t *testing.T) {
	store, err := NewFileSessionStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Put(SessionRecord{ConversationKey: "legacy", SessionID: "session-1"}); err != nil {
		t.Fatalf("put legacy: %v", err)
	}
	if err := store.Put(SessionRecord{ConversationKey: "preferred", SessionID: "session-2"}); err != nil {
		t.Fatalf("put preferred: %v", err)
	}

	responder := &Responder{store: store}
	got, legacy := responder.ResolveExistingConversationKey("preferred", "legacy")
	if got != "preferred" {
		t.Fatalf("conversation key = %q, want preferred", got)
	}
	if legacy {
		t.Fatalf("did not expect legacy route")
	}
}
