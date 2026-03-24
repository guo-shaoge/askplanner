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

func TestFileSessionStoreUpdateIf(t *testing.T) {
	store, err := NewFileSessionStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if err := store.Put(SessionRecord{ConversationKey: "a", UserKey: "u1", EnvironmentHash: "env-a"}); err != nil {
		t.Fatalf("put a: %v", err)
	}
	if err := store.Put(SessionRecord{ConversationKey: "b", UserKey: "u1", EnvironmentHash: "env-b"}); err != nil {
		t.Fatalf("put b: %v", err)
	}

	updated, err := store.UpdateIf(func(record SessionRecord) bool {
		return record.UserKey == "u1" && record.EnvironmentHash != "env-new"
	}, func(record *SessionRecord) bool {
		record.PendingNotice = &WorkspaceSessionNotice{
			Message:            "workspace changed",
			NewEnvironmentHash: "env-new",
		}
		return true
	})
	if err != nil {
		t.Fatalf("update if: %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	if record, ok := store.Get("a"); !ok || record.PendingNotice == nil {
		t.Fatalf("record a pending notice missing: %+v %t", record, ok)
	}
}
