import type { Position, PositionRack } from '../../api/types';

// Shared half-rack helpers. A physical position (mattonella) is "Half" when its
// type is half: it can host two racks, pos "A" (mezzo alto) and pos "B" (mezzo
// basso). A "Full" position hosts at most one rack (pos "F").

export type HalfSide = 'A' | 'B';

export function isHalfPosition(type: string | undefined): boolean {
  return (type ?? '').toLowerCase() === 'half';
}

export function rackAt(racks: PositionRack[] | undefined, pos: HalfSide | 'F'): PositionRack | undefined {
  return racks?.find((rack) => (rack.pos ?? '').toUpperCase() === pos);
}

export function fullRack(racks: PositionRack[] | undefined): PositionRack | undefined {
  return rackAt(racks, 'F') ?? racks?.[0];
}

// Occupancy of a Full tile comes from `position.status` (the operator's truth): a
// rack record can exist on a free position (the armadio physically exists but isn't
// sold), so rack presence must NOT be used to mark a Full tile occupied.
export function fullSlotStatus(position: { status: string }): 'occupied' | 'reserved' | 'free' {
  const status = (position.status ?? '').toLowerCase();
  if (status === 'occupied') return 'occupied';
  if (status === 'reserved') return 'reserved';
  return 'free';
}

// Occupancy of one half slot (A/B) of a Half tile. Half armadi are tracked as
// individual rack records, so a half is occupied iff a rack sits on that side;
// otherwise it inherits the tile status (reserved) or is free.
export function slotStatus(positionStatus: string | undefined, rack: PositionRack | undefined): 'occupied' | 'reserved' | 'free' {
  if (rack) return 'occupied';
  return (positionStatus ?? '').toLowerCase() === 'reserved' ? 'reserved' : 'free';
}

// Occupancy counted per rack slot (posto): a Full tile is 1 slot, a Half tile is 2
// (A/B). Each slot is classified exactly like its rendered colour, so the totals
// always match the coloured regions on the grid.
export function summarizeSlots(positions: Position[]): { occupied: number; free: number; reserved: number; total: number } {
  let occupied = 0;
  let free = 0;
  let reserved = 0;
  const tally = (status: 'occupied' | 'reserved' | 'free') => {
    if (status === 'occupied') occupied += 1;
    else if (status === 'reserved') reserved += 1;
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
  return { occupied, free, reserved, total: occupied + free + reserved };
}
