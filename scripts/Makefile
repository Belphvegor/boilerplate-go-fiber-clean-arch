SHELL := /bin/bash

GO ?= go
GOLANGCI_LINT ?= golangci-lint
# Runs golang-migrate through the compose `migrator` service; the host name is the compose service.
MIGRATE ?= docker compose --profile migrate run --rm migrator
MIGRATE_DATABASE_URL ?= postgres://app:app@database:5432/app?sslmode=disable
ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)

.PHONY: build run fmt lint test tidy migrate-up migrate-down migrate-status

build:
	cd $(ROOT_DIR) && $(GO) build ./...

run:
	cd $(ROOT_DIR) && $(GO) run ./cmd/api

fmt:
	@echo "Formatting Go sources"
	@files=$$(cd $(ROOT_DIR) && find cmd config internal pkg test -name '*.go' 2>/dev/null); \
	if [ -n "$$files" ]; then \
		cd $(ROOT_DIR) && gofmt -w $$files; \
	else \
		echo "No Go files to format yet"; \
	fi

lint:
	@echo "Running lint checks"
	@if command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		cd $(ROOT_DIR) && $(GOLANGCI_LINT) run ./...; \
	else \
		echo "golangci-lint not available, falling back to go vet"; \
		cd $(ROOT_DIR) && $(GO) vet ./...; \
	fi

test:
	cd $(ROOT_DIR) && $(GO) test ./...

tidy:
	cd $(ROOT_DIR) && $(GO) mod tidy

migrate-up:
	cd $(ROOT_DIR) && $(MIGRATE) "migrate -path=/migrations -database '$(MIGRATE_DATABASE_URL)' up"

# Rolls back one migration; a bare `down` would revert everything.
migrate-down:
	cd $(ROOT_DIR) && $(MIGRATE) "migrate -path=/migrations -database '$(MIGRATE_DATABASE_URL)' down 1"

migrate-status:
	cd $(ROOT_DIR) && $(MIGRATE) "migrate -path=/migrations -database '$(MIGRATE_DATABASE_URL)' version"
