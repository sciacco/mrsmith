package training

import "encoding/json"

// Pannello storia (#164, slice 5 del task 7): lettura di sola scrittura
// mai eseguita qui — training.audit_log e' scritto da s.audit/s.auditFields
// (store_mutations.go) in tutto il resto del package.

// AuditEntry e' una riga del registro modifiche gia' pronta per il pannello:
// changedFields distilla le chiavi cambiate (dal prima/dopo grezzi, o dal
// campo changed_fields scritto da auditFields quando prima non e' stato
// catturato), before/after restano grezzi per l'espansione per riga.
// valuesRecorded e' false per le righe scritte da auditFields (solo nomi di
// campo, nessun prima/dopo): il pannello deve elencare i campi senza inventare
// una freccia prima→dopo che non e' mai stata registrata.
type AuditEntry struct {
	OccurredAt     string          `json:"occurredAt"`
	ActorID        string          `json:"actorId,omitempty"`
	ActorName      string          `json:"actorName"`
	EntityType     string          `json:"entityType"`
	EntityID       string          `json:"entityId"`
	Action         string          `json:"action"`
	ChangedFields  []string        `json:"changedFields"`
	ValuesRecorded bool            `json:"valuesRecorded"`
	Before         json.RawMessage `json:"before"`
	After          json.RawMessage `json:"after"`
}

type AuditHistoryResponse struct {
	Entries []AuditEntry `json:"entries"`
}

// auditEntityTypes e' l'allow-list dei tipi di entita' realmente scritti da
// s.audit/s.auditFields in tutto il package (grep verificato su tutte le
// chiamate, letterali e a tabella variabile: store_mutations.go/upsertSimple
// copre anche vendor/team/skill_area/certification, altrimenti assenti da un
// grep solo sui letterali). Tenuta qui perche' e' un vincolo del contratto
// dati esposto dalla route, non solo un dettaglio di query.
var auditEntityTypes = map[string]bool{
	"certification_award":    true,
	"certification":          true,
	"course":                 true,
	"custom_groups":          true,
	"document":               true,
	"employee":               true,
	"employee_learning_path": true,
	"enrollment":             true,
	"enrollment_session":     true,
	"event_expense":          true,
	"learning_path":          true,
	"skill_area":             true,
	"skill_assessment":       true,
	"team":                   true,
	"team_membership":        true,
	"training_event":         true,
	"training_request":       true,
	"training_rule":          true,
	"training_session":       true,
	"vendor":                 true,
}
