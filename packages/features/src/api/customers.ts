import type { CustomerState } from './customerStates';

export interface CustomerGroup {
  id: number;
  name: string;
}

export interface CustomerVariable {
  resource: string;
  action: string;
}

export interface Customer {
  id: number;
  name: string;
  group: CustomerGroup;
  state: CustomerState;
  language: string;
  variables: CustomerVariable[] | null;
}

export interface UpdateStateRequest {
  state_id: number;
}

export const customersKeys = {
  all: ['cp-backoffice', 'customers'] as const,
};
