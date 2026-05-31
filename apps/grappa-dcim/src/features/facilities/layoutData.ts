// Pure consultation data layer for the Isole e posizioni page.
//
// It joins the three things every room payload already carries — positions,
// islets and the rich rack list (`RackListItem`, with customer/power/order) —
// into a single flat, filterable, searchable model. The 2D map, the islet
// index, the linked table and the actionable occupancy chips all read from
// this so selection, search and filters stay consistent across the three views.

import type { Islet, LayoutGridBlock, Position, PositionRack, RackListItem } from '../../api/types';
import { positionEffectiveStatus, type SlotStatus } from './positions';

export interface EnrichedRack extends PositionRack {
  detail?: RackListItem;
  customerId?: number;
  soldPower?: number;
  orderCode?: string;
  rackStatus?: string;
}

export interface PositionRow {
  position: Position;
  islet?: Islet;
  status: SlotStatus;
  racks: EnrichedRack[];
  customerIds: number[];
}

export interface LayoutFilters {
  query: string;
  statuses: SlotStatus[];
  formats: string[]; // lowercased position type, e.g. 'full' | 'half'
  isletId: number | null;
}

export const EMPTY_FILTERS: LayoutFilters = { query: '', statuses: [], formats: [], isletId: null };

export function hasActiveFilters(filters: LayoutFilters): boolean {
  return (
    filters.query.trim() !== '' ||
    filters.statuses.length > 0 ||
    filters.formats.length > 0 ||
    filters.isletId !== null
  );
}

function enrichRacks(position: Position, detailById: Map<number, RackListItem>): EnrichedRack[] {
  return position.racks.map((rack) => {
    const detail = detailById.get(rack.id);
    return {
      ...rack,
      detail,
      customerId: detail?.customerId,
      soldPower: detail?.soldPower,
      orderCode: detail?.orderCode,
      rackStatus: detail?.status,
    };
  });
}

export function buildPositionRows(positions: Position[], islets: Islet[], racks: RackListItem[]): PositionRow[] {
  const detailById = new Map(racks.map((rack) => [rack.id, rack]));
  const isletById = new Map(islets.map((islet) => [islet.id, islet]));
  return positions.map((position) => {
    const enriched = enrichRacks(position, detailById);
    return {
      position,
      islet: isletById.get(position.isletId),
      status: positionEffectiveStatus(position),
      racks: enriched,
      customerIds: enriched
        .map((rack) => rack.customerId)
        .filter((id): id is number => id !== undefined && id !== null),
    };
  });
}

export function rowMatchesQuery(row: PositionRow, query: string): boolean {
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  if (row.islet?.name?.toLowerCase().includes(needle)) return true;
  if (String(row.position.num).includes(needle)) return true;
  if (row.racks.some((rack) => rack.name?.toLowerCase().includes(needle))) return true;
  if (row.customerIds.some((id) => String(id).includes(needle))) return true;
  return false;
}

export function rowMatchesFilters(row: PositionRow, filters: LayoutFilters): boolean {
  if (filters.isletId !== null && row.position.isletId !== filters.isletId) return false;
  if (filters.statuses.length > 0 && !filters.statuses.includes(row.status)) return false;
  if (filters.formats.length > 0 && !filters.formats.includes((row.position.type ?? '').toLowerCase())) return false;
  if (!rowMatchesQuery(row, filters.query)) return false;
  return true;
}

export function filterRows(rows: PositionRow[], filters: LayoutFilters): PositionRow[] {
  return rows.filter((row) => rowMatchesFilters(row, filters));
}

export function matchingPositionIds(rows: PositionRow[], filters: LayoutFilters): Set<number> {
  const ids = new Set<number>();
  for (const row of rows) {
    if (rowMatchesFilters(row, filters)) ids.add(row.position.id);
  }
  return ids;
}

export interface StatusCounts {
  free: number;
  occupied: number;
  reserved: number;
  shared: number;
  total: number;
}

export function statusCounts(rows: PositionRow[]): StatusCounts {
  const counts: StatusCounts = { free: 0, occupied: 0, reserved: 0, shared: 0, total: rows.length };
  for (const row of rows) counts[row.status] += 1;
  return counts;
}

export interface IsletSummary {
  islet: Islet;
  total: number;
  occupied: number; // occupied + shared
  free: number;
  reserved: number;
  matchCount: number; // positions passing the active filters (for index highlight)
}

// One summary per configured islet (including empty ones), with occupancy and how
// many of its positions pass the active filters so the index can highlight/dim.
// --- Room canvas geometry (representative, non-metric) ------------------------
// Logical units of the virtual room plane (== px at zoom 1).
export const CANVAS_CELL = 84;
export const CANVAS_HEADER = 34;
export const CANVAS_PAD = 12;

export interface IsletFootprint {
  width: number;
  height: number;
  cols: number;
  rows: number;
}

// Footprint of an islet node, derived from its block grid (faithful within the
// islet) or, lacking a block, from a square-ish grid of its position count.
export function isletFootprint(block: LayoutGridBlock | undefined, positionCount: number): IsletFootprint {
  let cols: number;
  let rows: number;
  if (block && block.grid.length > 0) {
    rows = block.grid.length;
    cols = block.grid.reduce((max, row) => Math.max(max, row.length), 1);
  } else {
    const count = Math.max(positionCount, 1);
    cols = Math.max(2, Math.ceil(Math.sqrt(count)));
    rows = Math.max(1, Math.ceil(count / cols));
  }
  return {
    cols,
    rows,
    width: cols * CANVAS_CELL + CANVAS_PAD * 2,
    height: CANVAS_HEADER + rows * CANVAS_CELL + CANVAS_PAD * 2,
  };
}

// Starting placement for islets without saved coordinates: a coarse grid the
// operator then refines. Index follows the islets' display order.
const DEFAULT_CANVAS_COLS = 3;
const DEFAULT_CANVAS_STEP_X = 560;
const DEFAULT_CANVAS_STEP_Y = 460;
export function defaultIsletPosition(index: number): { x: number; y: number } {
  return {
    x: 40 + (index % DEFAULT_CANVAS_COLS) * DEFAULT_CANVAS_STEP_X,
    y: 40 + Math.floor(index / DEFAULT_CANVAS_COLS) * DEFAULT_CANVAS_STEP_Y,
  };
}

export function summarizeIslets(rows: PositionRow[], islets: Islet[], filters: LayoutFilters): IsletSummary[] {
  const byIslet = new Map<number, PositionRow[]>();
  for (const row of rows) {
    const list = byIslet.get(row.position.isletId) ?? [];
    list.push(row);
    byIslet.set(row.position.isletId, list);
  }
  return islets.map((islet) => {
    const isletRows = byIslet.get(islet.id) ?? [];
    let occupied = 0;
    let free = 0;
    let reserved = 0;
    for (const row of isletRows) {
      if (row.status === 'occupied' || row.status === 'shared') occupied += 1;
      else if (row.status === 'reserved') reserved += 1;
      else free += 1;
    }
    return {
      islet,
      total: isletRows.length,
      occupied,
      free,
      reserved,
      matchCount: isletRows.filter((row) => rowMatchesFilters(row, filters)).length,
    };
  });
}
