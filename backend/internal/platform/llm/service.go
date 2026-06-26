package llm

import (
	"database/sql"
	"net/http"
	"time"
)

// Service is the centralized LLM registry/resolver/audit backed by the shared
// mrsmith schema (on Anisetta). Provider config, model/prompt bindings and the
// per-call audit all live there — see deploy/migrations/047 and 048.
//
// Resolution is per-call (no client cache, see design #7): a single shared
// *http.Client pools connections per host across all providers, and the
// lightweight OpenAI-compatible client is constructed per call from the
// resolved provider config.
type Service struct {
	db   *sql.DB
	http *http.Client
}

// New builds a Service over the Anisetta DB handle (where the mrsmith schema
// lives). A nil db yields a Service whose methods return a "not configured"
// error, mirroring how the apps treat a missing AI dependency.
func New(db *sql.DB) *Service {
	return &Service{
		db:   db,
		http: &http.Client{Timeout: 60 * time.Second},
	}
}
