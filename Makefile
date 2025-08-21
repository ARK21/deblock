.PHONY: build run test lint

build:
	go build ./...

run:
	go run ./cmd/watcher

up:
	docker compose up -d

down:
	docker compose down -v

logs:
	docker compose logs -f watcher

fmt:
	go fmt ./...

lint:
	go vet ./...

test:
	go test ./...

test-kafka:
	go test -tags=kafka ./test -run TestCQRS_EventBus_PublishesToKafka -v