package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyAskplannerUserDataSkipsThirdPartyRepos(t *testing.T) {
	src := filepath.Join(t.TempDir(), ".askplanner")
	dst := filepath.Join(t.TempDir(), "backup")

	mustWriteFile(t, filepath.Join(src, "sessions.json"), []byte("session"))
	mustWriteFile(t, filepath.Join(src, "workspaces", "users", "u1", "data", "workspace.json"), []byte("{}"))
	mustWriteFile(t, filepath.Join(src, "workspaces", "users", "u1", "root", "notes.txt"), []byte("keep"))
	mustWriteFile(t, filepath.Join(src, "workspaces", "users", "u1", "root", "contrib", "tidb", "README.md"), []byte("skip"))
	mustWriteFile(t, filepath.Join(src, "workspaces", "users", "u1", "root", "contrib", "tidb-docs", "README.md"), []byte("skip"))
	mustWriteFile(t, filepath.Join(src, "workspaces", "users", "u1", "root", "contrib", "agent-rules", "README.md"), []byte("skip"))
	mustWriteFile(t, filepath.Join(src, "workspaces", "mirrors", "tidb.git", "HEAD"), []byte("skip"))

	_, err := CopyAskplannerUserData(src, dst)
	if err != nil {
		t.Fatalf("CopyAskplannerUserData error: %v", err)
	}

	assertExists(t, filepath.Join(dst, "sessions.json"))
	assertExists(t, filepath.Join(dst, "workspaces", "users", "u1", "data", "workspace.json"))
	assertExists(t, filepath.Join(dst, "workspaces", "users", "u1", "root", "notes.txt"))
	assertNotExists(t, filepath.Join(dst, "workspaces", "users", "u1", "root", "contrib", "tidb"))
	assertNotExists(t, filepath.Join(dst, "workspaces", "users", "u1", "root", "contrib", "tidb-docs"))
	assertNotExists(t, filepath.Join(dst, "workspaces", "users", "u1", "root", "contrib", "agent-rules"))
	assertNotExists(t, filepath.Join(dst, "workspaces", "mirrors"))
}

func TestCopyAskplannerUserDataRewritesWorkspaceSymlinks(t *testing.T) {
	srcParent := t.TempDir()
	dstParent := t.TempDir()
	src := filepath.Join(srcParent, ".askplanner")
	dst := filepath.Join(dstParent, "backup")

	uploadsDir := filepath.Join(src, "workspaces", "uploads", "u1")
	clinicDir := filepath.Join(src, "workspaces", "clinic", "u1")
	rootDir := filepath.Join(src, "workspaces", "users", "u1", "root")

	mustWriteFile(t, filepath.Join(uploadsDir, "a.txt"), []byte("upload"))
	mustWriteFile(t, filepath.Join(clinicDir, "b.json"), []byte("clinic"))
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.Symlink(uploadsDir, filepath.Join(rootDir, "user-files")); err != nil {
		t.Fatalf("create user-files symlink: %v", err)
	}
	if err := os.Symlink(clinicDir, filepath.Join(rootDir, "clinic-files")); err != nil {
		t.Fatalf("create clinic-files symlink: %v", err)
	}

	_, err := CopyAskplannerUserData(src, dst)
	if err != nil {
		t.Fatalf("CopyAskplannerUserData error: %v", err)
	}

	userLink := filepath.Join(dst, "workspaces", "users", "u1", "root", "user-files")
	target, err := os.Readlink(userLink)
	if err != nil {
		t.Fatalf("Readlink user-files: %v", err)
	}
	if filepath.IsAbs(target) {
		t.Fatalf("expected rewritten symlink target to be relative, got %q", target)
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(userLink), target))
	want := filepath.Join(dst, "workspaces", "uploads", "u1")
	if resolved != want {
		t.Fatalf("resolved symlink = %q, want %q", resolved, want)
	}
	assertExists(t, filepath.Join(resolved, "a.txt"))
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected path to exist %q: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected path to not exist %q, err=%v", path, err)
	}
}
