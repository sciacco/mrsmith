# Production Deploy

## Summary

La release di produzione usa un'immagine Docker `linux/amd64` esportata in tar, copiata sul server e ricaricata localmente con `docker image load`.

Il flusso versionato nel repo:

1. build locale dell'immagine `mrsmith:prod`
2. export in `artifacts/releases/mrsmith_amd64_<TIMESTAMP>.tar`
3. copia sul server in `/root/mrsmith_amd64_<TIMESTAMP>.tar`
4. `docker image load -i ...`
5. `docker compose -f /DATI/mrsmith/docker-compose.yaml up -d --force-recreate --no-deps mrsmith`

## Configuration

Creare `.env.deploy.prod` a partire da `.env.deploy.prod.example`.

Variabili richieste:

- `DEPLOY_HOST`
- `DEPLOY_USER`

Variabili con default repo:

- `DEPLOY_PORT=22`
- `DEPLOY_REMOTE_DIR=/root`
- `DEPLOY_COMPOSE_FILE=/DATI/mrsmith/docker-compose.yaml`
- `DEPLOY_SERVICE=mrsmith`
- `DEPLOY_RETENTION=5`
- `IMAGE_NAME=mrsmith:prod`
- `IMAGE_ARCHIVE_TAG=mrsmith:amd64`
- `ARTIFACT_DIR=artifacts/releases`

## Commands

Build amd64 senza export:

```sh
make docker-build-amd64
```

Build + export tar locale:

```sh
make package-prod-amd64
```

Deploy completo:

```sh
make deploy-prod
```

Rollback di una release già presente sul server:

```sh
make rollback-prod RELEASE_TS=20260407183000
```

Dry run dei comandi:

```sh
DRY_RUN=1 make deploy-prod
```

## Notes

- Il deploy ricrea il solo servizio `mrsmith`; il downtime breve durante il recreate è previsto.
- Ogni deploy mantiene solo gli ultimi `DEPLOY_RETENTION` tar in `DEPLOY_REMOTE_DIR`.
- Il rollback ricarica un tar già presente sul server; non ricopia artefatti dal client.
