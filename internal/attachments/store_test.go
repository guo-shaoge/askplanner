package attachments

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImportImageUsesTimestampedName(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir, 100)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	source := filepath.Join(dir, "source.png")
	if err := os.WriteFile(source, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ts := time.Date(2026, 3, 20, 15, 4, 5, 0, time.FixedZone("CST", 8*3600))
	result, err := manager.Import(ImportRequest{
		UserKey:      "user-1",
		OriginalName: "",
		MessageID:    "om_123",
		ResourceType: "image",
		SourcePath:   source,
		ImportedAt:   ts,
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	wantName := "image_20260320_150405_om_123.png"
	if result.Item.Name != wantName {
		t.Fatalf("name = %q, want %q", result.Item.Name, wantName)
	}
	if _, err := os.Stat(filepath.Join(result.UserDir, wantName)); err != nil {
		t.Fatalf("saved image missing: %v", err)
	}
}

func TestImportZipExtractsAndDeletesArchive(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir, 100)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	zipPath := filepath.Join(dir, "bundle.zip")
	makeZip(t, zipPath, map[string]string{
		"nested/a.sql": "select 1;",
		"nested/b.txt": "ok",
	})

	result, err := manager.Import(ImportRequest{
		UserKey:      "user-1",
		OriginalName: "bundle.zip",
		MessageID:    "om_zip",
		FileKey:      "file_zip",
		ResourceType: "file",
		SourcePath:   zipPath,
		ImportedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if result.Item.Type != ItemTypeZipDir {
		t.Fatalf("type = %q, want %q", result.Item.Type, ItemTypeZipDir)
	}
	if result.Item.Name != "bundle" {
		t.Fatalf("name = %q, want bundle", result.Item.Name)
	}
	if _, err := os.Stat(filepath.Join(result.UserDir, "bundle", "nested", "a.sql")); err != nil {
		t.Fatalf("extracted file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.UserDir, "bundle.zip")); !os.IsNotExist(err) {
		t.Fatalf("zip should not be kept, stat err = %v", err)
	}
}

func TestImportOverwriteRefreshesItem(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir, 100)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	first := filepath.Join(dir, "one.txt")
	if err := os.WriteFile(first, []byte("first"), 0o644); err != nil {
		t.Fatalf("WriteFile first: %v", err)
	}
	_, err = manager.Import(ImportRequest{
		UserKey:      "user-1",
		OriginalName: "report.txt",
		MessageID:    "m1",
		ResourceType: "file",
		SourcePath:   first,
		ImportedAt:   time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Import first: %v", err)
	}

	second := filepath.Join(dir, "two.txt")
	if err := os.WriteFile(second, []byte("second"), 0o644); err != nil {
		t.Fatalf("WriteFile second: %v", err)
	}
	result, err := manager.Import(ImportRequest{
		UserKey:      "user-1",
		OriginalName: "report.txt",
		MessageID:    "m2",
		ResourceType: "file",
		SourcePath:   second,
		ImportedAt:   time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Import second: %v", err)
	}
	if !result.Replaced {
		t.Fatalf("expected overwrite to report replaced")
	}
	data, err := os.ReadFile(filepath.Join(result.UserDir, "report.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "second" {
		t.Fatalf("content = %q, want second", data)
	}
	if len(result.Library.Items) != 1 {
		t.Fatalf("library size = %d, want 1", len(result.Library.Items))
	}
}

func TestQuotaEvictsOldestTopLevelItem(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir, 2)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	importAt := []time.Time{
		time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC),
	}
	for i, ts := range importAt {
		path := filepath.Join(dir, "file-"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile %d: %v", i, err)
		}
		result, err := manager.Import(ImportRequest{
			UserKey:      "user-1",
			OriginalName: filepath.Base(path),
			MessageID:    "m",
			ResourceType: "file",
			SourcePath:   path,
			ImportedAt:   ts,
		})
		if err != nil {
			t.Fatalf("Import %d: %v", i, err)
		}
		if i == 2 {
			if len(result.Evicted) != 1 || result.Evicted[0].Name != "file-a.txt" {
				t.Fatalf("evicted = %+v, want file-a.txt", result.Evicted)
			}
		}
	}

	library, err := manager.Snapshot("user-1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(library.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(library.Items))
	}
	if _, err := os.Stat(filepath.Join(library.RootDir, "file-a.txt")); !os.IsNotExist(err) {
		t.Fatalf("oldest item should be removed, stat err = %v", err)
	}
}

func TestSnapshotRebuildsManifestFromDisk(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir, 100)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	userDir := manager.UserDir("user-1")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "existing.sql"), []byte("select 1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	library, err := manager.Snapshot("user-1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(library.Items) != 1 || library.Items[0].Name != "existing.sql" {
		t.Fatalf("items = %+v, want existing.sql", library.Items)
	}
	if _, err := os.Stat(filepath.Join(userDir, manifestFileName)); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
}

func TestZipSlipIsRejected(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManager(dir, 100)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	zipPath := filepath.Join(dir, "bad.zip")
	makeZip(t, zipPath, map[string]string{
		"../evil.txt": "bad",
	})

	if _, err := manager.Import(ImportRequest{
		UserKey:      "user-1",
		OriginalName: "bad.zip",
		MessageID:    "m1",
		ResourceType: "file",
		SourcePath:   zipPath,
		ImportedAt:   time.Now(),
	}); err == nil {
		t.Fatalf("expected zip slip error")
	}
}

func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatalf("Create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("Write zip entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close zip writer: %v", err)
	}
}
