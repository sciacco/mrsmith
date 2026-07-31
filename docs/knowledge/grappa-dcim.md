# Grappa DCIM

Knowledge entries specific to `apps/grappa-dcim`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Grappa Rack Customer Display Uses `cli_fatturazione.intestazione`

- Context: Grappa DCIM rack search, rack registry filtering, and any Grappa UI that needs a human-readable customer name for `racks.id_anagrafica`.
- Discovery: `racks.id_anagrafica` stores the internal Grappa customer ID, which resolves to `cli_fatturazione.id`; the customer display name is `cli_fatturazione.intestazione`.
- Practical rule: for rack search/display, join `racks.id_anagrafica -> cli_fatturazione.id` and expose `cli_fatturazione.intestazione` as the customer name. Do not show only the numeric customer code when a customer name is needed.
- Evidence: `docs/grappa/grappa_cli_fatturazione.json`, `docs/grappa/grappa_racks.json`, Grappa DCIM rack search contract in `backend/internal/grappadcim/racks.go`.
- Used by: `apps/grappa-dcim` Rack search.
- Open questions: none.

### Grappa Rack Equipment Occupancy Uses `apparato.unit`

- Context: Grappa DCIM rack detail U-map and any UI that places `apparato` rows in rack units.
- Discovery: `apparato.unit_position` is the starting rack U, while `apparato.unit` is the number of rack units occupied by the equipment. A multi-U apparatus must cover consecutive rack units starting at `unit_position`.
- Practical rule: expose a business-facing occupied height field, such as `occupiedUnits`, derived from `apparato.unit` with a 1U fallback. Do not treat `apparato.unit` as an alternate position when rendering the rack map.
- Evidence: `docs/grappa/grappa_apparato.json`, Grappa DCIM audit note in `apps/grappa-dcim/docs/GRAPPA-DCIM.md`, and rack detail implementation in `apps/grappa-dcim/src/features/racks`.
- Used by: `apps/grappa-dcim` Rack detail.
- Open questions: none.

### Grappa Apparato Types Are Controlled By DCIM Lookup

- Context: Grappa DCIM apparato create/update and any UI that presents `apparato.type`.
- Discovery: MrSmith owns a Grappa-side `dcim_equipment_type_visuals` lookup for the allowed apparato types and their presentation metadata. The legacy `apparato.type` column remains textual for compatibility, but user create/update flows must use only active lookup values.
- Practical rule: read type choices from `GET /api/grappa-dcim/v1/equipment/type-options`; do not rebuild the picker from distinct `apparato.type` values. Existing historical rows with non-lookup values may still be displayed, but new writes must be rejected unless the type is active in the lookup.
- Evidence: `deploy/migrations/020_grappa_dcim_equipment_type_visuals.sql`, backend validation in `backend/internal/grappadcim/equipment.go`, and badges in `apps/grappa-dcim/src/features/equipment`.
- Used by: `apps/grappa-dcim` Apparati, rack U-map, Server, and Storage views.
- Open questions: none.

### Grappa DCIM Rack Media Is Not A V1 Feature

- Context: `apps/grappa-dcim` rack detail parity from the current Grappa application.
- Discovery: the Grappa `media` table exists in the schema, but the current application data does not populate it for rack operations. Treating `media.unit_id` and `side` as the basis for rack front/back UI created a target-only feature, not real parity.
- Practical rule: do not expose rack media endpoints, rack media mutation UI, or front/back photo controls in Grappa DCIM V1 unless product explicitly reopens the feature with live-data evidence. Keep `units` for the U-space grid; media may appear only in legacy cleanup paths such as deleting orphanable media rows when hard-deleting a rack.
- Evidence: `docs/grappa/grappa_media.json`; removed public media contract in `backend/internal/grappadcim/handler.go`, `backend/internal/grappadcim/racks_types.go`, `apps/grappa-dcim/src/api/types.ts`, `apps/grappa-dcim/src/features/racks/RackDetailPage.tsx`, and `apps/grappa-dcim/src/features/racks/RackPages.tsx`.
- Used by: `apps/grappa-dcim` rack detail and future Grappa DCIM migration corrections.
- Open questions: none for V1.

### Grappa DCIM Grid Layouts Are Visual Blocks, Not One Layout Per Islet

