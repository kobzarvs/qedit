GO ?= go
BIN ?= qedit
GOLANGCI_LINT ?= golangci-lint
ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

.DEFAULT_GOAL := help

.PHONY: help build run tidy test fmt lint

help:
	@printf "Targets:\n"
	@printf "  make build  - build ./cmd/qedit\n"
	@printf "  make run    - run ./cmd/qedit [args]\n"
	@printf "  make tidy   - go mod tidy\n"
	@printf "  make test   - go test ./...\n"
	@printf "  make fmt    - go fmt ./...\n"
	@printf "  make lint   - golangci-lint run\n"

build:
	$(GO) build -o $(BIN) ./cmd/qedit

run:
	$(GO) run ./cmd/qedit -- $(ARGS)

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

lint:
	$(GOLANGCI_LINT) run

%:
	@:
