# Grappa DCIM migspec - Phase D: logic, data, and integration contracts

Source audit: `apps/grappa-dcim/docs/GRAPPA-DCIM.md`

Status: draft, evidence-derived. This phase records product-level contracts and later validation candidates. It does not add tests.

## Logic allocation

### Backend/domain layer responsibilities

The rewrite must keep these rules server-side or in a domain layer that cannot be bypassed by the UI:

- Required-field and type validation for all mutating source workflows.
- Lifecycle transitions and verified cascades.
- Child record generation:
  - `racks` to `units`;
  - `racks` to `rack_sockets` when circuit fields require socket rows;
  - `apparato` to `nic`;
  - `cables` to `fibers`;
  - `anelli_fibra` to `nodi` and circular `archi`;
- Dependency checks before hard delete and before lifecycle transitions that would create parent/child inconsistencies.
- Composite-key lookup and mutation for tables that use composite identities.
- Xcon status transition availability equivalent to source `getOptionStatus()` behavior, once the exact transition map is available.
- Server credential encryption/decryption compatibility for legacy encrypted fields.
- File/artifact persistence or equivalent artifact services for maps, media, KML, and XLS exports.
- Double confirmation enforcement for destructive deletes at the action contract level, not only visual styling.
- Access/action logging equivalent to source audit expectations if required by product/security.

### Frontend/shared validation responsibilities

The UI may assist with these checks, but backend/domain validation remains authoritative:

- Required fields, max lengths, integer limits, and picklist hints.
- Unknown free-text status/type display and round-trip.
- Warnings before destructive actions and side-effectful lifecycle changes.
- Client-side prevention of obvious double-submit cases for large generated child operations.
- Presentation of map/U-space/topology/report data.

### External system responsibilities

The audit does not prove active external write systems for most DCIM operations. Treat these as external or historical concerns:

- Legacy Grappa DB remains the source evidence for schemas and values.
- Server credential encryption must remain compatible during coexistence.
- KML/Hive sync is V2, not V1.
- Rack power polling and alerting are out of V1; preserve fields/data.
- `portale_clienti=1` means the record should be exposed through Customer Portal; consumers must respect the flag.

## Data contracts

### Vocabulary contract

All status/type values are free-text unless the audit proves otherwise. V1 must preserve exact stored values and tolerate unknown ones.

| Domain | Known values | Contract |
|---|---|---|
| Facility/equipment lifecycle | `Attivo`, `Cessato` | Preserve exact case/spelling. |
| Rack socket lifecycle | `Spento` plus unknowns | Rack cessation sets `Spento`; do not restrict unknown statuses. |
| Storage lifecycle | `Chiuso`; active value unknown | Archive sets `Chiuso` and `closed_at`. |
| Port lifecycle | `Empty`, `Linked`, `Used`, `Xcon` | `freeport` sets `Empty`; `addport` sets `Linked` or `Used`; preserve `Xcon`. |
| Fiber lifecycle | `Libera`, `Occupata` | Cable create initializes `Libera`; preserve existing `Occupata`. |
| Xcon lifecycle | `Bozza`, `Verifica Tecnica`, `in attivazione`, `Intervento Utente Richiesto`, `Attiva`, `libera`, `non cablato`, `cessata`, `annullato` | Case-sensitive; `cessata` and `annullato` terminal; ceased tab is only `stato='cessata'`. |
| Xcon product type | `CDL-XLOCAL`, `CDL-XSEEODF`, `CDL-XCAMPUS`, `CDL-XIRI`, `CDL-XIRIODF`, `CDL-XMIX`, `CDL-XMIXODF` | Product selector, not workflow state. |
| Position status/type | `free`, `occupied`, possibly `reserved`; `full`, `half`, possibly `quarter` | Batch creates `free/full`; rack assignment sets `occupied`; `quarter` is tolerated as legacy but not supported for V1 create/edit. |
| Islet type | `isle`, `row`, `side` | Labels are `Isola`, `Fila`, `Lato`. |
| Server type | `Fisico`; virtual flow `createvirtua`; docs mention Physical/Virtual/Cluster | Physical sync applies to `tipologia='Fisico'`. |
| Camera status | `Active`, `Inactive`, `Maintenance`, `Failed`, `Replaced` | DDL free text; no enum/FK. |

