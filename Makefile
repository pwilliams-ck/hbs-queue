VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -ldflags "\
 -X github.com/CloudKey-io/hbs-queue/internal/config.Version=$(VERSION) \
 -X github.com/CloudKey-io/hbs-queue/internal/config.Commit=$(COMMIT) \
 -X github.com/CloudKey-io/hbs-queue/internal/config.BuildTime=$(BUILD_TIME)"

.PHONY: help run build test lint check clean dev-up dev-down dev-reset dev-logs

## help: show this help message
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'

## run: start the service locally
run:
	@go run ./cmd/hbsqueue || true

## build: compile binary with version info into bin/
build:
	go build $(LDFLAGS) -o bin/hbsqueue ./cmd/hbsqueue

## test: run all tests with race detector and coverage
test:
	go test -v -race -cover ./...

## lint: run golangci-lint
lint:
	golangci-lint run

## check: run lint + test (use before pushing)
check: lint test

## clean: remove build artifacts
clean:
	rm -rf bin/

## dev-up: start local dev dependencies (Postgres, Swagger UI)
dev-up:
	docker compose up -d
	@printf "Waiting for Postgres..."
	@until docker compose exec -T postgres pg_isready -U hbsqueue -d hbsqueue_dev -q 2>/dev/null; do \
		printf "."; sleep 1; \
	done
	@echo " ready!"
	@echo "Swagger UI at http://localhost:8081"

## dev-down: stop local dev dependencies
dev-down:
	docker compose down

## dev-reset: stop dev dependencies and delete volumes (clean DB)
dev-reset:
	docker compose down -v

## dev-logs: tail logs from dev dependencies
dev-logs:
	docker compose logs -f
