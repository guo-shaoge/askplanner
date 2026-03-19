package lark

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var knownPlanReplayerFiles = map[string]struct{}{
	"sql_meta.toml":             {},
	"config.toml":               {},
	"meta.txt":                  {},
	"variables.toml":            {},
	"table_tiflash_replica.txt": {},
	"session_bindings.sql":      {},
	"global_bindings.sql":       {},
	"schema_meta.txt":           {},
	"errors.txt":                {},
}

type PlanReplayerManifest struct {
	DetectedFiles []string
}

func ExtractPlanReplayer(zipPath, extractedDir string) (PlanReplayerManifest, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return PlanReplayerManifest{}, fmt.Errorf("open zip bundle: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(extractedDir, 0o755); err != nil {
		return PlanReplayerManifest{}, fmt.Errorf("create extracted dir: %w", err)
	}

	detected := make([]string, 0, len(knownPlanReplayerFiles))
	for _, file := range reader.File {
		targetPath, err := safeJoinUnder(extractedDir, file.Name)
		if err != nil {
			return PlanReplayerManifest{}, err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return PlanReplayerManifest{}, fmt.Errorf("create extracted subdir: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return PlanReplayerManifest{}, fmt.Errorf("create extracted parent dir: %w", err)
		}

		src, err := file.Open()
		if err != nil {
			return PlanReplayerManifest{}, fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}
		dst, err := os.Create(targetPath)
		if err != nil {
			_ = src.Close()
			return PlanReplayerManifest{}, fmt.Errorf("create extracted file %s: %w", targetPath, err)
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = src.Close()
			_ = dst.Close()
			return PlanReplayerManifest{}, fmt.Errorf("extract zip entry %s: %w", file.Name, err)
		}
		if err := src.Close(); err != nil {
			_ = dst.Close()
			return PlanReplayerManifest{}, fmt.Errorf("close zip entry %s: %w", file.Name, err)
		}
		if err := dst.Close(); err != nil {
			return PlanReplayerManifest{}, fmt.Errorf("close extracted file %s: %w", targetPath, err)
		}

		if _, ok := knownPlanReplayerFiles[strings.ToLower(filepath.Base(file.Name))]; ok {
			rel, err := filepath.Rel(extractedDir, targetPath)
			if err != nil {
				rel = filepath.Base(targetPath)
			}
			detected = append(detected, rel)
		}
	}

	sort.Strings(detected)
	return PlanReplayerManifest{DetectedFiles: detected}, nil
}

func safeJoinUnder(root, child string) (string, error) {
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Join(cleanRoot, child)
	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil {
		return "", fmt.Errorf("resolve extracted path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe zip entry path: %s", child)
	}
	return cleanTarget, nil
}
