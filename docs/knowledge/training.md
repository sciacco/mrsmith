# Training

Knowledge entries specific to `apps/training`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Training Request Team Is Optional Only Without Active Memberships; People Decides Without TL Opinion

- Context: `apps/training` training requests (`/richieste`), decision workflow, work queue (`/`), planning (`/pianificazione`); `training.training_request.selected_team_id` is nullable since migration `149_training_request_optional_team.sql` (#200).
- Discovery: the team on a request is mandatory **only** when the person has at least one active team membership (`training.team_membership.end_date IS NULL`); when present it must be one of those memberships. A person with no memberships keeps a fully operative teamless request. The TL opinion is a consultative fact, never a prerequisite: People can accept or reject any open request with, without, or against a recorded opinion, and no opinion is ever generated implicitly. `RecordTLOpinion` requires the request to have a team and the signer to be an active lead of that team.
- Practical rule: never reintroduce an opinion-based gate or a synthetic "pending opinion" queue. The single decision queue is all and only requests with `outcome IS NULL`, `suspended_at IS NULL`, `people_decision IS NULL`. Any read of request team must be nullable end-to-end (LEFT JOIN / `COALESCE`, `omitempty` in JSON so absence = key omitted, `?? 'Senza team'` / `Non registrato` neutral labels in UI, no empty option in team filters). Backend error codes: `selected_team_required` (memberships exist, team empty), `selected_team_invalid` (team foreign to the person), `request_without_team` (opinion on a teamless request).
- Evidence: `backend/internal/training/store_requests.go` (`validateSelectedTeam`, `requestDecisionPolicy`, `lockRequestFacts`, `RecordTLOpinion`), `store_queues.go` (`QueueRequestsAwaitingDecision`), `store_planning.go` + `planning_projection.go` (nullable `*PlanningRef` team), `types_requests.go`/`types_queues.go`/`types_planning.go` (`omitempty`), frontend `apps/training/src/api/types.ts`, `components/requests/*`, `pages/WorkQueuePage`, `pages/PlanningPage/PlanningCourseDrawer.tsx`.
- Used by: request registration/edit/decision flows, the «Da decidere» queue, planning projections and team filters, person scheda.
- Open questions: none.

### Training Directory Shows Facts, Not Synthetic Chips

- Context: `apps/training` People directory (`/persone`) and backend `GET /training/v1/people`.
- Discovery: the pre-rebuild directory (task 6 of #137 replaced it, migration `129_training_domain_restructure.sql`) exposed synthetic action-flag chips (`da_pianificare`, `compliance_gap`, …) computed from dormant HR-ish state. The rebuilt domain drops that layer entirely: the directory row carries only facts already on `training.employee` and its live joins — `status`, `directoryExempt`, current team memberships (with `role`), custom-group memberships. It does not compute or expose any derived planning flag.
- Practical rule: keep the directory a facts table. Actionable signals (undecided requests, uncovered seats, expiring coverage, stale enrollments…) belong to the work queue (`/`, `backend/internal/training/handler_queues.go`) and to the person's rule-coverage list (`GetPersonDetail`), never to a synthetic chip recomputed on the directory row itself.
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

Course-card fields seeded on import (August 2026 decisions, migration 135), all only while the local course is **not active** (once curated/activated the sync never touches the card again). Single exception — embryo adoption (#172): on the run that adopts a previously-unlinked course by exact title, the card is seeded once even if active (first contact with the sync, like a twin's birth); from the next run the course is correlated and the not-active gate applies again.

- `Training.external_provider` → vendor registry get-or-create by case-insensitive name (`training.vendor`, citext unique) + `course.vendor_id`. The literal string `"null"` is junk and is discarded.
- `Training.external` → `course.provider_kind` (`external`/`internal`). Real data is coherent: provider names exist only on external trainings; in the 3 observed category-vs-flag conflicts the flag wins.
- `Training.category_ids` (resolved to names via the categories resource, fetched with the graph) → `course.tags` (free-text tags, multiple), **except «Formazione interna»**, which never becomes a tag (delivery kind has its own field). Skill areas are NOT derived from categories: the intended taxonomy is granular (Kubernetes, Cloud, ISO 14001… — the People planning Excel), which Factorial macro-categories do not carry.
- Skill areas are the home for "competencies a course develops" and the link is **many-to-many** since migration 136: `training.course_skill_area` (courses) and `training.training_request_skill_area` (requests, which map onto courses); the single `skill_area_id` columns on course and request are gone. Paths, certifications and per-person assessments keep their single-area link.
- `Training.competency_ids` → skill areas, seeded on import (same not-active gate). The public Factorial API (2026-07-01) exposes NO endpoint listing the competencies catalog with names (verified across all resource paths; the job-catalog `node_attributes` competency probe returns empty), so the ID→name map lives in code (`factorial_competencies.go`), read manually from the Factorial UI on 2026-08-30 and cross-validated on all 9 trainings that use competencies (UI list order follows ascending IDs). Get-or-create in `training.skill_area` is keyed by `code` (slug of the name) so renaming an area in-app never creates duplicates; an unmapped ID emits `course_competency_unresolved` (fix = add one line to the map).
- `TrainingClass.name` (+description when present) → `training_event.notes`, **only when local notes are empty** — never over human text. This is where the edition identity lives (the local model has no event name by contract).
- Session operational data → new `training_session` columns `topic` (from `Session.name`; a name containing the `[MS:` correlator token is technical, not a topic, and is skipped), `modality` (raw Factorial values `inperson`/`online`/`mixed`), `duration_hours` (canonicalized decimal hours), `location`. These live OUTSIDE the 3-way checkpoint (which stays schedule/date-only, so existing checkpoints don't churn and outbound sees no phantom local edits): they are remote-owned while the integration is active, realigned without conflicts (`sessionOpState`), and only for sessions with a checkpoint or import provenance.
- `TrainingMembership.training_completed_at` → `enrollment.actual_end` cascade (`applyCompletionDates`): explicit membership date when remote status is also `completed`; otherwise, for enrollments whose local facts say completed, the date of the last completed session; only ever when `actual_end` IS NULL (a human-set date is never touched). This also fixes the latent coverage defect where historical completions fell back to `updated_at` (= import day). Draft trainings import like any other (real delivered history lives in drafts; the status is used loosely on Factorial and carries no rule).

#### Non-Representable Limits

- `learning_outcome` is local-only: Factorial has no field for it, so it is never exported or imported.
- A `scheduled` session spanning more than one calendar day cannot be exported: Factorial's session API accepts same-day sessions only.
- Costs and payment status sent to Factorial are technical placeholders (`0`, `pending`), never Training's own economic data; on an outbound update, an existing Factorial cost/payment value is preserved rather than overwritten.

#### Correlators

- `Training.code` (a dedicated Factorial field on the course-level resource) holds the local `course` UUID.
- Classes and sessions have no equivalent field: the local UUID is embedded as a `[MS:<uuid>]` token inside the Factorial `name`.
- Every correlator-bearing create is search-before-create against the correlator: 0 matches → create, 1 match → adopt, more than 1 → conflict (isolates that branch, does not fail the run). Membership and access creates carry no token: they instead diff a `BulkCreate` batch by employee id against who is already linked.
- Inbound course adoption by title (#172): a Training with no local correlated course first looks among local courses **without** correlator for an exact title match, case-insensitive (`lower(title)`, same convention as request-side embryo reuse). Exactly one candidate → the correlator is written on it (`factorial_training_id`, finding `course_adopted_by_title`) and the card is seeded once, active or not; more than one → new course as before plus finding `course_adopt_ambiguous` listing the candidate ids in the finding `detail`. Never on the fallback title (`Training Factorial <id>`). From the next run the adopted course is an ordinary correlated course (no re-adoption, no rewriting). Within one run the adopted course leaves the candidate pool, so two same-titled Trainings cannot adopt the same course.

#### Configuration

- `TRAINING_FACTORIAL_SYNC_ENABLED` (default `false`): gates the worker's periodic run; even when `true`, the run stays disabled unless directory sync is enabled and the Factorial client is configured (`JobRunner.WithFactorialSync`).
- Sync mode: `mrsmith.runtime_config` row `('training','factorial_sync_mode')`, JSON string `"import_only"` or `"full"`, read at the start of every run (`SQLStore.factorialSyncMode`, `runtime_config.go`). **Missing row = `import_only`**: every write toward Factorial is suspended — tombstone delete propagation (T2) and the whole outbound phase — while reads, tombstone selection/filtering, missing detection (local unlink included) and inbound keep running. Unknown value fails the run (never a silent fallback to a writing mode). The report carries `mode`; the CLI prints it in the summary line.
- Author employee id for Factorial writes: runtime configuration, not env — `mrsmith.runtime_config` row `('training','factorial_author_employee_id')`, JSON string or number, read at the start of every run (`SQLStore.factorialAuthorEmployeeID`, `runtime_config.go`). Required (run fails with an explicit error recorded in the run history) only when the sync mode is `full`; in `import_only` it is not read. The value can be changed without a restart.
- Attestati and document contents live in the database: `training.document_blob` keyed by `training.document.storage_key` (migration 134; `DBStorage` in `storage.go`). No local filesystem; `TRAINING_STORAGE_MAX_BYTES` still caps the per-file size (default 20 MB).
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

#### Operational Notes (accumulated during the final smoke)

- The enrollment facts `PUT` is a full replacement: omitted fields are cleared (omitting `actualEnd` wipes a date the sync had backfilled). Clients must resend the complete facts set on every update.
- `POST /jobs/run` executes the same jobs with the same switches as the periodic worker (`WithDirectorySync`/`WithFactorialSync` wired from the handler deps): with the flags off it is a safe no-op with no Factorial calls. The per-person notification job no longer exists.
- Seat-rule "in training" attribution: an enrollment counts as linked to a seat rule when it carries `source_rule_id` **or** lives on a non-cancelled round event of that rule — enrolling someone on the rule's round is the linking gesture, no dedicated action exists.
- Sync findings persist `local_entity`/`local_id`; the read query resolves the owning event (`localEventId`) for sessions and enrollments so the UI can always offer "Apri evento".
- Award deletion is a hard delete mirroring assessments (audit snapshot before the delete); `training.document` rows cascade by FK, but `training.document_blob` rows have no FK and are deleted explicitly in the same transaction.
- `enrollment.actual_end` is backfilled by the inbound sync only once (only when NULL and completed); coverage falls back to `COALESCE(actual_end, updated_at::date)` and the delivered report computes hours as `COALESCE(hours_actual, course.default_hours)`.
- `training_session.occupancy` counts only non-terminal assignments (assigned/in progress), never completed ones: capacity gates future seats, so a fully-delivered session shows occupancy 0.
- Portal gating for Training uses `app_training_people_admin` only: the POC role `app_training_access` was removed from `@mrsmith/auth-client` during the final static sweep.
