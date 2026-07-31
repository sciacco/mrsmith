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
