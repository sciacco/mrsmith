# Grappa DCIM migspec - Phase C: UX and workflow map

Source audit: `apps/grappa-dcim/docs/GRAPPA-DCIM.md`

Status: draft, platform-neutral. This document describes user intent and workflow behavior only. It does not choose target components, routes, visual layout, or MrSmith implementation details.

## View and workflow catalog

| Source view | Primary user intent | Interaction pattern | Inputs and filters | Actions and outputs | Notes |
|---|---|---|---|---|---|
| `dc-build` | Maintain building/facility records. | CRUD registry. | Building name/address/status/portal flag/rack count/dates; source grid filters. | Create, view, update, dependency-gated cessation, dependency-gated delete. | `portale_clienti=1` means Customer Portal exposure. |
| `datacenter-sala-cage` | Manage non-MMR datacenter rooms/cages and their rack/port context. | CRUD plus map/data workspace. | Default active filter; datacenter attributes; `ismmr=0`. | Create/update, view rack grid, free/add ports, dependency-gated delete, cessation cascade, map equivalent. | Preserve user-visible map behavior, not legacy PHP file writes. |
| `datacenter-mmr` | Manage MMR records used in interconnect workflows. | CRUD plus MMR hub and interconnect context. | Default active filter; datacenter attributes; `ismmr=1`; free-form path identifier `mmr_type`. | Create/update, view MMR hub, navigate to sale/isole with MMR context, cessation cascade, map equivalent. | `mmr_type` is not an enum. |
| `racks` | Manage physical rack inventory, U-space, power, positions, and media. | Data workspace with CRUD detail, move, and U-map. | Default active filter; rack identity, location, customer, U height, position, vertical `A/B` for half rack, power/circuit fields. | Create units/sockets, move, update, cease, dependency-gated delete, upload unit media, view apparati/socket lists. | `A/B` means high/low vertical position, not side/lato. |
| `rack-sockets` | Track rack power socket/PDU inventory and report power readings/history. | CRUD registry plus report. | Socket rack, breaker, SNMP/OID, detector IP, positions, status. | Create/update/delete sockets, report power. | Polling/alerting out of V1. |
| `apparato` | Manage racked devices and generated ports. | CRUD with type-specific side effects. | Default active filter; name/type/rack/U/customer/order/port defaults/device metadata. | Create/update/delete, generate NICs, server redirect, firewall escalation contacts, cease cascades. | Device taxonomy and monitoring semantics need confirmation. |
| `server` | Maintain detailed physical/virtual server configuration. | CRUD/detail aggregate. | Customer, hostname/name, OS, rack, status, order code, hardware, virtualization, credentials, related child records. | Create physical/virtual, update with apparato sync, view child cards/apps/services/ports. | Credential compatibility is a hard contract. |
| `storage` | Track storage allocations and close/archive them. | CRUD registry with archive action. | Customer, apparato, status, protocol, size/unit, order, serial. | Create/update, dependency-gated delete, archive to `Chiuso`. | No automatic billing side effects in V1. |
| `plenums` | Manage cable pathway structures and slots. | CRUD plus 288-cell visual connection map. | Plenum name/isle/type/datacenter/status. | Create/update/delete when unlinked, initialize 24-slot matrix, view 288 calculated fiber cells. | Missing slots render as incomplete configuration. |
| `anelli-fibra` | Manage fiber ring topology and KML map metadata. | Topology CRUD/detail. | Ring name, customer, node count, order/serial, note, KML, status. | Atomic create, upload KML, auto-create nodes/arcs, increase node count, dependency-gated delete or cease. | Decrease node count unsupported. Hive sync is V2. |
| `xcon` | Manage sold customer cross-connect circuits and optional path hops. | Workflow/state registry plus detail editor. | Active/ceased tabs; status, type, A/Z endpoints, racks, LOA, MMR port, order fields, source. | Create/update Xcon and optional hops; status options constrained by source controller. | Must not mutate inventory resources. |
| `kitgraph-kitview` | Residual TIM GEA kit report. | V2 investigation only. | N/A for V1. | None in V1. | Do not implement in V1. |
| `cwdm` | Residual CWDM optical workflow. | V2 investigation only. | N/A for V1. | None in V1. | Do not implement in V1. |
| `dcimadmin-cable` | Administer cable bundles and individual fiber-port assignments. | CRUD/detail plus assignment workflow. | Cable name/description/fiber count/status; fiber left/right ports. | Create cable and fibers, atomic fiber-port assignment, lookup available transit fibers, dependency-gated delete. | Concurrent assignment conflicts are rejected. |
| `dcimadmin-cam` | Maintain datacenter camera inventory. | Simple create/update registry. | Code/model/brand/position required; IP/status/serial optional; IP validates if present. | Create, update. | No delete action documented; no uniqueness in V1. |
| `islets` | Manage physical islets/rows/sides inside datacenters. | Admin CRUD. | Datacenter, name, rack count, type, floor, serial, order, optional customer. | Create/update/delete when no occupied positions. | Occupied islet delete is blocked. |
| `positions` | Manage rack positions and batch layout generation. | Admin CRUD plus batch creation. | Status, type, number, islet; optional rack assignment during update. | Create/update/delete when free, assign rack, batch create 1..N positions and map equivalent. | Rack move is explicit; `quarter` is legacy-tolerated only. |

