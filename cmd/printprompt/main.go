package main

import (
	"fmt"
	"log"
	"os"

	"lab/askplanner/internal/askplanner"
	askconfig "lab/askplanner/internal/askplanner/config"
	"lab/askplanner/internal/askplanner/tools"
)

func main() {
	log.SetFlags(0)

	cfg, err := askconfig.LoadPromptOnly()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	skillIdx, err := tools.BuildIndex(cfg.SkillsDir)
	if err != nil {
		log.Fatalf("build skill index: %v", err)
	}

	docsOverlay := tools.LoadDocsOverlay(cfg.DocsOverlayDir, cfg.TiDBDocsDir)
	agent := askplanner.New(askplanner.AgentConfig{
		SkillIndex:  skillIdx,
		DocsOverlay: docsOverlay,
	})

	if _, err := fmt.Fprint(os.Stdout, agent.SystemPrompt()); err != nil {
		log.Fatalf("write prompt: %v", err)
	}
}
