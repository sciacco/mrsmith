# MrSmith — Development & Build Commands

# Env di sviluppo da backend/.env. RUN_ENV è un runner che riceve il comando
# come singola stringa quotata: $(RUN_ENV) 'comando args'.
#  - file in chiaro  -> include + export (comportamento storico, sops non richiesto)
#  - file cifrato con sops (formato dotenv) -> `sops exec-env` decifra solo in memoria
#  - file cifrato ma sops assente -> errore chiaro, limitato ai target che usano l'env
ENV_FILE := backend/.env
RUN_ENV := sh -c
ifneq (,$(wildcard $(ENV_FILE)))
  ifeq (,$(shell grep -m1 '^sops_version=' $(ENV_FILE) 2>/dev/null))
    include $(ENV_FILE)
    export
  else ifeq (,$(shell command -v sops 2>/dev/null))
    RUN_ENV := sh -c 'echo "ERRORE: $(ENV_FILE) è cifrato con sops ma sops non è nel PATH (brew install sops)" >&2; exit 1'
  else
    RUN_ENV := sops exec-env $(abspath $(ENV_FILE))
  endif
endif

# Dev ──────────────────────────────────────────

.PHONY: install
install:              ## Installa dipendenze workspace con pnpm
	pnpm install

.PHONY: bootstrap
bootstrap: install    ## Alias di install

.PHONY: dev
dev:                  ## Avvia backend (air) + frontend (vite) dopo `make install`
	$(RUN_ENV) 'pnpm dev'

.PHONY: dev-docker
dev-docker:           ## Avvia tutto via Docker Compose
	@if grep -q '^sops_version=' $(ENV_FILE) 2>/dev/null; then \
		echo "ERRORE: $(ENV_FILE) è cifrato con sops: docker compose lo legge via env_file e riceverebbe ciphertext." >&2; \
		echo "Decifra prima il file (sops -d) oppure usa 'make dev'." >&2; \
		exit 1; \
	fi
	docker compose -f docker-compose.dev.yaml up --build

.PHONY: dev-backend
dev-backend:          ## Solo backend con air
	cd backend && $(RUN_ENV) 'air'

.PHONY: dev-portal
dev-portal:           ## Solo portal
	pnpm --filter mrsmith-portal dev

.PHONY: dev-aenad
dev-aenad:            ## Solo aenad app
	pnpm --filter mrsmith-aenad dev

.PHONY: dev-budget
dev-budget:           ## Solo budget app
	pnpm --filter mrsmith-budget dev

.PHONY: dev-fornitori
dev-fornitori:        ## Solo fornitori app
	pnpm --filter mrsmith-fornitori dev

.PHONY: dev-rda
dev-rda:              ## Solo RDA app
	pnpm --filter mrsmith-rda dev

.PHONY: dev-stats-rda
dev-stats-rda:        ## Solo Statistiche RDA app
	pnpm --filter mrsmith-stats-rda dev

.PHONY: dev-compliance
dev-compliance:       ## Solo compliance app
	pnpm --filter mrsmith-compliance dev

.PHONY: dev-kit-products
dev-kit-products:     ## Solo kit-products app
	pnpm --filter mrsmith-kit-products dev

.PHONY: dev-listini
dev-listini:          ## Solo listini-e-sconti app
	pnpm --filter mrsmith-listini-e-sconti dev

.PHONY: dev-manutenzioni
dev-manutenzioni:     ## Solo manutenzioni app
	pnpm --filter mrsmith-manutenzioni dev

.PHONY: dev-training
dev-training:         ## Solo training app
	pnpm --filter mrsmith-training dev

.PHONY: dev-panoramica
dev-panoramica:       ## Solo panoramica-cliente app
	pnpm --filter mrsmith-panoramica-cliente dev

.PHONY: dev-quotes
dev-quotes:           ## Solo quotes app
	pnpm --filter mrsmith-quotes dev

.PHONY: dev-ordini
dev-ordini:           ## Solo ordini app
	pnpm --filter mrsmith-ordini dev

.PHONY: dev-richieste-fattibilita
dev-richieste-fattibilita: ## Solo richieste-fattibilita app
	pnpm --filter mrsmith-richieste-fattibilita dev

.PHONY: dev-rdf-backend
dev-rdf-backend:      ## Solo RDF Backend app
	pnpm --filter mrsmith-rdf-backend dev

.PHONY: dev-reports
dev-reports:          ## Solo reports app
	pnpm --filter mrsmith-reports dev