## Cross-view workflows

### Facility hierarchy lifecycle

Source behavior:

1. Building groups datacenters.
2. Datacenter records are split into Sala/Cage and MMR by `ismmr`.
3. Datacenter cessation cascades to verified child resources: racks, apparati, NICs, and optical cassettes.

Target V1 parity:

- Preserve default active filters and `Attivo`/`Cessato` semantics.
- Preserve cascade breadth exactly as verified; do not add page-only plenum/port cascades without confirmation.
- Preserve equivalent maps for datacenter and islet/rack layout.
- Preserve `viewmmr` as an MMR hub: MMR -> sale in same building -> islets -> `viewisle` with MMR context.
- `portale_clienti=1` means Customer Portal exposure.

### Rack layout and occupancy

Source behavior:

1. Islets group positions.
2. Position batch creates numbered free/full positions.
3. Rack create assigns a position, generates U rows, and may generate socket rows.
4. Rack detail renders U-space from `units` and occupied apparati.
5. Rack media attaches front/back images to units.

Target V1 parity:

- Preserve position occupancy and rack U-map user outcome.
- Preserve unit media with existing referenced media.
- Preserve free-text rack/position types and statuses.
- Delete occupied positions/islets is blocked.
- Rack move is explicit and must free old position, occupy new position, and reject conflicts.
- `Full` rack uses a full position; `Half` rack uses vertical `A` high or `B` low.

### Equipment, server, and storage workflow

Source behavior:

1. Apparato create can generate NICs.
2. Apparato type `Server` leads into server create.
3. Physical server updates sync selected fields back to the linked apparato.
4. Storage has hard delete and separate archive to `Chiuso`.

Target V1 parity:

- Keep apparato/NIC generation and physical server sync.
- Preserve encrypted server credential compatibility while coexisting with legacy Grappa.
- Keep archive distinct from delete for storage.

Gaps:

- `pwd_utenza_cliente` encryption/storage behavior must be validated during implementation.
- Active storage status default should be selected from live/current DB values before implementation.

### Cabling, ports, and cross-connects

Source behavior:

1. Plenums, slots, ports, cables, and fibers document physical cabling.
2. Cable create generates child fibers.
3. Fiber update clears old port assignments and writes new assignments.
4. Xcon is an operational circuit/path registry over A/Z endpoints and optional hops.
5. Active Xcon persistence does not mutate ports, cables, racks, or fibers.

Target V1 parity:

- Keep cable/fiber generation and assignment semantics.
- Preserve port lifecycle values.
- Keep Xcon side-effect-free outside `xcon` and `xcon_hop`.
- Cable delete is blocked unless all fibers are `Libera` and unassigned.
- Fiber assignment is atomic and rejects concurrent double assignment.
- Plenum maps use 288 calculated cells; occupancy comes from `ports`, not `pl_slots.status`.

### Optical and ring workflow

Source behavior:

1. Fiber ring create auto-builds topology nodes and arcs.
2. KML metadata/files are preserved.
3. CWDM source behavior exists in the audit but is excluded from V1.

Target V1 parity:

- Preserve generated ring topology behavior.
- Preserve KML history; defer Hive sync to V2.
- Do not treat `cassetti_ottici` as active V1 workflow.
- Do not implement CWDM in V1; investigate later before implementation.

## Empty, error, confirmation, and navigation expectations

Evidence explicitly confirms some messages and redirects but not a full UX state matrix. V1 should define equivalent states during implementation planning while preserving these source-level constraints:

- Destructive deletes that remain available require double confirmation.
- Validation errors must block creation/update for required fields documented by the audit.
- Free-text status/type values unknown to picklists must remain visible and round-trip safe.
- Long-running/generated operations such as fiber creation, ring topology generation, media uploads, KML uploads, and approved exports need explicit success/failure handling.
- Partial-failure behavior is not consistently documented; target contracts should avoid silent partial writes where source lacks transaction evidence.

## Views to merge, split, remove, or redesign

Remove from V1:

- Dead legacy DCIM features not listed in the audit.
- Active first-class `cassetti_ottici` workflow.
- TIM GEA kit report and CWDM.
- Rack power polling/alerts and Hive KML sync.

Redesign while preserving user outcome:

- Legacy dynamic PHP map file generation becomes equivalent target map/layout behavior.
- Rack media upload preserves media records and images, not necessarily the exact image pipeline.
- XLS/KML/download behavior preserves user-visible artifacts, not exact filesystem paths.
- MMR hub, rack map, and `viewisle` preserve behavior and context, not static PHP map files.

Keep distinct in product scope unless later approved:

- Sala/Cage and MMR can share underlying entity logic, but the user distinction and filters must remain explicit.
- Xcon must remain separate from cable/fiber mutation workflows because source Xcon does not write inventory resources.
- Admin support screens `islets` and `positions` remain first-class because rack workflows depend on them.
