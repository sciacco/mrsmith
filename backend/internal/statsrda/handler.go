package statsrda

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	component = "statsrda"

	// List endpoint defaults and bounds.
	defaultListLimit = 50
	maxListLimit     = 100
	// Autocomplete endpoint bounds.
	defaultAutoLimit = 20
	maxAutoLimit     = 50
	// History endpoint bounds.
	defaultHistoryLimit = 20
	maxHistoryLimit     = 100

	errMsgDBUnavailable = "Archivio PA temporaneamente non disponibile"
)

// allowedSortKeys is the whitelist of sortable columns for /pa/issues.
// Order By is built from this set, never from raw user input.
var allowedSortKeys = map[string]string{
	"created":        "i.created",
	"importo_totale": "i.importo_totale",
	"issue_key":      "i.issue_key",
	"reporter_name":  "i.reporter_name",
}

// Handler serves read-only queries over the arak `pa` schema.
type Handler struct {
	db     *sql.DB
	logger *slog.Logger
}

// RegisterRoutes wires the stats-rda endpoints onto the provided mux.
// The module uses only arakDB (Postgres, schema pa).
func RegisterRoutes(mux *http.ServeMux, arakDB *sql.DB) {
	h := &Handler{
		db:     arakDB,
		logger: slog.Default().With("component", component),
	}

	protect := acl.RequireRole(applaunch.StatsRDAAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	handle("GET /stats-rda/v1/pa/filters", h.handleFilters)
	handle("GET /stats-rda/v1/pa/fornitori", h.handleFornitoriAutocomplete)
	handle("GET /stats-rda/v1/pa/richiedenti", h.handleRichiedentiAutocomplete)
	handle("GET /stats-rda/v1/pa/riepilogo", h.handleRiepilogo)
	handle("GET /stats-rda/v1/pa/riepilogo/export", h.handleRiepilogoExport)
	handle("GET /stats-rda/v1/rda/riepilogo", h.handleRiepilogoRDA)
	handle("GET /stats-rda/v1/rda/riepilogo/export", h.handleRiepilogoRDAExport)
	handle("GET /stats-rda/v1/pa/issues", h.handleIssueList)
	handle("GET /stats-rda/v1/pa/issues/{issueKey}", h.handleIssueDetail)
}

// requireDB responds 503 when the arak database is not configured/unavailable.
func (h *Handler) requireDB(w http.ResponseWriter) bool {
	if h.db != nil {
		return true
	}
	httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": errMsgDBUnavailable})
	return false
}

// badRequest writes a 400 with a readable {error} body.
func badRequest(w http.ResponseWriter, message string) {
	httputil.Error(w, http.StatusBadRequest, message)
}

// parseLimit validates a ?limit param against [min,max] with a default.
// On malformed/out-of-range it writes 400 and returns ok=false.
func parseLimit(w http.ResponseWriter, raw string, def, max int) (int, bool) {
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > max {
		badRequest(w, fmt.Sprintf("limit non valido (1-%d)", max))
		return 0, false
	}
	return n, true
}

// parsePage validates a ?page param (>=1, default 1).
func parsePage(w http.ResponseWriter, raw string) (int, bool) {
	if raw == "" {
		return 1, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		badRequest(w, "page non valido (>=1)")
		return 0, false
	}
	return n, true
}

// parseHistoryLimit validates ?history_limit for the detail endpoint.
func parseHistoryLimit(w http.ResponseWriter, raw string) (int, bool) {
	if raw == "" {
		return defaultHistoryLimit, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxHistoryLimit {
		badRequest(w, fmt.Sprintf("history_limit non valido (1-%d)", maxHistoryLimit))
		return 0, false
	}
	return n, true
}

// parseSort validates ?sort and ?dir against the whitelist.
// Returns the SQL column expression and direction ("ASC"/"DESC").
func parseSort(w http.ResponseWriter, sortRaw, dirRaw string) (string, string, bool) {
	col := "i.created"
	if sortRaw != "" {
		expr, ok := allowedSortKeys[sortRaw]
		if !ok {
			badRequest(w, "sort non valido")
			return "", "", false
		}
		col = expr
	}
	dir := "DESC"
	if dirRaw != "" {
		switch strings.ToLower(dirRaw) {
		case "asc":
			dir = "ASC"
		case "desc":
			dir = "DESC"
		default:
			badRequest(w, "dir non valido (asc|desc)")
			return "", "", false
		}
	}
	return col, dir, true
}

// parseDate validates a YYYY-MM-DD date param (nullable).
func parseDate(w http.ResponseWriter, raw string, field string) (string, bool) {
	if raw == "" {
		return "", true
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		badRequest(w, fmt.Sprintf("%s non valida (YYYY-MM-DD)", field))
		return "", false
	}
	return raw, true
}

// issueFilters holds parsed query params for /pa/issues.
type issueFilters struct {
	q           string
	budget      string
	stato       string
	tipo        string
	fornitore   string
	richiedente string
	valuta      string
	from        string
	to          string
	sortCol     string
	sortDir     string
	page        int
	limit       int
}

// parseIssueFilters reads and validates all /pa/issues query params.
func parseIssueFilters(w http.ResponseWriter, q url.Values) (issueFilters, bool) {
	f := issueFilters{
		q:           strings.TrimSpace(q.Get("q")),
		budget:      strings.TrimSpace(q.Get("budget")),
		stato:       strings.TrimSpace(q.Get("stato")),
		tipo:        strings.TrimSpace(q.Get("tipo")),
		fornitore:   strings.TrimSpace(q.Get("fornitore")),
		richiedente: strings.TrimSpace(q.Get("richiedente")),
		valuta:      strings.TrimSpace(q.Get("valuta")),
	}
	var ok bool
	if f.from, ok = parseDate(w, q.Get("from"), "from"); !ok {
		return f, false
	}
	if f.to, ok = parseDate(w, q.Get("to"), "to"); !ok {
		return f, false
	}
	if f.sortCol, f.sortDir, ok = parseSort(w, q.Get("sort"), q.Get("dir")); !ok {
		return f, false
	}
	if f.page, ok = parsePage(w, q.Get("page")); !ok {
		return f, false
	}
	if f.limit, ok = parseLimit(w, q.Get("limit"), defaultListLimit, maxListLimit); !ok {
		return f, false
	}
	return f, true
}

// sanitizeLike escapes LIKE special characters so user input is matched literally.
func sanitizeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// keep json import used
var _ = json.Marshal
