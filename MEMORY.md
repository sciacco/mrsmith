## 2026-04-05
- Updated [AGENTS.md](AGENTS.md) to mark `docs/mistra-dist.yaml` as an important reference.
- Treat `docs/mistra-dist.yaml` as the authoritative Mistra NG Internal API spec for most mini-app integrations, including backend contracts, client generation, and shared types.
- Portal workspaces now mirror the current Appsmith inventory in static frontend data: `Acquisti`, `MKT&Sales`, `SMART APPS`, and `Provisioning`; the still-unnamed in-progress workspace is intentionally excluded.
- Portal cards support `status: 'test'` for visible non-production badges, and card descriptions are optional because the Appsmith-style inventory is title-first.
- JS tooling is standardized on root `pnpm` workspaces: root scripts target `mrsmith-portal`, recursive commands use `--if-present`, and the repo should not rely on `npm install` inside `apps/portal`.
- `make install` / `make bootstrap` is the supported dependency setup step before `make dev`; the tracked `apps/portal/package-lock.json` was removed to avoid mixed package-manager workflows.
