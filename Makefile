VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -ldflags "\
 -X github.com/CloudKey-io/hbs-queue/internal/config.Version=$(VERSION) \
 -X github.com/CloudKey-io/hbs-queue/internal/config.Commit=$(COMMIT) \
 -X github.com/CloudKey-io/hbs-queue/internal/config.BuildTime=$(BUILD_TIME)"

.PHONY: run build test lint clean

## run: start the service locally
run:
	go run ./cmd/hbsqueue

## build: compile binary with version info into bin/
build:
	go build $(LDFLAGS) -o bin/hbsqueue ./cmd/hbsqueue

## test: run all tests with race detector and coverage
test:
	go test -v -race -cover ./...

## lint: run golangci-lint
lint:
	golangci-lint run

## clean: remove build artifacts
clean:
	rm -rf bin/
