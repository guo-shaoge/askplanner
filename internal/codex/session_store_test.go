package codex

import (
	"path/filepath"
	"testing"
)

func TestFileSessionStoreDeleteIf(t *testing.T) {
	store, err := NewFileSessionStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := store.Put(SessionRecord{ConversationKey: "a", WorkDir: "/tmp/ws/users/u1/root"}); err != nil {
		t.Fatalf("put a: %v", err)
	}
	if err := store.Put(SessionRecord{ConversationKey: "b", WorkDir: "/tmp/ws/users/u2/root"}); err != nil {
		t.Fatalf("put b: %v", err)
	}

	deleted, err := store.DeleteIf(func(record SessionRecord) bool {
		return record.WorkDir == "/tmp/ws/users/u1/root"
	})
	if err != nil {
		t.Fatalf("delete if: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, ok := store.Get("a"); ok {
		t.Fatalf("record a still exists")
	}
	if _, ok := store.Get("b"); !ok {
		t.Fatalf("record b missing")
	}
}
