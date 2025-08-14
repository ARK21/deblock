.PHONY: build run test lint

build:
	go build ./...

run:
	go run ./cmd/watcher

test:
	go test ./...