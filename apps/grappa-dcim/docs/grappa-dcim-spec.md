# Grappa DCIM - Migration Specification

Downstream input for `portal-miniapp-generator`. This spec is self-contained at the product/behavior level and should not require the original Yii2 source or reverse-doc bundle.

## Summary

- Application name: Grappa DCIM
- Audit source: `apps/grappa-dcim/docs/GRAPPA-DCIM.md`
- Spec status: domain decisions recorded for V1; ready for repo-fit planning.
- Scope directive: V1 parity for approved active DCIM migration surfaces, with residual/abandoned surfaces deferred as recorded below.
- Phase documents:
  - `grappa-dcim-migspec-phaseA.md` - scope and parity boundary
  - `grappa-dcim-migspec-phaseB.md` - entity and operation model
  - `grappa-dcim-migspec-phaseC.md` - UX and workflow map
  - `grappa-dcim-migspec-phaseD.md` - logic, data, and integration contracts

## Current-state evidence

The audit is a standalone legacy handoff. It covers reverse-engineered behavior from a Yii2/PHP Grappa application backed by MySQL DDL evidence. The runnable legacy application source is not present in this repository. Source paths in the audit are provenance only.

The wider Grappa database structure is documented in `docs/grappa/GRAPPA.md` and linked `grappa_*.json` files. This should be used during later validation for table, column, key, and relationship details. For authorization, the schema documents legacy structures such as `auth_item`, `auth_assignment`, `auth_item_child`, `auth_rule`, `user_grappa`, `profile_grappa`, `userprofile_grappa`, and `access_grappa`; these are evidence/mapping aids, not an approved target RBAC model.

Evidence status highlights:

- Active DCIM inventory/workflow surfaces: 15.
- Required admin support pages: `islets` and `positions`.
- Product review excludes `kitgraph-kitview` and `cwdm` from V1 despite audit coverage.
- DDL evidence: present.
- Original Yii2 source in this checkout: not present.
- Legacy authorization: intentionally out of target scope.
- Xcon source of truth: verified as `xcon` plus optional `xcon_hop`.
- Camera screen: verified in scope despite older conflicting notes.
- `rack_sockets`: authoritative table name; `rack_power_sockets` is terminology drift.

## In-scope behavior

V1 includes these source surfaces:

| Source slug | Behavior |
|---|---|
| `dc-build` | Building/facility registry. |
| `datacenter-sala-cage` | Non-MMR datacenter room/cage management, rack context, maps, port operations. |
| `datacenter-mmr` | MMR management and interconnect context. |
| `racks` | Rack CRUD, U-space map, power metadata, position occupancy, media. |
| `rack-sockets` | Rack PDU/socket inventory and power history/report context. |
| `apparato` | Equipment inventory, NIC generation, server/firewall side effects. |
| `server` | Physical/virtual server inventory and detailed child records. |
| `storage` | Storage allocation CRUD and archive. |
| `plenums` | Cable pathway/plenum-slot inventory and visual map. |
| `anelli-fibra` | Fiber ring topology, node/arc generation, KML metadata. |
| `xcon` | Cross-connect registry and optional ordered hops. |
| `dcimadmin-cable` | Cable/fiber admin and fiber-port assignment. |
| `dcimadmin-cam` | Camera create/update inventory. |
| `islets` | Islet/row/side admin. |
| `positions` | Position admin, rack assignment, batch creation. |

## Out-of-scope behavior

- Dead legacy DCIM menu/features not listed above.
- First-class active `cassetti_ottici` workflow. Preserve as dependency/archive data only.
- Recreating legacy authorization.
- Rack power device polling, polling cadence, and alerting.
- Hive upload/sync for KML maps.
- TIM GEA kit report (`kitgraph-kitview`). Current review treats the source data as residual; redesign/investigation is V2.
- CWDM. Current review treats the feature as likely abandoned/residual; investigation before any implementation is tracked in `docs/TODO.md`.
- Schema/domain cleanup that renames source tables/fields or normalizes free-text values.
- Safer delete/archive redesign beyond the V1 double-confirmation rule, unless approved later.
- Target routes, component selection, Go package structure, Vite configuration, deployment, or other MrSmith implementation planning.

## Recorded target deviations and parity notes

Recorded target deviations from exact source behavior:

- New role-based access model replaces legacy authorization with two V1 roles: Viewer and Operativo.
- Viewer can read approved non-secret DCIM data only.
- Operativo can read/write approved V1 surfaces, execute lifecycle/archive actions, hard-delete where permitted, and view/update encrypted server credential fields.
- Destructive hard deletes that remain available in V1 require double user confirmation and are allowed only when the record has no active operational dependencies.
- Generated PHP map files are not reproduced literally; V1 preserves equivalent user-visible map/layout behavior.
- Rack media, KML, and approved export artifacts preserve user outcome and historical artifacts where referenced, not exact legacy filesystem mechanics.
- Rack power OID fields and historical data are preserved, but polling/alerts are not V1.
- Hive KML sync is V2.
- UI picklists may offer known values, but unknown stored free-text values must remain visible and round-trip safe.

## Entity catalog

### Physical hierarchy

| Entity | Source keys | Operations | Critical relationships/contracts |
|---|---|---|---|
| `dc_build` | `id` | list, view, create, update, delete | Parent building for datacenters; `status` `Attivo`/`Cessato`; transition to `Cessato` and delete are blocked while active dependencies exist; `portale_clienti=1` means Customer Portal exposure. |
| `datacenter` | `id_datacenter` | list/filter, create, view, update, delete, port ops | `ismmr=0` Sala/Cage, `ismmr=1` MMR; `mmr_type` is a short MMR path identifier, not an enum; `portale_clienti=1` means Customer Portal exposure; cessation cascade verified for racks/apparati/NICs/optical cassettes. |
| `islets` | `(id, datacenter_id)` | CRUD | Type `isle`/`row`/`side`; delete is blocked if any child position is occupied. |
| `positions` | `(id, islets_id)` | CRUD, batch, rack assignment | Status `free`/`occupied`/possibly `reserved`; batch creates `free/full`; delete is blocked when occupied; rack moves must be explicit. |
| `racks` | `id_rack` | CRUD, move, cease, media | Creates `units`, updates position, creates sockets; `Full` racks occupy a full position with `pos=F`; `Half` racks use vertical position `A` high or `B` low; cessation cascades child equipment/NICs/optical cassettes/sockets/position. |
| `units` | `id` | generated/viewed indirectly | One per rack U; rack map and unit media depend on it. |
| `media` | `id` | rack unit media update | Preserve referenced front/back images and media records. |

### Equipment, compute, and storage

| Entity | Source keys | Operations | Critical relationships/contracts |
|---|---|---|---|
| `apparato` | `id_apparato` | CRUD, cease | Create with ports generates `nic`; type values remain legacy/free-text with picklist from DB; only proven types get side effects; no automatic NIC regeneration on update; cessation cascades NICs. |
| `nic` | `id_nic` | generated/updated indirectly | Used by apparato, historical/report contexts, and cascades; preserve statuses and link fields. CWDM usages are out of V1. |
| `server` | `id_server` | list, view, create physical/virtual, update | Physical server syncs customer/order/serial to apparato; Operativo can view/update sensitive credential fields; proven encrypted fields must preserve legacy `k_crypt` compatibility. |
| `server_schede`, `server_applicazioni`, `server_servizi`, `server_porte` | child IDs | child detail views | Preserve as server aggregate children. |
| `storage` | `(id, cli_fatturazione_id, apparato_id_apparato)` | CRUD, archive | Archive is preferred closure and sets `status='Chiuso'` plus `closed_at`; delete only when no operational/billing dependencies are known; closed records are read-only. |
| `cli_contatti_escalation` | `id` | written by firewall flow | Preserve firewall escalation side effect. |

### Power, cabling, ports, and cross-connects

