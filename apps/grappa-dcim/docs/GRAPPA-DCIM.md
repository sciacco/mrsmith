# Grappa DCIM Legacy Audit

Generated: 2026-05-09

This is an implementation-neutral legacy audit and migration fact sheet for the Grappa DCIM area. It follows the `legacy-app-auditor` purpose: evidence first, target design later. It records current behavior, source confidence, risky contracts, and unresolved gaps. It intentionally does not prescribe target routes, frontend components, backend modules, database redesign, deployment shape, or UI modernization.

## Standalone Handoff Contract

This document is intended to travel alone. Downstream agents may receive only this file and should not need access to `docs/reverse`, `docs/grappa_ddl.sql.dump`, the original Yii2 application, or any other file from `grappa-ng` to understand the audited DCIM behavior.

The original Yii2/PHP source application is not available as runnable source in this repository. The evidence behind this audit is an extracted reverse-documentation bundle, an authoritative MySQL DDL dump, and follow-up validation against the legacy PHP repository and production data notes where explicitly called out. Source paths are cited only as provenance; every important route, workflow, mutation, table, field, status value, side effect, risk, conflict, and open question is summarized inline here.

Use this file as a legacy behavior contract. If a fact is marked `verified`, preserve it as current behavior unless product/domain owners explicitly decide otherwise. If a fact is marked `conflicting` or `unresolved`, treat it as a validation task, not as license to invent behavior.

## Application Inventory

### Source Classification

- Source type: reverse-engineering evidence for a legacy Yii2 Basic Template/PHP application, backed by a MySQL DDL dump.
- Original application source availability: not present in this checkout as runnable Yii2 source. Reverse docs quote original controller/model/view paths, but those files are not available to downstream agents unless separately provided.
- Application domain: telecom/ISP datacenter infrastructure management, colocation, fiber/cable plant, equipment inventory, server inventory, and cross-connect workflows.
- Primary DCIM scope: active DCIM migration surfaces under `DCIM`; dead legacy menu/features are intentionally omitted from this handoff.
- Required support scope: `islets` and `positions`, under `IMPOSTAZIONI > Dcim Admin`, are first-class support screens because rack layout and position workflows depend on them.
- Referenced dependency, not first-class audited screen: `cassetti_ottici` exists as a table dependency and participates in verified cascades, but production data is fully decommissioned (`stato='Cessato'`). Do not rebuild an active optical cassette workflow for V1.
- Legacy authorization is intentionally out of target scope. The new application will use a new role-based access model.

### Evidence Coverage

| Area | Coverage | Inline outcome |
|---|---:|---|
| Active DCIM migration surfaces | 15 | Active DCIM inventory/workflow surfaces are covered below; dead ticket/navigation artifacts are omitted. |
| Requested DCIM admin support pages | 2/2 | `islets` and `positions` are covered as first-class support screens. |
| Feature evidence for audited screens | Present | Reverse docs are used where they match active legacy behavior; polluted/dead feature evidence is intentionally excluded. |
| DDL evidence | Present | Key source tables and fields are summarized in the Data and Integration Catalog. |
| Original Yii2 source | Not present | Behavior is reconstructed from reverse docs and DDL, not direct source inspection. |

### Audited Menu Inventory

| # | Slug | Menu path | Route | Screen shape | Primary source data |
|---:|---|---|---|---|---|
| 1 | `dc-build` | `DCIM > Building Datacenter` | `dc-build` | Building/facility registry | `dc_build` |
| 2 | `datacenter-sala-cage` | `DCIM > Sala/Cage` | `datacenter&DatacenterSearch[stato]=Attivo` | Datacenter room/cage CRUD and maps | `datacenter`, `racks`, `islets`, `positions`, `plenums` |
| 3 | `datacenter-mmr` | `DCIM > MeetMeRooms` | `datacenter/mmr&DatacenterSearch[stato]=Attivo` | MMR inventory and interconnect context | `datacenter`, `xcon`, `plenums`, `ports` |
| 4 | `racks` | `DCIM > Racks` | `racks&RacksSearch[stato]=Attivo` | Rack CRUD, U-space map, power, positions | `racks`, `units`, `positions`, `rack_sockets`, `apparato`, `media` |
| 5 | `rack-sockets` | `DCIM > Rack Power Sockets` | `rack-sockets` | Rack PDU/socket inventory and reports | `rack_sockets`, `rack_power_readings`, `rack_power_daily_summary` |
| 6 | `apparato` | `DCIM > Apparati` | `apparato&ApparatoSearch[stato]=Attivo` | Equipment CRUD with NIC generation | `apparato`, `nic`, `racks`, `server`, `cli_contatti_escalation` |
| 7 | `server` | `DCIM > Server` | `server` | Physical/virtual server inventory | `server`, `server_schede`, `server_servizi`, `server_applicazioni`, `server_porte` |
| 8 | `storage` | `DCIM > Storage` | `storage` | Storage allocation CRUD/archive | `storage`, `apparato`, `cli_fatturazione` |
| 9 | `plenums` | `DCIM > Plenum` | `plenums` | Cable pathway and plenum-slot CRUD | `plenums`, `pl_slots`, `ports` |
| 10 | `anelli-fibra` | `DCIM > Anelli Fibra` | `anelli-fibra` | Fiber ring topology and KML mapping | `anelli_fibra`, `nodi`, `archi`, `mappa_tracciati_anelli` |
| 11 | `xcon` | `DCIM > Cross Connect` | `xcon` | Cross-connect circuit/path registry | `xcon`, `xcon_hop` |
| 12 | `kitgraph-kitview` | `DCIM > Kit Gea Tim` | `kitgraph/kitview` | Read-only TIM GEA kit utilization report | `apparato`, `nic`, `eth`, `foglio_linee` |
| 13 | `cwdm` | `DCIM > Cwdm` | `cwdm&CwdmSearch[stato]=Attivo` | CWDM optical equipment CRUD | `cwdm`, `nic`, `datacenter`, `racks` |
| 14 | `dcimadmin-cable` | `DCIM > Cavi` | `dcimadmin/cable` | Cable/fiber admin workflow | `cables`, `fibers`, `ports` |
| 15 | `dcimadmin-cam` | `DCIM > Telecamere` | `dcimadmin/cam` | Camera inventory create/update | `cams` |
| 16 | `islets` | `IMPOSTAZIONI > Dcim Admin > Isole` | `islets` | Islet admin CRUD | `islets`, `positions`, `datacenter` |
| 17 | `positions` | `IMPOSTAZIONI > Dcim Admin > Posizioni` | `positions` | Position CRUD and batch creation | `positions`, `islets`, `racks` |

## Evidence Map

### Precedence Used

1. DDL and generated table docs are preferred for table names, columns, indexes, and declared foreign keys.
2. Feature backend/data model docs are preferred for mutations, validation, side effects, and workflow rules.
3. Page docs and menu map are preferred for visible navigation, routes, and labels.
4. Progress docs and older analysis are lower precedence; they are useful for context but can be stale.

### Inline Provenance

Key provenance sources were:

- Menu and page inventory: `docs/reverse/02_FRONTEND/menu_map.md`, `docs/reverse/02_FRONTEND/pages/*.md`.
- Feature behavior: `docs/reverse/05_FEATURES/<slug>/{overview,ui,backend,data_model,flows,edge_cases}.md`.
- Endpoint summaries: `docs/reverse/03_BACKEND/endpoints/dcim-infrastructure.md`.
- Schema evidence: `docs/reverse/04_DATA_MODEL/tables/*.md`, `docs/reverse/99_PROGRESS/SCHEMA_DCIM.md`, `docs/grappa_ddl.sql.dump`.
- Older progress analysis: `docs/reverse/99_PROGRESS/extended-dcim-features.md`.

