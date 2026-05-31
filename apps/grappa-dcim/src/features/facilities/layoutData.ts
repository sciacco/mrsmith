// Pure consultation data layer for the Isole e posizioni page.
//
// It joins the three things every room payload already carries — positions,
// islets and the rich rack list (`RackListItem`, with customer/power/order) —
// into a single flat, filterable, searchable model. The 2D map, the islet
// index, the linked table and the actionable occupancy chips all read from
// this so selection, search and filters stay consistent across the three views.

import type { Islet, LayoutGridBlock, LayoutGridCell, Position, PositionRack, RackListItem } from '../../api/types';
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
// --- Room overview geometry (representative, non-metric) ----------------------
// The room overview renders each islet as a compact occupancy "glyph": its real
// rows×cols grid of status-colored cells. Footprint is DECOUPLED from the rich
// detail cell size — the detail (full LayoutCell grid) lives in the focus panel.
// Logical units == px at fit-scale 1.
export const GLYPH_CELL = 22; // px per position cell in the overview glyph
export const GLYPH_HEADER = 24; // px for the glyph name/occupancy header
export const GLYPH_GAP = 18; // px gutter between glyphs when auto-arranging

// Base (canonical, unrotated) cell grid for an islet: its imported block grid, or
// — lacking a block — its positions chunked into a square-ish grid.
export function isletCells(block: LayoutGridBlock | undefined, positions: Position[]): LayoutGridCell[][] {
  if (block && block.grid.length > 0) return block.grid;
  const sorted = [...positions].sort((a, b) => a.num - b.num);
  const cols = Math.max(2, Math.ceil(Math.sqrt(Math.max(sorted.length, 1))));
  const rows: LayoutGridCell[][] = [];
  for (let i = 0; i < sorted.length; i += cols) {
    rows.push(
      sorted.slice(i, i + cols).map((p) => ({
        type: 'position' as const,
        pos: p.num,
        positionId: p.id,
        positionStatus: p.status,
        positionType: p.type,
        racks: p.racks,
      })),
    );
  }
  return rows.length > 0 ? rows : [[]];
}

function padRectangular(cells: LayoutGridCell[][]): LayoutGridCell[][] {
  const cols = cells.reduce((max, row) => Math.max(max, row.length), 0);
  return cells.map((row) =>
    row.length === cols ? row : [...row, ...Array.from({ length: cols - row.length }, () => ({ type: 'empty' as const }))],
  );
}

// Rotate a cell grid by 0/90/180/270 degrees (for the room overview glyph only —
// the focus detail panel always reads the canonical orientation).
export function orientCells(cells: LayoutGridCell[][], rotation: number): LayoutGridCell[][] {
  const steps = (((Math.round((rotation || 0) / 90) % 4) + 4) % 4);
  let grid = padRectangular(cells);
  const rotate90 = (matrix: LayoutGridCell[][]): LayoutGridCell[][] => {
    const rows = matrix.length;
    const cols = matrix[0]?.length ?? 0;
    const out: LayoutGridCell[][] = [];
    for (let c = 0; c < cols; c += 1) {
      const row: LayoutGridCell[] = [];
      for (let r = rows - 1; r >= 0; r -= 1) row.push(matrix[r]?.[c] ?? { type: 'empty' });
      out.push(row);
    }
    return out;
  };
  for (let i = 0; i < steps; i += 1) grid = rotate90(grid);
  return grid;
}

export interface GlyphSize {
  cols: number;
  rows: number;
  width: number;
  height: number;
}

export function glyphSize(orientedCells: LayoutGridCell[][]): GlyphSize {
  const rows = orientedCells.length;
  const cols = orientedCells.reduce((max, row) => Math.max(max, row.length), 0);
  const width = Math.max(cols, 1) * GLYPH_CELL + (Math.max(cols, 1) - 1) * 1;
  const height = GLYPH_HEADER + 3 + Math.max(rows, 1) * GLYPH_CELL + (Math.max(rows, 1) - 1) * 1;
  return { cols, rows, width, height };
}

// Auto-arrange glyphs into rows (shelf packing) at a target plane width, so islets
// never overlap on first load. Saved per-islet coordinates override these defaults.
export function autoArrangePositions(items: Array<{ id: number; width: number; height: number }>, targetWidth = 1100): Record<number, { x: number; y: number }> {
  const out: Record<number, { x: number; y: number }> = {};
  let x = 0;
  let y = 0;
  let rowHeight = 0;
  for (const item of items) {
    if (x > 0 && x + item.width > targetWidth) {
      x = 0;
      y += rowHeight + GLYPH_GAP;
      rowHeight = 0;
    }
    out[item.id] = { x, y };
    x += item.width + GLYPH_GAP;
    rowHeight = Math.max(rowHeight, item.height);
  }
  return out;
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
