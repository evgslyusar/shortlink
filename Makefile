.PHONY: dev test lint migrate build

dev:
	docker-compose up -d
	go run ./cmd/server

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

migrate:
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

build:
	go build -o bin/server ./cmd/server

tidy:
	go mod tidy
