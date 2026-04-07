# Project TODOs

## Developer Experience

### Single-Origin Dev Gateway
Future implementation plan is tracked in [docs/DEV-GATEWAY-IMPLEMENTATION-PLAN.md](DEV-GATEWAY-IMPLEMENTATION-PLAN.md). This work would replace browser-visible per-app localhost ports with a backend-owned single-origin dev gateway while preserving independent app Vite servers as opt-in processes.

## Budget Management App

### Audit Logging
Audit logging for budget and approval rule changes (create, edit, delete) is not in scope for the budget app migration. It needs a separate implementation — either server-side middleware on the Arak API or a dedicated audit service. No client-side audit logging exists in the current Appsmith app either.

### Budget Year-End Lifecycle
Unresolved: what happens to budgets at year-end? Do old-year budgets get archived, copied forward, or accumulate indefinitely in the list? This affects long-term UX of the budget list view. Needs a decision from the domain owner before the budget app grows historical data.
