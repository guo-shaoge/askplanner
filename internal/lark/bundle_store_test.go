package lark

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBundleStoreCreateUsesConversationKeyAndMessageID(t *testing.T) {
	root := t.TempDir()
	store := NewBundleStore(root)

	bundle, err := store.Create("lark:thread:oc_123", "om_456", "file", time.Hour)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	expectedDir := filepath.Join(root, "lark:thread:oc_123", "om_456")
	if bundle.Dir != expectedDir {
		t.Fatalf("unexpected bundle dir: got %q want %q", bundle.Dir, expectedDir)
	}
	if _, err := os.Stat(filepath.Join(expectedDir, "meta.json")); err != nil {
		t.Fatalf("expected meta.json to exist: %v", err)
	}
}
