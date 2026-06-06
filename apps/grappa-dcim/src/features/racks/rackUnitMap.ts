import type { EquipmentItem } from '../../api/types';

export type RackOccupant = Pick<EquipmentItem, 'id' | 'name' | 'type' | 'typeVisual'> &
  Partial<Pick<EquipmentItem, 'unitPosition' | 'unit' | 'occupiedUnits' | 'status'>>;

export type RackUnitMapRow =
  | {
      kind: 'free';
      unitNum: number;
    }
  | {
      kind: 'device';
      unitNum: number;
      device: RackOccupant;
      startUnit: number;
      endUnit: number;
      span: number;
    }
  | {
      kind: 'covered';
      unitNum: number;
      device: RackOccupant;
      startUnit: number;
      endUnit: number;
    };

interface RackUnitClaim {
  device: RackOccupant;
  startUnit: number;
  endUnit: number;
}

export function normalizeRackUnitCount(unitCount?: number): number {
  if (unitCount !== undefined && Number.isFinite(unitCount) && unitCount > 0) {
    return Math.floor(unitCount);
  }
  return 42;
}

export function normalizeOccupiedUnits(device: RackOccupant): number {
  const raw = device.occupiedUnits ?? device.unit ?? 1;
  if (Number.isFinite(raw) && raw > 0) {
    return Math.floor(raw);
  }
  return 1;
}

export function buildRackUnitMap(unitCount: number, equipment: RackOccupant[]): RackUnitMapRow[] {
  const normalizedUnitCount = normalizeRackUnitCount(unitCount);
  const claimsByUnit = new Map<number, RackUnitClaim>();
  const positionedEquipment = equipment
    .filter((device) => device.unitPosition !== undefined && Number.isFinite(device.unitPosition))
    .sort((a, b) => (a.unitPosition ?? 0) - (b.unitPosition ?? 0) || a.id - b.id);

  for (const device of positionedEquipment) {
    const startUnit = Math.floor(device.unitPosition ?? 0);
    if (startUnit < 1 || startUnit > normalizedUnitCount) {
      continue;
    }

    const occupiedUnits = normalizeOccupiedUnits(device);
    const endUnit = Math.min(normalizedUnitCount, startUnit + occupiedUnits - 1);
    let overlaps = false;
    for (let unitNum = startUnit; unitNum <= endUnit; unitNum++) {
      if (claimsByUnit.has(unitNum)) {
        overlaps = true;
        break;
      }
    }
    if (overlaps) {
      continue;
    }

    const claim = { device, startUnit, endUnit };
    for (let unitNum = startUnit; unitNum <= endUnit; unitNum++) {
      claimsByUnit.set(unitNum, claim);
    }
  }

  return Array.from({ length: normalizedUnitCount }, (_, index) => normalizedUnitCount - index).map((unitNum) => {
    const claim = claimsByUnit.get(unitNum);
    if (!claim) {
      return { kind: 'free', unitNum };
    }
    if (unitNum === claim.endUnit) {
      return {
        kind: 'device',
        unitNum,
        device: claim.device,
        startUnit: claim.startUnit,
        endUnit: claim.endUnit,
        span: claim.endUnit - claim.startUnit + 1,
      };
    }
    return {
      kind: 'covered',
      unitNum,
      device: claim.device,
      startUnit: claim.startUnit,
      endUnit: claim.endUnit,
    };
  });
}
