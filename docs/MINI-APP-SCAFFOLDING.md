# Mini-App Scaffolding Guide

Guida operativa per aggiungere una mini-app al portale MrSmith. Questo documento assembla i passaggi e i touchpoint di repository; le fonti normative collegate restano autoritative per design, API e pianificazione.

## 1. Fonti obbligatorie

Prima di implementare:

- usare `.agents/skills/portal-miniapp-generator/` per pianificazione, archetipo e review gate;
- leggere [IMPLEMENTATION-PLANNING.md](IMPLEMENTATION-PLANNING.md) e le sezioni pertinenti di [IMPLEMENTATION-KNOWLEDGE.md](IMPLEMENTATION-KNOWLEDGE.md);
- applicare [UI-UX.md](UI-UX.md) a tutto il frontend;
- applicare [API-CONVENTIONS.md](API-CONVENTIONS.md) ai contratti browser/BFF;
- per migrazioni legacy, seguire anche il playbook e la skill specifici della sorgente.

Questa guida non sostituisce tali regole e non introduce un secondo design system.

## 2. Decisioni da fissare prima dello scaffold

Registrare le decisioni in `apps/<app-slug>/docs/IMPLEMENTATION-PLAN.md`, usando il template del `portal-miniapp-generator`.

| Decisione | Default |
| --- | --- |
| Cartella frontend | `apps/<app-slug>/` |
| Nome package | `mrsmith-<app-slug>` (salvo convenzioni già motivate) |
| Mount SPA produzione | `/apps/<app-slug>/` |
| Namespace API pubblico | `/api/<api-prefix>/v1/...` |
| Namespace nel modulo Go | `/<api-prefix>/v1/...`, senza `/api` |
| Ruolo Keycloak | `app_<appname>_access` |
| Tema | `clean`, light-only |
| Lingua/formati | italiano, `it-IT` |
| Porta Vite | porta libera e univoca |

Stabilire inoltre:

- archetipo UI e almeno due schermate comparabili reali;
- dipendenze (Mistra, Grappa, Anisetta, servizi esterni) e strategia quando non configurate;
- forma esatta dei contratti dati, auth e download;
- necessità di migrazioni, job, configurazione o osservabilità;
- deep link finali e URL locale split-server.

Non iniziare il codice finché il piano non supera il pre-gate previsto dal `portal-miniapp-generator`.

## 3. Scaffold frontend

### 3.1 Struttura minima

Adattare una mini-app recente dello stesso archetipo; non copiare alla cieca app legacy.

```text
apps/<app-slug>/
  docs/
    IMPLEMENTATION-PLAN.md
  src/
    api/
    components/
    pages/                 # oppure views/, coerente con l'app comparabile
    styles/
      global.css
      tokens.css           # solo se servono estensioni locali
    App.tsx
    main.tsx
  index.html
  package.json
  tsconfig.json
  tsconfig.build.json      # se necessario per escludere file test dalla build
  vite.config.ts
```

La struttura può essere ridotta o estesa in base all'archetipo; evitare cartelle vuote e astrazioni speculative.

### 3.2 Dipendenze condivise

Usare prima i package del monorepo:

- `@mrsmith/ui` per shell e componenti visuali;
- `@mrsmith/auth-client` per bootstrap e sessione Keycloak;
- `@mrsmith/api-client` per chiamate autenticate;
- `@mrsmith/vite-config` per la configurazione Vite.

Aggiungere `@mrsmith/vite-config: "workspace:*"` alle `devDependencies`. Le altre dipendenze vanno dichiarate secondo l'uso effettivo e i peer dependency richiesti.

### 3.3 Vite

`apps/<app-slug>/vite.config.ts` deve usare il helper condiviso:

```ts
import { defineMrSmithAppConfig } from '@mrsmith/vite-config';

export default defineMrSmithAppConfig({
  appSlug: '<app-slug>',
  port: <porta-libera>,
});
```

Non usare direttamente `defineConfig`. Il helper configura React, deduplica React, imposta il base path di produzione e crea i proxy locali per `/api` e `/config`. Opzioni specifiche passano da `overrides`; dettagli in `packages/vite-config/README.md`.

Prima di scegliere la porta:

```bash
grep -R "port:" apps/*/vite.config.ts
```

### 3.4 Bootstrap applicazione

- Impostare `data-theme="clean"` alla radice.
- Caricare `GET /config` e inizializzare `AuthProvider`.
- Gestire bootstrap loading, errore e accesso negato con `AccessNotice` o componenti condivisi equivalenti.
- Creare il client API con base same-origin `/api`; non chiamare upstream direttamente dal browser.
- Usare `AppShell` e gli altri componenti di `@mrsmith/ui` prima di introdurre componenti locali.
- Usare solo `Icon` per le icone delle mini-app.

