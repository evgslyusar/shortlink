.PHONY: env-up env-down env-reset env-status env-logs \
       build dev-api dev-bot \
       migrate-up migrate-down migrate-status migrate-new \
       gen-keys \
       test-unit test-int test-all lint tidy \
       web-install web-dev web-build web-test web-lint dev

# Load .env.local if it exists.
-include .env.local
export

DATABASE_URL ?= postgres://shortlink:shortlink@localhost:5432/shortlink?sslmode=disable

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
	CGO_ENABLED=0 go build -o bin/slinkapi ./cmd/slinkapi
	CGO_ENABLED=0 go build -o bin/slinkbot ./cmd/slinkbot

dev-api: env-up
	go run ./cmd/slinkapi

dev-bot:
	go run ./cmd/slinkbot

# --- Migrations ---

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-status:
	migrate -path migrations -database "$(DATABASE_URL)" version

migrate-new:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-new name=create_foo"; exit 1; fi
	@num=$$(printf "%06d" $$(($$(ls migrations/*.up.sql 2>/dev/null | wc -l) + 1))); \
	touch "migrations/$${num}_$(name).up.sql" "migrations/$${num}_$(name).down.sql"; \
	echo "Created migrations/$${num}_$(name).{up,down}.sql"

# --- Keys ---

gen-keys:
	mkdir -p keys
	openssl genrsa -out keys/private.pem 4096
	openssl rsa -in keys/private.pem -pubout -out keys/public.pem
	@echo "Keys generated in keys/"

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

# --- Frontend ---

web-install:
	cd web && npm ci

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

web-test:
	cd web && npm test

web-lint:
	cd web && npm run lint && npx tsc --noEmit

dev:
	$(MAKE) dev-api & $(MAKE) web-dev