These paths are provenance only. The reader should not need them to understand the audit.

### Important Conflicts and Resolution Rules

| Item | Status | Inline resolution |
|---|---|---|
| `rack_sockets` vs `rack_power_sockets` | `conflicting` | The DDL defines `rack_sockets`, `rack_power_readings.rack_socket_id` references `rack_sockets.id`, and the generated table doc maps Yii model concept `RackPowerSockets` to table `rack_sockets`. Some page/endpoint docs say `rack_power_sockets`; treat that as terminology drift. |
| Camera implementation | `conflicting/resolved` | Older progress notes say camera management was not implemented. Higher-precedence current docs and DDL prove `dcimadmin/cam`, model `Cam`, and table `cams` exist. Camera management is in scope. |
| Xcon source of truth | `verified/resolved` | Follow-up validation against the legacy PHP repository confirms the current Cross Connect menu uses `controllers/XconController.php` and table `xcon`; child route data uses `xcon_hop`. Treat `xcon` plus `xcon_hop` as the only active cross-connect contract for this handoff. |
| `cassetti_ottici` scope | `dependency/archive only` | Production data is fully decommissioned (`stato='Cessato'`). It remains a historical dependency in verified cascades, but should not become a first-class active V1 migration surface. |
| DC build create behavior | `partial/conflicting` | Page docs describe an index with inline create. Backend docs list index/view/update/delete and generated notes include a recommendation to add a dedicated create action. Treat building CRUD intent as verified, but exact create POST action is not fully proven. |
| Datacenter cessation breadth | `resolved for V1` | Backend evidence verifies cascading `Cessato` to racks, apparati, NICs, and optical cassettes. Page-only claims about plenums, ports, and plenum slots are not V1 behavior requirements. |

## Status Vocabulary and Shared Rules

| Domain | Values observed | Notes |
|---|---|---|
| Facility/equipment lifecycle | `Attivo`, `Cessato` | Used by `dc_build.status`, `datacenter.stato`, `racks.stato`, `apparato.stato`, `nic.stato`, `cwdm.stato`, `cassetti_ottici.stato`, and fiber rings. DDL generally uses free text, not enum/check constraints. |
| Rack socket lifecycle | `Spento` plus free text `status` values | Rack cessation sets rack power sockets to `Spento`; other socket status values are not fully enumerated. |
| Storage lifecycle | `Chiuso` on archive; active value not fully proven | `StorageController::actionArchive()` sets `storage.status='Chiuso'` and `closed_at=now`. |
| Port lifecycle | `Empty`, `Linked`, `Used`, `Xcon` | `freeport` sets `Empty`; `addport` sets `Linked` when plenum-linked and `Used` otherwise. Preserve existing `Xcon` values exactly; active `XconController` does not mutate port rows. |
| Fiber lifecycle | `Libera`, `Occupata` | Cable creation initializes child fibers to `Libera`. Preserve existing `Occupata` values exactly; active `XconController` does not mutate fiber rows. |
| Xcon lifecycle | `Bozza`, `Verifica Tecnica`, `in attivazione`, `Intervento Utente Richiesto`, `Attiva`, `libera`, `non cablato`, `cessata`, `annullato` | `xcon.stato` is free text and case-sensitive. UI transition availability is controlled by `XconController::getOptionStatus()`; `cessata` and `annullato` are terminal. Index tabs use `stato != 'cessata'` and `stato = 'cessata'`, so `annullato` appears with active-tab records. |
| Xcon product type | `CDL-XLOCAL`, `CDL-XSEEODF`, `CDL-XCAMPUS`, `CDL-XIRI`, `CDL-XIRIODF`, `CDL-XMIX`, `CDL-XMIXODF` | `xcon.tipo` is a static product/type selector chosen according to the cross-connect product sold to the customer; it is not a workflow state. |
| Position status | `free`, `occupied`, possibly `reserved` | Batch position creation uses `free`; rack assignment sets `occupied`. |
| Position type | `full`, `half`, `quarter` in position docs; rack docs use `Full`/`Half` for rack type | Case and domain differ by table/form. Batch creation uses lower-case `full`. |
| Islet type | `isle`, `row`, `side` | Labels translate to `Isola`, `Fila`, `Lato`. |
| Server type/tipologia | `Fisico`, virtual/createvirtua; page docs also mention Physical/Virtual/Cluster | Physical sync logic is documented for `tipologia='Fisico'`. |
| Camera status | Common values include `Active`, `Inactive`, `Maintenance`, `Failed`, `Replaced` | `cams.status` is free text; no FK or enum in DDL. |

## Screen and Workflow Audits

### 1. `dc-build` - Building Datacenter

Status: `verified/partial`.

Intent: maintain top-level physical building/facility records that group datacenter rooms, cages, MMRs, and building-level service/resource mappings.

Load behavior: menu route `dc-build`; page docs describe an index grid with filters and an inline create form. Backend docs confirm index/view/update/delete actions, access logging, `DcBuildSearch`, and pagination. Exact create POST routing is inconsistent in generated docs and should be validated if reproducing CRUD action names.

Inputs and validation: `name` max 50, `address` max 70, `status` max 10, `portale_clienti` boolean-ish tinyint/string flag, `n_rack` integer, `created_at`, optional `updated_at`, `ceased_at`. Required by feature docs: `name`, `address`, `status`, `portale_clienti`, `n_rack`, `created_at`. Status values in UI docs: `Attivo`, `Cessato`.

Mutations and side effects: creates/updates/deletes rows in `dc_build`; update of `status='Cessato'` should set or require `ceased_at` in workflows, but automatic population is not fully proven. Deletes are hard deletes by controller docs and may fail or corrupt references if child datacenters/services still point to the building.

Audit logging: actions call `ToolGr::RegAcc()`.

Data contract: `dc_build(id, name, address, status, portale_clienti, n_rack, created_at, updated_at, ceased_at)`. `datacenter.dc_build_id` references the building concept. Other docs mention ResourcePool/CDL account mappings to buildings.

Open points: exact create route, building delete protections, automatic `ceased_at` behavior, and customer portal sync details.

### 2. `datacenter-sala-cage` - Sala/Cage

Status: `verified` for route/table/filter/map/cascade core; cascade breadth is `partial`.

Intent: manage datacenter rooms/cages below buildings and above racks/equipment. These are non-MMR `datacenter` records.

Load behavior: route `datacenter&DatacenterSearch[stato]=Attivo` uses `DatacenterSearch::searchOnlyDC()`, filters `ismmr=0`, defaults to active records, and paginates at 100 rows. Detail view shows datacenter attributes and a rack grid joined to customer data.

Inputs and validation: `name`, `address`, `rack` capacity, `data_attivazione`, and `serialnumber` are required; `rack` must be integer and no more than 300. Other fields include `note`, `stato`, `id_anagrafica`, `portale_clienti`, `codice_ordine`, `dc_build_id`, `ismmr`, `set_order`, `mmr_type`, `floor`.

Mutations and side effects: create inserts `datacenter` and writes a dynamic PHP map file named `views/datacenter/{dc|mmr}/map{Name}.php`. Update can rename that map file when the datacenter name changes. When `stato='Cessato'`, backend evidence marks all racks in the datacenter, apparati in those racks, NICs on or linked to those apparati, and optical cassettes connected by `id_datacenter` or `id_datacenter_coll` as `Cessato`.

