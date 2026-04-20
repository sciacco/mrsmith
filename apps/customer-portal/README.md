# Customer Portal Migration Workspace

This folder is the migration workspace for `apps/cp-backoffice/`.

The SPA lives at `apps/cp-backoffice/`. Do not add product code here; all SPA code, routes, and views belong under `apps/cp-backoffice/`.

This folder holds the original Appsmith export (`customer-portal.json.gz`), the phased audit (`audit/`), the staged migspecs (`migspec/`), the approved spec ([SPEC.md](./SPEC.md)), the implementation plan ([IMPL.md](./IMPL.md)), the executor prompt ([PROMPT.md](./PROMPT.md)), and the final execution contract ([FINAL.md](./FINAL.md)).

The split follows the same pattern already used by `apps/zammu/`, where the migration-analysis workspace lives separately from the final app folders.
