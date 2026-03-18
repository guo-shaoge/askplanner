all: cli larkbot

cli:
	go build -o bin/askplanner_cli ./cmd/askplanner
larkbot:
	go build -o bin/askplanner_lark ./cmd/larkbot
clean:
	rm -f bin/askplanner_cli bin/askplanner_lark
fmt:
	go fmt ./...
