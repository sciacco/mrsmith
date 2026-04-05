# Appsmith Migration - Kickoff Prompt For An LLM Agent

You are running the first real migration-specification iteration for a new repository.

Your job is not to implement the new system yet. Your job is to:
- validate that `appsmith-audit` extracts enough signal from one real Appsmith app
- drive one disciplined Phase 2 specification pass using `appsmith-migration-spec`
- leave behind a reviewable set of artifacts and a clear backlog of gaps

Treat this as a scoped execution task, not an open-ended research project.

## Scope of this run

This run covers exactly one Appsmith application and one iteration.

Do not:
- scaffold React code
- design Go service packages
- translate Appsmith widgets directly into implementation components
- silently infer missing business semantics that the audit does not prove
- broaden the run into multiple apps unless explicitly instructed

The output of this run must be platform-neutral and suitable for review by product, engineering, and migration stakeholders.

## Objective

Use one real Appsmith export plus the downstream `appsmith-migration-spec` skill to produce:
1. a structural audit artifact set
2. an audit-adequacy assessment
3. a first-pass platform-neutral application specification
4. a focused list of open questions for the domain expert
5. a feedback report describing what the audit extracted well and what still needs improvement

## Required inputs

You need:
- one Appsmith export zip
- access to `appsmith-audit`
- access to an installed `appsmith-migration-spec` skill
- a domain expert or user who can answer business questions the audit cannot resolve

If the export zip is missing, stop and ask for it.

Do not assume bundled example exports, tests, prompts, or sibling skill checkouts are present in the production environment unless the user explicitly provides them.

## Working directories and output paths

Unless the repository already defines an equivalent location, create artifacts under:

- `docs/migration/iteration-01/audit/`
- `docs/migration/iteration-01/spec.md`
- `docs/migration/iteration-01/audit-gap-report.md`
- `docs/migration/iteration-01/open-questions.md`
- `docs/migration/iteration-01/iteration-notes.md`

If the repository has an established documentation layout, follow that instead and note the chosen paths explicitly in your summary.

## Available tools and references

- Audit tool:
  `python3 scripts/inspect_appsmith_export.py <export.zip> --pretty`
- Audit Markdown artifacts:
  `python3 scripts/inspect_appsmith_export.py <export.zip> --artifacts-dir <dir>`
- Installed downstream skill:
  `appsmith-migration-spec`
- Bundled audit templates shipped with this tool:
  `templates/app-inventory.md`
  `templates/datasource-catalog.md`
  `templates/findings-summary.md`
  `templates/page-audit.md`

The Appsmith export zip is external input. It is not expected to be bundled with the production fileset.

## Execution rules

- Work in this order: audit, assess, question, specify, summarize.
- Extract facts first. Ask questions only for true business or design gaps.
- Keep current-state evidence separate from intended future behavior.
- Cite current Appsmith names exactly when describing source behavior.
- Keep the specification platform-neutral. Do not prescribe React components, Go types, route trees, or database schemas unless the user explicitly asks for an implementation phase later.
- If the audit is noisy or incomplete, document that precisely instead of compensating with guesses.
- If domain knowledge is required, stop and ask grouped, high-signal questions. Do not continue as if the answers were known.
- Do not edit upstream `appsmith-audit` or `appsmith-migration-spec` source during this repository kickoff run. Record improvement opportunities instead.
- Do not rely on development-only assets such as bundled example exports or source-repo-relative paths.

## Stop conditions requiring user or domain-expert input

Pause and ask for input when any of the following is true:
- entity meaning is ambiguous and cannot be resolved from names, actions, or bindings
- a workflow appears business-critical but the audit cannot reveal the decision rule
- multiple pages or actions appear to overlap and only a domain owner can say whether they should merge
- the intended future behavior differs from the current Appsmith behavior
- integration purpose, ownership, or trust boundary is unclear

When you pause, ask only the minimum set of questions needed to unblock the next section of the spec.

## Step-by-step procedure

### Step 1: Run the structural audit

Run `appsmith-audit` on one real export.

Produce:
- normalized JSON output
- Markdown artifacts under `docs/migration/iteration-01/audit/` or the repo-equivalent path

Record:
- which export was used
- audit command(s) run
- where the resulting artifacts were written

### Step 2: Assess whether the audit is adequate for Phase 2

Evaluate the audit against these questions:
- Are the primary entities and their operations identifiable?
- Can likely data shapes be reconstructed from action parameters, columns, and bindings?
- Are page-level UX patterns inferable from widget composition and page purpose?
- Is JSObject and inline logic classified clearly enough to reason about backend, frontend, or shared placement?
- Are dependency edges and cross-page flows good enough to reconstruct user journeys?

Write the result to `audit-gap-report.md`.

For each gap, include:
- the missing or weak signal
- why it blocks or slows Phase 2
- whether the expert can answer it manually
- whether it looks like an `appsmith-audit` enhancement opportunity

### Step 3: Start the specification pass

Use the installed `appsmith-migration-spec` skill and its bundled template to create `spec.md`.

Work through the five phases in order:
1. Entity-Operation Model
2. UX Pattern Map
3. Logic Placement
4. Integration and Data Flow
5. Specification Assembly

For each phase:
- write down extracted facts first
- list ambiguities second
- ask only the questions needed to resolve material ambiguities
- update `spec.md` immediately after answers are received

### Step 4: Maintain open questions explicitly

Keep `open-questions.md` as a live list of unresolved items.

Each question must include:
- the affected entity, page, flow, or integration
- the current evidence from Appsmith
- what cannot be concluded from the audit
- the decision needed from the expert
- the impact if the question remains unresolved

### Step 5: Write iteration notes

After the first-pass spec is complete, write `iteration-notes.md` with:
- what translated directly from the audit into the spec
- what the expert had to supply manually
- what audit output was noisy or low-value for Phase 2
- what process changes would improve the next iteration
- what `appsmith-audit` enhancements should be considered before the next app

## Definition of done for this run

This kickoff run is complete only when all of the following are true:
- one real export has been audited
- audit artifacts are saved in the repository
- `audit-gap-report.md` exists and is specific
- `spec.md` exists and follows the Phase 2 structure
- unresolved business decisions are captured in `open-questions.md`
- `iteration-notes.md` captures the audit-to-spec feedback loop
- the final summary clearly states whether the Phase 2 workflow is viable for this app and what must improve before repeating the process

## Final response format

When you finish, provide a concise summary with:
- the export analyzed
- the artifact paths created
- whether the audit was sufficient for a productive Phase 2 session
- the highest-impact open questions
- the highest-priority analyzer or process improvements

If you are blocked on expert input, say exactly what is blocked and point to `open-questions.md`.
