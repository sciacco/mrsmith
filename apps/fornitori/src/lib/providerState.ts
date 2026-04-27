import { stateLabel } from './reference.ts';

export const EDITABLE_PROVIDER_STATES = ['DRAFT', 'ACTIVE', 'INACTIVE'] as const;

export type EditableProviderState = (typeof EDITABLE_PROVIDER_STATES)[number];

function normalizeState(value?: string | null) {
  return (value ?? '').toUpperCase();
}

export function hasProviderErp(value?: string | number | null) {
  if (value === null || value === undefined) return false;
  if (typeof value === 'number') return value > 0;
  const trimmed = value.trim();
  if (!trimmed) return false;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) && parsed > 0;
}

export function providerStateValues(currentState?: string | null, erpId?: string | number | null): EditableProviderState[] {
  const state = normalizeState(currentState);

  if (state === 'ACTIVE') return ['ACTIVE', 'INACTIVE'];
  if (state === 'DRAFT') return hasProviderErp(erpId) ? [...EDITABLE_PROVIDER_STATES] : ['DRAFT'];
  if (state === 'INACTIVE') return ['INACTIVE'];

  return [];
}

export function providerStateSelectOptions(currentState?: string | null, erpId?: string | number | null) {
  return providerStateValues(currentState, erpId).map((value) => ({
    value,
    label: stateLabel(value),
  }));
}
