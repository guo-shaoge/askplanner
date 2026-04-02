package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type dedupQuestionEvent struct {
	EventID             string `json:"event_id"`
	Source              string `json:"source"`
	UserKey             string `json:"user_key"`
	ConversationKey     string `json:"conversation_key"`
	Question            string `json:"question"`
	QuestionFingerprint string `json:"question_fingerprint,omitempty"`
	Backfilled          bool   `json:"backfilled,omitempty"`
}

type DedupSummary struct {
	LinesRead         int
	LinesWritten      int
	DuplicatesRemoved int
}

func DedupQuestionEventsFile(srcPath, dstPath string) (DedupSummary, error) {
	srcPath = filepath.Clean(strings.TrimSpace(srcPath))
	dstPath = filepath.Clean(strings.TrimSpace(dstPath))
	if srcPath == "" {
		return DedupSummary{}, fmt.Errorf("source path is empty")
	}
	if dstPath == "" {
		return DedupSummary{}, fmt.Errorf("destination path is empty")
	}

	entries, summary, err := loadDedupEntries(srcPath)
	if err != nil {
		return DedupSummary{}, err
	}
	filtered := dedupQuestionEventEntries(entries)
	summary.LinesWritten = len(filtered)
	summary.DuplicatesRemoved = summary.LinesRead - summary.LinesWritten
	if err := writeDedupEntries(dstPath, filtered); err != nil {
		return DedupSummary{}, err
	}
	return summary, nil
}

type dedupEntry struct {
	raw   json.RawMessage
	event dedupQuestionEvent
}

func loadDedupEntries(path string) ([]dedupEntry, DedupSummary, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, DedupSummary{}, fmt.Errorf("open question event file: %w", err)
	}
	defer closeQuietly(file)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 8*1024*1024)

	var entries []dedupEntry
	var summary DedupSummary
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event dedupQuestionEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, DedupSummary{}, fmt.Errorf("parse question event: %w", err)
		}
		entries = append(entries, dedupEntry{
			raw:   json.RawMessage(append([]byte(nil), line...)),
			event: event,
		})
		summary.LinesRead++
	}
	if err := scanner.Err(); err != nil {
		return nil, DedupSummary{}, fmt.Errorf("scan question event file: %w", err)
	}
	return entries, summary, nil
}

func dedupQuestionEventEntries(entries []dedupEntry) []dedupEntry {
	liveGroups := make(map[string]struct{}, len(entries))
	seenEventIDs := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.event.Backfilled {
			continue
		}
		if key := dedupGroupKey(entry.event); key != "" {
			liveGroups[key] = struct{}{}
		}
	}

	filtered := make([]dedupEntry, 0, len(entries))
	for _, entry := range entries {
		if id := strings.TrimSpace(entry.event.EventID); id != "" {
			if _, exists := seenEventIDs[id]; exists {
				continue
			}
			seenEventIDs[id] = struct{}{}
		}
		if entry.event.Backfilled {
			if _, exists := liveGroups[dedupGroupKey(entry.event)]; exists {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func dedupGroupKey(event dedupQuestionEvent) string {
	fingerprint := strings.TrimSpace(event.QuestionFingerprint)
	if fingerprint == "" {
		fingerprint = questionFingerprint(event.Question)
	}
	return strings.Join([]string{
		normalizeSource(event.Source),
		normalizeUserKey(event.UserKey, event.Source, event.ConversationKey),
		strings.TrimSpace(event.ConversationKey),
		fingerprint,
	}, "|")
}

func writeDedupEntries(path string, entries []dedupEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), "usage-questions-dedup-*.jsonl")
	if err != nil {
		return fmt.Errorf("create temp output file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	for _, entry := range entries {
		if _, err := tmpFile.Write(entry.raw); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("write deduped question event: %w", err)
		}
		if _, err := tmpFile.Write([]byte{'\n'}); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("write deduped question newline: %w", err)
		}
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp output file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace destination file: %w", err)
	}
	return nil
}
