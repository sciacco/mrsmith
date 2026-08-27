# Training

Knowledge entries specific to `apps/training`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Training Directory Shows Facts, Not Synthetic Chips

- Context: `apps/training` People directory (`/persone`) and backend `GET /training/v1/people`.
- Discovery: the pre-rebuild directory (task 6 of #137 replaced it, migration `129_training_domain_restructure.sql`) exposed synthetic action-flag chips (`da_pianificare`, `compliance_gap`, …) computed from dormant HR-ish state. The rebuilt domain drops that layer entirely: the directory row carries only facts already on `training.employee` and its live joins — `status`, `directoryExempt`, current team memberships (with `role`), custom-group memberships. It does not compute or expose any derived planning flag.
- Practical rule: keep the directory a facts table. Actionable signals (requests without opinion, uncovered seats, expiring coverage, stale enrollments…) belong to the work queue (`/`, `backend/internal/training/handler_queues.go`) and to the person's rule-coverage list (`GetPersonDetail`), never to a synthetic chip recomputed on the directory row itself.
- Evidence: `backend/internal/training/store_people_reads.go` (`ListPeople`, `GetPersonDetail`), `backend/internal/training/types_reads.go` (`PersonListRow`, `PersonDetail`), `apps/training/src/pages/PeoplePage/PeoplePage.tsx`, `apps/training/src/pages/PersonPage/PersonPage.tsx`.
- Used by: `apps/training` `/persone` directory and `/persone/:id` scheda formativa.
- Open questions: none.

### Training Rule Populations Stay Training-Side

- Context: `apps/training` training rules (`/regole`), local groups (`/persone` § gruppi), skill areas (`/catalogo` § anagrafiche).
- Discovery: rule populations are owned by the Training domain and remain limited to `all`, `team`, `skill_area`, `custom_group`, and `people` (single-person allow-list, added by the rebuild for one-off mandatory assignments); local-group membership is resolved live from `training.custom_group_members`, never cached.
- Practical rule: do not add manager, role, hire-date, site, or external HR population predicates to `training.training_rule.population_target`. If a population does not fit team, skill area, an explicit person list, or the whole organization, model it as a Training custom group. A `skill_area` population only resolves when the area has a `custom_group_id` link (migration `131_training_skill_area_group.sql`) — an unlinked area fails hard with `422 skill_area_without_group`, both blocking rule activation and failing coverage calculation, not a silent skip.
- Evidence: `deploy/migrations/129_training_domain_restructure.sql` and `131_training_skill_area_group.sql`; `backend/internal/training/store_coverage.go` (`skillAreaGroupID`), `backend/internal/training/store_rules.go` (`SetRuleActive`, `populationKindPeople` and siblings), `backend/internal/training/store_groups.go`, `backend/internal/training/store_mutations.go` (`ensureCustomGroupSelectable`, `ensureSkillAreaGroupCanBeRemoved`).
- Used by: `apps/training` `/regole`, `/persone` (gruppi locali), `/catalogo` (collegamento area → gruppo), `/persone/:id` (coperture).
- Open questions: none.

### Training Compliance Courses Become Mandatory Through Rules

- Context: `apps/training` catalog course metadata (`/catalogo`) and rule CRUD (`/regole`).
- Discovery: a Training course can be linked to a compliance framework without being mandatory for anyone. Per-person obbligatorietà exists only when an active `training.training_rule` row (`is_mandatory = true`) applies to that person and course; the rebuild kept this separation intact.
- Practical rule: catalog and course UI describe `course.is_compliance_course` + `course.compliance_framework` as compliance metadata only (`CourseListRow.complianceRelated`/`complianceFramework`). Mandatory status, coverage, and queue alerts must come from the rule (`RuleListRow.isMandatory`, `PersonRuleCoverageRef`), never from course metadata alone.
- Evidence: `backend/internal/training/types.go` (`CourseInput`, `RuleInput`), `backend/internal/training/store_mutations.go` (`UpsertCourse`), `backend/internal/training/store_rules.go`, `backend/internal/training/store_people_reads.go` (`personRuleCoverage`); frontend in `apps/training/src/components/catalog/CourseEditorModal.tsx` and `apps/training/src/pages/PersonPage/PersonPage.tsx` (sezione coperture).
- Used by: `apps/training` `/catalogo`, `/regole`, `/persone/:id`, and the work queue's coverage sections.
- Open questions: none.

### Training People Admin Can Create Local Employees

- Context: `apps/training` People directory (`/persone`) and backend `POST /training/v1/people`.
- Discovery: `training.employee` remains primarily a directory-synced read model, but People admins need a manual escape hatch to add people immediately for training planning. `directory_exempt` doubles as the same escape hatch on existing directory-managed people: setting it unlocks name/email/status/team editing that the backend otherwise blocks with `person_managed_by_directory`.
- Practical rule: only People-admin flows may create or edit local employee rows; both are audited. Creation enforces a unique email and, when a team is selected, an active team. Modifying a directory-managed person's identity fields or team without first setting `directoryExempt` is rejected — the UI does not predict this locally, it lets the backend error surface as-is.
- Evidence: `backend/internal/training/store_mutations.go` (`CreatePerson`, `UpdatePerson`, `directoryManagedPersonState`), `backend/internal/training/handler_actions.go` (`handleCreatePerson`, `handleUpdatePerson`), `apps/training/src/components/people/PersonEditorModal.tsx`.
- Used by: `apps/training` `/persone` (creazione) and `/persone/:id` (modifica, incluso lo sblocco `directoryExempt`).
- Open questions: none.

### Training-Factorial Sync Correlates Objects By Embedded Tokens, Not Foreign Keys

- Context: outbound/inbound reconciliation between Training's course/event/session/enrollment data and Factorial's Trainings API (`backend/internal/training/factorial_sync_*.go`, `backend/cmd/training-factorial-sync`, `JobRunner.WithFactorialSync`). Successor to the one-shot CSV bootstrap (`backend/cmd/training-import`, removed in #150) — the anagrafica/directory sync is now the only channel for employee data.
- Discovery: Factorial's Trainings resources have no foreign-key field for an external id except `Training.code`, so the sync correlates classes and sessions by embedding the local UUID as a token inside the Factorial `name` instead. The entity mapping is fixed one-to-one; several Training concepts (`learning_outcome`, multi-day `scheduled` sessions, real cost/payment data) have no representable counterpart on the Factorial side.
- Practical rule: never invent another field to carry the local id — extend the token/`code` correlator instead. Never purge the `training_session/delete` and `enrollment_session/remove` audit rows: they are this sync's only durable tombstones, read back by tombstone propagation to know what to delete remotely.
- Evidence: `backend/internal/training/factorial_sync_types.go`, `factorial_sync_fetch.go`, `factorial_sync_project.go`, `factorial_sync_outbound.go`, `factorial_sync_inbound.go`, `factorial_sync_local.go`, `factorial_sync_tombstones.go`, `factorial_sync_perimeter.go`, `factorial_sync_run.go`; `backend/pkg/factorial/types.gen.go` (`TrainingsTraining`, `TrainingsTrainingClass`, `TrainingsSession`, `TrainingsTrainingMembership`, `TrainingsSessionAccessMembership`, `TrainingsSessionAttendance`); #141 and slices #143–#149.
- Used by: ops running `training-factorial-sync` or the `TRAINING_FACTORIAL_SYNC_ENABLED` worker path; task 6 of #137 (planning/compliance UI once it depends on synced Factorial state, including the due-date UTC-midnight convention below); task 8 of #137 (live cutover smoke).
- Open questions: none beyond the registered debt and the re-evaluation trigger listed under Operational Notes.
- Read surface: migration `133_training_factorial_sync_runs.sql` persists every run and its findings (`training.factorial_sync_run`, `training.factorial_sync_finding`); `backend/internal/training/factorial_sync_store.go` writes them at the end of `RunFactorialSync`, `backend/internal/training/handler_factorial_sync.go` exposes `GET /training/v1/factorial/sync/runs` and `/runs/{id}` (read-only, no start route — the CLI/worker remain the only way to run it), and `apps/training/src/api/factorialSync.ts` + the "Sync formativo" section of `apps/training/src/pages/FactorialPage.tsx` render the history and the findings grouped by phase/severity.

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