.PHONY: dev-coperture
dev-coperture:        ## Solo coperture app
	pnpm --filter mrsmith-coperture dev

.PHONY: dev-energia-dc
dev-energia-dc:       ## Solo energia-dc app
	pnpm --filter mrsmith-energia-dc dev

.PHONY: dev-grappa-dcim
dev-grappa-dcim:      ## Solo grappa-dcim app
	pnpm --filter mrsmith-grappa-dcim dev

.PHONY: dev-simulatori-vendita
dev-simulatori-vendita: ## Solo simulatori-vendita app
	pnpm --filter mrsmith-simulatori-vendita dev

.PHONY: dev-afc-tools
dev-afc-tools:        ## Solo afc-tools app
	pnpm --filter @mrsmith/afc-tools dev

.PHONY: dev-cp-backoffice
dev-cp-backoffice:    ## Solo cp-backoffice app
	pnpm --filter mrsmith-cp-backoffice dev

# Build ────────────────────────────────────────

.PHONY: build
build:                ## Build completo (frontend + backend)
	pnpm -r --if-present build
	cd backend && go build -o bin/server ./cmd/server

.PHONY: build-frontend
build-frontend:       ## Solo frontend
	pnpm -r --if-present build

.PHONY: build-backend
build-backend:        ## Solo backend
	cd backend && go build -o bin/server ./cmd/server

# Docker ───────────────────────────────────────

.PHONY: docker-build
docker-build:         ## Build immagine produzione
	docker build -f deploy/Dockerfile -t mrsmith .

.PHONY: docker-build-amd64
docker-build-amd64:   ## Build immagine produzione linux/amd64
	./scripts/deploy/prod.sh build

.PHONY: package-prod-amd64
package-prod-amd64:   ## Build + export tar produzione linux/amd64 in artifacts/releases
	./scripts/deploy/prod.sh package --build

.PHONY: deploy-prod
deploy-prod:          ## Stream sorgente + build remoto + restart del servizio produzione
	./scripts/deploy/prod.sh deploy

.PHONY: rollback-prod
rollback-prod:        ## Retag + restart di una release remota con RELEASE_TS=YYYYmmddHHMMSS
	RELEASE_TS=$(RELEASE_TS) ./scripts/deploy/prod.sh rollback

# Test ─────────────────────────────────────────

.PHONY: test
test:                 ## Tutti i test
	cd backend && $(RUN_ENV) 'go test ./...'
	pnpm -r --if-present test

.PHONY: test-backend
test-backend:         ## Solo test Go
	cd backend && $(RUN_ENV) 'go test ./...'

.PHONY: test-frontend
test-frontend:        ## Solo test frontend
	pnpm -r --if-present test

# Lint ─────────────────────────────────────────

.PHONY: lint
lint:                 ## Lint tutto
	cd backend && golangci-lint run
	pnpm -r --if-present lint

.PHONY: lint-backend
lint-backend:         ## Solo lint Go
	cd backend && golangci-lint run

.PHONY: lint-frontend
lint-frontend:        ## Solo lint frontend
	pnpm -r --if-present lint

# Utility ──────────────────────────────────────

.PHONY: clean
clean:                ## Pulisci artefatti
	rm -rf backend/bin backend/tmp
	pnpm -r exec rm -rf dist

.PHONY: tidy
tidy:                 ## go mod tidy
	cd backend && go mod tidy

.PHONY: update-mistra-api
update-mistra-api:    ## Scarica e aggiorna la specifica Mistra API da remoto
	./scripts/update-mistra-dist.sh

BINOCOLO_LLM_EVAL_SAMPLE_SIZE ?= 10
BINOCOLO_LLM_EVAL_ITERATIONS ?= 10
BINOCOLO_LLM_EVAL_CONCURRENCY ?= 10

.PHONY: binocolo-llm-eval
binocolo-llm-eval:    ## Test varianza LLM Binocolo ma_deep_brief (ARGS="--dry-run" per solo manifest)
	cd backend && $(RUN_ENV) 'go run ./cmd/binocolo-llm-eval --sample-size $(BINOCOLO_LLM_EVAL_SAMPLE_SIZE) --iterations $(BINOCOLO_LLM_EVAL_ITERATIONS) --concurrency $(BINOCOLO_LLM_EVAL_CONCURRENCY) $(ARGS)'

.PHONY: help
help:                 ## Mostra questo help
	@grep -h -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
