# MrSmith — Development & Build Commands

# Load backend/.env if it exists
ifneq (,$(wildcard backend/.env))
  include backend/.env
  export
endif

# Dev ──────────────────────────────────────────
install:              ## Installa dipendenze workspace con pnpm
	pnpm install

bootstrap: install    ## Alias di install

dev:                  ## Avvia backend (air) + frontend (vite) dopo `make install`
	pnpm dev

dev-docker:           ## Avvia tutto via Docker Compose
	docker compose -f docker-compose.dev.yaml up --build

dev-backend:          ## Solo backend con air
	cd backend && air

dev-portal:           ## Solo portal
	pnpm --filter mrsmith-portal dev

dev-budget:           ## Solo budget app
	pnpm --filter mrsmith-budget dev

dev-compliance:       ## Solo compliance app
	pnpm --filter mrsmith-compliance dev

dev-kit-products:     ## Solo kit-products app
	pnpm --filter mrsmith-kit-products dev

dev-listini:          ## Solo listini-e-sconti app
	pnpm --filter mrsmith-listini-e-sconti dev

dev-panoramica:       ## Solo panoramica-cliente app
	pnpm --filter mrsmith-panoramica-cliente dev

dev-quotes:           ## Solo quotes app
	pnpm --filter mrsmith-quotes dev

dev-richieste-fattibilita: ## Solo richieste-fattibilita app
	pnpm --filter mrsmith-richieste-fattibilita dev

dev-rdf-backend:      ## Solo RDF Backend app
	pnpm --filter mrsmith-rdf-backend dev

dev-reports:          ## Solo reports app
	pnpm --filter mrsmith-reports dev

dev-coperture:        ## Solo coperture app
	pnpm --filter mrsmith-coperture dev

dev-energia-dc:       ## Solo energia-dc app
	pnpm --filter mrsmith-energia-dc dev

# Build ────────────────────────────────────────
build:                ## Build completo (frontend + backend)
	pnpm -r --if-present build
	cd backend && go build -o bin/server ./cmd/server

build-frontend:       ## Solo frontend
	pnpm -r --if-present build

build-backend:        ## Solo backend
	cd backend && go build -o bin/server ./cmd/server

# Docker ───────────────────────────────────────
docker-build:         ## Build immagine produzione
	docker build -f deploy/Dockerfile -t mrsmith .

docker-build-amd64:   ## Build immagine produzione linux/amd64
	./scripts/deploy/prod.sh build

package-prod-amd64:   ## Build + export tar produzione linux/amd64 in artifacts/releases
	./scripts/deploy/prod.sh package --build

deploy-prod:          ## Build + export + upload + load + restart del servizio produzione
	./scripts/deploy/prod.sh deploy --build

rollback-prod:        ## Ricarica una release remota esistente con RELEASE_TS=YYYYmmddHHMMSS
	RELEASE_TS=$(RELEASE_TS) ./scripts/deploy/prod.sh rollback

# Test ─────────────────────────────────────────
test:                 ## Tutti i test
	cd backend && go test ./...
	pnpm -r --if-present test

test-backend:         ## Solo test Go
	cd backend && go test ./...

test-frontend:        ## Solo test frontend
	pnpm -r --if-present test

# Lint ─────────────────────────────────────────
lint:                 ## Lint tutto
	cd backend && golangci-lint run
	pnpm -r --if-present lint

lint-backend:         ## Solo lint Go
	cd backend && golangci-lint run

lint-frontend:        ## Solo lint frontend
	pnpm -r --if-present lint

# Utility ──────────────────────────────────────
clean:                ## Pulisci artefatti
	rm -rf backend/bin backend/tmp
	pnpm -r exec rm -rf dist

tidy:                 ## go mod tidy
	cd backend && go mod tidy

help:                 ## Mostra questo help
	@grep -h -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: install bootstrap dev dev-docker dev-backend dev-portal dev-budget dev-compliance dev-kit-products dev-listini dev-panoramica dev-quotes dev-richieste-fattibilita dev-rdf-backend dev-reports dev-coperture dev-energia-dc build build-frontend build-backend docker-build docker-build-amd64 package-prod-amd64 deploy-prod rollback-prod test test-backend test-frontend lint lint-backend lint-frontend clean tidy help
.DEFAULT_GOAL := help
