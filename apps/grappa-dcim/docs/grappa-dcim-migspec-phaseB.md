# Grappa DCIM migspec - Phase B: entity and operation model

Source audit: `apps/grappa-dcim/docs/GRAPPA-DCIM.md`

Status: draft, evidence-derived. Source names are preserved. Any cleaner model would be a proposed deviation and is not introduced here.

## Entity catalog

### `dc_build`

- Purpose: building/facility catalog above datacenter rooms, cages, MMRs, and service/resource mappings.
- Identifiers: `id`.
- Fields: `name`, `address`, `status`, `portale_clienti`, `n_rack`, `created_at`, `updated_at`, `ceased_at`.
- Relationships: `datacenter.dc_build_id` references the building concept; other docs mention ResourcePool/CDL account mappings.
- Operations: list, view, create, update, delete. Building CRUD intent is verified even though exact legacy create route is partial/conflicting.
- Lifecycle: `status` values observed `Attivo`, `Cessato`.
- Approved V1 behavior: transition to `Cessato` is blocked while active dependencies exist; set `ceased_at` if empty when cessation is allowed. Delete only without dependencies and with double confirmation. `portale_clienti=1` means Customer Portal exposure.

### `datacenter`

- Purpose: rooms/cages and MMRs.
- Identifiers: `id_datacenter`.
- Fields: `name`, `address`, `note`, `rack`, `stato`, `id_anagrafica`, `portale_clienti`, `data_attivazione`, `data_cessazione`, `codice_ordine`, `dc_build_id`, `ismmr`, `set_order`, `mmr_type`, `serialnumber`, `floor`.
- Relationships: parent building; children/related `racks`, `islets`, `positions`, `plenums`, `pl_slots`, `ports`, `apparato`, `nic`, `xcon`; optical cassette dependency.
- Operations: list/filter active, create, view, update, hard delete, free port, add port.
- Lifecycle: `stato` values include `Attivo`, `Cessato`; `ismmr=0` for Sala/Cage, `ismmr=1` for MMR. `mmr_type` is a short MMR path identifier, not an enum. `portale_clienti=1` means Customer Portal exposure.
- Side effects: create/update writes or renames dynamic map files in legacy; cessation cascades to verified child resources; `freeport` and `addport` mutate `ports`.
- Approved V1 behavior: preserve `viewmmr` MMR hub, sale/cage rack maps, and `viewisle` MMR-context fiber maps as behavior; do not reproduce PHP map files literally. Delete only without active dependencies; otherwise use `stato=Cessato`. Map-only `crossconnects` references require implementation validation.

### `islets`

- Purpose: physical islets/rows/sides grouping rack positions inside a datacenter.
- Identifiers: composite key `(id, datacenter_id)`.
- Fields: `datacenter_id`, `name`, `rack_num`, `type`, `floor`, `serial`, `order`, `clifat_id`.
- Relationships: datacenter parent; child `positions`; related `racks`; optional `cli_fatturazione`.
- Operations: list, view, create, update, delete.
- Lifecycle/classification: `type` values `isle`, `row`, `side`; label computed as datacenter plus type label plus name.
- Approved V1 behavior: delete is blocked if any child position is occupied. Hard delete otherwise requires double confirmation.
- Ambiguities: exclusivity of `clifat_id`, rack count limit beyond 35.

### `positions`

- Purpose: numbered rack locations inside islets and layout occupancy state.
- Identifiers: composite key `(id, islets_id)`.
- Fields: `status`, `type`, `num`, `islets_id`.
- Relationships: parent `islets`; `racks.positions_id` references position.
- Operations: list, view, create, update, delete, batch create, assign rack through update.
- Lifecycle/classification: status values `free`, `occupied`, possibly `reserved`; type values `full`, `half`, possibly `quarter`.
- Side effects: rack assignment sets `racks.positions_id` and `positions.status='occupied'`; batch creates `1..rack_num` as `free/full` and writes legacy map file.
- Approved V1 behavior: delete occupied positions is blocked. Rack move is an explicit operation that frees the previous position and occupies the new one while rejecting conflicts. `quarter` values are tolerated as legacy data but not supported for V1 creation/editing until verified.
- Ambiguities: file generation failure after position rows are created.

