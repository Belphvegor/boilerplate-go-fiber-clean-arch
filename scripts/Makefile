SHELL := /bin/bash

GO ?= go
GOLANGCI_LINT ?= golangci-lint
MIGRATE ?= go run ./cmd/tools/migrate

.PHONY: build run fmt lint test tidy migrate-up migrate-down migrate-status

build:
	$(GO) build ./...

run:
	$(GO) run ./cmd/api

fmt:
	@echo "Formatting Go sources"
	@files=$$(find cmd internal pkg -name '*.go' 2>/dev/null); \
	if [ -n "$$files" ]; then \
		gofmt -w $$files; \
	else \
		echo "No Go files to format yet"; \
	fi

lint:
	@echo "Running lint checks"
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		$(GOLANGCI_LINT) run ./...; \
	else \
		echo "golangci-lint not available, falling back to go vet"; \
		$(GO) vet ./...; \
	fi

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down

migrate-status:
	$(MIGRATE) status
