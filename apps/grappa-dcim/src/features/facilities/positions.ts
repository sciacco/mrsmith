import type { Position, PositionRack } from '../../api/types';

// Shared half-rack helpers. A physical position (mattonella) is "Half" when its
// type is half: it can host two racks, pos "A" (mezzo alto) and pos "B" (mezzo
// basso). A "Full" position hosts at most one rack (pos "F").

export type HalfSide = 'A' | 'B';

// A "shared" (condiviso) cabinet hosts equipment from multiple customers, so it is
// occupied — it can NEVER be free, even when the position status still reads 'free'.
export type SlotStatus = 'occupied' | 'reserved' | 'free' | 'shared';

export interface GridCellSlot {
  side?: HalfSide;
  status: SlotStatus;
  rack?: PositionRack;
}

export function isHalfPosition(type: string | undefined): boolean {
  return (type ?? '').toLowerCase() === 'half';
}

export function rackAt(racks: PositionRack[] | undefined, pos: HalfSide | 'F'): PositionRack | undefined {
  return racks?.find((rack) => (rack.pos ?? '').toUpperCase() === pos);
}

export function fullRack(racks: PositionRack[] | undefined): PositionRack | undefined {
  return rackAt(racks, 'F') ?? racks?.[0];
}

export function isSharedRack(rack: PositionRack | undefined): boolean {
  return Boolean(rack?.shared);
}

// Occupancy of a Full tile. A shared cabinet is occupied by multiple customers and
// can never be free, so it always wins. Otherwise `position.status` is the operator's
// truth: a (non-shared) rack record can exist on a free position (the armadio
// physically exists but isn't sold), so plain rack presence does NOT mark it occupied.
export function fullSlotStatus(position: { status: string; racks?: PositionRack[] }): SlotStatus {
  if (isSharedRack(fullRack(position.racks))) return 'shared';
  const status = (position.status ?? '').toLowerCase();
  if (status === 'occupied') return 'occupied';
  if (status === 'reserved') return 'reserved';
  return 'free';
}

// Occupancy of one half slot (A/B) of a Half tile. Half armadi are tracked as
// individual rack records: a half with a shared rack is condiviso, with any other
// rack is occupied; an empty half inherits the tile status (reserved) or is free.
export function slotStatus(positionStatus: string | undefined, rack: PositionRack | undefined): SlotStatus {
  if (rack) return isSharedRack(rack) ? 'shared' : 'occupied';
  return (positionStatus ?? '').toLowerCase() === 'reserved' ? 'reserved' : 'free';
}

export function gridCellSlots(cell: {
  type: string;
  positionId?: number;
  positionStatus?: string;
  positionType?: string;
  racks?: PositionRack[];
}): GridCellSlot[] {
  if (cell.type !== 'position' || !cell.positionId) return [];
  if (isHalfPosition(cell.positionType)) {
    const rackA = rackAt(cell.racks, 'A');
    const rackB = rackAt(cell.racks, 'B');
    return [
      { side: 'A', status: slotStatus(cell.positionStatus, rackA), rack: rackA },
      { side: 'B', status: slotStatus(cell.positionStatus, rackB), rack: rackB },
    ];
  }
  const rack = fullRack(cell.racks);
  return [{ status: fullSlotStatus({ status: cell.positionStatus ?? '', racks: cell.racks }), rack }];
}

function slotRank(status: SlotStatus): number {
  return status === 'shared' ? 3 : status === 'occupied' ? 2 : status === 'reserved' ? 1 : 0;
}

// Effective status of a layout-grid cell (for the room overview heatmap glyph):
// null for non-position/unbound cells; otherwise the per-position effective status.
export function gridCellStatus(cell: {
  type: string;
  positionId?: number;
  positionStatus?: string;
  positionType?: string;
  racks?: PositionRack[];
}): SlotStatus | null {
  const slots = gridCellSlots(cell);
  if (slots.length === 0) return null;
  return slots.reduce((best, slot) => (slotRank(slot.status) > slotRank(best.status) ? slot : best), slots[0]!).status;
}

// Effective per-position status for badges/lists/rails: a shared cabinet is condiviso
// (never free); otherwise a Full follows position.status and a Half takes the most
// occupied of its two sides (shared > occupied > reserved > free).
export function positionEffectiveStatus(position: Position): SlotStatus {
  if (!isHalfPosition(position.type)) return fullSlotStatus(position);
  const a = slotStatus(position.status, rackAt(position.racks, 'A'));
  const b = slotStatus(position.status, rackAt(position.racks, 'B'));
  return slotRank(a) >= slotRank(b) ? a : b;
}

// Occupancy counted per rack slot (posto): a Full tile is 1 slot, a Half tile is 2
// (A/B). Each slot is classified exactly like its rendered colour, so the totals
// always match the coloured regions on the grid. `shared` is a flavour of occupied
// (tracked separately so the panel can surface it) — it is never counted as free.
export function summarizeSlots(positions: Position[]): { occupied: number; free: number; reserved: number; shared: number; total: number } {
  let occupied = 0;
  let free = 0;
  let reserved = 0;
  let shared = 0;
  const tally = (status: SlotStatus) => {
    if (status === 'occupied') occupied += 1;
    else if (status === 'reserved') reserved += 1;
    else if (status === 'shared') shared += 1;
    else free += 1;
  };
  for (const position of positions) {
    if (isHalfPosition(position.type)) {
      tally(slotStatus(position.status, rackAt(position.racks, 'A')));
      tally(slotStatus(position.status, rackAt(position.racks, 'B')));
    } else {
      tally(fullSlotStatus(position));
    }
  }
  return { occupied, free, reserved, shared, total: occupied + free + reserved + shared };
}
