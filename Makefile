SHELL := /bin/bash

GO ?= go
NPM ?= npm
BINARY := bin/rfs-api
IMAGE ?= rwanda-free-space:local

.DEFAULT_GOAL := help

.PHONY: help dev generate generate-check format format-check test test-go test-web vet check build web-build api-build cli migrate-status bootstrap-superadmin backup verify-backup smoke docker-build docker-run clean

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*## "; printf "Rwanda Free Space commands\n\n"} /^[a-zA-Z_-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: generate ## Run Astro and Go together with coordinated shutdown
	@./scripts/dev.sh

generate: ## Generate the TypeScript API client from the Go contract
	@$(GO) generate ./backend/...

generate-check: ## Verify generated API code is current
	@$(GO) run ./backend/cmd/gen-client -out src/lib/api/generated.ts -check

format: ## Format Go source
	@gofmt -w backend

format-check: ## Verify Go formatting
	@files="$$(gofmt -l backend)"; if [[ -n "$$files" ]]; then echo "Go files need formatting:"; echo "$$files"; exit 1; fi

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

backup: ## Create a consistent backup: make backup FILE=backup.tar.gz
	@$(GO) run ./backend/cmd/blogctl backup "$(FILE)"

verify-backup: ## Verify backup checksums: make verify-backup FILE=backup.tar.gz
	@$(GO) run ./backend/cmd/blogctl verify-backup "$(FILE)"

smoke: build ## Run the compiled site and API smoke test
	@./scripts/smoke.sh

docker-build: ## Build the optimized production image
	@docker build --pull -t $(IMAGE) .

docker-run: ## Run the production container on port 8080
	@docker run --rm -p 8080:8080 -v rfs-data:/data --env-file .env $(IMAGE)

clean: ## Remove local build output
	@rm -rf bin dist .astro
