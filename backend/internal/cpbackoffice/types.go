package cpbackoffice

import "time"

// BiometricRequestRow is the wire contract for a single row returned by
// GET /cp-backoffice/v1/biometric-requests. Keys and types are locked by
// apps/customer-portal/FINAL.md (§Slice S4 locks) and must not drift.
type BiometricRequestRow struct {
	ID               int64      `json:"id"`
	Nome             string     `json:"nome"`
	Cognome          string     `json:"cognome"`
	Email            string     `json:"email"`
	Azienda          string     `json:"azienda"`
	TipoRichiesta    string     `json:"tipo_richiesta"`
	StatoRichiesta   bool       `json:"stato_richiesta"`
	DataRichiesta    time.Time  `json:"data_richiesta"`
	DataApprovazione *time.Time `json:"data_approvazione"`
	IsBiometricLenel bool       `json:"is_biometric_lenel"`
}

// Customer is the wire contract for a single row returned by
// GET /cp-backoffice/v1/customers. Upstream-owned shape; the
// backend passes fields through without inventing a second vocabulary.
type Customer struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CustomerState is the wire contract for a single row returned by
// GET /cp-backoffice/v1/customer-states.
type CustomerState struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// User is the wire contract for a single row returned by
// GET /cp-backoffice/v1/users.
type User struct {
	ID        int64  `json:"id"`
	Nome      string `json:"nome"`
	Cognome   string `json:"cognome"`
	Email     string `json:"email"`
	IsAdmin   bool   `json:"is_admin"`
}

// UpdateStateRequest is the body accepted by
// PUT /cp-backoffice/v1/customers/{id}/state.
type UpdateStateRequest struct {
	StateID int64 `json:"state_id"`
}

// CreateAdminRequest is the body accepted by
// POST /cp-backoffice/v1/admins. skip_keycloak is never accepted from the
// caller; request assembly pins it to false downstream (see Slice S3 lock).
type CreateAdminRequest struct {
	CustomerID                 int64  `json:"customer_id"`
	Nome                       string `json:"nome"`
	Cognome                    string `json:"cognome"`
	Email                      string `json:"email"`
	Telefono                   string `json:"telefono"`
	MaintenanceOnPrimaryEmail  bool   `json:"maintenance_on_primary_email"`
	MarketingOnPrimaryEmail    bool   `json:"marketing_on_primary_email"`
}

// CompletionRequest is the body accepted by
// POST /cp-backoffice/v1/biometric-requests/{id}/completion.
type CompletionRequest struct {
	Completed bool `json:"completed"`
}

// CompletionResponse is the success shape returned by the completion
// mutation. The locked success payload is { "ok": true }.
type CompletionResponse struct {
	Ok bool `json:"ok"`
}
