package training

// Tipi di risposta per la persistenza e le letture delle run del sync
// Factorial (#154, task 6.3 di #151; parent #141): la migrazione 133
// introduce le tabelle, factorial_sync_store.go la persistenza e le query.

// FactorialSyncRunRecord e' una run persistita, per la lista (ultime 20) e
// come base del dettaglio.
type FactorialSyncRunRecord struct {
	ID                   string         `json:"id"`
	StartedAt            string         `json:"startedAt"`
	FinishedAt           string         `json:"finishedAt,omitempty"`
	DurationMS           int64          `json:"durationMs"`
	Actor                string         `json:"actor"`
	DryRun               bool           `json:"dryRun"`
	Outcome              string         `json:"outcome"`
	Error                string         `json:"error,omitempty"`
	SessionsWithoutClass int            `json:"sessionsWithoutClass"`
	Counters             map[string]int `json:"counters"`
	FindingCount         int            `json:"findingCount"`
}

// FactorialSyncFindingRecord e' un finding persistito (conflitto o
// warning): LocalEntity/LocalID/EmployeeID sono i riferimenti locali gia'
// disponibili al momento della run, EmployeeName e' risolto in lettura.
type FactorialSyncFindingRecord struct {
	ID           string         `json:"id"`
	Phase        string         `json:"phase"`
	Severity     string         `json:"severity"`
	Kind         string         `json:"kind"`
	Ref          string         `json:"ref"`
	LocalEntity  string         `json:"localEntity,omitempty"`
	LocalID      string         `json:"localId,omitempty"`
	LocalEventID string         `json:"localEventId,omitempty"` // evento proprietario, risolto in lettura per sessioni e iscrizioni
	EmployeeID   string         `json:"employeeId,omitempty"`
	EmployeeName string         `json:"employeeName,omitempty"`
	Detail       map[string]any `json:"detail,omitempty"`
}

// FactorialSyncRunDetail e' la run con tutti i suoi finding, ordinati per
// fase/severita'/kind (GET /training/v1/factorial/sync/runs/{id}).
type FactorialSyncRunDetail struct {
	FactorialSyncRunRecord
	Findings []FactorialSyncFindingRecord `json:"findings"`
}

// FactorialSyncRunListResponse e' la busta di
// GET /training/v1/factorial/sync/runs.
type FactorialSyncRunListResponse struct {
	Runs []FactorialSyncRunRecord `json:"runs"`
}
