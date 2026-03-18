package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const toolAdaptationHeader = `Treat the following bootstrap prompt as the domain-specific instruction for this session.

This runtime uses Codex CLI instead of askplanner's custom tools.

Tool adaptation rules:
- If the prompt mentions read_file, inspect files with shell commands such as rg, nl -ba, sed -n, and cat.
- If the prompt mentions search_code, search the workspace with rg.
- If the prompt mentions list_dir, inspect directories with rg --files, find, or ls.
- If the prompt mentions list_skills or read_skill, browse the skills repository directly by filename.
- If the prompt mentions search_docs, search the curated TiDB docs inside the local workspace and then open the matching file sections directly.

Runtime rules:
- The workspace is read-only unless the user explicitly asks for code changes.
- Prefer local code, local docs, and local skills over speculation.
- Reply in the user's language unless the user asks for a different language.`

func NormalizePrompt(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, toolAdaptationHeader) {
		return raw + "\n"
	}
	return fmt.Sprintf("%s\n\n%s\n", toolAdaptationHeader, raw)
}

func PromptHash(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

func BuildInitialPrompt(normalizedPrompt, summary, question string) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(normalizedPrompt))
	sb.WriteString("\n\n## Runtime Context\n")
	sb.WriteString("- You are serving a TiDB query tuning chat relay backed by Codex CLI.\n")
	sb.WriteString("- Answer the user's latest message directly.\n")
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

func BuildResumePrompt(question string) string {
	return fmt.Sprintf(
		"Continue the existing TiDB query tuning conversation.\n\nNew user message:\n%s\n",
		strings.TrimSpace(question),
	)
}
