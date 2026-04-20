// Wire contract for the biometric-requests surface.
// Keys and types are locked by apps/customer-portal/FINAL.md §Slice S4 / S5c
// and mirror backend/internal/cpbackoffice/types.go `BiometricRequestRow`.
// `data_richiesta` and `data_approvazione` are ISO timestamps emitted by the
// backend `time.Time` encoder; `data_approvazione` is nullable.
export interface BiometricRequestRow {
  id: number;
  nome: string;
  cognome: string;
  email: string;
  azienda: string;
  tipo_richiesta: string;
  stato_richiesta: boolean;
  data_richiesta: string;
  data_approvazione: string | null;
  // Returned by the backend for contract parity but intentionally never
  // rendered in v1 (locked by FINAL.md §Slice S5c).
  is_biometric_lenel: boolean;
}

// Body shape accepted by
// POST /api/cp-backoffice/v1/biometric-requests/{id}/completion.
// `completed` maps 1:1 onto the boolean argument of the stored function
// customers.biometric_request_set_completed.
export interface CompletionRequest {
  completed: boolean;
}

// Success shape returned by the completion mutation. The locked payload
// is `{ "ok": true }` (FINAL.md §Slice S4).
export interface CompletionResponse {
  ok: boolean;
}

// Query key factory for biometric-requests cache entries.
// Kept alongside the contract so the hooks and UI stay in lockstep.
export const biometricKeys = {
  all: ['cp-backoffice', 'biometric-requests'] as const,
};
