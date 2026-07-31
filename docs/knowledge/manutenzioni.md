# Manutenzioni

Knowledge entries specific to `apps/manutenzioni`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Manutenzioni Radar Excludes Terminal Maintenance States

- Context: `GET /api/manutenzioni/v1/maintenances/radar`, used by `apps/manutenzioni` on the Registro Manutenzioni page.
- Discovery: maintenance records with status `cancelled` or `superseded` are lifecycle history, not actionable operational windows. Counting them in the radar makes the upcoming-window buckets noisy and misleading.
- Practical rule: Manutenzioni radar-style operational summaries must always exclude `cancelled` and `superseded`, even when the caller passes explicit status filters. Keep the full register/list endpoints available for searching those terminal records.
- Evidence: `backend/internal/manutenzioni/read.go` `handleMaintenanceRadar`; regression coverage in `backend/internal/manutenzioni/radar_test.go`.
- Used by: `apps/manutenzioni` Registro Manutenzioni radar.
- Open questions: none.

### Manutenzioni Service Taxonomy Is A Catalog, Targets Are Instances

- Context: `apps/manutenzioni` impact modeling, create/detail flows, service-dependency graph, and configuration pages.
- Discovery: `maintenance.service_taxonomy` is not a generic label lookup. It is the stable catalog of maintainable services/objects, with an owning technical domain, target nature, default audience, and dependency-graph participation. `maintenance.maintenance_service_taxonomy` records how a catalog item participates in one maintenance (`operated` or `dependent`, expected severity, audience override, source). `maintenance.maintenance_target` is the concrete per-maintenance instance layer and can exist without a catalog item.
- Practical rule: do not create `service_taxonomy` rows for punctual instances such as a specific node, tenant, circuit, room, or customer asset. Model those as `maintenance_target` rows with `target_type_id`, `display_name`, and optional `service_taxonomy_id`. `maintenance_target.target_type_id` does not have to equal the linked catalog item's `target_type_id`, because the target may be a subpart or instance of the catalog object. Treat `ref_table`, `ref_id`, and `external_key` as optional external anchors, not guaranteed typed FKs or user-facing labels.
- Evidence: `docs/manutenzioni_schema.sql` tables `service_taxonomy`, `maintenance_service_taxonomy`, `service_dependency`, and `maintenance_target`; backend validation in `backend/internal/manutenzioni/mutations_impact.go`; read/API shape in `backend/internal/manutenzioni/reference.go` and `children_read.go`; frontend impact workbench in `apps/manutenzioni/src/components/ImpactWorkbench.tsx`; detailed rulebook in `apps/manutenzioni/SERVICE-TAXONOMY-REFERENCE.md`.
- Used by: `apps/manutenzioni` create page, maintenance detail impact workspace, target management, cockpit readiness, service-dependency configuration.
- Open questions: none for the current model; a future registry may type allowed external reference sources.