Port workflow side effects: `datacenter/freeport?id={port_id}` sets `ports.status='Empty'` and clears `pl_slots_id` and `pl_port_num`. `datacenter/addport` sets `ports.status='Linked'` when a plenum slot is supplied, otherwise `Used`.

Data contract: `datacenter(id_datacenter, name, address, note, rack, stato, id_anagrafica, portale_clienti, data_attivazione, data_cessazione, codice_ordine, dc_build_id, ismmr, set_order, mmr_type, serialnumber, floor)`. Related: `racks`, `apparato`, `nic`, `cassetti_ottici`, `plenums`, `pl_slots`, `ports`, `islets`, `positions`.

Open points: exact map rendering expectations and hard deletion safety.

### 3. `datacenter-mmr` - MeetMeRooms

Status: `verified/partial`.

Intent: manage Meet-Me Rooms as specialized `datacenter` records used for carrier and fiber interconnection workflows.

Load behavior: route `datacenter/mmr&DatacenterSearch[stato]=Attivo` uses `DatacenterSearch::searchOnlyMMR()` and filters `ismmr=1`, default active records. MMR and Sala/Cage share controller/table mechanics.

Inputs and validation: same `datacenter` fields as Sala/Cage, with `ismmr=1`. `mmr_type` appears as a compact type field; workflow docs mention `T`, `A`, and `B` MMR types. Type `T` is transit and affects cross-connect transit cable/fiber requirements.

Mutations and side effects: create/update share dynamic map file generation and rename behavior under `views/datacenter/mmr/`. Cessation follows the backend-verified datacenter cascade.

Cross-screen relationships: MMR is part of Xcon path documentation, plenum routing, ports, and optical cassette/fiber patching.

Data contract: `datacenter` with `ismmr=1`; related `xcon`, `plenums`, `pl_slots`, `ports`, `cassetti_ottici`.

Open points: exact MMR type enum behavior and carrier presence model.

### 4. `racks` - Racks

Status: `verified`.

Intent: manage physical racks inside datacenters, including space, customer, power, position occupancy, U-map visualization, and child equipment.

Load behavior: route `racks&RacksSearch[stato]=Attivo`; index uses `RacksSearch`, defaults to active status via menu query, and paginates at 100 records. Detail view builds a U-space occupancy array from U1 through rack height.

Inputs and validation: key fields are `name`, `unit`, `id_datacenter`, `id_anagrafica`, `positions_id`, `type`, `pos`, `magnetotermico`, `ampere`, `magnetotermico2`, `ampere2`, `circuitnum1`, `circuitnum2`, `sold_power`, `committed_power`, `variable_billing`, `billing_start_date`, `billing_end_date`, `serialnumber`, `codice_ordine`, `stato`. `unit` is rack height. Rack `type` is documented as `Full`/`Half`; half racks use side `pos` such as A/B.

Mutations and side effects: create inserts `racks`, generates one `units` row per U position with `num=1..unit`, validates position compatibility, updates `positions.status`, and auto-creates rack socket rows based on circuit counts. If a new rack reuses a position with a ceased rack, power socket configs may be copied. Update to `stato='Cessato'` cascades apparati and NICs to `Cessato`, optical cassettes to `Cessato`, rack sockets to `Spento`, and frees/updates position occupancy. Delete is hard delete by controller docs.

View behavior: rack detail maps empty units as `Vuoto` and active/spento apparati as occupied; multi-U apparati span consecutive entries. Detail also lists apparati and rack sockets.

Media behavior: `racks/unit?id={unit_id}` shows unit front/back photos. `racks/updunitmedia` accepts uploaded file, converts/saves PNG under `media/device/` with `{unit}-{side}-{timestamp}.png`, and inserts or updates `media(path, unit_id, side, updated_at)`.

Data contract: `racks(id_rack, name, unit, id_anagrafica, id_datacenter, stato, magnetotermico, ampere, floor, island, type, pos, racknum, positions_id, shared, reserved, note, data_attivazione, data_cessazione, codice_ordine, sold_power, serialnumber, committed_power, variable_billing, billing_start_date, billing_end_date, magnetotermico2, ampere2, circuitnum1, circuitnum2, last_update, islet_id, sconto)`. Related: `units(id, num, racks_id, device_id)`, `positions`, `rack_sockets`, `apparato`, `nic`, `cassetti_ottici`, `media`.

Open points: hard delete protections and exact half-rack pairing edge cases.

### 5. `rack-sockets` - Rack Power Sockets

Status: `verified`, with table-name conflict resolved.

Intent: track rack PDU/power socket assets, breaker identifiers, OID/device mapping fields, physical positions, and socket status.

Load behavior: route `rack-sockets`; controller `RackSocketsController` provides index/view/create/update/delete and `actionReportPower()`. Create/update/delete can take a `type` parameter so rack-context operations return to the rack page.

Inputs: `rack_id`, `magnetotermico`, `snmp_monitoring_device`, `detector_ip`, `oid`, `oid2`, `oid3`, `oid4`, `posizione`, `posizione2`, `posizione3`, `posizione4`, `status`.

Mutations and side effects: CRUD persists to `rack_sockets`. Rack create/update may generate or update socket rows based on rack circuit fields. Rack cessation sets sockets to `Spento`. Deleting a rack cascades to sockets by DDL FK `rack_sockets.rack_id -> racks.id_rack ON DELETE CASCADE`.

Reporting: DDL includes `rack_power_readings(id, oid, date, ampere, rack_socket_id)` and `rack_power_daily_summary(id, giorno, kilowatt, id_anagrafica)`. `rack_power_readings.rack_socket_id` references `rack_sockets.id`.

Data contract: source table is `rack_sockets`, not `rack_power_sockets`. Yii model terminology may be `RackPowerSockets`; controller is `RackSocketsController`.

Scope decision: device polling, import cadence, and alerting are out of V1 scope. Preserve inventory/OID fields and existing reading/summary data for compatibility/history.

### 6. `apparato` - Apparati

Status: `verified`.

Intent: manage racked equipment such as routers, switches, firewalls, servers, storage, UPS/ATS, housing/passacavo/cassetto ottico entries, and virtualization clusters.

Load behavior: route `apparato&ApparatoSearch[stato]=Attivo`; index uses `ApparatoSearch` and 100-row pagination.

Inputs and validation: key fields are `name`, `type`, `id_rack`, `unit_position`, `unit`, `ip_management`, `note`, `serial`, `os`, `model`, `id_anagrafica`, `stato`, `td_kit`, `banda`, `ordine_view_kit_gea`, `numero_porte`, `nome_porte`, `tipo_porte`, `layer_porte`, `data_attivazione`, `data_cessazione`, `proprieta_cdlan`, `cluster_name`, `tipologia_firewall`, `serialnumber`, `codice_ordine`. Required fields include `name`, `type`, and `codice_ordine`; if `type='Server'`, customer assignment is required before redirecting to server details.

Mutations and side effects: create inserts `apparato`. If `numero_porte > 0`, it auto-creates that many `nic` rows with identifiers `0/01`, `0/02`, etc., inherits `nome_porte`, `tipo_porte`, `layer_porte`, and sets `nic.stato='Attivo'`. Type-specific behavior: `Server` redirects to `server/create` with `apparato_id` and `codice_ordine`; `Firewall` saves escalation contact data in `cli_contatti_escalation`; housing/passacavo/cassetto-like types redirect to view. Update to `stato='Cessato'` cascades direct and linked NICs to `Cessato`. If a linked physical server exists, update syncs customer, order code, and serial number to `server`.