### Source table contract

These source table names and relationships must remain visible in migration contracts until downstream implementation planning maps them:

- Core hierarchy: `dc_build`, `datacenter`, `islets`, `positions`, `racks`, `units`, `slots`, `media`.
- Equipment/compute/storage: `apparato`, `nic`, `server`, `server_schede`, `server_applicazioni`, `server_servizi`, `server_porte`, `storage`, `cli_contatti_escalation`, `transito`.
- Power/cabling/cross-connect: `rack_sockets`, `rack_power_readings`, `rack_power_daily_summary`, `plenums`, `pl_slots`, `ports`, `cables`, `fibers`, `xcon`, `xcon_hop`.
- Fiber/optical/camera/report: `anelli_fibra`, `nodi`, `archi`, `archi_tratta`, `mappa_tracciati_anelli`, `cassetti_ottici`, `cams`, `eth`, `foglio_linee`, `cli_fatturazione`. `cwdm` remains documented source evidence but is excluded from V1.

Resolved conflict:

- `rack_sockets` is authoritative. References to `rack_power_sockets` in some docs are terminology drift.

### Mutation and side-effect contract

| Workflow | Required contract |
|---|---|
| Datacenter create/update | Preserve record mutation and equivalent map behavior. Rename behavior for maps must preserve user-visible continuity. |
| Datacenter cessation | Cascade only to verified child resources: racks, apparati, NICs, optical cassettes. |
| Datacenter `freeport` | Set `ports.status='Empty'`, clear `pl_slots_id` and `pl_port_num`. |
| Datacenter `addport` | Set `ports.status='Linked'` when plenum slot supplied, otherwise `Used`. |
| Rack create | Insert rack, generate `units` 1..height, validate/update position, generate socket rows as source fields require. |
| Rack move | Explicitly free the old position, occupy the new position, and reject full/half or A/B vertical conflicts. |
| Rack cessation | Cascade apparati/NICs/optical cassettes, set sockets `Spento`, update/free position occupancy. |
| Rack media | Preserve front/back unit media records and referenced image artifacts. |
| Apparato create | Generate NICs when `numero_porte > 0`; preserve identifier format `0/01`, `0/02`, etc. |
| Apparato update to `Cessato` | Cascade direct and linked NICs to `Cessato`. |
| Server physical update | Sync customer, order code, and serial number to linked `apparato`. |
| Server credential update | Preserve `k_crypt` compatibility for proven encrypted fields; Operativo can view/update credentials, Viewer cannot; omitted credential field means unchanged and explicit empty value means clear. |
| Storage archive | Set `status='Chiuso'` and `closed_at` to current timestamp; prefer archive over delete and do not trigger billing side effects. |
| Ring create | Atomically insert ring, optional KML metadata, N nodes, N circular arcs with initial distance/attenuation `0`. |
| Cable create | Insert cable and generate fibers 1..`fibers_num`, all `Libera`. |
| Fiber update | Atomically clear old ports' `cable_fiber_id`, save new left/right ports, set new ports to fiber ID, and reject concurrent double assignment. |
| Xcon create/update | Mutate only `xcon` and `xcon_hop`; do not mutate inventory resources. |
| CWDM | No V1 workflow. Investigate source usage before any implementation. |
| Islet delete | Block when any child position is occupied; otherwise allow only with double confirmation and no active dependencies. |
| Position delete | Block when occupied; otherwise allow only with double confirmation and no active dependencies. |
| Position batch | Block if any positions already exist for the islet; otherwise create positions `1..rack_num` as `free/full` and preserve map behavior. |
| Plenum matrix initialization | Explicitly create missing `pl_slots` for `cable=1/2` and `num=1..12`; plenum create does not do this implicitly. |
| Plenum/fiber map | Calculate 288 fiber cells from `pl_slots` and `ports.pl_port_num`; occupancy is derived from linked `ports`, not from `pl_slots.status`. |
| Camera create/update | Do not enforce uniqueness on `code`, `ipaddr`, or `serial`; validate `ipaddr` as IP when provided. |

