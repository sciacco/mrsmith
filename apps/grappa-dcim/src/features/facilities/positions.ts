import type { PositionRack } from '../../api/types';

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

// Status of a single slot (a half, or a full tile). Occupied when a rack sits in
// it; otherwise it inherits the tile status (reserved or free).
export function slotStatus(positionStatus: string | undefined, rack: PositionRack | undefined): 'occupied' | 'reserved' | 'free' {
  if (rack) return 'occupied';
  return (positionStatus ?? '').toLowerCase() === 'reserved' ? 'reserved' : 'free';
}