Data contract: `apparato(id_apparato, name, id_rack, unit_position, unit, ip_management, note, type, serial, os, model, id_anagrafica, stato, td_kit, banda, ordine_view_kit_gea, numero_porte, nome_porte, tipo_porte, layer_porte, data_attivazione, data_cessazione, indirizzo_installazione, indirizzo_spedizione, proprieta_cdlan, cluster_name, cliente_finale, tipo_configurazione, spedizione, installazione_onsite, monitoraggio_attivo, tipologia_firewall, serialnumber, codice_ordine, ultima_notifica)`. Related: `nic`, `racks`, `cli_fatturazione`, `server`, `cli_contatti_escalation`, `transito`.

Open points: exact device type taxonomy, monitoring fields semantics, and delete protections.

### 7. `server` - Server

Status: `verified`.

Intent: store detailed server configuration extending an `apparato` record for physical servers and supporting standalone virtual server records.

Load behavior: route `server`; index uses `ServerSearch`, 100-row pagination, and filters by customer, hostname, OS, rack, status, and order code. Detail view loads server plus related cards/applications/services/ports.

Inputs and validation: required fields in docs include `tipologia`, `codice_ordine`, `name`, `stato`, `sistema_operativo`, `n_cpu`, `totale_ram`, `dischi`, `data_attivazione`, `id_anagrafica`. DDL fields include `hostname`, `id_rack`, `unit`, `unit_position`, `slot`, virtualization fields, hardware specs (`n_socket_cpu`, `n_cpu`, `n_core`, `totale_ram`, `banchi_ram`, `dischi`, `livello_raid`, `hotspare`), management fields (`ilo_idrac`, `user_ilo`, `pwd_ilo`, `ip_mngt`), access fields (`root_administrator_password`, `utenza_cliente`, `pwd_utenza_cliente`, `utenza_cdlan`, `pwd_utenza_cdlan`), syslog/backup/patching notes, `apparato_id`, `serialnumber`, `porte`.

Mutations and side effects: physical create is usually reached from `apparato/create` and links by `apparato_id`; successful create redirects to the linked apparato view. `actionCreatevirtua()` creates virtual servers without physical apparato linkage. Physical update with `tipologia='Fisico'` and `apparato_id > 0` syncs `id_anagrafica`, `codice_ordine`, and `serialnumber` back to `apparato`. Password fields `pwd_ilo`, `root_administrator_password`, and `pwd_utenza_cdlan` are encrypted on insert/update with `Yii::$app->params['k_crypt']`. `pwd_utenza_cliente` is credential-like in DDL but encryption is not proven by the cited behavior.

Related views: `server_schede(id_server, nome_fisico, nome_os, ip, id_subnetmask, note)`, `server_applicazioni(id_server, name, gestito_da_cdlan)`, `server_servizi(id_server, name)`, `server_porte(id_server, interface_name, destination_interface, port_type)`.

Data contract: `server` plus related component tables listed above; related `apparato`, `racks`, `cli_fatturazione`.

Coexistence requirement: the new app must use existing stored credential values while coexisting with Grappa. Preserve legacy encrypted fields and encryption/decryption compatibility where those fields remain used.

Open points: backup/syslog execution semantics and delete protections.

### 8. `storage` - Storage

Status: `verified`.

Intent: track storage allocations/volumes assigned to customers and underlying storage apparati.

Load behavior: route `storage`; index uses `StorageSearch`, 100-row pagination, and filters by customer, apparato, status, access protocol, and size.

Inputs and validation: `cli_fatturazione_id`, `apparato_id_apparato`, `size`, `size_type`, `status`, and `serial_number` are required by page docs; optional fields include `access_protocol`, `codice_ordine`, `note`. `size_type` is documented as GB/TB but DDL is `varchar(2)`.

Mutations and side effects: create/update persist `storage`. View/update/delete/find require composite key `(id, cli_fatturazione_id, apparato_id_apparato)`. Delete hard-deletes. Archive sets `status='Chiuso'`, sets `closed_at` to current timestamp, flashes success, and redirects to index.

Data contract: `storage(id, access_protocol, size, cli_fatturazione_id, apparato_id_apparato, note, size_type, status, created_at, closed_at, codice_ordine, serial_number)` with composite PK `(id, cli_fatturazione_id, apparato_id_apparato)` and FKs to `cli_fatturazione.id` and `apparato.id_apparato`.

Open points: billing integration, active status value, capacity unit normalization, and delete-vs-archive policy.

### 9. `plenums` - Plenum

Status: `verified`.

Intent: manage cable pathway structures and plenum slots used to route fibers/ports between racks, MMRs, and datacenters.

Load behavior: route `plenums`; index uses `PlenumsSearch`, 100-row pagination, and filters by name, isle, type, datacenter, and status. Detail view requires composite key `(id, datacenter_id)` and lists `pl_slots` for the plenum.

Inputs and validation: `datacenter_id` and `status` are required; `name`, `isle`, and `type` are optional strings max 45. `type` is used as pathway/MMR classification in docs.

Mutations and side effects: create inserts `plenums` but does not auto-create slots. Update/delete redirect inconsistently to `dcimadmin/plenums` in backend docs. Delete hard-deletes `plenums`; docs call out no cascade validation and risk orphaning `pl_slots` or `ports` that reference plenum slots.

Visual behavior: extended feature docs describe the plenum view as a 2-cable by 12-port map, 24 connection points total, with occupied/available styling and popovers for connected rack/slot/customer/FO details.

Data contract: `plenums(id, name, isle, type, datacenter_id, status)` with composite PK `(id, datacenter_id)` and FK to `datacenter.id_datacenter`. `pl_slots(id, plenums_id, num, cable, type, status, slots_id, mmr_slot_id)`. `ports(id, slots_id, num, status, pl_slots_id, pl_port_num, fo_in_id, fo_out_id, unit, rack_id, plenum_id, device_id, name, cable_fiber_id)`.

Open points: exact plenum/slot status values, slot creation workflow owner, and safe delete rules.

### 10. `anelli-fibra` - Anelli Fibra

Status: `verified`.

Intent: manage circular fiber ring topologies, their nodes/arcs, customer ownership, and optional geographic KML visualization.

Load behavior: route `anelli-fibra`; index uses `AnelliFibraSearch`, 100-row pagination, and filters by name, customer, node count, and status. Detail view lists nodes and arcs for the ring.

Inputs and validation: `nome` and `n_nodi` are required; `id_anagrafica` optional integer; `note`, `serialnumber`, `codice_ordine`, `kml_file_path`, `stato` strings/text. Create sets `stato='Attivo'`.

Mutations and side effects: create inserts `anelli_fibra`, optionally uploads a KML file to `uploads/kml/`, checks duplicate KML filename against existing ring rows, inserts `mappa_tracciati_anelli`, auto-creates `n_nodi` `nodi` rows with `identificativo=1..n` and `posizione=n*100`, and auto-creates `n_nodi` `archi` rows connecting each node to the next and the last back to the first. Initial `archi.distanza` and `archi.attenuazione` are `0`. Update can increase `n_nodi` and add nodes/arcs; decreasing node count is not supported. No transaction wrapping is documented.

V2 decision: Hive upload/sync for KML maps is post-porting work, not a V1 requirement.

