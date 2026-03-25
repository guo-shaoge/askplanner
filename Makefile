GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

all: cli larkbot

cli:
	go build -o bin/askplanner_cli ./cmd/askplanner

larkbot:
	go build -o bin/askplanner_larkbot ./cmd/larkbot
	go build -o bin/askplanner_larkbot_staging ./cmd/larkbot

clean:
	rm -f bin/askplanner_cli bin/askplanner_larkbot

fmt:
	go fmt ./...

lint:
	$(GOLANGCI_LINT) run ./...

.PHONY: all cli larkbot clean fmt lint