| Entity | Source keys | Operations | Critical relationships/contracts |
|---|---|---|---|
| `rack_sockets` | `id` | CRUD, report context | Authoritative table; rack cessation sets `Spento`; readings reference sockets. |
| `rack_power_readings`, `rack_power_daily_summary` | `id` | history/report | Preserve historical data; polling out of V1. |
| `plenums` | `(id, datacenter_id)` | CRUD, visual map | Visual capacity is 288 fiber cells: 2 cables x 12 termination points x 12 fibers; plenum create does not implicitly create slots. |
| `pl_slots`, `slots`, `ports` | composite keys | linked by plenum/port workflows | `pl_slots` represent 24 termination points per initialized plenum; fiber-cell occupancy is calculated from `ports.pl_slots_id` and `ports.pl_port_num`; port statuses `Empty`, `Linked`, `Used`, `Xcon`. |
| `cables` | `id` | create/list/view/delete when unused | Create generates fibers 1..N; delete only when all fibers are free and unassigned. |
| `fibers` | `id` | atomic update assignment | New fibers `Libera`; assigned fibers become `Occupata`; update clears old ports and sets new port links atomically with conflict checks. |
| `xcon` | `id` | list active/ceased, create/update | Source of truth for cross-connects; mutates only `xcon` and `xcon_hop`. |
| `xcon_hop` | `id` | optional ordered hops | Intermediate path data ordered by `hop_num`; A/Z endpoints live on `xcon`. |

### Fiber rings, optical devices, cameras, and reports

| Entity | Source keys | Operations | Critical relationships/contracts |
|---|---|---|---|
| `anelli_fibra` | `id_anello` | CRUD/topology | Create sets `Attivo`, optional KML, generates N nodes and N circular arcs. |
| `nodi`, `archi`, `archi_tratta` | child IDs | generated/updated topology | Preserve ring topology and route detail semantics. |
| `mappa_tracciati_anelli` | `id` | KML metadata | Preserve history; Hive sync V2. |
| `cwdm` | `id_cwdm` | V2 investigation only | Audit-proven source surface but excluded from V1 as likely abandoned/residual. |
| `cams` | `id` | list, create, update | Standalone inventory; no delete documented; no uniqueness enforcement for `code`, `ipaddr`, or `serial`; `ipaddr` must be a valid IP when provided. |
| `eth`, `foglio_linee` | source IDs | V2 report investigation only | TIM GEA report source data is residual; `kitgraph-kitview` excluded from V1. |
| `cassetti_ottici` | `id_cassetto_ottico` | dependency only | Historical/archive dependency and cascade participant, not active V1 workflow. |

## View and workflow specifications

### Buildings

- User intent: maintain top-level facilities.
- Pattern: CRUD registry.
- Source behavior: grid/index with filters and inline create evidence; view/update/delete verified.
- V1 contract: building CRUD intent is in scope. `status` can move to `Cessato` only when no active datacenters/MMRs, racks, apparati, services, or Customer Portal-exposed references depend on it. When cessation is allowed, set `ceased_at` if empty. Delete only without dependencies and with double confirmation.

### Sala/Cage and MMR

- User intent: manage datacenter rooms/cages and MMRs.
- Pattern: CRUD plus rack/interconnect context, physical maps, and MMR hub navigation.
- Source behavior: active filters, `ismmr` split, dynamic map files, port free/add operations, datacenter cessation cascade.
- V1 contract: preserve active filters, map outcome, port operations, MMR distinction, and verified cascade breadth. `mmr_type` is a short MMR path identifier, not an enum. `portale_clienti=1` means expose through Customer Portal. Delete only without active dependencies; otherwise use `stato=Cessato`.
- Physical visualization contract: preserve `viewmmr` as an MMR hub, sale/cage rack maps, and `viewisle` plenum/fiber connection maps as behavior. Do not reproduce PHP map files literally.
- Implementation validation: map reports reference `crossconnects` for visual indicators; investigate this for maps without changing the V1 workflow source of truth (`xcon`/`xcon_hop`).

### Racks, islets, and positions

- User intent: manage physical rack layout and occupancy.
- Pattern: data workspace plus admin support CRUD.
- Source behavior: rack create generates U rows and socket rows, updates positions, shows U-map and media; position batch creates maps; islet delete deletes positions.
- V1 contract: preserve rack U-map, unit/media behavior, position occupancy, batch creation block-if-existing rule, and destructive delete double confirmation. Delete occupied positions/islets is blocked. Rack move is explicit: free old position, occupy new position, and reject conflicts.
- Half-rack contract: `Full` racks occupy a full position and use `pos=F`; `Half` racks use vertical position `A` high or `B` low. At most one `A` and one `B` half rack can share the same physical position. Do not call A/B "side" or "lato".

### Equipment, servers, and storage

