GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

all: cli larkbot usage migrate-userdata dedup-usage-questions

cli:
	go build -o bin/askplanner_cli ./cmd/askplanner

larkbot:
	go build -o bin/askplanner_larkbot ./cmd/larkbot
	go build -o bin/askplanner_larkbot_staging ./cmd/larkbot

usage:
	go build -o bin/askplanner_usage ./cmd/askplanner_usage

migrate-userdata:
	go build -o bin/askplanner_migrate_userdata ./cmd/askplanner_migrate_userdata

dedup-usage-questions:
	go build -o bin/askplanner_dedup_usage_questions ./cmd/askplanner_dedup_usage_questions

clean:
	rm -f bin/askplanner_cli bin/askplanner_larkbot bin/askplanner_larkbot_staging bin/askplanner_usage bin/askplanner_migrate_userdata bin/askplanner_dedup_usage_questions

fmt:
	go fmt ./...

lint:
	$(GOLANGCI_LINT) run ./...

.PHONY: all cli larkbot usage migrate-userdata dedup-usage-questions clean fmt lint
