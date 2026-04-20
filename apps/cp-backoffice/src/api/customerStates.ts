// Wire contract for the customer-state select surface.
// Shape comes unwrapped from GET /api/cp-backoffice/v1/customer-states
// (the backend strips the upstream `items` envelope).
export interface CustomerState {
  id: number;
  name: string;
}

// Query key factory for every customer-state cache entry.
export const customerStatesKeys = {
  all: ['cp-backoffice', 'customer-states'] as const,
};