### `racks`

- Purpose: rack inventory, U-space, customer ownership, power metadata, position occupancy, and child equipment.
- Identifiers: `id_rack`.
- Fields: `name`, `unit`, `id_anagrafica`, `id_datacenter`, `stato`, `magnetotermico`, `ampere`, `floor`, `island`, `type`, `pos`, `racknum`, `positions_id`, `shared`, `reserved`, `note`, dates, order/serial, power billing fields, circuit fields, `last_update`, `islet_id`, `sconto`.
- Relationships: datacenter, positions/islets, `units`, `rack_sockets`, `apparato`, `nic`, optical cassette dependency, `media`.
- Operations: list/filter active, view, create, update, hard delete, rack-unit media workflow.
- Lifecycle/classification: `type` documented `Full`/`Half`; `Full` uses `pos=F`; half racks use vertical `pos=A` high or `pos=B` low; `stato` includes `Attivo`, `Cessato`.
- Side effects: create inserts one `units` row per U, validates position compatibility, updates position occupancy, auto-creates socket rows; cessation cascades equipment, NICs, optical cassettes, sockets, and position state.
- Approved V1 behavior: one physical position can contain either one full rack or at most two half racks, one `A` and one `B`. Do not use "lato"/"side" for A/B vertical position. Hard delete only without active dependencies.

### `units`

- Purpose: rack U positions.
- Identifiers: `id`.
- Fields: `num`, `racks_id`, `device_id`.
- Relationships: belongs to rack; used by rack map and rack media.
- Operations: generated on rack create; viewed indirectly; media update by unit.
- Ambiguities: behavior when rack height changes after create is not specified.

### `rack_sockets`, `rack_power_readings`, `rack_power_daily_summary`

- Purpose: PDU/socket inventory and historical/summary power data.
- Identifiers: `rack_sockets.id`; reading/summary `id`.
- Fields:
  - `rack_sockets`: `rack_id`, `magnetotermico`, SNMP/device/OID fields, physical position fields, `status`.
  - `rack_power_readings`: `oid`, `date`, `ampere`, `rack_socket_id`.
  - `rack_power_daily_summary`: `giorno`, `kilowatt`, `id_anagrafica`.
- Relationships: socket belongs to rack; readings reference socket; summary indexed by day/customer.
- Operations: socket CRUD, report power; readings/summary preserved for compatibility/history.
- Lifecycle: rack cessation sets sockets to `Spento`; other statuses not fully enumerated.
- Ambiguities: polling cadence and alerts are out of V1; full status vocabulary unknown.

### `apparato`

- Purpose: racked equipment inventory: routers, switches, firewalls, servers, storage, UPS/ATS, housing/passacavo/cassetto-like entries, clusters.
- Identifiers: `id_apparato`.
- Fields: location, customer, device type, serial/model/OS, status, kit fields, port defaults, dates, order/serial, firewall/monitoring fields, installation/shipping fields.
- Relationships: rack, customer/billing, `nic`, `server`, `cli_contatti_escalation`, `transito`.
- Operations: list/filter active, view, create, update, hard delete, type-specific redirects/workflows.
- Lifecycle: `stato` includes `Attivo`, `Cessato`.
- Side effects: create with `numero_porte > 0` generates sequential `nic` rows; `Server` redirects to server create; `Firewall` saves escalation contact data; cessation cascades NICs; update syncs selected fields to linked physical server.
- Approved V1 behavior: no rigid enum for `type`; use a picklist from current DB values and preserve unknown values. Only proven types get side effects. Do not regenerate NICs automatically on update; adding ports later is explicit. Monitoring fields are preserved/editable but do not trigger polling/alerts. Hard delete only without active dependencies.

### `nic`

- Purpose: equipment, server, CWDM, and optical connection ports/channels.
- Identifiers: `id_nic`.
- Fields: `id_apparato`, `identificativo`, `name`, `id_anagrafica`, `type`, link fields, `layer`, `stato`, CWDM link fields.
- Relationships: generated by apparato; referenced by services and historical/report contexts; cascaded by parent lifecycle changes. CWDM and TIM GEA report usages are V2/out of V1.
- Operations: generated, updated indirectly, listed in device detail/report contexts.
- Lifecycle: `stato` includes `Attivo`, `Cessato`.
- Ambiguities: exact link-field semantics outside documented cascades.