- Context: `apps/grappa-dcim` rack/island map parity from the previous Yii2 PHP implementation and `artifacts/mappe/totali.json`.
- Discovery: the map shape is a collection of visual blocks. Classical DCs usually map one islet to one block, but MMRs can split the same logical islet into multiple blocks: `MMRB` and `MMRA` both use `islet_name = side` twice, and `MMRA` duplicates the visible title `Fila D`.
- Practical rule: persisted Step 1 layouts must be keyed per visual block, not uniquely per `islets.id`. Use neutral current-model names such as `dcim_layout_blocks` and `layout-grid-v1`, not user-facing or schema names with `legacy`. Bind rack cells by `datacenter_id + islet_id + cell.pos -> positions.num`; never treat the JSON `pos` value as `positions.id`. Occupancy and rack details must stay live from `positions`/`racks`, not copied into layout JSON. Import is a CLI flow; plenum cells are linked to `plenums` through a dedicated binding table.
- Evidence: `artifacts/mappe/handoff.md`, `artifacts/mappe/schema.json`, `artifacts/mappe/totali.json`, and Step 1 spec `apps/grappa-dcim/docs/LAYOUT-STEP1.md`.
- Used by: `apps/grappa-dcim` layout transition planning.
- Open questions: none.

### Grappa DCIM Positions Are Whole Tiles; Half Racks Live On `racks`, Not `positions`

- Context: `apps/grappa-dcim` rack/island maps (the Sale e MMR detail panel, the 2D layout grid, and the 3D scene), all fed by `positions` joined to `racks`.
- Discovery: a `positions` row is one physical tile (mattonella) and has no A/B half concept — its columns are only `id, status, type, num, islets_id`, where `type` is `Full` or `Half`. The half-rack split lives entirely on `racks` (`racks.type` = `Full`/`Half`, `racks.pos` = `F`/`A`/`B`, `racks.positions_id` -> `positions.id`). So a `Half` tile can host two active racks (`A` = mezzo alto, `B` = mezzo basso) that both point at the same `positions.id`. A naive `positions LEFT JOIN racks ON racks.positions_id = positions.id` fans that one tile into two rows; counting rows double-counts the tile (e.g. DC4 reported 80 "posizioni" for 78 physical tiles) and any first-row-wins dedup silently drops the B rack. Separately, `listRacksForDatacenter` (the map `racks` array) has no active-state filter, so it returns every rack incl. `cessato`/`spento`/`chiuso` — never use its length as a rack count next to a tile grid (DC4: 152 vs 57 active). The occupancy signal differs by tile type and is **not** "is there a rack record": for a **Full** tile the truth is `positions.status` — a `status='free'` tile can legitimately carry an active rack record (the armadio physically exists but is not sold; CED2A1 had 3 such tiles num 1/5/18), so plain rack presence must NOT mark it occupied. The exception is **condiviso**: `racks.shared` (varchar(2), value `'Si'`/`'No'`) flags a cabinet hosting equipment from multiple customers — it is occupied and can **never** be free, even when `positions.status='free'` (CED2A1 num 5, `P0-I1-R5`). For a **Half** tile each side is occupied iff a rack sits on that `pos`, and `shared` again means condiviso.
- Practical rule: group join rows by `positions.id` into one tile carrying its 0–2 racks before counting or rendering. Select `r.shared` alongside the rack columns and carry it as a bool (`EqualFold(shared, "Si")`). Then classify per **rack slot (posto): Full = 1, Half = 2** (not per tile, not per rack row — the only granularity where totals match the coloured regions; a partially-filled Half is otherwise ambiguous): **Full** = `shared` rack → `shared` (condiviso); else `positions.status` mapped to occupied/reserved/free. **Half** side = rack present → (`shared` ? `shared` : `occupied`); else `reserved` (if tile reserved) else `free`. Treat `shared`/condiviso as a distinct, important state with its own colour (never folded into free; surfaced separately in counts and legend). For any "racks in this room" figure, filter to active racks — do not reuse the unfiltered map `racks` array. Keep this map metric (layout occupancy per posto) distinct from the room-list "rack" column (active racks / declared `datacenter.rack` capacity), which is the only metric valid for rooms without a layout.
- Evidence: `docs/grappa/grappa_positions.json` (no `pos` column), `docs/grappa/grappa_racks.json` (`type`/`pos`/`positions_id`/`shared`); backend grouping in `backend/internal/grappadcim/layout.go` (`scanPositions`, `positionsSelectSQL` selecting `r.shared`) returning `Position.racks []PositionRack{...Shared bool}`; shared FE helpers `apps/grappa-dcim/src/features/facilities/positions.ts` (`fullSlotStatus`, `slotStatus`, `isSharedRack`, `summarizeSlots` → `{occupied, free, reserved, shared, total}`, `rackAt`, `fullRack`, `isHalfPosition`); renderers in `FacilitiesPages.tsx` (`DatacenterMapPanel` + LayoutPage occupancy + `positionEffectiveStatus`), `LayoutGrid.tsx`, `LayoutScene.tsx` (`STATUS_COLORS.shared`).
- Used by: `apps/grappa-dcim` facilities map, 2D layout grid, and 3D scene.
- Open questions: none.