- User intent: manage device inventory, detailed server data, and storage allocations.
- Pattern: CRUD/detail aggregates with side-effectful create/update.
- Source behavior: apparato generates NICs; server physical update syncs apparato; selected server passwords are encrypted; storage archive sets `Chiuso`.
- V1 contract: preserve generated NICs on create, physical server sync, credential compatibility, and archive-vs-delete distinction. Device types remain legacy/free-text with picklist from current DB values; monitoring fields are preserved but do not trigger polling/alerts. Operativo can view/update encrypted server credentials; Viewer cannot.
- Storage contract: archive is preferred and sets `Chiuso`; hard delete only without known operational/billing dependencies. No automatic billing side effect in V1.

### Plenums, cables, fibers, and ports

- User intent: manage physical cabling and port/fiber assignments.
- Pattern: admin CRUD plus connection assignment workflow.
- Source behavior: plenum visual map; cable create generates fibers; fiber update rewires port links; datacenter port ops set exact port statuses.
- V1 contract: preserve generated fibers, port/fiber assignment semantics, port statuses, and in-use values. Cable delete only when all fibers are `Libera` and unassigned. Fiber assignment is atomic and rejects concurrent double assignment.
- Plenum contract: view is 288 calculated fiber cells. Create plenum creates only `plenums`; an explicit initialize-matrix action creates missing `pl_slots` for cable 1/2 and `num=1..12`. Missing slots render as incomplete configuration, not as free fibers.

### Cross Connect

- User intent: manage sold customer cross-connect circuits and path documentation.
- Pattern: workflow state registry with optional ordered hops.
- Source behavior: active tab is `stato != 'cessata'`, ceased tab is `stato='cessata'`; `annullato` is terminal but appears in active-tab query; `tipo` is product selector; `sorgente` usually `CustomerPortal` or `AssetManager`.
- V1 contract: preserve exact status values, active/ceased query semantics, product selector values, A/Z endpoint fields, LOA/MMR fields, optional hop order, and no inventory side effects.
- Field labels: `ticket_esteso` -> `Ticket Esteso`, `num_ordine` -> `Codice Ordine`, `riga_ordine` -> `Serial Number`. `xcon.tipo` shows raw `CDL-X*` codes with a legacy picklist and tolerance for unknown values.

### Fiber rings

- User intent: manage optical ring topology.
- Pattern: topology CRUD with generated child records.
- Source behavior: rings generate nodes/arcs; distance and attenuation default to `0`; node count can increase but not decrease.
- V1 contract: create is atomic and generates N nodes plus N circular arcs. Increasing node count is allowed; decreasing is blocked. Distance and attenuation are manually editable, with no automatic calculation. Hard delete only for rings without meaningful operational data, KML, routes, coordinates, or references; otherwise use `stato=Cessato`.

### Cameras

- User intent: maintain security camera inventory.
- Pattern: simple create/update registry.
- Source behavior: required app-level fields `code`, `model`, `brand`, `position`; optional IP/status/serial; no delete documented.
- V1 contract: implement create/update inventory only; do not infer NVR/DVR integration. No uniqueness enforcement on `code`, `ipaddr`, or `serial`; validate `ipaddr` as IP when provided.

## Product-level API contract summary

This spec does not prescribe endpoint paths. Any target API must provide behavior-equivalent capabilities:

- List/search active and historical inventory with source filters and pagination behavior where relevant.
- Read/create/update/archive/cease/delete operations for in-scope entities according to source contracts.
- Explicit lifecycle operations for cascades instead of relying on client-side multi-step writes.
- Explicit generated-child operations that are atomic or define partial failure behavior.
- Report/history operations for power history context.
- Artifact operations for rack media, datacenter/islet/MMR maps, plenum/fiber maps, and KML metadata/files.
- Permission-gated destructive actions with double confirmation.
- Credential-safe server detail operations compatible with legacy encrypted data.

## Logic allocation

Backend/domain-owned:

- Cascades, generated child rows, dependency checks before delete/cessation, credential compatibility, storage archive, Xcon status availability, cross-entity sync, composite-key mutation, artifact persistence, and destructive-action confirmation enforcement.

Frontend/shared-owned:

- User interaction, validation hints, warning/confirmation UI, map/U-space/topology/report presentation, client-side duplicate-submit prevention, and round-trip display of unknown free-text values.

