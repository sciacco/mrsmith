import type { CustomerState } from './customerStates';

// Wire contract for the customer list surface.
// Shape comes unwrapped from GET /api/cp-backoffice/v1/customers: the backend
// strips the upstream `items` envelope and forwards the row array as-is
// (see backend/internal/cpbackoffice/arak.go writeItemsPassthrough).
export interface CustomerGroup {
  id: number;
  name: string;
}

export interface Customer {
  id: number;
  name: string;
  group: CustomerGroup;
  state: CustomerState;
  language: string;
}

// Body shape accepted by PUT /api/cp-backoffice/v1/customers/{id}/state.
// state_id is an integer id from the customer-states list.
export interface UpdateStateRequest {
  state_id: number;
}

// Query key factory for every customer-related cache entry.
// Kept alongside the contract so the UI and the hooks stay in lockstep.
export const customersKeys = {
  all: ['cp-backoffice', 'customers'] as const,
};
