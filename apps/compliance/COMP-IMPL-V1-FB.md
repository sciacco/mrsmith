# Compliance App — Implementation Plan V1.1 Feedback

Reviewed against:
- `apps/compliance/COMP-IMPL-V1.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- Current repo wiring in `backend/`, `apps/`, `packages/`, `deploy/`, and `docker-compose.dev.yaml`

## Verdict

The plan is materially stronger than the earlier version and it fixes the largest path/auth assumptions, but it is not implementation-ready yet. There are still a few repo-fit and contract gaps that will either break the build/runtime outright or leave the repo in an inconsistent state.

## Blocking Findings

### 1. `DELETE /origins/:id` is incompatible with the current shared API client

**Plan references**
- `apps/compliance/COMP-IMPL-V1.md:174-178`
- `apps/compliance/COMP-IMPL-V1.md:337-342`

**Repo evidence**
- `packages/api-client/src/client.ts:18-23`
- `packages/api-client/src/client.ts:25-55`
- `apps/compliance/compliance-migspec.md:283-286`

The plan keeps the origin delete contract at `204 No Content`, but the shared `ApiClient` always calls `res.json()` on successful responses. That means any frontend mutation using `api.delete(...)` against `/compliance/origins/:id` will fail on a successful 204 before React Query can settle cleanly.

This is a real contract bug, not just a documentation mismatch.

**Required plan change**
- Pick one of these explicitly:
- Change the endpoint contract to return JSON, for example `200 {method_id}`.
- Or extend `@mrsmith/api-client` so successful empty responses are supported across verbs, then call that out as part of the shared-package change, not just `getBlob()`.

### 2. The DB bootstrap section omits the actual PostgreSQL driver registration needed for `database/sql`

**Plan references**
- `apps/compliance/COMP-IMPL-V1.md:46-52`
- `apps/compliance/COMP-IMPL-V1.md:194-199`

**Repo evidence**
- `backend/internal/platform/database/database.go:13-39`
- `backend/go.mod:1-10`

The plan correctly reuses `backend/internal/platform/database/database.go`, and that code maps `"postgres"` to the `"pgx"` driver. But the repo currently has no `pgx` stdlib dependency and no side-effect import that registers the driver with `database/sql`.

As written, `database.New(database.Config{Driver: "postgres", ...})` will fail at runtime with an unknown driver error.

**Required plan change**
- Add `github.com/jackc/pgx/v5/stdlib` to the backend dependency plan.
- State where the side-effect import lives, for example in `backend/cmd/server/main.go` or a dedicated database package file.
- Treat this as part of the deliverables, not an implied implementation detail.

### 3. The plan promises JSON `401`/`403` bodies, but the shared auth stack currently returns plain text

**Plan/spec references**
- `apps/compliance/compliance-migspec.md:288-295`
- `apps/compliance/COMP-IMPL-V1.md:189`
- `apps/compliance/COMP-IMPL-V1.md:480-481`

**Repo evidence**
- `backend/internal/auth/middleware.go:58-79`
- `backend/internal/acl/acl.go:13-27`

The spec says auth failures return JSON bodies like `{error: "unauthorized"}` and `{error: "forbidden"}`. The actual shared middleware uses `http.Error(...)`, which produces plain-text responses. Because compliance routes sit behind that shared middleware, the documented contract is not currently achievable as written.

This matters for frontend error handling, automated tests, and consistency with the published API contract.

**Required plan change**
- Narrow the documented contract so compliance does not promise JSON for middleware-generated `401`/`403` responses.
- Reserve JSON error-shape guarantees for compliance-handler errors such as validation/not-found/internal failures that the compliance module itself produces.

### 4. Deployment and dev wiring are still incomplete for the actual repo

**Plan references**
- `apps/compliance/COMP-IMPL-V1.md:41-58`
- `apps/compliance/COMP-IMPL-V1.md:256-281`
- `apps/compliance/COMP-IMPL-V1.md:588-597`

**Repo evidence**
- `docker-compose.dev.yaml:1-38`
- `deploy/k8s/configmap.yaml:1-12`

The plan updates root `package.json` and `deploy/Dockerfile`, but it does not account for two repo paths that are part of the documented dev/deploy flow:

- `make dev-docker` uses `docker-compose.dev.yaml`, and there is no compliance frontend service there yet.
- Production config currently has no declared wiring for `ANISETTA_DSN`. The plan names the env var, but it does not say where it comes from in K8s or any deploy manifest.

This fails the repo-fit checklist on both Dev Fit and Deployment Fit.

**Required plan change**
- Add `docker-compose.dev.yaml` updates so `make dev-docker` actually includes the compliance app.
- Add deployment/env wiring tasks for `ANISETTA_DSN`.
- Because DSNs are sensitive, this should likely be a Secret/deployment env change, not a ConfigMap change.

### 5. Changing compliance visibility from default roles to a dedicated role will break existing launcher tests, but the plan does not include those updates

**Plan references**
- `apps/compliance/COMP-IMPL-V1.md:62-66`

**Repo evidence**
- `backend/internal/platform/applaunch/catalog_test.go:25-56`
- `backend/internal/portal/handler_test.go:14-47`

Today the compliance app is part of the default-role placeholder set. The plan correctly moves it to `app_compliance_access`, but that changes the visible app counts and portal expectations immediately.

The current tests hard-code totals assuming compliance is still visible to `default-roles-cdlan`. Those tests will fail as soon as the catalog change lands.

**Required plan change**
- Explicitly include updates to `backend/internal/platform/applaunch/catalog_test.go`.
- Add or update portal handler coverage so a user with `app_compliance_access` sees the compliance launcher entry and a default-role-only user no longer does.

### 6. The plan and the source spec still disagree on the `POST /origins` request contract

**Plan references**
- `apps/compliance/COMP-IMPL-V1.md:176`

**Spec evidence**
- `apps/compliance/compliance-migspec.md:283-286`

The plan resolves origin creation to require `{method_id, description}`, which is reasonable given the existing PK strategy. But the source spec it cites still says `POST /origins` accepts only `{description}`.

That leaves the plan in a self-contradictory state: implementation would follow one contract while the stated source of truth says another.

**Required plan change**
- Update `apps/compliance/compliance-migspec.md` first, or explicitly mark the plan as superseding that section of the spec.
- Do not start backend/frontend implementation while those two documents disagree.

## Important Non-Blocking Improvements

### 7. The frontend scaffold is internally inconsistent with the repo conventions

**Plan references**
- `apps/compliance/COMP-IMPL-V1.md:209-216`
- `apps/compliance/COMP-IMPL-V1.md:466-467`

**Repo evidence**
- `apps/budget/package.json:6-10`
- `apps/budget/tsconfig.json:1-13`

The proposed `apps/compliance/package.json` only defines `dev` and `build`, but Phase 4 later runs `pnpm --filter mrsmith-compliance lint`. That command will fail unless a `lint` script is added.

Also, the existing app pattern uses `tsc -b && vite build`, while the plan says `tsc && vite build`. Since the app is meant to copy the budget scaffold, it should stay aligned unless there is a deliberate reason not to.

**Recommended plan change**
- Add `"lint": "tsc --noEmit"` and `"preview": "vite preview"` to match the current workspace convention.
- Use the same `build` script shape as the existing apps unless the plan intentionally wants something different.

### 8. The deep-link risk that motivated the path correction should be covered by an automated static SPA test, not only a manual smoke check

**Plan references**
- `apps/compliance/COMP-IMPL-V1.md:13`
- `apps/compliance/COMP-IMPL-V1.md:482`

**Repo evidence**
- `backend/internal/platform/staticspa/handler_test.go:12-80`

The plan correctly fixes the app path to `/apps/compliance/`, but the only explicit verification is a manual browser-refresh smoke test. This exact issue already proved subtle enough to require a plan correction.

The repo already has targeted `staticspa` tests. This is the right place to lock the compliance path behavior down.

**Recommended plan change**
- Add a unit test proving `/apps/compliance/...` falls back to `/apps/compliance/index.html`.
- Optionally add a negative/assertion case showing the old nested path would not be used.

### 9. The planned export-auth test cannot prove Bearer-token behavior if it only exercises `compliance.RegisterRoutes(...)`

**Plan references**
- `apps/compliance/COMP-IMPL-V1.md:185-190`

**Repo evidence**
- `backend/cmd/server/main.go:71-76`
- `backend/internal/auth/middleware.go:58-68`
- `backend/internal/acl/acl.go:13-27`

`compliance.RegisterRoutes(...)` will only attach role middleware. It does not attach the top-level auth middleware that enforces the `Authorization: Bearer ...` header. If the test setup injects claims directly into context, it can verify ACL, but not real token-gated transport.

**Recommended plan change**
- Split the test intent into:
- handler/ACL tests at the compliance package level
- one higher-level server/mux test for unauthenticated export access through the real `/api` middleware stack

## Suggested Approval Conditions

The plan is ready to execute once these are folded back in:

1. Resolve the `DELETE /origins` vs shared client contract.
2. Add explicit `pgx` stdlib dependency/import work.
3. Narrow the auth error-body contract to match the shared middleware behavior.
4. Add `docker-compose.dev.yaml` and deployment env wiring tasks.
5. Include launcher/catalog test updates caused by the new compliance role.
6. Reconcile the origin-create contract between the plan and `compliance-migspec.md`.

After those fixes, the remaining items are implementation details rather than plan-level blockers.
