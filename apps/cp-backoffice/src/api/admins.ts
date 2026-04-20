// Wire contract for POST /api/cp-backoffice/v1/admins.
//
// Field names on the wire match what the backend accepts
// (see backend/internal/cpbackoffice/types.go CreateAdminRequest):
//   customer_id, nome, cognome, email, telefono,
//   maintenance_on_primary_email, marketing_on_primary_email.
//
// The backend translates nome/cognome/telefono to the upstream
// first_name/last_name/phone DTO and ALWAYS pins the hidden upstream
// skip-switch to false; we never send it from the front-end.
export interface CreateAdminRequest {
  customer_id: number;
  nome: string;
  cognome: string;
  email: string;
  telefono: string;
  maintenance_on_primary_email: boolean;
  marketing_on_primary_email: boolean;
}
