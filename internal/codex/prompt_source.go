package codex

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"lab/askplanner/internal/askplanner"
	askconfig "lab/askplanner/internal/askplanner/config"
	"lab/askplanner/internal/askplanner/tools"
)

// PromptSource loads the askplanner bootstrap prompt for Codex sessions.
type PromptSource struct {
	projectRoot   string
	promptCommand string
	skillsDir     string
	docsOverlay   string
	tidbDocsDir   string
}

func NewPromptSource(cfg *askconfig.Config) *PromptSource {
	return &PromptSource{
		projectRoot:   cfg.CodexProjectRoot,
		promptCommand: cfg.CodexPromptCommand,
		skillsDir:     cfg.SkillsDir,
		docsOverlay:   cfg.DocsOverlayDir,
		tidbDocsDir:   cfg.TiDBDocsDir,
	}
}

func (s *PromptSource) Load(ctx context.Context) (string, error) {
	if prompt, err := s.loadFromCommand(ctx); err == nil && prompt != "" {
		return prompt, nil
	}
	return s.loadFallback()
}

func (s *PromptSource) loadFromCommand(ctx context.Context) (string, error) {
	if strings.TrimSpace(s.promptCommand) == "" {
		return "", fmt.Errorf("prompt command is empty")
	}

	parts := strings.Fields(s.promptCommand)
	if len(parts) == 0 {
		return "", fmt.Errorf("prompt command is empty")
	}

	commandPath := parts[0]
	if !filepath.IsAbs(commandPath) {
		commandPath = filepath.Join(s.projectRoot, commandPath)
	}

	cmd := exec.CommandContext(ctx, commandPath, parts[1:]...)
	cmd.Dir = s.projectRoot
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run prompt command %q: %w", commandPath, err)
	}

	prompt := strings.TrimSpace(string(output))
	if prompt == "" {
		return "", fmt.Errorf("prompt command %q returned empty output", commandPath)
	}
	return prompt, nil
}

func (s *PromptSource) loadFallback() (string, error) {
	skillIdx, err := tools.BuildIndex(s.skillsDir)
	if err != nil {
		return "", fmt.Errorf("build skill index: %w", err)
	}

	docsOverlay := tools.LoadDocsOverlay(s.docsOverlay, s.tidbDocsDir)
	agent := askplanner.New(askplanner.AgentConfig{
		SkillIndex:  skillIdx,
		DocsOverlay: docsOverlay,
	})
	return strings.TrimSpace(agent.SystemPrompt()), nil
}
