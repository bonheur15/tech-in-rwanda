SHELL := /bin/bash

GO ?= go
NPM ?= npm
BINARY := bin/rfs-api
IMAGE ?= rwanda-free-space:local

.DEFAULT_GOAL := help

.PHONY: help dev generate generate-check format format-check test test-go test-web vet check build web-build api-build cli migrate-status bootstrap-superadmin seed-demo recover-account backup verify-backup media-check media-cleanup smoke docker-smoke docker-build docker-run clean

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*## "; printf "Rwanda Free Space commands\n\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: generate ## Run Astro and Go together with coordinated shutdown
	@./scripts/dev.sh

generate: ## Generate the TypeScript API client from the Go contract
	@$(GO) run ./backend/cmd/gen-client -out src/lib/api/generated.ts

generate-check: ## Verify generated API code is current
	@$(GO) run ./backend/cmd/gen-client -out src/lib/api/generated.ts -check

format: ## Format Go, TypeScript, Astro and CSS
	@gofmt -w backend
	@$(NPM) exec biome -- format --write .

format-check: ## Verify Go, TS, Astro and CSS formatting
	@files="$$(gofmt -l backend)"; if [[ -n "$$files" ]]; then echo "Go files need formatting:"; echo "$$files"; exit 1; fi
	@$(NPM) exec biome -- format .

test: test-go test-web ## Run backend and frontend tests

test-go: ## Run Go tests with the race detector
	@$(GO) test -race ./backend/...

test-web: ## Run generated-client tests
	@$(NPM) test

vet: ## Run Go static analysis
	@$(GO) vet ./backend/...

check: generate-check format-check vet test ## Run the complete quality gate
	@$(NPM) run check
	@$(NPM) run build
	@$(MAKE) --no-print-directory api-build
	@$(MAKE) --no-print-directory smoke
	@$(MAKE) --no-print-directory docker-smoke

build: web-build api-build ## Build the static site and Go server

web-build: generate-check ## Build Astro
	@$(NPM) run build

api-build: ## Build the production Go binary
	@mkdir -p bin
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BINARY) ./backend/cmd/api

cli: ## Build the operational CLI
	@mkdir -p bin
	@CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o bin/blogctl ./backend/cmd/blogctl

migrate-status: ## Show applied database migrations
	@$(GO) run ./backend/cmd/blogctl migrate-status

bootstrap-superadmin: ## Bootstrap idempotently: make bootstrap-superadmin EMAIL=x HANDLE=x NAME=x
	@$(GO) run ./backend/cmd/blogctl bootstrap-superadmin "$(EMAIL)" "$(HANDLE)" "$(NAME)"

seed-demo: ## Seed a realistic, idempotent local newsroom dataset
	@$(GO) run ./backend/cmd/blogctl seed-demo

recover-account: ## Revoke every session for an account: make recover-account EMAIL=x
	@$(GO) run ./backend/cmd/blogctl recover-account "$(EMAIL)"

backup: ## Create a consistent backup: make backup FILE=backup.tar.gz
	@$(GO) run ./backend/cmd/blogctl backup "$(FILE)"

verify-backup: ## Verify backup checksums: make verify-backup FILE=backup.tar.gz
	@$(GO) run ./backend/cmd/blogctl verify-backup "$(FILE)"

media-check: ## Verify every database media asset exists on disk
	@$(GO) run ./backend/cmd/blogctl media-check

media-cleanup: ## Purge unreferenced media orphaned for at least seven days
	@$(GO) run ./backend/cmd/blogctl media-cleanup

smoke: build ## Run the compiled site and API smoke test
	@./scripts/smoke.sh

docker-smoke: ## Build and verify the supervised production container
	@IMAGE="$(IMAGE)" ./scripts/docker-smoke.sh

docker-build: ## Build the optimized production image
	@docker build --pull -t $(IMAGE) .

docker-run: ## Run the production container on port 8080
	@docker run --rm -p 8080:8080 -v rfs-data:/data --env-file .env $(IMAGE)

clean: ## Remove local build output
	@rm -rf bin dist .astro
