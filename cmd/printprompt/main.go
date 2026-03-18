package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"lab/askplanner/internal/askplanner"
	askconfig "lab/askplanner/internal/askplanner/config"
	"lab/askplanner/internal/askplanner/tools"
	"lab/askplanner/internal/codex"
)

func main() {
	log.SetFlags(0)

	normalized := flag.Bool("normalized", false, "output the Codex-normalized system prompt")
	flag.Parse()

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

	prompt := agent.SystemPrompt()
	if *normalized {
		prompt = codex.NormalizePrompt(prompt)
	}

	if _, err := fmt.Fprint(os.Stdout, prompt); err != nil {
		log.Fatalf("write prompt: %v", err)
	}
}
