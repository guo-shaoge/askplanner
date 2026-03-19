package lark

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Cleaner struct {
	Root string
}

func (c *Cleaner) CleanupExpired(now time.Time) error {
	entries, err := os.ReadDir(c.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read attachment root: %w", err)
	}

	var errs []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		conversationDir := filepath.Join(c.Root, entry.Name())
		messageEntries, err := os.ReadDir(conversationDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("read conversation dir %s: %w", conversationDir, err))
			continue
		}
		for _, messageEntry := range messageEntries {
			if !messageEntry.IsDir() {
				continue
			}
			messageDir := filepath.Join(conversationDir, messageEntry.Name())
			expired, err := messageDirExpired(messageDir, now)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if !expired {
				continue
			}
			if err := os.RemoveAll(messageDir); err != nil {
				errs = append(errs, fmt.Errorf("remove expired bundle %s: %w", messageDir, err))
			}
		}
		remaining, err := os.ReadDir(conversationDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("re-read conversation dir %s: %w", conversationDir, err))
			continue
		}
		if len(remaining) == 0 {
			if err := os.Remove(conversationDir); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("remove empty conversation dir %s: %w", conversationDir, err))
			}
		}
	}

	return errors.Join(errs...)
}

func messageDirExpired(messageDir string, now time.Time) (bool, error) {
	metaPath := filepath.Join(messageDir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read bundle metadata %s: %w", metaPath, err)
	}
	var meta BundleMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return false, fmt.Errorf("parse bundle metadata %s: %w", metaPath, err)
	}
	if meta.ExpiresAt.IsZero() {
		return false, nil
	}
	return !meta.ExpiresAt.After(now), nil
}
