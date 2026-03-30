.PHONY: env-up env-down env-reset env-status env-logs \
       build build-bot run run-bot test-unit test-int test-all lint tidy

# --- Infrastructure ---

env-up:
	docker compose up -d

env-down:
	docker compose down

env-reset:
	docker compose down -v
	docker compose up -d

env-status:
	docker compose ps

env-logs:
	docker compose logs -f $(s)

# --- Build & Run ---

build:
	go build -o bin/slinkapi ./cmd/slinkapi

build-bot:
	go build -o bin/slinkbot ./cmd/slinkbot

run: build
	source .env.local 2>/dev/null; ./bin/slinkapi

run-bot: build-bot
	source .env.local 2>/dev/null; ./bin/slinkbot

# --- Testing ---

test-unit:
	go test ./... -short -race -count=1

test-int: env-status
	go test -tags=integration -race -count=1 ./...

test-all: test-unit test-int

# --- Quality ---

lint:
	golangci-lint run ./...

tidy:
	go mod tidy