Data contract: `anelli_fibra(id_anello, nome, id_anagrafica, n_nodi, note, serialnumber, codice_ordine, kml_file_path, stato)`, `nodi(id_nodo, identificativo, indirizzo, id_foglio_linee, id_anagrafica, id_anello, longitudine, latitudine, posizione, switch and east/west port/transceiver fields, note)`, `archi(id_arco, id_anello, id_nodo_da, id_nodo_a, distanza, attenuazione, riferimento, riferimento_metroweb, data_rilascio)`, `mappa_tracciati_anelli(id, nome, kml, nome_anello, dettagli_tracciato)`.

Open points: delete cascade behavior and how arc distances/attenuation are later updated.

### 11. `xcon` - Cross Connect

Status: `verified` from legacy PHP repository validation and production data notes.

Intent: manage sold customer cross-connect circuits/interconnections, including A/Z endpoint documentation, status, product type, purchase/source references, Letter of Agency references, MMR port notes, and optional multi-hop path documentation.

Source of truth: the current Cross Connect menu uses `controllers/XconController.php`, model `Xcon`, table `xcon`, and views under `views/xcon`. `XconController` saves `xcon` and `xcon_hop` only; it does not update ports, cables, racks, fibers, or other inventory entities.

Load behavior: route `xcon` opens the active Cross Connect workspace. The index separates records into `Attive` and `Cessate` tabs using `stato != 'cessata'` and `stato = 'cessata'`; `annullato` records therefore appear in the active-tab query rather than the ceased-tab query.

Status behavior: `xcon.stato` values are `Bozza`, `Verifica Tecnica`, `in attivazione`, `Intervento Utente Richiesto`, `Attiva`, `libera`, `non cablato`, `cessata`, and `annullato`. `XconController::getOptionStatus()` controls which options are enabled in the UI based on current state. `cessata` and `annullato` are terminal states. Preserve exact case and spelling.

Product type: `xcon.tipo` is a static product/type selector chosen according to the cross-connect product sold to the customer. Values are `CDL-XLOCAL`, `CDL-XSEEODF`, `CDL-XCAMPUS`, `CDL-XIRI`, `CDL-XIRIODF`, `CDL-XMIX`, and `CDL-XMIXODF`.

`xcon` table contract: `xcon(id, ticket, pa, cliente, stato, num_ordine, riga_ordine, tipo, data_attivazione, data_cessazione, aend_unita_app, aend_slot, aend_fibre, aend_apparato, zend_unita_app, zend_slot, zend_fibre, zend_apparato, note, ticket_esteso, note_cliente, sorgente, created_at, aend_rack_id, zend_rack_id, loa_name, loa_id, mmr_port)`. FKs: `aend_rack_id` and `zend_rack_id` to `racks.id_rack`. `xcon_hop(id, xcon_id, hop_room, hop_rack, hop_unita_app, hop_slot, hop_fibre, hop_num, rack_id)` supports intermediate hops.

Field semantics: `loa_name` and `loa_id` store Letter of Agency references. `mmr_port` stores Meet-Me Room port information. `pa` is a purchase reference. `sorgente` is the non-user-editable source system, usually `CustomerPortal` for portal-originated records or `AssetManager` for records created through Grappa.

Hop behavior: `xcon_hop` stores optional ordered intermediate path points for multi-hop paths only. Endpoint A/Z data lives on `xcon`; intermediate routing detail lives in `xcon_hop` and is ordered by `hop_num`. Hop fields capture rack/location, unit, slot, and fiber information for each intermediate segment.

Open points: exact operational meaning of `ticket_esteso`, `num_ordine`, `riga_ordine`, and any product-specific interpretation of the `CDL-X*` codes beyond the static selector.

### 12. `kitgraph-kitview` - Kit Gea Tim

Status: `verified` as read-only report/export.

Intent: visualize TIM GEA kit devices, port use, bandwidth use, and customer line assignments.

Load behavior: route `kitgraph/kitview` calls helper `Kit()` and renders arrays for kit devices and per-port data. No search/filter parameters are documented.

Data logic: selects `apparato` rows where `stato='Attivo'` and `td_kit` is non-empty, ordered by `ordine_view_kit_gea`. For each device, loads NICs and joins `eth` to `foglio_linee` through `Eth::joinWith('accessi')`, filtered by `id_apparato_fornitore`, `id_nic_fornitore`, `lato='cdlan'`, and `foglio_linee.stato='Attiva'`. It sums `foglio_linee.banda`, counts used/free ports, and derives port index from the last two characters of `nic.identificativo`.

Mutations and side effects: main view is read-only. `kitgraph/kitexp` exports XLS using PhpSpreadsheet, writes `downloads/reportkit-tim-gea.xls`, then sends the file. Device blocks are 10 rows each; port grid spans 24 ports with occupied/free styling.

Data contract: `apparato(id_apparato, name, td_kit, stato, banda, ordine_view_kit_gea, id_rack)`, `nic(id_nic, id_apparato, name, identificativo, stato, type, layer)`, `eth(id_eth, id_linea, lato, id_apparato_fornitore, id_nic_fornitore, ...)`, `foglio_linee(id, stato, banda, cogn_rsoc_intest_linea, comune, td_linea_telecom, ...)`.

Open points: whether every kit is exactly 24 Fast Ethernet ports and how non-standard kit devices should render.

### 13. `cwdm` - CWDM

Status: `verified`.

Intent: manage CWDM optical multiplexing devices and their wavelength channels, including mirrored pairs across datacenters/racks.

Load behavior: route `cwdm&CwdmSearch[stato]=Attivo`; index uses `CwdmSearch`, defaults to active status via menu query, and paginates at 100. Detail view lists associated `nic` rows where `id_cwdm` matches.

Inputs and validation: required by model docs: `identificativo`, `stato`, `tipo_connettore`, `unit`, `unit_position`. Optional/location fields: `id_datacenter`, `id_rack`, `id_datacenter_coll`, `id_rack_coll`, `serial`, `id_anagrafica`, `note`.

Mutations and side effects: create inserts `cwdm`, creates 10 `nic` optical channels for `trunk`, `1310`, `1470`, `1490`, `1510`, `1530`, `1550`, `1570`, `1590`, `1610` with `name='Collegamento Fisico (CO)'`, `type='Physical'`, `layer='Collegamento-Fisico'`, `stato='Attivo'`, and `id_cwdm` set. If remote datacenter/rack fields are present and no existing mirror exists at `(id_datacenter_coll, id_rack_coll, unit_position)`, it creates a reciprocal mirror CWDM with inverted local/remote references and another 10 NICs. It appends direction suffixes to `identificativo`, such as `(DC-A - DC-B)`. Update to `stato='Cessato'` runs a bulk NIC update where `id_cwdm = X OR link_id_cwdm = X`, setting NICs to `Cessato`. Delete hard-deletes the CWDM but does not automatically delete or update the mirror device per backend notes.

Data contract: `cwdm(id_cwdm, identificativo, id_datacenter, id_rack, id_datacenter_coll, id_rack_coll, tipo_connettore, unit_position, unit, serial, id_anagrafica, note, stato)`. Related `nic` fields: `id_cwdm`, `link_id_cwdm`, `link_id_nic_cwdm`, `identificativo`, `name`, `type`, `layer`, `stato`.

Open points: no explicit mirror FK, mirror deletion orphaning, active wavelength usage validation before cessation/delete, and connector type vocabulary.

### 14. `dcimadmin-cable` - Cavi

Status: `verified`.

Intent: administer physical cables and individual fibers used by port, plenum, and cross-connect workflows.

Load behavior: route `dcimadmin/cable`; `actionCable()` displays cable grid and create form.

Inputs and validation: cable create requires `name`, `description`, `fibers_num`, and `status`; `name` and `description` max 150, `status` max 50, `fibers_num` integer. Fiber records carry `num`, `status`, `cable_id`, `left_port_id`, `right_port_id`.