### `server`, `server_schede`, `server_applicazioni`, `server_servizi`, `server_porte`

- Purpose: detailed server configuration and related cards/applications/services/ports.
- Identifiers: `server.id_server` and child IDs.
- Fields: server identity, customer, state, OS/hardware/virtualization, rack/unit location, credentials/access fields, backup/syslog notes, `apparato_id`, order/serial, `porte`.
- Relationships: optional physical link to `apparato`; rack/customer; children listed above.
- Operations: list, view, create physical, create virtual, update; delete follows the general dependency-gated hard-delete policy.
- Lifecycle/classification: `tipologia='Fisico'` sync behavior verified; virtual create uses `createvirtua`.
- Side effects: physical create commonly follows apparato create; physical update syncs customer/order/serial to apparato. Selected password fields are encrypted with legacy `k_crypt` on insert/update.
- Approved V1 behavior: Viewer cannot access credential values. Operativo can view/update credential fields. `pwd_utenza_cliente` is treated as sensitive even though encryption evidence is incomplete; implementation must validate storage/encryption behavior. Empty explicit write means clear value; omitted field means do not change.
- Ambiguities: backup/syslog execution semantics and delete protections.

### `storage`

- Purpose: storage allocations/volumes assigned to customers and storage apparati.
- Identifiers: composite PK `(id, cli_fatturazione_id, apparato_id_apparato)`.
- Fields: `access_protocol`, `size`, `cli_fatturazione_id`, `apparato_id_apparato`, `note`, `size_type`, `status`, `created_at`, `closed_at`, `codice_ordine`, `serial_number`.
- Relationships: customer/billing and apparato.
- Operations: list, view, create, update, hard delete, archive.
- Lifecycle: archive sets `status='Chiuso'` and `closed_at=now`; active status value not fully proven.
- Approved V1 behavior: archive is preferred closure. Hard delete only for records without known operational/billing dependencies. `Chiuso` records are read-only unless a future explicit reopen is approved. No billing side effects in V1. `size_type` supports `GB`/`TB` while tolerating legacy values.
- Ambiguities: active status default should be chosen from the most common DB value before implementation.

### `plenums`, `pl_slots`, `ports`, `slots`

- Purpose: cable pathway structures, plenum slots, rack/MMR slots, and individual ports.
- Identifiers: `plenums` composite `(id, datacenter_id)`; `pl_slots` composite `(id, plenums_id)`; `ports` composite `(id, slots_id)`; `slots` composite `(id, racks_id)`.
- Fields:
  - `plenums`: `name`, `isle`, `type`, `datacenter_id`, `status`.
  - `pl_slots`: `num`, `cable`, `type`, `status`, `slots_id`, `mmr_slot_id`.
  - `ports`: `num`, `status`, plenum/slot/FO/unit/rack/device fields, `name`, `cable_fiber_id`.
  - `slots`: rack panel/slot fields, port count, cable/plenum mappings.
- Relationships: datacenter/rack/MMR/plenum/port/fiber connections.
- Operations: plenum list/view/create/update/delete; explicit matrix initialization; port free/add from datacenter workflows; fiber assignment through cable workflow.
- Lifecycle: port statuses `Empty`, `Linked`, `Used`, `Xcon`; plenum/slot statuses not fully enumerated.
- Approved V1 behavior: plenum visual capacity is 288 calculated fiber cells: 2 cables x 12 termination points x 12 fibers. Plenum create inserts only `plenums`; explicit initialize-matrix creates missing `pl_slots` for `cable=1/2` and `num=1..12`. Fiber-cell occupancy is calculated from `ports.pl_slots_id` plus `ports.pl_port_num`. Delete plenum/slot is blocked when linked ports exist.
- Implementation validation: verify whether `mmr_slots` is a real table or legacy relation/model naming over `slots`.

### `cables`, `fibers`

