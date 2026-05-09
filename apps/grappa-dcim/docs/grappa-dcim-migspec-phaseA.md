# Grappa DCIM migspec - Phase A: scope and parity boundary

Source audit: `apps/grappa-dcim/docs/GRAPPA-DCIM.md`

Status: draft, extracted from the legacy audit. This phase is implementation-neutral and does not choose target routes, components, packages, or deployment shape.

## Evidence basis

The source audit declares itself a standalone handoff contract for the active Grappa DCIM migration area. It was generated from reverse documentation, MySQL DDL evidence, follow-up legacy PHP validation, and production data notes where explicitly called out.

The Grappa database structure is also documented under `docs/grappa/GRAPPA.md` and the linked `grappa_*.json` table files. This is useful for validating source tables, columns, keys, and legacy auth/profile tables during planning. It does not, by itself, approve target permissions because the audit explicitly says legacy authorization is out of target scope.

Important evidence sections:

- `Application Inventory`
- `Audited Menu Inventory`
- `Status Vocabulary and Shared Rules`
- `Screen and Workflow Audits`
- `Data and Integration Catalog`
- `Migration Fact Sheet`
- `Risks and Open Questions`

## Current app purpose

Grappa DCIM manages telecom/ISP datacenter infrastructure: buildings, datacenter rooms/cages, MMRs, racks, rack positions, rack power sockets, equipment, servers, storage allocations, plenums, cable/fiber inventory, cross-connect records, fiber rings, and cameras. The audit also covers CWDM and TIM GEA kit utilization reporting, but product review excludes both from V1.

The application is operational inventory plus workflow tooling. It is not only a reporting surface: several screens create child records, perform lifecycle cascades, generate user-visible artifacts, and keep related source records in sync.

## User groups and permissions

Source behavior:

- Legacy authorization is explicitly out of target scope in the audit.
- Many source actions call `ToolGr::RegAcc()` for access/action logging.
- No complete legacy role matrix is provided in the audit.
- The documented Grappa schema includes legacy auth/profile structures such as `auth_item`, `auth_assignment`, `auth_item_child`, `auth_rule`, `user_grappa`, `profile_grappa`, `userprofile_grappa`, and `access_grappa`. These are source evidence and possible mapping aids, not the target RBAC contract.

Target V1 decision:

- The new application uses two roles only.
- Viewer can read approved non-secret DCIM data.
- Operativo can read/write approved V1 surfaces, execute lifecycle/archive actions, perform allowed hard deletes, and view/update encrypted server credential fields.
- Hard delete is always double-confirmed and allowed only when no active operational dependencies exist.
- Legacy auth/profile tables remain source evidence only; they are not the V1 RBAC contract.

## In-scope V1 surfaces

The following source surfaces are in V1 scope because the audit marks them active or first-class support pages.

| Source slug | Source menu path | Product scope |
|---|---|---|
| `dc-build` | `DCIM > Building Datacenter` | Building/facility registry. |
| `datacenter-sala-cage` | `DCIM > Sala/Cage` | Datacenter room/cage CRUD, rack context, maps, port operations. |
| `datacenter-mmr` | `DCIM > MeetMeRooms` | MMR inventory and interconnect context. |
| `racks` | `DCIM > Racks` | Rack CRUD, U-space map, power, positions, media. |
| `rack-sockets` | `DCIM > Rack Power Sockets` | Rack PDU/socket inventory and power reports/history. |
| `apparato` | `DCIM > Apparati` | Equipment CRUD, NIC generation, server/firewall side effects. |
| `server` | `DCIM > Server` | Physical/virtual server inventory and server detail records. |
| `storage` | `DCIM > Storage` | Storage allocation CRUD/archive. |
| `plenums` | `DCIM > Plenum` | Cable pathway and plenum-slot workflow. |
| `anelli-fibra` | `DCIM > Anelli Fibra` | Fiber ring topology and KML metadata. |
| `xcon` | `DCIM > Cross Connect` | Sold customer cross-connect circuit/path registry. |
| `dcimadmin-cable` | `DCIM > Cavi` | Cable/fiber admin workflow. |
| `dcimadmin-cam` | `DCIM > Telecamere` | Camera inventory create/update. |
| `islets` | `IMPOSTAZIONI > Dcim Admin > Isole` | Islet admin CRUD. |
| `positions` | `IMPOSTAZIONI > Dcim Admin > Posizioni` | Position CRUD and batch creation. |