Mutations and side effects: create checks duplicate cable name in an odd way documented as checking against the `Plenums` table, then saves `cables` and auto-generates `fibers_num` child `fibers` rows numbered 1..N, all `status='Libera'`. Cable view lists fibers ordered by `num`. `actionFiberUpdate($id)` clears old port assignments by setting old `ports.cable_fiber_id=NULL`, saves new `left_port_id`/`right_port_id`, and sets new ports' `cable_fiber_id` to the fiber id. `actionFiberbytransit()` returns JSON for available transit fibers filtered by selected cable, `status='Libera'`, and both ports assigned.

Data contract: `cables(id, name, description, fibers_num, status)`, `fibers(id, num, status, cable_id, left_port_id, right_port_id)`, `ports.cable_fiber_id`.

Open points: large fiber counts can create performance/timeouts; concurrent port/fiber assignment can double-book resources; cable deletion with in-use fibers is high risk.

### 15. `dcimadmin-cam` - Telecamere

Status: `verified`, despite older conflicting note.

Intent: maintain a simple surveillance camera inventory for datacenter physical security.

Load behavior: route `dcimadmin/cam`; `actionCam()` handles GET list/create form and POST create. `actionCamUpd($id)` handles update form and update save, then redirects to `dcimadmin/cam`.

Inputs and validation: app-level required fields are `code`, `model`, `brand`, `position`, each max 50. Optional fields: `ipaddr`, `serial`, `status`, max 50. DDL columns are nullable, so requiredness is model-level, not database-level.

Mutations and side effects: create inserts `cams` and flashes "Record inserted". Update changes `cams` and flashes "Record updated". No delete action is documented as supported in current camera behavior; decommissioning is done by changing `status`/metadata. Actions are logged via `ToolGr`.

Data contract: `cams(id, code, model, brand, position, ipaddr, status, serial)`. DDL has indexes on `serial`, `ipaddr`, and `code`. No foreign keys to datacenter/rack/position are declared.

Known risks: duplicate IPs and codes are not DB-enforced; invalid IP formats are possible; no optimistic locking; update redirects lose pagination context; older progress docs wrongly state cameras were not implemented.

Scope decision: cameras are inventory only. Do not infer or implement external camera monitoring/NVR/DVR integration for V1.

### 16. `islets` - Isole

Status: `verified`.

Intent: manage physical islets/rows/sides that group rack positions inside a datacenter.

Load behavior: route `islets`; index uses `IsletsSearch` and 100-row pagination. View/update/delete require composite key `(id, datacenter_id)`.

Inputs and validation: required `datacenter_id`, `name`, `rack_num`, `type`; optional `floor`, `serial`, `order`, `clifat_id`. `rack_num` integer max 35. `type` values: `isle`, `row`, `side`. `clifat_id` optionally assigns a dedicated customer. Label `lbl_islet` is computed as `{datacenter_name}-{type_label}-{name}`, where type labels are `Isola`, `Fila`, `Lato`.

Mutations and side effects: create/update persist `islets`. Delete manually finds all `positions` where `islets_id` equals the islet id, deletes them one by one, then deletes the islet. This can orphan or invalidate rack assignments because racks can reference deleted positions.

Data contract: `islets(id, datacenter_id, name, rack_num, type, floor, serial, order, clifat_id)` with composite PK `(id, datacenter_id)` and FK `datacenter_id -> datacenter.id_datacenter`. Related: `positions`, `racks`, `cli_fatturazione`.

Open points: delete behavior for occupied islets, whether dedicated customer assignment is exclusive, and whether large datacenters need more than 35 positions per islet.

### 17. `positions` - Posizioni

Status: `verified`.

Intent: manage numbered rack positions within islets and track free/occupied layout state.

Load behavior: route `positions`; index uses `PositionsSearch` and 400-row pagination. View/update/delete require composite key `(id, islets_id)`.

Inputs and validation: required `status`, `type`, `num`, `islets_id`; `num` integer max 35; `status` and `type` strings max 45. Common status values are `free`, `occupied`, possibly `reserved`; type values are `full`, `half`, possibly `quarter`. Computed labels: `lbl_position = lbl_islet + ' pos: ' + num`.

Mutations and side effects: individual create/update/delete persist `positions`. Update can assign a rack when a POST field `rack` is present: it finds `racks.id_rack`, sets `racks.positions_id` to the position id, saves the rack, sets `positions.status='occupied'`, and saves the position. Delete does not explicitly clear rack assignments.

Batch workflow: `positions/batch&id={islet_id}&rack_num={num}&datacenter_id={dc_id}` first checks whether any positions already exist for the islet. If any exist, it blocks batch creation with an error flash. If none exist, it creates positions 1..`rack_num`, all `status='free'`, `type='full'`, then writes a dynamic map view file at `views/datacenter/dc/islets/map{datacenter}-{islet}.php` that renders a row for each position via the datacenter rack detail partial. It flashes success and redirects to the islet view.

Data contract: `positions(id, status, type, num, islets_id)` with composite PK `(id, islets_id)` and FK `islets_id -> islets.id`. Related: `racks.positions_id`.

Open points: whether moving/removing a rack resets previous position to `free`, how occupied position deletion should be handled, and what happens if file generation fails after position rows are created.

## Data and Integration Catalog

### Core Physical Hierarchy

| Table | Purpose | Important fields and contracts |
|---|---|---|
| `dc_build` | Building/facility catalog | `id`, `name`, `address`, `status`, `portale_clienti`, `n_rack`, `created_at`, `updated_at`, `ceased_at`. Status UI values `Attivo`/`Cessato`; portal flag controls customer visibility. |
| `datacenter` | Rooms, cages, MMRs | `id_datacenter`, `name`, `address`, `rack`, `stato`, `id_anagrafica`, `portale_clienti`, `data_attivazione`, `data_cessazione`, `codice_ordine`, `dc_build_id`, `ismmr`, `set_order`, `mmr_type`, `serialnumber`, `floor`. `ismmr=0` for Sala/Cage, `ismmr=1` for MMR. |
| `islets` | Physical groups inside datacenters | Composite PK `(id, datacenter_id)`, fields `name`, `rack_num`, `type`, `floor`, `serial`, `order`, `clifat_id`; `rack_num <= 35`; type `isle`/`row`/`side`. |
| `positions` | Numbered rack locations inside islets | Composite PK `(id, islets_id)`, fields `status`, `type`, `num`, `islets_id`; batch creation initializes `free/full`; rack assignment sets `occupied`. |
| `racks` | Rack inventory | `id_rack`, `name`, `unit`, `id_datacenter`, `id_anagrafica`, `stato`, `positions_id`, `type`, `pos`, power/circuit fields, dates, order/serial, `islet_id`. Creates `units` and `rack_sockets`; cessation cascades to child equipment/power/position state. |
| `units` | Rack U positions | `id`, `num`, `racks_id`, `device_id`; one row per rack U generated on rack create. |
| `slots` | Rack slots/panels for ports | Composite PK `(id, racks_id)`, fields `unit`, `name`, `ports_num`, `cabled`, `status`, `type`, `mapped_dc`, `unit_lenght`, `isle`, `cable`, `plslots_id`; used by port and plenum workflows and referenced textually in Xcon endpoint fields. |
| `media` | Rack unit photos | `id`, `path`, `unit_id`, `side`, `updated_at`; rack media upload stores PNG files under `media/device/`. |

### Equipment, Compute, and Storage

