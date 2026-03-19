package codex

import (
	"fmt"
	"strings"
)

type Attachment struct {
	Kind         string
	OriginalName string
	SavedPath    string
	ExtractedDir string
	Notes        []string
}

type Request struct {
	UserMessage  string
	RuntimeNotes []string
	Attachments  []Attachment
}

func NewTextRequest(message string) Request {
	return Request{
		UserMessage: strings.TrimSpace(message),
	}
}

// todo check if this too long, output it by hand to check
func (r Request) HistoryText() string {
	var parts []string
	if msg := strings.TrimSpace(r.UserMessage); msg != "" {
		parts = append(parts, msg)
	}

	if len(r.RuntimeNotes) > 0 {
		var sb strings.Builder
		sb.WriteString("Runtime Context:\n")
		for _, note := range r.RuntimeNotes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			fmt.Fprintf(&sb, "- %s\n", note)
		}
		text := strings.TrimSpace(sb.String())
		if text != "" {
			parts = append(parts, text)
		}
	}

	if len(r.Attachments) > 0 {
		var sb strings.Builder
		sb.WriteString("Attachments:\n")
		for _, attachment := range r.Attachments {
			line := fmt.Sprintf("- %s", strings.TrimSpace(attachment.Kind))
			if name := strings.TrimSpace(attachment.OriginalName); name != "" {
				line += fmt.Sprintf(": %s", name)
			}
			if path := strings.TrimSpace(attachment.SavedPath); path != "" {
				line += fmt.Sprintf(" (saved at %s)", path)
			}
			if dir := strings.TrimSpace(attachment.ExtractedDir); dir != "" {
				line += fmt.Sprintf(" [extracted: %s]", dir)
			}
			sb.WriteString(line)
			sb.WriteByte('\n')
			for _, note := range attachment.Notes {
				note = strings.TrimSpace(note)
				if note == "" {
					continue
				}
				fmt.Fprintf(&sb, "  - %s\n", note)
			}
		}
		parts = append(parts, strings.TrimSpace(sb.String()))
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}
