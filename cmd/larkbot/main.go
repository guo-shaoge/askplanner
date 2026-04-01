package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"lab/askplanner/internal/config"
	botapp "lab/askplanner/internal/larkbot"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalStartup("load config", err, "Check PROJECT_ROOT and PROMPT_FILE, or start the process from the repository root.")
	}

	logFile, err := config.SetupLogging(cfg.LogFile)
	if err != nil {
		fatalStartup("setup logging", err, "Check LOG_FILE and make sure the target directory is writable.")
	}
	defer func() {
		_ = logFile.Close()
	}()

	if _, err := exec.LookPath(cfg.CodexBin); err != nil {
		fatalStartup("locate Codex CLI", err, fmt.Sprintf("Install Codex CLI or point CODEX_BIN to a valid executable. Current value: %s", cfg.CodexBin))
	}

	app, err := botapp.New(cfg)
	if err != nil {
		fatalStartup("build larkbot app", err, "Check FEISHU_APP_ID/FEISHU_APP_SECRET or FEISHU_BOTS_JSON, FEISHU_FILE_DIR, and any local storage paths used by attachments or Clinic snapshots.")
	}
	if err := app.Run(context.Background()); err != nil {
		fatalStartup("start lark websocket client", err, "Check FEISHU_APP_ID/FEISHU_APP_SECRET or FEISHU_BOTS_JSON and verify outbound network access to Feishu.")
	}
}

func fatalStartup(component string, err error, hints ...string) {
	var sb strings.Builder
	sb.WriteString("startup error: ")
	sb.WriteString(component)
	if err != nil {
		sb.WriteString(": ")
		sb.WriteString(err.Error())
	}
	for _, hint := range hints {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			continue
		}
		sb.WriteString("\n- ")
		sb.WriteString(hint)
	}
	message := sb.String()
	fmt.Fprintln(os.Stderr, message)
	log.Printf("%s", strings.ReplaceAll(message, "\n", " | "))
	os.Exit(1)
}