| Table | Purpose | Important fields and contracts |
|---|---|---|
| `apparato` | Racked equipment | `id_apparato`, `name`, `id_rack`, `unit_position`, `unit`, `ip_management`, `type`, `serial`, `os`, `model`, `id_anagrafica`, `stato`, `td_kit`, `banda`, `numero_porte`, port defaults, activation/cessation dates, firewall/monitoring flags, `serialnumber`, `codice_ordine`. Auto-generates NICs on create. |
| `nic` | Equipment/CWDM/cassette ports | `id_nic`, `id_apparato`, `identificativo`, `name`, `id_anagrafica`, `type`, link fields to apparatus/cassettes/server/CWDM, `layer`, `stato`. Used heavily for port generation and cessation cascades. |
| `server` | Server details | `id_server`, `tipologia`, `name`, `id_anagrafica`, `stato`, OS/hardware/virtualization/location/access/backup fields, `apparato_id`, `codice_ordine`, `serialnumber`, `porte`. Physical server syncs to `apparato`; selected passwords encrypted with `k_crypt`. |
| `server_schede` | Server interfaces/cards | `id_server_scheda`, `id_server`, `nome_fisico`, `nome_os`, `ip`, `id_subnetmask`, `note`. |
| `server_applicazioni` | Server applications | `id_server_applicazione`, `id_server`, `name`, `gestito_da_cdlan`. |
| `server_servizi` | Server services | `id_server_servizio`, `id_server`, `name`. |
| `server_porte` | Server port exposure/config | `id`, `id_server`, `interface_name`, `destination_interface`, `port_type`. |
| `storage` | Storage allocation/device records | Composite PK `(id, cli_fatturazione_id, apparato_id_apparato)`, fields `access_protocol`, `size`, `size_type`, `status`, `created_at`, `closed_at`, `codice_ordine`, `serial_number`; archive sets `Chiuso`. |
| `cli_contatti_escalation` | Firewall escalation contacts | `id`, `servizio`, `id_cliente`, `id_contatto`, `priorita_ingaggio`, `apparati`; used when creating firewall apparati. |
| `transito` | Transit/service references tied to equipment/NICs | `id_transito`, order/customer/status/device/NIC/line fields; referenced by apparato relations. |

### Power, Cabling, Ports, and Cross-Connects

| Table | Purpose | Important fields and contracts |
|---|---|---|
| `rack_sockets` | Rack PDU/socket inventory | `id`, `rack_id`, `magnetotermico`, `snmp_monitoring_device`, `detector_ip`, `oid`..`oid4`, `posizione`..`posizione4`, `status`; FK to `racks` cascades on rack delete. |
| `rack_power_readings` | Raw power readings | `id`, `oid`, `date`, `ampere`, `rack_socket_id`; FK to `rack_sockets.id`. Large table by DDL auto-increment evidence. |
| `rack_power_daily_summary` | Aggregated power summary | `id`, `giorno`, `kilowatt`, `id_anagrafica`; indexed by day/customer. |
| `plenums` | Cable pathways | Composite PK `(id, datacenter_id)`, fields `name`, `isle`, `type`, `datacenter_id`, `status`. |
| `pl_slots` | Plenum cable slots | Composite PK `(id, plenums_id)`, fields `num`, `cable`, `type`, `status`, `slots_id`, `mmr_slot_id`; connects plenums to rack/MMR slots. |
| `ports` | Fiber/physical ports | Composite PK `(id, slots_id)`, fields `num`, `status`, `pl_slots_id`, `pl_port_num`, `fo_in_id`, `fo_out_id`, `unit`, `rack_id`, `plenum_id`, `device_id`, `name`, `cable_fiber_id`. Status drives availability. |
| `cables` | Cable bundles | `id`, `name`, `description`, `fibers_num`, `status`; create auto-generates child fibers. |
| `fibers` | Individual fiber strands | `id`, `num`, `status`, `cable_id`, `left_port_id`, `right_port_id`; new fibers start `Libera`; preserve existing values such as `Occupata` exactly. |
| `xcon` | Active cross-connect circuit/path registry | Fields `ticket`, `pa`, `cliente`, `stato`, `num_ordine`, `riga_ordine`, `tipo`, activation/cessation dates, A/Z endpoint text fields, A/Z rack FKs, notes, `sorgente`, Letter of Agency fields, `mmr_port`. |
| `xcon_hop` | Optional multi-hop Xcon path points | `id`, `xcon_id`, `hop_room`, `hop_rack`, `hop_unita_app`, `hop_slot`, `hop_fibre`, `hop_num`, `rack_id`; only used when a path has intermediate hops. |

### Fiber Rings, Optical Devices, Cameras, and Reports

| Table | Purpose | Important fields and contracts |
|---|---|---|
| `anelli_fibra` | Fiber ring definitions | `id_anello`, `nome`, `id_anagrafica`, `n_nodi`, `note`, `serialnumber`, `codice_ordine`, `kml_file_path`, `stato`; create auto-generates nodes/arcs. |
| `nodi` | Ring nodes | `id_nodo`, `identificativo`, `indirizzo`, `id_foglio_linee`, `id_anagrafica`, `id_anello`, coordinates, `posizione`, switch/port/transceiver metadata. |
| `archi` | Ring arcs | `id_arco`, `id_anello`, `id_nodo_da`, `id_nodo_a`, `distanza`, `attenuazione`, references, release date; unique key on ring/from/to. |
| `archi_tratta` | Arc segment/channel details | `id_tratta`, `id_arco`, source/destination cabinet/level/cable/fiber/segment fields, lengths. |
| `mappa_tracciati_anelli` | Ring map/KML metadata | `id`, `nome`, `kml`, `nome_anello`, `dettagli_tracciato`. |
| `cwdm` | Optical multiplexers | `id_cwdm`, local and remote datacenter/rack fields, connector, rack unit fields, serial, customer, note, `stato`; creates 10 NIC wavelength channels and optional mirror. |
| `cassetti_ottici` | Optical cassettes/patch panels | `id_cassetto_ottico`, local/remote datacenter/rack fields, connector, unit position/height, serial, customer, mono/multi-mode port counts, note, `stato`; dependency only in this audit. |
| `cams` | Camera inventory | `id`, `code`, `model`, `brand`, `position`, `ipaddr`, `status`, `serial`; standalone table, no FKs. |
| `eth` | Ethernet/customer access link used by kit graph | `id_eth`, `id_linea`, `lato`, CDLAN/provider apparatus/NIC fields, `kit_fornitore`, speed, radius user, notes. |
| `foglio_linee` | Customer line/access records used by kit graph | Many telecom fields; kit graph uses `id`, `stato='Attiva'`, `banda`, `cogn_rsoc_intest_linea`, `comune`, `td_linea_telecom`. |
| `cli_fatturazione` | Customer/billing master | `id`, `intestazione`, customer grouping/codes, portal/access flags, xcon eligibility flags (`localxcon`, `mixxcon`, `irideosxcon`, `seewebxcon`), `stato`. |

### Integrations and Generated Artifacts