### 3.5 Stili

Applicare integralmente `docs/UI-UX.md`, in particolare:

- background canonico in `src/styles/global.css`;
- token del tema per colori, spaziature, raggi, ombre e motion;
- CSS Modules, niente Tailwind o CSS-in-JS;
- keyframe globali e blocco `prefers-reduced-motion`;
- contenuto entro 1400 px in `AppShell`;
- accessibilità WCAG 2.1 AA, focus e navigazione da tastiera;
- loading con skeleton, stati vuoto/errore persistenti e copy business in italiano.

## 4. Scaffold backend

### 4.1 Modulo applicativo

Creare `backend/internal/<app-package>/` seguendo un modulo recente e comparabile. La forma minima tipica è:

```text
backend/internal/<app-package>/
  handler.go
  types.go              # se utile
  service.go / store.go # solo se il dominio lo richiede
```

Regole:

- dipendenze iniettate esplicitamente, niente nuovo stato globale di package;
- browser isolato dagli upstream tramite BFF Go;
- errori `5xx` sanitizzati al client e dettagli diagnostici nei log;
- invarianti di ownership e autorizzazione applicate lato backend;
- namespace versionato e specifico dell'app;
- download autenticati esposti dal BFF e consumati come blob.

### 4.2 Route registration

Il modulo registra route senza prefisso `/api`, ad esempio `/<api-prefix>/v1/...`. In `backend/cmd/server/main.go`:

- importare il modulo;
- costruire/iniettare le dipendenze;
- chiamare `RegisterRoutes` sul sub-mux API;
- aggiungere l'href override locale verso la porta Vite;
- applicare il ruolo dell'app e gli eventuali ruoli elevati.

Il server applica `/api` una sola volta con `http.StripPrefix`; non duplicarlo nel modulo.

### 4.3 Configurazione e dipendenze

Sempre richiesto per ogni nuova app:

