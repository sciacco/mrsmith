// Wire contract for the user list surface.
// Shape comes unwrapped from GET /api/cp-backoffice/v1/users?customer_id=...
// (the backend strips the upstream `items` envelope and pins
// disable_pagination=true upstream — see backend/internal/cpbackoffice/arak.go).
// This matches the upstream `user-brief` DTO; the frontend unwraps `role.name`
// for the legacy Appsmith-visible column.
export interface UserRole {
  id: number;
  name: string;
  color?: string;
}

export interface User {
  id: number;
  customer_id: number;
  first_name: string;
  last_name: string;
  email: string;
  enabled: boolean;
  role: UserRole;
  phone?: string;
  created: string;
  last_login?: string;
}

// Query key factory for every user-list cache entry. Kept scoped by
// customerId so switching selection invalidates the table cleanly.
export const usersKeys = {
  all: ['cp-backoffice', 'users'] as const,
  byCustomer: (customerId: number) =>
    ['cp-backoffice', 'users', customerId] as const,
};
