# Production Deploy

## Summary

La release di produzione usa un build remoto sul server target. La workstation non deve avere Docker: serve solo `git` + `ssh`. Lo script crea un archivio Git del codice committato, lo invia via SSH al server di produzione e lì esegue `docker buildx build --load`.

Il flusso versionato nel repo:

1. risolve `DEPLOY_SOURCE_REF` a un commit Git e, nei deploy reali, richiede worktree pulito
2. crea uno stream `git archive` con il contesto Docker necessario
3. invia lo stream via SSH a `docker buildx build --platform linux/amd64 --load -` sul server
4. tagga l'immagine come `mrsmith:prod`, `mrsmith:amd64` e `mrsmith:prod-<TIMESTAMP>`
5. `docker compose -f /DATI/mrsmith/docker-compose.yaml up -d --force-recreate --no-deps mrsmith`
6. mantiene solo le ultime `DEPLOY_RETENTION` release image tag

## Configuration

Creare `.env.deploy.prod` a partire da `.env.deploy.prod.example`.

Variabili richieste:

- `DEPLOY_HOST`
- `DEPLOY_USER`

Variabili con default repo:

- `DEPLOY_PORT=22`
- `DEPLOY_COMPOSE_FILE=/DATI/mrsmith/docker-compose.yaml`
- `DEPLOY_SERVICE=mrsmith`
- `DEPLOY_RETENTION=5`
- `DEPLOY_SOURCE_REF=HEAD`
- `DEPLOY_REQUIRE_CLEAN=1`
- `IMAGE_NAME=mrsmith:prod`
- `IMAGE_ARCHIVE_TAG=mrsmith:amd64`
- `IMAGE_RELEASE_TAG_PREFIX=mrsmith:prod-`
- `ARTIFACT_DIR=artifacts/releases` solo per il package tar locale manuale

Requisiti workstation:

- `git`
- `ssh`

Requisiti server produzione:

- Docker Engine
- `docker buildx`
- `docker compose` v2
- accesso outbound per immagini base Docker, registry npm/pnpm e moduli Go

## Commands

Build amd64 locale senza export, solo per uso manuale e richiede Docker locale:

```sh
make docker-build-amd64
```

Build + export tar locale, solo per uso manuale e richiede Docker locale:

```sh
make package-prod-amd64
```

Deploy completo:

```sh
make deploy-prod
```

Rollback di una release image tag già presente sul server:

```sh
make rollback-prod RELEASE_TS=20260407183000
```

Dry run dei comandi:

```sh
DRY_RUN=1 make deploy-prod
```

## Notes

- Il deploy ricrea il solo servizio `mrsmith`; il downtime breve durante il recreate è previsto.
- I deploy reali usano solo codice committato. Con `DEPLOY_REQUIRE_CLEAN=1`, modifiche staged, unstaged o untracked non ignorate bloccano il deploy.
- `DRY_RUN=1` stampa i comandi remoti e non valida worktree pulito o strumenti remoti.
- Il rollback non usa più tar: retagga `mrsmith:prod-<TIMESTAMP>` come `mrsmith:prod` e ricrea il servizio.
- Ogni deploy mantiene solo le ultime `DEPLOY_RETENTION` release image tag con prefisso `IMAGE_RELEASE_TAG_PREFIX`.
- Se `deploy/Dockerfile` aggiunge nuovi `COPY` dal root repo, aggiornare anche l'allowlist del `git archive` in `scripts/deploy/prod.sh`.
