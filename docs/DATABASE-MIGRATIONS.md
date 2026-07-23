# Database Migration Compatibility Rule

## Non-negotiable rule

Every database change MUST preserve uninterrupted coexistence between the application version running before the migration and the version introduced after it.

Assume rolling deployments, multiple live replicas, delayed workers, retries, and the possible rollback of application code. Applying a migration MUST NOT make the database incompatible with binaries that may still be running.

Database changes follow four distinct phases:

1. **Expand**
2. **Coexist and migrate data**
3. **Cut over**
4. **Post-cleanup**

Never combine an incompatible cleanup with the expand or cutover required to release a feature.

## 1. Expand

The first production migration is additive and backward-compatible.

Typical safe expand operations include:

- adding nullable columns;
- adding tables without removing old ones;
- adding indexes that do not block production traffic unacceptably;
- widening accepted values or constraints;
- adding new states only when old binaries cannot consume or terminally fail them;
- introducing new storage alongside the existing contract.

The expand migration MUST allow the old application to continue reading and writing successfully. New code may depend on the expanded schema only after the migration has been applied.

Renames are not additive: add the new object and keep the old contract during coexistence instead of renaming in place.

## 2. Coexist and migrate data

During coexistence, old and new representations may both be present. The application MUST tolerate partially migrated data.

Use one or more of these patterns as required:

- dual-read with an explicit precedence rule;
- read-new with fallback-to-old;
- dual-write;
- write-new plus compatibility projection for old readers;
- idempotent synchronization or reconciliation.

Backfills MUST be:

- idempotent and safe to repeat;
- resumable after interruption;
- bounded or batched when volume can affect production;
- observable through counts, errors, and progress;
- safe while normal reads and writes continue;
- designed so rows already migrated and rows not yet migrated can coexist.

A backfill is not proof of cutover. Verify data parity and writer behavior separately.

## 3. Cut over

Switch readers and writers only after the expanded schema is available and the compatibility path is active.

A cutover plan MUST define:

- which version writes each representation;
- how old and new replicas coexist during a rolling deployment;
- the read precedence and fallback behavior;
- how duplicate or divergent writes are reconciled;
- how completion and parity are measured;
- what application rollback does while coexistence is active.

Producers of new states, job types, or values MUST NOT be enabled while old consumers can claim or reject them incorrectly. Use rollout-safe states, feature gates, or consumer capability boundaries.

## 4. Post-cleanup

Destructive or compatibility-breaking SQL belongs to a separate, explicitly identified post-cleanup migration.

Examples include:

- `DROP TABLE`, `DROP COLUMN`, or `DROP VIEW`;
- removing legacy values from constraints or enums;
- tightening nullability or constraints after a backfill;
- narrowing or incompatibly changing a column type;
- removing compatibility columns, triggers, projections, or indexes;
- deleting the old representation after a storage cutover;
- any SQL that would break an older application binary.

Post-cleanup may run only after all of the following are true:

- every live application and worker replica uses the new contract;
- old writers are disabled and no longer receive traffic;
- the backfill is complete and verified;
- parity and reconciliation checks pass;
- production observability shows no dependency on the legacy path;
- rollback to a version requiring the old contract is no longer required;
- the cleanup has its own rollback or recovery procedure.

Until those conditions are met, retain the old schema even when it appears unused.

## Rollback principle

Application rollback during expand, coexistence, or cutover MUST NOT require restoring a dropped schema object. If rollback would fail because a table, column, value, or compatibility path has already been removed, cleanup happened too early.

Database rollback is not assumed to be an instantaneous reverse migration. Prefer forward-compatible recovery and retained contracts over destructive reversals.

## Planning requirements

Every implementation plan that changes database schema or data ownership MUST state explicitly:

1. the pre-migration readers and writers;
2. the additive expand migration;
3. the coexistence strategy;
4. the backfill and its idempotency boundary;
5. rolling-deploy behavior with mixed binary versions;
6. cutover criteria and parity checks;
7. application rollback behavior;
8. deferred post-cleanup operations and their prerequisites.

A plan that says only “migrate data and drop the old table/column” is incomplete and MUST NOT be approved.

## Review checklist

Before approving or applying a migration, verify:

- Can the old binary run after the migration?
- Can old and new replicas read and write concurrently?
- Can the new binary read data that has not been backfilled yet?
- Can the backfill be stopped, resumed, and rerun safely?
- Are new states protected from old consumers?
- Is parity measurable before cutover?
- Can application code roll back without restoring deleted schema?
- Are all destructive statements and compatibility removals deferred to post-cleanup?
