all:
	go build -o bin/askplanner ./cmd/askplanner
clean:
	rm bin/askplanner
