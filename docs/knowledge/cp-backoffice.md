# CP Backoffice

Knowledge entries specific to `apps/cp-backoffice`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### CP Backoffice Active Biometric Users Are Balance-Based

- Context: `apps/cp-backoffice` Accessi biometrici PDF export and any future biometric active-user report.
- Discovery: an active biometric user is not simply the latest completed row or the hidden Lenel flag. The business rule is the per-user completed-request balance: completed `activation` count minus completed `deactivation` count must be greater than zero.
- Practical rule: group `customers.biometric_request` by `user_struct_id`, filter `br.request_completed IS TRUE`, and include only users where activation/deactivation balance is `> 0`. Use the biometric user (`user_struct_id`) as the requester; report the latest completed activation confirmation date when present.
- Evidence: product clarification during CP Backoffice PDF export implementation; Mistra `customers.biometric_request` request type enum values `activation` and `deactivation`.
- Used by: `apps/cp-backoffice` Accessi biometrici active-users PDF export.
- Open questions: none.