- Purpose: cable bundles and individual fiber strands.
- Identifiers: `cables.id`; `fibers.id`.
- Fields: cable `name`, `description`, `fibers_num`, `status`; fiber `num`, `status`, `cable_id`, `left_port_id`, `right_port_id`.
- Relationships: fibers belong to cable; fibers reference left/right ports; ports store `cable_fiber_id`.
- Operations: cable create/list/view; fiber update; available transit fiber JSON lookup.
- Lifecycle: new fibers start `Libera`; preserve existing values such as `Occupata`.
- Side effects: cable create generates `fibers_num` child fibers; fiber update clears old port assignments and sets new port assignments.
- Approved V1 behavior: cable delete only when all fibers are `Libera` and have no assigned ports. Fiber assignment is atomic with port updates and rejects concurrent double assignment. Unassigned fibers are `Libera`; assigned fibers are `Occupata`; legacy anomalous values are preserved but not newly generated.
- Ambiguities: duplicate check against `Plenums` appears odd.

### `xcon`, `xcon_hop`

- Purpose: customer cross-connect circuit/path registry and optional ordered intermediate hops.
- Identifiers: `xcon.id`; `xcon_hop.id`.
- Fields: ticket/order/customer/status/type, activation/cessation dates, A/Z endpoint text fields, A/Z rack FKs, notes, `sorgente`, Letter of Agency fields, `mmr_port`; hop room/rack/unit/slot/fiber/order fields.
- Relationships: A/Z rack FKs to `racks.id_rack`; hops belong to Xcon and optionally reference rack.
- Operations: list active/ceased tabs, create/update/delete as supported by active Xcon controller, manage optional hops.
- Lifecycle: `xcon.stato` values are exact free-text workflow states; `cessata` and `annullato` are terminal; index treats only `cessata` as ceased tab.
- Product type: `CDL-XLOCAL`, `CDL-XSEEODF`, `CDL-XCAMPUS`, `CDL-XIRI`, `CDL-XIRIODF`, `CDL-XMIX`, `CDL-XMIXODF`.
- Side effects: verified controller saves `xcon` and `xcon_hop` only. It does not update inventory resources.
- Approved V1 behavior: `ticket_esteso` label is `Ticket Esteso`, `num_ordine` is `Codice Ordine`, and `riga_ordine` is `Serial Number`. `xcon.tipo` displays raw `CDL-X*` code values; use legacy known values as picklist while tolerating unknown values. Do not invent business descriptions for product codes.

### `anelli_fibra`, `nodi`, `archi`, `archi_tratta`, `mappa_tracciati_anelli`

- Purpose: fiber ring topology, ring nodes/arcs, optional route/segment details, and KML metadata.
- Identifiers: `anelli_fibra.id_anello`, child IDs.
- Fields: ring name/customer/node count/order/serial/KML/status; node coordinates/switch/port/transceiver fields; arc endpoints/distance/attenuation/references; KML metadata.
- Relationships: ring has nodes and circular arcs; arcs can have route segments.
- Operations: list, view, create, update increasing node count, KML upload/metadata preservation.
- Lifecycle: create sets `stato='Attivo'`.
- Side effects: create auto-generates N nodes and N circular arcs; optional KML file is stored and metadata inserted. Decreasing node count is not supported.
- Approved V1 behavior: create is atomic. Increasing node count is allowed; decreasing is blocked. `distanza` and `attenuazione` default to `0` and are manually editable with no automatic calculation. Delete only for rings without meaningful operational data, KML, routes, coordinates, or references; otherwise use `stato=Cessato`.

### `cwdm`

- Purpose: CWDM optical multiplexing devices and wavelength channels.
- Identifiers: `id_cwdm`.
- Fields: local and remote datacenter/rack fields, connector, rack unit fields, serial, customer, note, `stato`.
- Relationships: generated `nic` wavelength channels; optional mirrored CWDM has no explicit mirror FK.
- Operations: none in V1; investigate before any implementation.
- Lifecycle: `stato` includes `Attivo`, `Cessato`.
- Source behavior: create generates 10 wavelength NICs; optional remote fields create reciprocal mirror and another 10 NICs; cessation bulk-updates related NICs.
- Approved V1 behavior: out of V1 as likely abandoned/residual. See `docs/TODO.md` for investigation before implementation.

### `cams`

