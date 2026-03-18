package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"lab/askplanner/internal/askplanner"
	askconfig "lab/askplanner/internal/askplanner/config"
	"lab/askplanner/internal/askplanner/llmprovider"
	"lab/askplanner/internal/askplanner/tools"
	"lab/askplanner/internal/askplanner/util"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	cfg, err := askconfig.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Build LLM provider
	var provider llmprovider.Provider
	switch cfg.LLMProvider {
	case "kimi":
		provider = llmprovider.NewKimiProvider(cfg.KimiAPIKey, cfg.KimiModel, cfg.KimiBaseURL)
	default:
		log.Fatalf("unsupported LLM provider: %s", cfg.LLMProvider)
	}

	// Build skill index
	skillIdx, err := tools.BuildIndex(cfg.SkillsDir)
	if err != nil {
		log.Fatalf("build skill index: %v", err)
	}
	log.Printf("Skills loaded: %d core, %d oncall, %d customer issues",
		len(skillIdx.CoreFiles), len(skillIdx.OncallFiles), skillIdx.CustomerIssues)

	// Build sandbox with allowed paths
	sandbox := util.NewSandbox(cfg.ProjectRoot, []string{
		"contrib/agent-rules/skills/tidb-query-tuning/references",
		"contrib/tidb/pkg/planner",
		"contrib/tidb/pkg/statistics",
		"contrib/tidb/pkg/expression",
		"contrib/tidb/pkg/parser",
		"contrib/tidb/.agents/skills",
		"contrib/tidb/AGENTS.md",
	})

	// Build tool registry
	toolReg := tools.NewRegistry(
		tools.NewReadFileTool(sandbox),
		tools.NewSearchCodeTool(sandbox, "contrib/tidb/pkg/planner"),
		tools.NewListDirTool(sandbox),
		tools.NewListSkillsTool(cfg.SkillsDir),
		tools.NewReadSkillTool(cfg.SkillsDir),
	)

	// Build agent
	a := askplanner.New(askplanner.AgentConfig{
		Provider:       provider,
		ToolRegistry:   toolReg,
		SkillIndex:     skillIdx,
		Temperature:    cfg.Temperature,
		MaxToolSteps:   cfg.MaxToolSteps,
		MaxResultChars: cfg.MaxResultChars,
		StepDelay:      time.Duration(cfg.StepDelayMS) * time.Millisecond,
	})

	// Run REPL
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Printf("askplanner (model: %s, provider: %s)\n", cfg.KimiModel, cfg.LLMProvider)
	fmt.Println("Type your question, or 'quit' to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}
		if question == "quit" || question == "exit" {
			fmt.Println("Bye!")
			break
		}

		fmt.Println()
		answer, err := a.Answer(ctx, question, func(toolName, args string) {
			// Show tool call progress to the user
			argsSummary := args
			if len(argsSummary) > 100 {
				argsSummary = argsSummary[:100] + "..."
			}
			fmt.Printf("  [tool] %s(%s)\n", toolName, argsSummary)
		})
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		fmt.Println()
		fmt.Println(answer)
		fmt.Println()
	}
}
