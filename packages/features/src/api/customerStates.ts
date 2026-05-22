export interface CustomerState {
  id: number;
  name: string;
}

export const customerStatesKeys = {
  all: ['cp-backoffice', 'customer-states'] as const,
};
