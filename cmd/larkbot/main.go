package main

import (
	"context"
	"log"

	"lab/askplanner/internal/config"
	botapp "lab/askplanner/internal/larkbot"
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

	app, err := botapp.New(cfg)
	if err != nil {
		log.Fatalf("build larkbot app: %v", err)
	}
	if err := app.Run(context.Background()); err != nil {
		log.Fatalf("lark websocket start: %v", err)
	}
}
