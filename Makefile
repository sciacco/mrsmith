# MrSmith — Development & Build Commands

# Dev ──────────────────────────────────────────
dev:                  ## Avvia backend (air) + frontend (vite)
	pnpm dev

dev-docker:           ## Avvia tutto via Docker Compose
	docker compose -f docker-compose.dev.yaml up --build

dev-backend:          ## Solo backend con air
	cd backend && air

dev-portal:           ## Solo portal
	pnpm --filter portal dev

# Build ────────────────────────────────────────
build:                ## Build completo (frontend + backend)
	pnpm -r build
	cd backend && go build -o bin/server ./cmd/server

build-frontend:       ## Solo frontend
	pnpm -r build

build-backend:        ## Solo backend
	cd backend && go build -o bin/server ./cmd/server

# Docker ───────────────────────────────────────
docker-build:         ## Build immagine produzione
	docker build -f deploy/Dockerfile -t mrsmith .

# Test ─────────────────────────────────────────
test:                 ## Tutti i test
	cd backend && go test ./...
	pnpm -r test

test-backend:         ## Solo test Go
	cd backend && go test ./...

test-frontend:        ## Solo test frontend
	pnpm -r test

# Lint ─────────────────────────────────────────
lint:                 ## Lint tutto
	cd backend && golangci-lint run
	pnpm -r lint

lint-backend:         ## Solo lint Go
	cd backend && golangci-lint run

lint-frontend:        ## Solo lint frontend
	pnpm -r lint

# Utility ──────────────────────────────────────
clean:                ## Pulisci artefatti
	rm -rf backend/bin backend/tmp
	pnpm -r exec rm -rf dist

tidy:                 ## go mod tidy
	cd backend && go mod tidy

help:                 ## Mostra questo help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: dev dev-docker dev-backend dev-portal build build-frontend build-backend docker-build test test-backend test-frontend lint lint-backend lint-frontend clean tidy help
.DEFAULT_GOAL := help
