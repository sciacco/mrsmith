# Training

Knowledge entries specific to `apps/training`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Training Directory Chips Are Action-First

- Context: `apps/training` People directory (`/persone`) and backend `GET /api/training/v1/people/directory`.
- Discovery: directory chips are planning/action flags, not passive HR-style person statuses. The directory must not derive chips from dormant employee fields such as hire date, manager hierarchy, or other external HR-ish attributes.
- Practical rule: expose `PersonSummary.flags` with action-first booleans (`da_pianificare`, `compliance_gap`, `scadenze_imminenti`, `failed_recente`, `senza_formazione_attiva`) and derive them only from Training-domain plans, enrollments, mandatory rules, courses, certifications, and mandatory coverage. Do not reintroduce legacy passive chips such as "a norma", "nuovo assunto", or "senza piano".
- Evidence: Training M4 directory refactor in `backend/internal/training/store_directory.go`, `backend/internal/training/types.go`, and `apps/training/src/pages/PeoplePage.tsx`.
- Used by: `apps/training` `/persone` directory and planning bulk assignment flows.
- Open questions: none.

### Training Rule Populations Stay Training-Side

- Context: `apps/training` compliance rules, custom groups, directory filters, and planning suggestions.
- Discovery: rule populations are owned by the Training domain and must remain limited to `all`, `team`, `skill_area`, and `custom_group`; custom group membership is resolved live from Training tables.
- Practical rule: do not add manager, role, hire-date, site, or external HR population predicates to Training mandatory rules. If a population does not fit team, skill area, or all employees, model it as a Training custom group.
- Evidence: Training M5 rule/group schema in `deploy/migrations/016_training_m5_rules_groups.sql`, population resolver view `training.v_mandatory_rule_population`, and backend handlers under `backend/internal/training`.
- Used by: `apps/training` `/compliance/regole`, `/persone/gruppi`, `/persone`, and `/pianificazione`.
- Open questions: none.

### Training Compliance Courses Become Mandatory Through Rules

- Context: `apps/training` catalog course metadata and compliance rule CRUD.
- Discovery: a Training course can be linked to a compliance framework without being mandatory for anyone. Per-person obbligatorietà exists only when an active `training.mandatory_rules` row applies to that person and course.
- Practical rule: catalog and course UI should describe `course.is_compliance_course` + `course.compliance_framework` as compliance metadata. Pipeline badges, alert priority, and enrollment exports must use rule-derived `requiredByRule`, not course metadata.
- Evidence: migration `deploy/migrations/017_training_compliance_course_semantics.sql`, enrollment rule resolution in `backend/internal/training/store.go`, rule validation in `backend/internal/training/store_rules_groups.go`, and frontend badge logic in `apps/training/src/components/PipelineCard/PipelineCard.tsx`.
- Used by: `apps/training` `/catalogo`, `/compliance/regole`, `/pipeline`, `/persone`, and planning suggestions from mandatory gaps.
- Open questions: none.

### Training People Admin Can Create Local Employees

- Context: `apps/training` People directory (`/persone`) and backend `POST /api/training/v1/people`.
- Discovery: `training.employee` remains primarily a local read model, but People admins need a manual escape hatch to add people immediately for training planning.
- Practical rule: only People-admin flows may create local employee rows. Creation must be audited, enforce unique email and active team selection, and must not be available from login or employee self-service workflows.
- Evidence: approved Persone page create-person implementation in `backend/internal/training/store_mutations.go`, `backend/internal/training/handler.go`, and `apps/training/src/components/PersonCreateModal/PersonCreateModal.tsx`.
- Used by: `apps/training` `/persone` manual create flow.
- Open questions: none.

### Training-Factorial Sync Correlates Objects By Embedded Tokens, Not Foreign Keys

