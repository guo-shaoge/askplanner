all: cli larkbot

cli:
	go build -o bin/askplanner ./cmd/askplanner
larkbot:
	go build -o bin/larkbot ./cmd/larkbot
clean:
	rm bin/askplanner bin/larkbot
fmt:
	go fmt ./...
