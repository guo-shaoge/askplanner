package lark

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanerRemovesExpiredBundles(t *testing.T) {
	root := t.TempDir()
	store := NewBundleStore(root)

	expired, err := store.Create("lark:chat:oc_1:user:ou_1", "om_expired", "file", time.Minute)
	if err != nil {
		t.Fatalf("create expired bundle: %v", err)
	}
	expired.Meta.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	if err := expired.Save(); err != nil {
		t.Fatalf("save expired bundle: %v", err)
	}

	alive, err := store.Create("lark:chat:oc_1:user:ou_1", "om_alive", "image", time.Hour)
	if err != nil {
		t.Fatalf("create alive bundle: %v", err)
	}

	cleaner := &Cleaner{Root: root}
	if err := cleaner.CleanupExpired(time.Now().UTC()); err != nil {
		t.Fatalf("CleanupExpired returned error: %v", err)
	}

	if _, err := os.Stat(expired.Dir); !os.IsNotExist(err) {
		t.Fatalf("expected expired bundle to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(alive.Dir); err != nil {
		t.Fatalf("expected alive bundle to remain: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(alive.Dir)); err != nil {
		t.Fatalf("expected conversation dir to remain: %v", err)
	}
}