- Context: outbound/inbound reconciliation between Training's course/event/session/enrollment data and Factorial's Trainings API (`backend/internal/training/factorial_sync_*.go`, `backend/cmd/training-factorial-sync`, `JobRunner.WithFactorialSync`). Successor to the one-shot CSV bootstrap (`backend/cmd/training-import`, removed in #150) — the anagrafica/directory sync is now the only channel for employee data.
- Discovery: Factorial's Trainings resources have no foreign-key field for an external id except `Training.code`, so the sync correlates classes and sessions by embedding the local UUID as a token inside the Factorial `name` instead. The entity mapping is fixed one-to-one; several Training concepts (`learning_outcome`, multi-day `scheduled` sessions, real cost/payment data) have no representable counterpart on the Factorial side.
- Practical rule: never invent another field to carry the local id — extend the token/`code` correlator instead. Never purge the `training_session/delete` and `enrollment_session/remove` audit rows: they are this sync's only durable tombstones, read back by tombstone propagation to know what to delete remotely.
- Evidence: `backend/internal/training/factorial_sync_types.go`, `factorial_sync_fetch.go`, `factorial_sync_project.go`, `factorial_sync_outbound.go`, `factorial_sync_inbound.go`, `factorial_sync_local.go`, `factorial_sync_tombstones.go`, `factorial_sync_perimeter.go`, `factorial_sync_run.go`; `backend/pkg/factorial/types.gen.go` (`TrainingsTraining`, `TrainingsTrainingClass`, `TrainingsSession`, `TrainingsTrainingMembership`, `TrainingsSessionAccessMembership`, `TrainingsSessionAttendance`); #141 and slices #143–#149.
- Used by: ops running `training-factorial-sync` or the `TRAINING_FACTORIAL_SYNC_ENABLED` worker path; task 6 of #137 (planning/compliance UI once it depends on synced Factorial state, including the due-date UTC-midnight convention below); task 8 of #137 (live cutover smoke).
- Open questions: none beyond the registered debt and the re-evaluation trigger listed under Operational Notes.

#### Entity Mapping

Names on the left are Factorial Trainings-API resources (`Trainings*` in `pkg/factorial/types.gen.go`); names on the right are local `training.*` tables/columns.

| Factorial resource | Local training concept |
| --- | --- |
| `Training` | `training.course` |
| `TrainingClass` | `training.training_event` |
| `Session` | `training.training_session` |
| `TrainingMembership` | `training.enrollment` (course-level, aggregate membership) |
| `SessionAccessMembership` | `training.enrollment` + `training.enrollment_session` (session-level access; a remote access membership links both) |
| `SessionAttendance` | `training.enrollment_session.participation_status` |

#### Non-Representable Limits

- `learning_outcome` is local-only: Factorial has no field for it, so it is never exported or imported.
- A `scheduled` session spanning more than one calendar day cannot be exported: Factorial's session API accepts same-day sessions only.
- Costs and payment status sent to Factorial are technical placeholders (`0`, `pending`), never Training's own economic data; on an outbound update, an existing Factorial cost/payment value is preserved rather than overwritten.

#### Correlators

- `Training.code` (a dedicated Factorial field on the course-level resource) holds the local `course` UUID.
- Classes and sessions have no equivalent field: the local UUID is embedded as a `[MS:<uuid>]` token inside the Factorial `name`.
- Every correlator-bearing create is search-before-create against the correlator: 0 matches → create, 1 match → adopt, more than 1 → conflict (isolates that branch, does not fail the run). Membership and access creates carry no token: they instead diff a `BulkCreate` batch by employee id against who is already linked.

#### Configuration

- `TRAINING_FACTORIAL_SYNC_ENABLED` (default `false`): gates the worker's periodic run; even when `true`, the run stays disabled unless directory sync is enabled and both the Factorial client and `FACTORIAL_TRAINING_AUTHOR_EMPLOYEE_ID` are configured.
- `FACTORIAL_TRAINING_AUTHOR_EMPLOYEE_ID`: required technical/author employee id used as the author on Factorial writes. The `training-factorial-sync` CLI fails fast without it; the worker instead logs `training factorial sync disabled: prerequisites not met` and leaves the periodic run disabled (`JobRunner.WithFactorialSync`, `jobs.go:61-68`).
- Factorial HTTP requests use a 30s timeout, configured identically on the worker's and the CLI's client.

#### Operational Notes (accumulated across #143–#149)

- Two-date convention: the canonical projection reduces instants to `YYYY-MM-DD` in UTC; sync writes use explicit UTC midnight. The future task 6 UI must also write deadlines as UTC midnight to stay consistent with this projection.
- Dry-run means no *domain* write: `training.directory_sync_run` is still recorded even in dry-run (pre-existing, declared behavior); `rda_exceptions` cannot be computed in dry-run, so the CLI prints `n/d` instead of a misleading `0`.
- `FactorialSyncRun.Error` may carry the raw upstream error text on a failed run (infrastructural error classes: auth/rate-limit/5xx/transport/DB failures); structured log samples carry only technical ids and HTTP status codes, never response bodies or query strings.
- Tombstone counters (`Tombstone.AccessDestroyed`, `Tombstone.SessionsDeleted`) are cumulative historical totals re-propagated on every run (a 404 on delete counts as success), not a per-run delta — a ratified decision, with no "already propagated" marker kept.
- Ambiguous provenance (ownership of a missing remote object cannot be attributed) is conserved with a `*_missing_remote_provenance_unknown` warning, never reset to the opposite side's default.
- Remote sessions with no linked class fall outside the perimeter graph — a documented gap, not a bug — and are counted in the run report as `sessionsWithoutClass`.
- The active-employee filter applies to both sync directions: inbound and outbound.
- Registered debt: create audits use `auditFields` (a named list of changed fields) instead of the package's full before/after snapshot. Provenance checks still work (they only read `action`), but conflict forensics lose the pre-update state on session updates.
- Each run performs two full anagrafica censuses (a prerequisite directory sync, then a fresh active-employee snapshot) — factor this into `TRAINING_JOBS_INTERVAL` sizing.
- Composition lesson: the inbound dry-run gap (the #141 no-domain-write guarantee wasn't honored by `applyInboundSync`; it slipped past two slice-4 gates and only surfaced when slice 7 composed the full run; closed by threading a `dryRun` parameter through `applyInboundSync`) — a cross-cutting guarantee needs a check on every slice that writes, not just the slice that states it.
