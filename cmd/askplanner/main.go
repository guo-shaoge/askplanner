package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	askconfig "lab/askplanner/internal/askplanner/config"
	"lab/askplanner/internal/codex"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	cfg, err := askconfig.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Run REPL
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	responder, err := codex.NewResponder(ctx, cfg)
	if err != nil {
		log.Fatalf("build codex responder: %v", err)
	}

	fmt.Printf("askplanner (backend: codex-cli, model: %s)\n", cfg.CodexModel)
	fmt.Println("Type your question, or 'quit' to exit. Use 'reset' to start a new local session.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	const conversationKey = "cli:default"
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
		if question == "reset" {
			if err := responder.Reset(conversationKey); err != nil {
				fmt.Printf("Error: %v\n\n", err)
			} else {
				fmt.Println("Local Codex session reset.")
				fmt.Println()
			}
			continue
		}

		fmt.Println()
		answer, err := responder.Answer(ctx, conversationKey, question)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		fmt.Println()
		fmt.Println(answer)
		fmt.Println()
	}
}
