package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"lab/askplanner/internal/config"
	"lab/askplanner/internal/usage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalStartup("load config", err, "Check PROJECT_ROOT and PROMPT_FILE, or start the process from the repository root.")
	}

	logFile, err := config.SetupLogging(cfg.UsageServerLogFile)
	if err != nil {
		fatalStartup("setup logging", err, "Check USAGE_LOG_FILE and make sure the target directory is writable.")
	}
	defer func() {
		_ = logFile.Close()
	}()

	collector, err := usage.NewCollector(cfg)
	if err != nil {
		fatalStartup("build usage collector", err)
	}
	server, err := usage.NewServer(collector)
	if err != nil {
		fatalStartup("build usage dashboard", err)
	}

	addr := strings.TrimSpace(cfg.UsageHTTPAddr)
	if addr == "" {
		addr = "127.0.0.1:18080"
	}
	log.Printf("[usage] dashboard listening on http://%s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		fatalStartup("start usage dashboard", err, "Check USAGE_HTTP_ADDR and whether the port is already in use.")
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
