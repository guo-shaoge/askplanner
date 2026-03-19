package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
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

func BuildInitialPrompt(normalizedPrompt, summary string, request Request) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(normalizedPrompt))
	sb.WriteString("\n\n## Runtime Context\n")
	sb.WriteString("- You are serving a TiDB query tuning chat relay backed by Codex CLI.\n")
	sb.WriteString("- Answer the user's latest message directly.\n")
	sb.WriteString("- If attachments have a public ID, use that ID in user-facing replies and never expose the hidden .askplanner root path.\n")
	sb.WriteString("- If the latest message is only an attachment upload, acknowledge receipt, return the public ID, and ask the user to send a follow-up question with that ID. Do not analyze the file yet.\n")
	sb.WriteString("- If a later question appears to depend on a prior attachment but no public ID is provided, ask the user to resend the question with the returned ID.\n")
	if strings.TrimSpace(summary) != "" {
		sb.WriteString("\n## Conversation Summary\n")
		sb.WriteString(strings.TrimSpace(summary))
		sb.WriteByte('\n')
	}
	if strings.TrimSpace(request.HistoryText()) != "" {
		sb.WriteString("\n## Message Context\n")
		sb.WriteString(strings.TrimSpace(renderMessageContext(request)))
		sb.WriteByte('\n')
	}
	sb.WriteString("\n## User Message\n")
	sb.WriteString(strings.TrimSpace(request.UserMessage))
	sb.WriteByte('\n')
	return sb.String()
}

func BuildResumePrompt(request Request) string {
	var sb strings.Builder
	sb.WriteString("Continue the existing TiDB query tuning conversation.\n")
	sb.WriteString("If attachments have a public ID, use that ID in user-facing replies and never expose the hidden .askplanner root path.\n")
	sb.WriteString("If a follow-up question needs a prior attachment but no public ID is provided, ask the user to resend the question with the ID.\n")
	if strings.TrimSpace(request.HistoryText()) != "" {
		sb.WriteString("\nMessage context:\n")
		sb.WriteString(strings.TrimSpace(renderMessageContext(request)))
		sb.WriteByte('\n')
	}
	sb.WriteString("\nNew user message:\n")
	sb.WriteString(strings.TrimSpace(request.UserMessage))
	sb.WriteByte('\n')
	return sb.String()
}

// todo check if this too long, output it by hand to check
func renderMessageContext(request Request) string {
	var sb strings.Builder
	for _, note := range request.RuntimeNotes {
		note = strings.TrimSpace(note)
		if note == "" {
			continue
		}
		fmt.Fprintf(&sb, "- %s\n", note)
	}
	for _, attachment := range request.Attachments {
		fmt.Fprintf(&sb, "- attachment kind=%s", strings.TrimSpace(attachment.Kind))
		if publicID := strings.TrimSpace(attachment.PublicID); publicID != "" {
			fmt.Fprintf(&sb, ", public_id=%s", publicID)
		}
		if name := strings.TrimSpace(attachment.OriginalName); name != "" {
			fmt.Fprintf(&sb, ", name=%s", name)
		}
		if path := strings.TrimSpace(attachment.SavedPath); path != "" {
			fmt.Fprintf(&sb, ", saved_path=%s", path)
		}
		if dir := strings.TrimSpace(attachment.ExtractedDir); dir != "" {
			fmt.Fprintf(&sb, ", extracted_dir=%s", dir)
		}
		sb.WriteByte('\n')
		for _, note := range attachment.Notes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			fmt.Fprintf(&sb, "  - %s\n", note)
		}
	}
	return strings.TrimSpace(sb.String())
}