- Purpose: datacenter camera inventory.
- Identifiers: `id`.
- Fields: `code`, `model`, `brand`, `position`, `ipaddr`, `status`, `serial`.
- Relationships: no declared FKs.
- Operations: list/create/update. No delete action is documented.
- Lifecycle: status examples include `Active`, `Inactive`, `Maintenance`, `Failed`, `Replaced`; DDL is free text.
- Approved V1 behavior: no uniqueness enforcement on `code`, `ipaddr`, or `serial`; validate `ipaddr` as IP when provided.
- Ambiguities: status vocabulary.

### TIM GEA kit report entities

- Purpose: read-only utilization report/export for active TIM GEA kit devices and active customer line assignments.
- Source entities: `apparato`, `nic`, `eth`, `foglio_linee`.
- Operations: none in V1; V2 redesign/investigation only.
- Rules: select active `apparato` with non-empty `td_kit`, order by `ordine_view_kit_gea`; join active `foglio_linee`; sum bandwidth; count used/free ports; derive port index from last two characters of `nic.identificativo`.
- Approved V1 behavior: out of V1 because current review treats the source data as residual.

### `cassetti_ottici`

- Purpose in this spec: dependency/archive table only.
- Status: production data is fully decommissioned (`stato='Cessato'`).
- Operations: no first-class active V1 workflow.
- Required parity: preserve as dependency in verified cascades and historical references.

## Operation summary

| Area | Read/list | Create | Update | Delete | Archive/cessation | Export/report | Generated children/artifacts |
|---|---|---|---|---|---|---|---|
| Buildings | Yes | Partial evidence | Yes | Hard delete | `Cessato` | No | Legacy inline create evidence partial |
| Datacenters/MMR | Yes | Yes | Yes | Hard delete | Cascades verified children | Maps | Map file equivalent |
| Islets/Positions | Yes | Yes | Yes | Hard delete | Position occupancy | No | Batch positions, map equivalent |
| Racks | Yes | Yes | Yes | Hard delete | Cascades equipment/socket/position | Media view | Units, sockets, media |
| Rack sockets | Yes | Yes | Yes | Yes | `Spento` through rack | Power report/history | Reading/summary preserved |
| Apparati/NICs | Yes | Yes | Yes | Dependency-gated | NIC cascade | Historical/report source only | NIC generation |
| Servers | Yes | Yes | Yes | Unresolved | `stato` free text | Detail children | Credential encryption compatibility |
| Storage | Yes | Yes | Yes | Hard delete | Archive to `Chiuso` | No | None |
| Plenums/Ports | Yes | Yes | Yes | Dependency-gated | Port free/link/use | 288-cell visual map | Explicit matrix initialization |
| Cables/Fibers | Yes | Yes | Fiber update | Dependency-gated | Fiber status values | JSON lookup | Fibers generation |
| Xcon | Yes | Yes | Yes | As source | Terminal statuses | No | Optional hops only |
| Fiber rings | Yes | Yes | Increase nodes | Dependency-gated | Status | KML metadata | Nodes/arcs, KML |
| CWDM | V2 investigation | No | No | No | No | No | No V1 workflow |
| Cameras | Yes | Yes | Yes | No documented delete | Status metadata | No | None |
| Kit graph | V2 investigation | No | No | No | No | No V1 export | No V1 workflow |

## Ambiguous identifiers, enums, defaults, and states

- `rack_sockets` is the authoritative table. `rack_power_sockets` is terminology drift.
- `datacenter.ismmr` splits Sala/Cage from MMR. `mmr_type` is a short MMR path identifier and is not an enum.
- Several tables use composite keys. Preserve composite identity until downstream design maps them.
- Status/type values are mostly free text and must round-trip unknown values.
- `xcon.stato` is case-sensitive and must preserve exact values.
- `xcon.tipo` is a product selector, not a workflow state.
- Rack `type` uses `Full`/`Half`; rack `pos=A/B` means high/low vertical position for half racks, not side/lato. Position `quarter` values, if present, are tolerated as legacy but not supported for V1 create/edit.
- `server` credential fields have asymmetric encryption evidence; do not infer encryption for unproven credential-like fields.
- `storage.status` active value is not proven; archive value `Chiuso` is verified.
- Camera uniqueness is not enforced in V1; `ipaddr` is validated as IP when provided.