## Out-of-scope V1 behavior

The following items are intentionally out of V1 product scope unless the domain owner overrides this spec:

- Dead legacy menu/features not listed in the active DCIM inventory.
- A first-class active `cassetti_ottici` workflow. It remains a dependency/archive table and participates in verified cascades, but production data is fully decommissioned (`stato='Cessato'`).
- TIM GEA kit report (`kitgraph-kitview`), deferred to V2 redesign/investigation because the data is residual.
- CWDM, deferred to investigation before any implementation because product review treats it as likely abandoned/residual.
- Legacy authorization recreation.
- Device polling, polling cadence, alerting, and monitoring around rack power readings.
- Hive upload/sync for fiber-ring KML maps. KML files/metadata must be preserved for compatibility/history, but Hive sync is V2.
- Enum normalization, schema redesign, table/column renames, or "clean" domain-model rewrites unless later approved as explicit deviations.
- Safer delete/archive redesign beyond the V1 double-confirmation rule below.

## Parity requirements

V1 must preserve these source contracts unless explicitly changed by a product decision:

- Preserve source table names, field names, identifiers, relationships, and free-text values as evidence until downstream implementation planning maps them.
- Preserve legacy lifecycle values exactly, including case and language: for example `Attivo`, `Cessato`, `Spento`, `Chiuso`, `Empty`, `Linked`, `Used`, `Xcon`, `Libera`, `Occupata`, and all `xcon.stato` values.
- Preserve default active filters for source menus that open with `stato=Attivo`.
- Preserve source child-row generation:
  - rack create generates `units`;
  - apparato create with ports generates `nic`;
  - fiber ring create generates `nodi` and circular `archi`;
  - cable create generates `fibers`;
  - position batch creates sequential `positions`.
- Preserve backend-verified lifecycle cascades without broadening them to page-only claims:
  - datacenter cessation cascades to racks, apparati, NICs, and optical cassettes as documented;
  - rack cessation cascades to apparati, NICs, optical cassettes, rack sockets, and position state;
  - apparato cessation cascades direct and linked NICs;
- Preserve equivalent user-visible behavior for generated maps, rack media, KML metadata/files, and approved export artifacts. Exact legacy PHP file generation is an implementation artifact, not a target requirement.
- Preserve server credential storage compatibility for encrypted legacy fields while Grappa coexists.
- Preserve the verified Xcon source of truth: current Cross Connect uses `xcon` plus optional `xcon_hop`; active Xcon create/update does not mutate ports, cables, racks, fibers, or other inventory records.

## Intentional target behavior already present in the audit

These are target decisions or deviations recorded by the audit and should be treated as V1 constraints unless changed by the expert:

- New role-based access model replaces legacy authorization with Viewer and Operativo.
- Hard deletes that remain available in V1 require double user confirmation and no active dependencies.
- Dependency-aware delete blocking is a V1 rule.
- Dynamic PHP map files are replaced by equivalent target map behavior, not reproduced literally.
- Device polling/alerts are out of V1; OID fields and existing reading/summary data are preserved.
- Hive upload/sync for KML is V2.
- V1 UI picklists may offer known values but must tolerate and round-trip unknown stored free-text values.

## Approved decisions

The following decisions were approved during migration specification review:

1. V1 roles are Viewer and Operativo.
2. Hard delete is blocked when active dependencies exist.
3. Occupied islets/positions cannot be deleted; conflicting layout edits are blocked.
4. TIM GEA kit report and CWDM are out of V1.
5. Camera fields `code`, `ipaddr`, and `serial` are not unique; `ipaddr` is validated as IP when provided.
6. Xcon labels are `ticket_esteso` -> `Ticket Esteso`, `num_ordine` -> `Codice Ordine`, `riga_ordine` -> `Serial Number`; `xcon.tipo` stays raw code.
