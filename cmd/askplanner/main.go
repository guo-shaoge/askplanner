package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"lab/askplanner/internal/clinic"
	"lab/askplanner/internal/codex"
	"lab/askplanner/internal/config"
	"lab/askplanner/internal/workspace"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logFile, err := config.SetupLogging(cfg.LogFile)
	if err != nil {
		log.Fatalf("setup logging: %v", err)
	}
	defer logFile.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	responder, err := codex.NewResponder(cfg)
	if err != nil {
		log.Fatalf("build codex responder: %v", err)
	}
	prefetcher, err := clinic.NewPrefetcher(cfg)
	if err != nil {
		log.Fatalf("build clinic prefetcher: %v", err)
	}
	workspaceManager, err := workspace.NewManager(cfg)
	if err != nil {
		log.Fatalf("build workspace manager: %v", err)
	}

	fmt.Printf("askplanner v2 (backend: codex-cli, model: %s)\n", cfg.CodexModel)
	fmt.Println("Type your question, or 'quit' to exit. Use 'reset' to start a new session.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	const conversationKey = "cli:default"
	const clinicUserKey = "cli_default"
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
		if err := workspaceManager.MaybeSweep(ctx); err != nil {
			log.Printf("[askplanner] workspace GC failed: %v", err)
		}
		if question == "reset" {
			if err := responder.Reset(conversationKey); err != nil {
				fmt.Printf("Error: %v\n\n", err)
			} else {
				fmt.Println("Session reset.")
				fmt.Println()
			}
			continue
		}
		if cmd, matched, err := workspace.ParseCommand(question); matched {
			fmt.Println()
			if err != nil {
				fmt.Printf("Error: %v\n\n", err)
				continue
			}
			answer, err := runWorkspaceCommand(ctx, workspaceManager, responder, prefetcher, conversationKey, clinicUserKey, cmd)
			if err != nil {
				fmt.Printf("Error: %v\n\n", err)
				continue
			}
			fmt.Println(answer)
			fmt.Println()
			continue
		}

		fmt.Println()
		ws, err := workspaceManager.Ensure(ctx, clinicUserKey)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}
		enriched, err := prefetcher.Enrich(ctx, clinicUserKey, question, workspace.BindRuntimeContext(codex.RuntimeContext{}, ws))
		if err != nil {
			if msg := clinic.UserFacingMessage(err); msg != "" {
				log.Printf("[askplanner] clinic prefetch user-visible error: %v", err)
				fmt.Printf("%s\n\n", msg)
				continue
			}
			fmt.Printf("Error: %v\n\n", err)
			continue
		}
		enriched.RuntimeContext = workspace.BindRuntimeContext(enriched.RuntimeContext, ws)
		if strings.TrimSpace(enriched.IntroReply) != "" {
			fmt.Println(enriched.IntroReply)
			fmt.Println()
			continue
		}

		answer, err := responder.AnswerWithContext(ctx, conversationKey, question, enriched.RuntimeContext)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		fmt.Println()
		fmt.Println(answer)
		fmt.Println()
	}
}

func runWorkspaceCommand(ctx context.Context, manager *workspace.Manager, responder *codex.Responder, prefetcher *clinic.Prefetcher, conversationKey, userKey string, cmd *workspace.Command) (string, error) {
	var (
		ws  *workspace.Workspace
		err error
	)
	switch cmd.Action {
	case "status":
		ws, err = manager.Status(ctx, userKey)
	case "switch":
		ws, err = manager.SwitchRepo(ctx, userKey, cmd.Repo, cmd.Ref)
	case "sync":
		ws, err = manager.Sync(ctx, userKey, cmd.Repo)
	case "reset":
		ws, err = manager.Reset(ctx, userKey, cmd.Repo)
	default:
		return "", fmt.Errorf("unsupported workspace command: %s", cmd.Action)
	}
	if err != nil {
		return "", err
	}
	status := workspace.FormatStatus(ws)
	if strings.TrimSpace(cmd.Question) == "" {
		return status, nil
	}

	enriched, err := prefetcher.Enrich(ctx, userKey, cmd.Question, workspace.BindRuntimeContext(codex.RuntimeContext{}, ws))
	if err != nil {
		if msg := clinic.UserFacingMessage(err); msg != "" {
			return status + "\n\n" + msg, nil
		}
		return "", err
	}
	enriched.RuntimeContext = workspace.BindRuntimeContext(enriched.RuntimeContext, ws)
	if strings.TrimSpace(enriched.IntroReply) != "" {
		return status + "\n\n" + enriched.IntroReply, nil
	}
	answer, err := responder.AnswerWithContext(ctx, conversationKey, cmd.Question, enriched.RuntimeContext)
	if err != nil {
		return "", err
	}
	return status + "\n\n" + answer, nil
}
