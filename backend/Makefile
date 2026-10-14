GO ?= go
BIN := bin/server

# Matches the db service in ../docker-compose.yml (set DB_HOST_PORT if 5432 is taken).
# Only `run` and `test-db` use it; an exported DATABASE_URL takes precedence.
DB_HOST_PORT ?= 5432
DEV_DATABASE_URL := postgres://geofence:geofence@localhost:$(DB_HOST_PORT)/geofence?sslmode=disable

.PHONY: run build test test-db lint check

run:
	DATABASE_URL="$${DATABASE_URL:-$(DEV_DATABASE_URL)}" $(GO) run ./cmd/server

build:
	$(GO) build -o $(BIN) ./cmd/server

# Unit tests; database integration tests skip unless DATABASE_URL is set.
test:
	$(GO) test ./...

# Unit + integration tests against the compose database.
test-db:
	DATABASE_URL="$${DATABASE_URL:-$(DEV_DATABASE_URL)}" $(GO) test -count=1 ./...

lint:
	$(GO) vet ./...
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:"; gofmt -l .; exit 1)

# The per-task gate from CLAUDE.md.
check:
	$(GO) build ./... && $(GO) vet ./... && $(GO) test ./...
