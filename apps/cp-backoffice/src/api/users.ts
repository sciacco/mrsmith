// Wire contract for the user list surface.
// Shape comes unwrapped from GET /api/cp-backoffice/v1/users?customer_id=...
// (the backend strips the upstream `items` envelope and pins
// disable_pagination=true upstream — see backend/internal/cpbackoffice/arak.go).
export interface User {
  id: number;
  nome: string;
  cognome: string;
  email: string;
  is_admin: boolean;
}

// Query key factory for every user-list cache entry. Kept scoped by
// customerId so switching selection invalidates the table cleanly.
export const usersKeys = {
  all: ['cp-backoffice', 'users'] as const,
  byCustomer: (customerId: number) =>
    ['cp-backoffice', 'users', customerId] as const,
};