## Integration and artifact contracts

| Integration/artifact | V1 contract |
|---|---|
| ToolGr access logging | Source logs many actions. Product/security must decide exact target logging, but access/action auditability should not be silently dropped. |
| Generated datacenter/islet maps | Preserve equivalent user-visible map/layout behavior. Exact PHP files are not a target requirement. |
| Rack unit media | Preserve existing media rows and referenced images; support equivalent front/back upload/update. |
| Server credential encryption | Must remain compatible with legacy `k_crypt` values while coexistence is required. |
| KML files/metadata | Preserve history and file references. Hive upload/sync is V2. |
| TIM GEA XLS export | Out of V1; report requires V2 redesign/investigation. |
| Rack power readings | Preserve inventory/OID fields and reading/summary data. Polling/alerts out of V1. |

## Auth and permission expectations

Evidence proves that legacy authorization should not be recreated. Product review approved a simple V1 target model.

Additional source evidence exists in the documented Grappa schema: `auth_item`, `auth_assignment`, `auth_item_child`, `auth_rule`, `user_grappa`, `profile_grappa`, `userprofile_grappa`, and `access_grappa`. These tables can help compare legacy users/profiles and historical grants, but they must not be treated as the new application's permission model unless product owners explicitly decide to map them.

V1 roles:

| Role | Permissions |
|---|---|
| Viewer | Read approved non-secret DCIM inventory, topology, maps, reports/history, and artifacts. No creates, updates, lifecycle transitions, deletes, or credential value access. |
| Operativo | Includes Viewer. Can create/update approved V1 records, execute lifecycle/archive actions, perform allowed hard deletes with double confirmation and dependency checks, manage support data, upload/update artifacts, and view/update encrypted server credential fields. |

Audit/log visibility remains an implementation/security decision if a user-facing audit view is later exposed.

## Partial failure and retry behavior

The audit often documents multi-row side effects but not transaction boundaries. V1 should make partial failure behavior explicit before implementation for:

- Rack create after `racks` insert but before all `units` or sockets are generated.
- Position batch after some positions are inserted but before map artifact generation.
- Fiber ring create after partial node/arc generation.
- Cable create after partial fiber generation.
- Server credential update failures.
- Rack media, KML, and XLS file writes.

Where source transaction behavior is not proven, do not infer silent retry or automatic rollback. Define target behavior during implementation planning.

## Risky facts for later contract validation

These should become contract tests or validation checks during implementation planning, but this skill does not add tests:

1. Default active filters for datacenter, racks, and apparato.
2. `rack_sockets` table mapping, not `rack_power_sockets`.
3. Datacenter/rack/apparato cessation cascades with exact breadth.
4. Rack create generates exactly one `units` row per U.
5. Rack create/update socket generation and rack cessation sets sockets `Spento`.
6. Position batch blocks when any positions already exist for the islet.
7. Position/rack assignment and rack move update occupancy without double-booking.
8. Apparato port generation creates sequential NIC identifiers.
9. Physical server update syncs selected fields to linked apparato.
10. Server encrypted fields remain readable/writable with legacy compatibility.
11. Storage archive preserves the row and sets `Chiuso` plus `closed_at`.
12. Cable create generates the requested number of `Libera` fibers.
13. Fiber update clears old port assignments, sets new ones, and rejects concurrent double assignment.
14. Xcon create/update does not mutate inventory tables.
15. Xcon state values and active/ceased tab behavior preserve exact source semantics.
16. Fiber ring create generates N nodes and N circular arcs.
17. Plenum matrix initialization creates 24 termination points and map calculates 288 cells.
18. MMR hub and `viewisle` preserve MMR context and physical connection state.
19. Camera create/update exists, validates IP when present, and does not enforce uniqueness.
20. Destructive deletes require double confirmation and no active dependencies.