- aggiungere il campo `{App}AppURL` e la relativa env var `{APP}_APP_URL` in `backend/internal/platform/config/config.go` (tutte le app registrate nel portale lo hanno; alimenta l'href override in `main.go`).

Se si aggiungono altri URL o DSN:

- aggiungere il campo in `backend/internal/platform/config/config.go`;
- documentare la variabile in `backend/.env.example` e `.env.preprod.example`;
- riportare la variabile nei manifest `deploy/k8s/` (es. `configmap.yaml`) se serve in produzione;
- aggiornare il wiring in `backend/cmd/server/main.go`;
- decidere se una dipendenza assente nasconde la tile oppure lascia l'app visibile con risposta `503` esplicita.

Non collegarsi direttamente ai database configurati negli env del repository. Consegnare le modifiche schema solo come migration SQL in `deploy/migrations/`, con strategia di applicazione e seed stabili documentati.

## 5. Registrazione nel portale

Aggiornare `backend/internal/platform/applaunch/catalog.go` con:

- costanti ID e href;
- titolo, descrizione business e sezione;
- ruolo `app_<appname>_access`;
- chiave icona già disponibile nel portale;
- stato e condizioni di visibilità coerenti con le dipendenze.

Il ruolo `app_<appname>_access` deve essere creato e assegnato agli utenti sul Keycloak remoto (non esiste un'istanza locale): senza questo passo la tile non compare e lo smoke test finale fallisce. Concordare la creazione con chi amministra il realm.

Aggiornare anche:

- `backend/cmd/server/main.go` per href override locale, filtri e route;
- `apps/portal/src/components/Icon/icons.tsx` solo se non esiste un'icona portal adatta, rispettando il linguaggio Matrix portal-only;
- i test catalogo/portal/static SPA esistenti quando i contratti verificati cambiano, previa applicazione della Test Rule del repository.

Non riusare le icone Matrix dentro la mini-app e non usare componenti clean nel launcher senza verificarne i token.

## 6. Workspace e sviluppo locale

### Obbligatorio

- `package.json` root: aggiungere `dev:<app-slug>` con il filtro package corretto.
- `Makefile`: aggiungere `dev-<app-slug>` e inserirlo in `.PHONY`.
- `pnpm-lock.yaml`: rigenerare dopo le modifiche alle dipendenze.

Il comando root `dev` usa già `pnpm --filter './apps/*' --parallel --if-present dev`, quindi una nuova app con script locale `dev` viene inclusa automaticamente; non serve aggiungerla manualmente alla stringa aggregata.

### Se supportata da `make dev-docker`

Aggiornare `docker-compose.dev.yaml` con servizio, volume, porta e `VITE_DEV_BACKEND_URL=http://backend:8080`. Se l'app non viene aggiunta al compose, documentare esplicitamente che lo sviluppo supportato è split-server locale.

## 7. Build, hosting e deploy

Aggiornare `deploy/Dockerfile` per copiare:

```dockerfile
COPY --from=frontend /app/apps/<app-slug>/dist /static/apps/<app-slug>
```

Verificare che:

- la base Vite sia `/apps/<app-slug>/`;
- il backend serva la SPA e il fallback di deep link sotto lo stesso mount;
- refresh e accesso diretto a route annidate funzionino;
- URL generati da backend/email includano sia mount sia route applicativa, componendoli tramite `applaunch.NewURLResolver` (già istanziato in `main.go` come single source of truth per i deep link cross-app), senza ricostruire gli href a mano;
- eventuali env var di produzione siano riportate nei manifest `deploy/k8s/`;
- eventuali test TypeScript non entrino nella build di produzione quando richiedono un `tsconfig.build.json` dedicato.

## 8. Touchpoint file-per-file

### Sempre

- [ ] `apps/<app-slug>/docs/IMPLEMENTATION-PLAN.md`
- [ ] `apps/<app-slug>/package.json`
- [ ] `apps/<app-slug>/vite.config.ts`
- [ ] `apps/<app-slug>/src/...`
- [ ] `package.json` (`dev:<app-slug>`)
- [ ] `pnpm-lock.yaml`
- [ ] `Makefile` (`dev-<app-slug>` e `.PHONY`)
- [ ] `backend/internal/<app-package>/...`
- [ ] `backend/internal/platform/applaunch/catalog.go`
- [ ] `backend/internal/platform/config/config.go` (`{App}AppURL` + env var)
- [ ] `backend/cmd/server/main.go`
- [ ] `deploy/Dockerfile`
- [ ] ruolo `app_<appname>_access` creato sul Keycloak remoto (fuori repo)

### Quando applicabile

- [ ] `backend/internal/platform/config/config.go` (altri URL o DSN)
- [ ] `backend/.env.example`
- [ ] `.env.preprod.example`
- [ ] `deploy/k8s/` (env var di produzione, es. `configmap.yaml`)
- [ ] `docker-compose.dev.yaml`
- [ ] `deploy/migrations/<NNN>_<descrizione>.sql`
- [ ] `apps/portal/src/components/Icon/icons.tsx`
- [ ] test catalogo, portal handler, static SPA e contratti dati approvati
- [ ] `docs/IMPLEMENTATION-KNOWLEDGE.md` per nuove conoscenze riutilizzabili
- [ ] `docs/UI-UX.md` se si estendono token, componenti condivisi o regole

## 9. Verifica finale

Eseguire verifiche proporzionate al cambiamento e concordare l'aggiunta di nuovi test secondo la Test Rule.

### Frontend

```bash
pnpm --filter <package-name> exec tsc --noEmit
pnpm --filter <package-name> build
```

Non usare `npx tsc`.

### Backend

```bash
cd backend && go test ./internal/<app-package>/...
cd backend && go test ./internal/platform/applaunch/... ./internal/platform/staticspa/...
cd backend && go build ./cmd/server
```

Eseguire solo suite già esistenti o test approvati; non aggiungere test automaticamente.

### Smoke test integrato

- [ ] la tile è visibile solo al ruolo previsto;
- [ ] il click dal portale apre l'URL corretto;
- [ ] `/config`, login e refresh token funzionano;
- [ ] API senza token, ruolo errato e dipendenza assente hanno esito previsto;
- [ ] refresh di una route annidata non restituisce 404;
- [ ] loading, populated, empty, error e conferma distruttiva sono verificati;
- [ ] layout desktop e narrow viewport sono usabili da tastiera;
- [ ] gli export autenticati funzionano tramite blob;
- [ ] il post-gate di `portal-miniapp-ui-review` è approvato.

Prima di avviare browser o Playwright, riusare un server `make dev`/Vite già attivo; non avviarne un secondo se esiste un URL adatto.

## 10. Definition of done

Lo scaffold è completo quando la mini-app:

1. ha un piano approvato, un archetipo esplicito e riferimenti a schermate comparabili;
2. usa auth, API client, tema e componenti condivisi secondo le convenzioni;
3. è registrata nel catalogo con ruolo, href e comportamento dipendenze espliciti;
4. funziona in sviluppo, build e hosting statico, inclusi i deep link;
5. dichiara configurazione, migrazioni e contratti senza placeholder;
6. supera type-check, build, smoke test e review UI post-implementazione;
7. aggiorna la documentazione canonica quando introduce una regola riutilizzabile.
