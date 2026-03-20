package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func LoadPrompt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read prompt file %s: %w", path, err)
	}
	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return "", fmt.Errorf("prompt file %s is empty", path)
	}
	return prompt, nil
}

func PromptHash(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

type AttachmentItem struct {
	Name         string
	Type         string
	SavedAt      time.Time
	OriginalName string
}

type AttachmentContext struct {
	RootDir string
	Items   []AttachmentItem
}

func BuildInitialPrompt(normalizedPrompt, summary, question string, attachment AttachmentContext) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(normalizedPrompt))
	sb.WriteString("\n\n## Runtime Context\n")
	sb.WriteString("- You are serving a TiDB query tuning chat relay backed by Codex CLI.\n")
	sb.WriteString("- Answer the user's latest message directly.\n")
	writeAttachmentContext(&sb, attachment)
	if strings.TrimSpace(summary) != "" {
		sb.WriteString("\n## Conversation Summary\n")
		sb.WriteString(strings.TrimSpace(summary))
		sb.WriteByte('\n')
	}
	sb.WriteString("\n## User Message\n")
	sb.WriteString(strings.TrimSpace(question))
	sb.WriteByte('\n')
	return sb.String()
}

func BuildResumePrompt(question string, attachment AttachmentContext) string {
	var sb strings.Builder
	sb.WriteString("Continue the existing TiDB query tuning conversation.\n")
	writeAttachmentContext(&sb, attachment)
	sb.WriteString("\nNew user message:\n")
	sb.WriteString(strings.TrimSpace(question))
	sb.WriteByte('\n')
	return sb.String()
}

func writeAttachmentContext(sb *strings.Builder, attachment AttachmentContext) {
	rootDir := strings.TrimSpace(attachment.RootDir)
	if rootDir == "" {
		return
	}

	sb.WriteString("- The current user's uploaded-file library is stored under: ")
	sb.WriteString(rootDir)
	sb.WriteString("\n")
	sb.WriteString("- If the user asks you to inspect or analyze a file, first inspect this user library.\n")
	sb.WriteString("- If you cannot tell which file the user means, do not guess. Use only the visible top-level entries below and ask the user which one to inspect.\n")

	items := append([]AttachmentItem(nil), attachment.Items...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].SavedAt.Equal(items[j].SavedAt) {
			return items[i].Name < items[j].Name
		}
		return items[i].SavedAt.After(items[j].SavedAt)
	})
	if len(items) == 0 {
		sb.WriteString("- Current top-level entries: none.\n")
		return
	}

	sb.WriteString("- Current top-level entries (newest first):\n")
	for _, item := range items {
		sb.WriteString("  - ")
		sb.WriteString(item.Name)
		if strings.TrimSpace(item.Type) != "" {
			sb.WriteString(" [")
			sb.WriteString(strings.TrimSpace(item.Type))
			sb.WriteByte(']')
		}
		if !item.SavedAt.IsZero() {
			sb.WriteString(" saved_at=")
			sb.WriteString(item.SavedAt.Format(time.RFC3339))
		}
		if original := strings.TrimSpace(item.OriginalName); original != "" && original != item.Name {
			sb.WriteString(" original_name=")
			sb.WriteString(original)
		}
		sb.WriteByte('\n')
	}
}