Implementation/security allocation:

- Exact target access/action logging requirements replacing source `ToolGr::RegAcc()`.

## Integrations and side effects

| Item | Contract |
|---|---|
| Legacy data compatibility | Preserve source table/field/value semantics during migration and coexistence. |
| Server credentials | Preserve `k_crypt` compatibility for proven encrypted fields. |
| Generated maps | Preserve user-visible map/layout behavior; exact PHP file generation is not required. |
| Rack media | Preserve existing referenced images and front/back media update behavior. |
| KML files | Preserve metadata/files for history; Hive sync V2. |
| TIM GEA XLS | Out of V1; report requires V2 redesign/investigation. |
| Rack power readings | Preserve fields/data; polling/alerts V2/out of scope. |
| Customer portal flags | `portale_clienti=1` means the record should be exposed through Customer Portal. Consumers must respect the flag. |

## Risky contract list

Candidate validation checks for implementation planning:

1. Default active filters for datacenter, racks, and apparato.
2. `rack_sockets` mapping and power reading FK.
3. Exact lifecycle cascades for datacenter, rack, and apparato.
4. Rack create generates exact `units` count and socket side effects.
5. Position batch blocks if positions already exist.
6. Rack assignment and explicit rack move update position occupancy without double-booking.
7. Apparato port generation creates sequential NICs.
8. Physical server update syncs selected fields to apparato.
9. Server credential fields remain compatible with legacy encrypted values.
10. Storage archive sets `Chiuso` and preserves row.
11. Cable create generates requested `Libera` fibers.
12. Fiber update clears old port links and sets new links.
13. Xcon writes only `xcon`/`xcon_hop`.
14. Xcon statuses, terminal states, and active/ceased tab semantics are exact.
15. Fiber ring create produces N nodes and N circular arcs.
16. Camera create/update exists; delete is not assumed.
17. Plenum map calculates 288 cells from `pl_slots` plus `ports`.
18. MMR hub and `viewisle` preserve MMR context and physical connection state.
19. Destructive deletes require double confirmation and no active dependencies.

## Approved decisions and deferred investigations

Approved V1 product decisions:

1. Roles: V1 has only Viewer and Operativo. Viewer is read-only. Operativo can read/write, perform allowed hard deletes, and view/update encrypted server credentials.
2. General delete safety: hard delete is allowed only with double confirmation and only when the record has no active operational dependencies. Otherwise use lifecycle/archive where available.
3. Occupied islets/positions: delete is blocked. Edits that cause layout conflicts are blocked. Rack move must be explicit.
4. Rack half positions: `A` means high and `B` means low vertical position. These are not lateral sides.
5. TIM GEA kit report: out of V1; redesign/investigation is V2.
6. Cameras: no uniqueness enforcement on `code`, `ipaddr`, or `serial`; validate `ipaddr` as IP when provided.
7. Xcon labels and product codes: approved field labels are listed above; `xcon.tipo` remains raw code.
8. CWDM: out of V1; investigate before any implementation.
9. Plenums: preserve 288-cell visual model, explicit matrix initialization, and delete blocking when ports are linked.
10. Storage: archive is preferred; delete only without dependencies; no billing side effects in V1.
11. Datacenter/MMR: `mmr_type` is a path identifier; `portale_clienti=1` means Customer Portal exposure; MMR hub and map context are in scope.
12. Fiber rings: create/update/delete policy approved as described above.
13. Apparati: legacy/free-text type values, proven side effects only, monitoring fields preserved without polling/alerts.
14. `dc_build`: cessation and delete are blocked while active dependencies exist.

Implementation validation notes:

- Verify whether map-only `crossconnects` references are still active, historical, or reconcilable with `xcon`/`xcon_hop`.
- Verify whether `mmr_slots` is a real table or legacy relation/model naming over `slots`.
- Verify the most common active `storage.status` value before choosing default for new records.
- Validate `pwd_utenza_cliente` storage/encryption behavior during implementation.
- Verify `quarter` position values in production data; tolerate legacy values but do not add V1 creation/edit support for quarter positions.

## Handoff rule

This spec can be handed to `portal-miniapp-generator` for MrSmith repo-fit planning, archetype selection, UI review gates, route/base-path/API-prefix decisions, and implementation planning.
