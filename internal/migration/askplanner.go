package migration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Summary struct {
	DirectoriesCreated int
	FilesCopied        int
	SymlinksCreated    int
	BytesCopied        int64
}

func CopyAskplannerUserData(srcRoot, dstRoot string) (Summary, error) {
	srcRoot = filepath.Clean(strings.TrimSpace(srcRoot))
	dstRoot = filepath.Clean(strings.TrimSpace(dstRoot))
	if srcRoot == "" || srcRoot == "." {
		return Summary{}, fmt.Errorf("source root is empty")
	}
	if dstRoot == "" || dstRoot == "." {
		return Summary{}, fmt.Errorf("destination root is empty")
	}

	srcAbs, err := filepath.Abs(srcRoot)
	if err != nil {
		return Summary{}, fmt.Errorf("resolve source root: %w", err)
	}
	dstAbs, err := filepath.Abs(dstRoot)
	if err != nil {
		return Summary{}, fmt.Errorf("resolve destination root: %w", err)
	}
	if srcAbs == dstAbs {
		return Summary{}, fmt.Errorf("source and destination must be different")
	}
	if rel, err := filepath.Rel(srcAbs, dstAbs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return Summary{}, fmt.Errorf("destination cannot be inside source root")
	}

	info, err := os.Stat(srcAbs)
	if err != nil {
		return Summary{}, fmt.Errorf("stat source root: %w", err)
	}
	if !info.IsDir() {
		return Summary{}, fmt.Errorf("source root is not a directory")
	}

	var summary Summary
	if err := ensureDir(dstAbs, 0o755, &summary); err != nil {
		return Summary{}, err
	}

	err = filepath.WalkDir(srcAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == srcAbs {
			return nil
		}

		relPath, err := filepath.Rel(srcAbs, path)
		if err != nil {
			return fmt.Errorf("compute relative path for %s: %w", path, err)
		}
		if shouldSkip(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(dstAbs, relPath)
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			if err := copySymlink(path, dstPath, srcAbs, dstAbs, &summary); err != nil {
				return err
			}
			return nil
		}
		if d.IsDir() {
			return ensureDir(dstPath, info.Mode().Perm(), &summary)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, dstPath, info.Mode().Perm(), &summary)
	})
	if err != nil {
		return Summary{}, err
	}
	return summary, nil
}

func shouldSkip(relPath string) bool {
	parts := splitPath(relPath)
	if len(parts) == 0 {
		return false
	}
	if len(parts) >= 2 && parts[0] == "workspaces" && parts[1] == "mirrors" {
		return true
	}
	if len(parts) >= 6 &&
		parts[0] == "workspaces" &&
		parts[1] == "users" &&
		parts[3] == "root" &&
		parts[4] == "contrib" {
		switch parts[5] {
		case "tidb", "tidb-docs", "agent-rules":
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return nil
	}
	return strings.Split(cleaned, string(filepath.Separator))
}

func ensureDir(path string, perm os.FileMode, summary *Summary) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("destination path exists but is not a directory: %s", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("stat destination dir %s: %w", path, err)
	}
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("create destination dir %s: %w", path, err)
	}
	if summary != nil {
		summary.DirectoriesCreated++
	}
	return nil
}

func copyFile(srcPath, dstPath string, perm os.FileMode, summary *Summary) error {
	if err := ensureDir(filepath.Dir(dstPath), 0o755, summary); err != nil {
		return err
	}
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source file %s: %w", srcPath, err)
	}
	defer func() {
		_ = srcFile.Close()
	}()

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("open destination file %s: %w", dstPath, err)
	}
	defer func() {
		_ = dstFile.Close()
	}()

	written, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy file %s -> %s: %w", srcPath, dstPath, err)
	}
	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("close destination file %s: %w", dstPath, err)
	}
	modTime := fileModTimeOrNow(srcPath)
	_ = os.Chtimes(dstPath, modTime, modTime)
	if summary != nil {
		summary.FilesCopied++
		summary.BytesCopied += written
	}
	return nil
}

func fileModTimeOrNow(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now()
	}
	return info.ModTime()
}

func copySymlink(srcPath, dstPath, srcRoot, dstRoot string, summary *Summary) error {
	if err := ensureDir(filepath.Dir(dstPath), 0o755, summary); err != nil {
		return err
	}
	target, err := os.Readlink(srcPath)
	if err != nil {
		return fmt.Errorf("read symlink %s: %w", srcPath, err)
	}
	rewritten, err := rewriteSymlinkTarget(target, dstPath, srcRoot, dstRoot)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dstPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing destination path %s: %w", dstPath, err)
	}
	if err := os.Symlink(rewritten, dstPath); err != nil {
		return fmt.Errorf("create symlink %s -> %s: %w", dstPath, rewritten, err)
	}
	if summary != nil {
		summary.SymlinksCreated++
	}
	return nil
}

func rewriteSymlinkTarget(target, dstPath, srcRoot, dstRoot string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("symlink target is empty for %s", dstPath)
	}
	if !filepath.IsAbs(target) {
		return target, nil
	}
	rel, err := filepath.Rel(srcRoot, target)
	if err != nil {
		return "", fmt.Errorf("rewrite symlink target %s: %w", target, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return target, nil
	}
	mappedTarget := filepath.Join(dstRoot, rel)
	relativeTarget, err := filepath.Rel(filepath.Dir(dstPath), mappedTarget)
	if err != nil {
		return "", fmt.Errorf("rewrite symlink target %s: %w", target, err)
	}
	return relativeTarget, nil
}
