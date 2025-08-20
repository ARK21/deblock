.PHONY: build run test lint

build:
	go build ./...

run:
	go run ./cmd/watcher

test:
	go test ./...

test-kafka:
	go test -tags=kafka ./test -run TestCQRS_EventBus_PublishesToKafka -v