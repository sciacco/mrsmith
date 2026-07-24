# Database Migration Compatibility Rule

## Purpose

A database change must let the application currently in production continue operating while the new code is tested against the same production database.

For this repository, the normal workflow is simple:

1. apply a backward-compatible schema change;
2. keep the current production application running;
3. test the new build in development against the production database;
4. deploy the new build after validation;
5. perform destructive cleanup separately, only if it is still useful.

Do not design for replicas, rolling deployments, long-lived version coexistence, dual writes, or synchronization unless the actual deployment requires them.

## Primary rule: expand without breaking the current application

The migration needed to test a feature must be additive and compatible with the application already running.

Typical safe changes are:

- adding nullable columns;
- adding a table without removing the previous storage;
- adding a safe index;
- widening a constraint when the current application tolerates the new values;
- adding optional data that old queries can ignore.

Before applying the migration, verify that the current application can continue to read and write normally. In particular, check actual SQL scans, constraints, triggers, and consumers rather than assuming that an additive statement is harmless.

Do not rename or remove an object as part of the migration required to test new code. Add the replacement and defer cleanup.

## Testing against the production database

The current production application and the development build may use the expanded schema at the same time for a short validation window.

The minimum requirements are:

- the production application remains operational;
- the development build can read existing rows;
- writes made by the development build do not invalidate the current application's contract;
- the test can be stopped without reversing the schema migration;
- test actions that affect real data are limited to agreed records or workflows.

Perfect feature parity between the current UI and the development UI is not required during this short test. Temporary visibility differences are acceptable when they do not block users, corrupt data, or hide an operationally relevant change unexpectedly.

No feature flag or parallel deployment is required when the developer runs the new build directly in development.

## Data migration and cutover

Do not add a backfill merely because the schema changed. Use one only when the accepted feature needs existing data in the new representation.

Prefer the simplest sufficient operation:

- one idempotent `INSERT … SELECT … ON CONFLICT`;
- one `UPDATE` limited to rows where the new value is missing;
- a small, explicit conversion performed at cutover.

Batching, progress tracking, reconciliation jobs, dual reads, and dual writes are not defaults. Add them only when justified by concrete data volume, runtime, or concurrency risk.

A normal cutover is:

1. validate the development build;
2. deploy the accepted code;
3. run the required idempotent backfill, if any;
4. stop writing the legacy representation;
5. retain the old schema until cleanup is demonstrably safe.

If the test is rejected, stop the development build. The additive schema may remain unused; removing it immediately is usually unnecessary.

## Rollback

The previous application version must remain compatible with the expanded schema.

Rolling back application code must not require recreating a table or column removed by the feature migration. This is the main reason destructive SQL is deferred.

Do not require perfect rollback of test data unless the feature has a specific business need for it. Prefer soft deletion or explicit correction of test records over a database-wide reverse migration.

## Post-cleanup

Compatibility-breaking SQL belongs in a later, separate migration. Examples include:

- `DROP TABLE`, `DROP COLUMN`, or `DROP VIEW`;
- making a new column `NOT NULL` after population;
- removing legacy values or constraints;
- narrowing a type;
- deleting the old representation after a storage change.

Cleanup is optional. Perform it only after the new code is established, the old path is no longer used, and rollback no longer depends on it. Keeping an unused legacy object temporarily is safer than complicating the feature rollout.

## When more elaborate coexistence is justified

Use patterns such as dual-read, dual-write, synchronization, batched backfills, capability gates, or parity monitoring only when the repository actually has one of these conditions:

- multiple application versions serving traffic concurrently;
- workers that may run old code for an extended period;
- a large backfill that cannot complete safely in one operation;
- independent systems writing both representations;
- a business requirement for old and new interfaces to show identical data during validation.

The implementation plan must name the concrete condition. Do not introduce these mechanisms as generic migration ceremony.

## Planning questions

A database-related implementation plan should answer only what is relevant:

1. What schema does the current production application require?
2. What additive migration allows the new build to be tested safely?
3. What production data may the development test read or modify?
4. Is a backfill needed at acceptance, and can it be idempotent?
5. Which destructive operations, if any, are deferred to post-cleanup?

## Review checklist

Before applying a migration, verify:

- Can the current production application still read and write?
- Can the new build handle rows created before the migration?
- Can the development test be stopped without repairing the schema?
- Are production test writes controlled and understandable to active users?
- Is any backfill actually necessary and as simple as the data permits?
- Are destructive statements excluded from the feature migration?
- Is every coexistence mechanism justified by the real deployment rather than a hypothetical one?
