import assert from 'node:assert/strict';
import test from 'node:test';
import {
  buildRackUnitMap,
  normalizeOccupiedUnits,
  normalizeRackUnitCount,
  type RackOccupant,
  type RackUnitMapRow,
} from './rackUnitMap.ts';

type DeviceRow = Extract<RackUnitMapRow, { kind: 'device' }>;

function device(input: Partial<RackOccupant> & Pick<RackOccupant, 'id' | 'name'>): RackOccupant {
  return {
    type: 'Switch',
    ...input,
  };
}

function expectDeviceRow(row: RackUnitMapRow | undefined): DeviceRow {
  assert.ok(row);
  assert.equal(row.kind, 'device');
  if (row.kind !== 'device') {
    throw new Error('expected device row');
  }
  return row;
}

test('buildRackUnitMap renders a multi-U device as one top block and covered continuation rows', () => {
  const rows = buildRackUnitMap(12, [device({ id: 1, name: 'Switch core', unitPosition: 10, occupiedUnits: 3 })]);

  assert.deepEqual(rows.slice(0, 4).map((row) => [row.unitNum, row.kind]), [
    [12, 'device'],
    [11, 'covered'],
    [10, 'covered'],
    [9, 'free'],
  ]);

  const top = expectDeviceRow(rows[0]);
  assert.equal(top.startUnit, 10);
  assert.equal(top.endUnit, 12);
  assert.equal(top.span, 3);
});

test('buildRackUnitMap keeps one-U devices as a single occupied row', () => {
  const rows = buildRackUnitMap(4, [device({ id: 1, name: 'Router', unitPosition: 2, occupiedUnits: 1 })]);
  const occupied = rows.find((row) => row.kind === 'device');

  const occupiedDevice = expectDeviceRow(occupied);
  assert.equal(occupiedDevice.unitNum, 2);
  assert.equal(occupiedDevice.startUnit, 2);
  assert.equal(occupiedDevice.endUnit, 2);
  assert.equal(occupiedDevice.span, 1);
});

test('normalizeOccupiedUnits falls back to legacy unit and then 1U', () => {
  assert.equal(normalizeOccupiedUnits(device({ id: 1, name: 'Firewall', unit: 2 })), 2);
  assert.equal(normalizeOccupiedUnits(device({ id: 2, name: 'Patch panel', unit: 0 })), 1);
  assert.equal(normalizeOccupiedUnits(device({ id: 3, name: 'Appliance' })), 1);
});

test('buildRackUnitMap clamps device height to the rack boundary', () => {
  const rows = buildRackUnitMap(42, [device({ id: 1, name: 'Top device', unitPosition: 41, occupiedUnits: 5 })]);
  const top = expectDeviceRow(rows[0]);

  assert.equal(top.unitNum, 42);
  assert.equal(top.startUnit, 41);
  assert.equal(top.endUnit, 42);
  assert.equal(top.span, 2);
});

test('buildRackUnitMap skips devices without a valid rack position', () => {
  const rows = buildRackUnitMap(3, [
    device({ id: 1, name: 'Missing position', occupiedUnits: 2 }),
    device({ id: 2, name: 'Zero position', unitPosition: 0, occupiedUnits: 2 }),
  ]);

  assert.deepEqual(rows.map((row) => row.kind), ['free', 'free', 'free']);
});

test('buildRackUnitMap resolves overlaps deterministically by first start unit and id', () => {
  const rows = buildRackUnitMap(12, [
    device({ id: 2, name: 'Later device', unitPosition: 11, occupiedUnits: 2 }),
    device({ id: 1, name: 'First device', unitPosition: 10, occupiedUnits: 3 }),
  ]);
  const deviceRows = rows.filter((row) => row.kind === 'device');

  assert.equal(deviceRows.length, 1);
  const firstDevice = expectDeviceRow(deviceRows[0]);
  assert.equal(firstDevice.device.id, 1);
  assert.equal(firstDevice.startUnit, 10);
  assert.equal(firstDevice.endUnit, 12);
});

test('normalizeRackUnitCount defaults invalid rack heights to 42U', () => {
  assert.equal(normalizeRackUnitCount(undefined), 42);
  assert.equal(normalizeRackUnitCount(0), 42);
  assert.equal(normalizeRackUnitCount(24.9), 24);
});