| Integration/artifact | Status | Behavior contract |
|---|---|---|
| ToolGr access logging | `verified` | Many actions instantiate `ToolGr` and call `RegAcc(controller, action)` to log access/action. |
| Generated map files | `verified/equivalent-target` | Datacenter and position workflows write PHP map files in legacy. V1 target must preserve equivalent user-visible floor/islet map behavior, not exact PHP view-file generation. Existing historical files should be migrated only when referenced by active records or required for history. |
| Rack unit media files | `verified/equivalent-target` | Rack unit upload saves converted PNG under `media/device/` and updates `media` rows. V1 target must preserve equivalent media behavior and existing referenced media. |
| Server credential encryption | `verified/coexistence requirement` | `pwd_ilo`, `root_administrator_password`, and `pwd_utenza_cdlan` are encrypted with `Yii::$app->params['k_crypt']` on insert/update. Because V1 must coexist with Grappa, preserve existing stored values and encryption/decryption compatibility. |
| KML/map integration | `V2` | Fiber ring KML files and map metadata should be preserved for compatibility/history. Hive upload/sync is post-porting V2 work, not a V1 requirement. |
| TIM GEA XLS export | `verified` | `kitgraph/kitexp` writes and sends `downloads/reportkit-tim-gea.xls` using PhpSpreadsheet. |
| Rack power readings | `out of V1 polling scope` | `rack_sockets` OID fields and reading/summary tables exist. Device polling, cadence, and alerts are out of scope for V1; preserve existing fields/data only. |

## Migration Fact Sheet

| Fact | Status | Verified behavior | Implementation risk | Validation check |
|---|---|---|---|---|
| Standalone scope | `verified` | This audit covers active DCIM migration surfaces plus `islets` and `positions`. | Dead or dependency-only screens could distract implementation. | Inventory contains active V1 surfaces only. |
| Original source unavailable | `verified` | Evidence is reverse docs plus DDL, not runnable Yii2 source in this repo. | Downstream agent may over-trust path citations. | Handoff explicitly states source limitation. |
| `rack_sockets` authoritative | `conflicting/resolved` | DDL/table docs prove table `rack_sockets`; some docs say `rack_power_sockets`. | Import/mapping to wrong table. | Schema mapping uses `rack_sockets`; treats `RackPowerSockets` as model concept. |
| Cameras implemented | `conflicting/resolved` | `dcimadmin/cam`, `Cam`, and `cams` are documented and in DDL. | Omitting camera screen due older notes. | Camera CRUD/create/update covered; no delete assumed. |
| Xcon authoritative | `verified` | Legacy PHP validation proves current Cross Connect uses `XconController`, `xcon`, and `xcon_hop`. | Building the wrong workflow would break active customer cross-connect data. | V1 contract uses only `xcon` and optional `xcon_hop`. |
| Default active filters | `verified` | Datacenter, racks, apparato, cwdm menu routes include active status filters. | Cessato records vanish or appear incorrectly. | Fixture with active/ceased rows confirms default filters. |
| Cessation cascades | `verified` | Datacenter/rack/apparato/CWDM cascades mark backend-verified child resources ceased/spento. | Active children under ceased parents or over-broad deactivation. | Mark parent ceased and assert only backend-verified child status changes. |
| Generated maps/media/exports | `verified/equivalent-target` | Legacy writes PHP map files, rack media files, KML files, and XLS exports. | Losing user-visible artifacts changes operations. | Preserve equivalent behavior and migrate referenced historical artifacts where needed. |
| Rack unit generation | `verified` | Rack create generates one `units` row per rack U. | Rack U-map/media workflows fail. | Create 42U rack and assert 42 units. |
| Position occupancy | `verified/partial` | Batch initializes `free/full`; rack assignment sets position `occupied`; previous position cleanup unresolved. | Double-booked positions or stale occupancy. | Assign/move/delete rack and assert position states. |
| Apparato NIC generation | `verified` | `numero_porte` creates sequential `nic` rows `0/01` etc. | Devices lose port inventory. | Create apparato with 4 ports and assert 4 NICs. |
| Server password encryption | `verified/coexistence` | Selected server passwords encrypted with `k_crypt`; V1 must use existing values while coexisting with Grappa. | Plaintext leakage or unreadable migrated credentials. | Confirm compatibility with existing encrypted fields. |
| Server/apparato sync | `verified` | Physical server updates sync customer/order/serial to linked apparato. | Ownership/order drift. | Update server and assert linked apparato fields. |
| Storage archive | `verified` | Archive sets `status='Chiuso'` and `closed_at`; delete is separate hard delete. | Soft-closed storage could be deleted. | Archive row remains with `Chiuso`. |
| Fiber ring topology | `verified` | Ring creation makes N nodes and N circular arcs; nodes spaced by `posizione=n*100`. | Broken topology or partial rows after failure. | Create N-node ring and assert N nodes/N arcs. |
| Cable fiber generation | `verified` | Cable create generates fibers 1..`fibers_num`, all `Libera`. | Cable exists without strands. | Create 48-fiber cable and assert 48 fibers. |
| Xcon no inventory side effects | `verified` | `XconController` saves `xcon` and `xcon_hop` only. | Accidental port/fiber/rack mutations would diverge from active legacy behavior. | Xcon create/update/delete tests assert no unrelated inventory writes. |
| CWDM channels/mirror | `verified` | CWDM create makes 10 NICs; optional remote mirror makes another 10. | Optical topology loses channel/mirror parity. | Create linked CWDM and assert both devices/channel sets. |
| Kit graph export | `verified` | Read-only view plus XLS export from active `td_kit` apparati and active lines. | Report differs from legacy utilization. | Fixture with 24 ports renders/export expected cells. |
| Islet delete cascade | `verified/risky` | Islet delete deletes child positions first. | Racks linked to those positions become invalid. | Delete occupied islet scenario requires domain decision. |

## Risks and Open Questions

### High-Risk Verified Behaviors

- Hard deletes exist for multiple infrastructure records while other workflows use soft statuses (`Cessato`, `Chiuso`, `Spento`). Parity requires knowing which destructive actions remain available.
- V1 preserves hard-delete behavior but every destructive delete must require double user confirmation. Safer deactivate/archive semantics and dependency-aware delete blocking are V2 improvements.
- Dynamic filesystem writes are legacy implementation artifacts; V1 must preserve equivalent user-visible behavior for maps, rack media, KML metadata/files, and XLS exports.
- Credential-bearing server fields must remain compatible with Grappa while the new app coexists with the legacy app.
- V1 vocabulary policy: preserve legacy free-text status/type values exactly as stored, including case, spelling, and mixed-language labels. UI picklists may offer known values but must tolerate and round-trip unknown existing values.

### Required Validation Questions

1. What should happen when deleting or editing occupied islets/positions in V1 beyond double confirmation?
2. Are TIM GEA kit devices always 24 ports, or is 24 a legacy assumption?
3. Are camera `code`, `ipaddr`, or `serial` expected to be unique even though DDL does not enforce it?
4. What are the exact product-specific meanings of `xcon.ticket_esteso`, `num_ordine`, `riga_ordine`, and each `CDL-X*` product code if they must be shown with explanatory labels?

## Handoff Notes for Migration Specification

- Treat this file as a source audit and fact sheet, not a target implementation plan.
- Preserve original source names in migration contracts until domain owners approve changes: `dc_build`, `datacenter`, `racks`, `rack_sockets`, `apparato`, `nic`, `server`, `storage`, `plenums`, `pl_slots`, `ports`, `cables`, `fibers`, `xcon`, `xcon_hop`, `cwdm`, `cams`, `islets`, `positions`.
- Acceptance tests should focus on legacy behavior: default filters, generated child rows, backend-verified status cascades, Xcon field/value compatibility, credential encryption compatibility, generated artifact equivalence, exports, and double confirmation for destructive deletes.
- Do not use local provenance paths as required context. They are evidence citations only.
- Keep migration parity separate from improvements. Safer deletes, normalized enums, Hive sync, device polling, and exact file-to-data layout replacement are V2 modernization decisions, not V1 parity requirements.
