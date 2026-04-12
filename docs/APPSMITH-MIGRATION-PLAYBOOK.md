# Appsmith Migration Playbook

This document defines the default migration method for moving legacy Appsmith applications into the MrSmith repo.

The rule is simple: verify facts first, encode the risky facts as tests, then implement. Do not let an LLM, a spec draft, or an engineer's memory invent contracts that were never validated against the legacy app and the live schemas.

## Why This Exists

The repeated failure mode in migrations is not usually UI code. It is contract drift:

- inferred SQL column names that do not exist in the live datasource
- generalized filters that were only correct for one screen
- cross-system joins that use the wrong key
- fallback values that were present in Appsmith but lost in translation
- external API state transitions that look reasonable but do not match the legacy flow

This repo already contains examples of that drift being caught and corrected in `apps/quotes`, `backend/internal/quotes`, `backend/internal/kitproducts`, and `docs/IMPLEMENTATION-KNOWLEDGE.md`.

## Source-Of-Truth Order

When migrating an Appsmith app, treat sources in this order:

1. Legacy Appsmith query/action code and JSObjects
2. Database schema, views, functions, stored procedures, and authoritative API specs
3. Existing verified repo knowledge in `docs/IMPLEMENTATION-KNOWLEDGE.md`
4. Current backend/frontend implementation in this repo
5. Human memory, assumptions, or LLM suggestions

If a lower-priority source conflicts with a higher-priority source, the higher-priority source wins unless new evidence disproves it.

## Required Workflow

### 1. Audit The Legacy App

Before writing code, extract the actual behavior of the Appsmith app:

- queries and mutations per screen
- JSObject orchestration and hidden rules
- widget-driven filters and default values
- cross-system lookups
- external API calls and status transitions
- auth/role assumptions and feature gates

Do not summarize the app at a feature level only. Capture the concrete contracts that implementation must preserve or intentionally change.

### 2. Build A Migration Fact Sheet

For each risky flow, write down the validated facts that code must honor:

- exact table, view, procedure, and column names
- exact filters, exclusions, ordering, and grouping
- primary keys and cross-system join keys
- null handling and fallback/default values
- string formats, status values, and enum-like constants
- side effects and orchestration order across systems

The fact sheet is the bridge between the audit and implementation. If a fact is missing here, it is likely to drift later.

### 3. Convert Risky Facts Into Contract Tests

Before implementing handlers or UI logic, add narrow tests that pin the facts most likely to drift.

Typical categories:

- query-shape tests for exact columns, joins, filters, and sort order
- fallback/default tests for values previously hidden inside Appsmith SQL or JS
- orchestration tests for multi-step external API flows
- identity-mapping tests for cross-database joins
- regression tests for previously wrong assumptions

These tests should be small and sharp. Their job is not to prove the whole feature works. Their job is to stop implementation drift.

### 4. Implement Against The Pinned Contracts

Only after the risky facts are pinned should implementation proceed.

During implementation:

- avoid rewriting verified behavior into "cleaner" abstractions without proving equivalence
- keep context-specific rules explicit instead of silently generalizing them
- prefer dependency injection and testable seams around legacy integrations
- keep request/response contracts aligned with verified source behavior or document intentional deltas

### 5. Verify Behavior End-To-End

After implementation, verify both the feature and the migration fidelity:

- happy path behavior
- boundary conditions and null/empty cases
- rollback or partial-failure behavior
- auth and permission behavior
- parity with legacy data-loading semantics where parity is intended
- intentional deviations from legacy behavior where parity is not intended

### 6. Capture Reusable Discoveries

Any reusable discovery that affects future work must be added to `docs/IMPLEMENTATION-KNOWLEDGE.md` in the same change set or immediate follow-up.

Examples:

- stable cross-system key mappings
- datasource-specific column-name quirks
- required fallback semantics
- external API state-machine rules
- exclusions or eligibility rules that should not be rediscovered from scratch

## Contract Test Guidance

The default question is not "what broad integration test can we add?" The default question is "what exact fact is most likely to drift if not pinned?"

Prefer focused tests like:

- "this query must use `CODICE_PAGAMENTO`, not the stale alias"
- "this endpoint must preserve the `402` fallback"
- "this order lookup must filter by `ID_CLIENTE` and `STATO_ORDINE`"
- "this republish flow must unlock the HubSpot quote before updating it"

These tests should fail loudly when a future refactor reintroduces a plausible but wrong assumption.

## How To Use LLMs Safely

Use the model for:

- extracting facts from Appsmith exports and schema docs
- summarizing evidence across files
- drafting test cases from verified evidence
- identifying likely drift points and missing validation

Do not use the model as an authority for:

- inventing SQL against legacy schemas
- choosing join keys without evidence
- normalizing hidden business rules into a guessed abstraction
- filling gaps in external API behavior without source verification

A good prompt asks the model to reason over evidence. A bad prompt asks it to guess the contract.

## PR Gate For Migration Work

Before approving an Appsmith migration PR, check:

- Which legacy facts were explicitly verified?
- Which tests pin the risky contracts?
- Which intentional deviations from Appsmith were documented?
- Which reusable discoveries were added to `docs/IMPLEMENTATION-KNOWLEDGE.md`?
- Does the implementation plan still satisfy `docs/IMPLEMENTATION-PLANNING.md` for repo fit, auth, data contracts, and verification?

If those answers are vague, the migration is probably not ready.

## Quotes-Led Examples

The quotes migration surfaced several concrete drift patterns that this playbook is meant to prevent:

- Category exclusions drifted when a single "standard" rule replaced a context-specific Appsmith filter. The verified rule is captured in `docs/IMPLEMENTATION-KNOWLEDGE.md` under "Quotes Create Flow Uses Context-Specific Category Exclusions".
- Replacement-order lookup drifted when the Alyante query was inferred with the wrong customer column. The pinned contract is exercised in `backend/internal/quotes/handler_reference_test.go`.
- Customer default payment drifted when a stale alias replaced the live Alyante column. The verified contract and `402` fallback are pinned in `backend/internal/quotes/handler_reference_test.go`.
- Publish payment labels drifted when old loader aliases were assumed. The pinned query contract lives in `backend/internal/quotes/handler_publish_test.go`.
- HubSpot republish behavior drifted when the lock/unlock transition was not modeled from the legacy flow. The required unlock step is pinned in `backend/internal/quotes/handler_publish_test.go`.
- Alyante translation sync drifted when backend code guessed generic column names instead of using the live Appsmith datasource contract. The verified write contract is pinned in `backend/internal/kitproducts/alyante_test.go`.

## Default Standard

For future Appsmith migrations in this repo, the default order of work is:

1. Audit
2. Fact sheet
3. Contract tests
4. Implementation
5. End-to-end verification
6. Knowledge capture

If implementation starts before steps 1 through 3 are materially complete, expect avoidable drift.
