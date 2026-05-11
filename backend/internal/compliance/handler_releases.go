package compliance

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListReleases(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT r.id, r.request_date, r.reference, r.method_id, m.description
		FROM dns_bl_release r
		LEFT JOIN dns_bl_method m ON m.method_id = r.method_id
		ORDER BY r.request_date DESC, r.id DESC`)
	if err != nil {
		h.dbFailure(w, r, "list_releases", err)
		return
	}
	defer rows.Close()

	releases := make([]ReleaseRequest, 0)
	for rows.Next() {
		var rel ReleaseRequest
		var methodID sql.NullString
		var methodDescription sql.NullString
		if err := rows.Scan(&rel.ID, &rel.RequestDate, &rel.Reference, &methodID, &methodDescription); err != nil {
			h.dbFailure(w, r, "list_releases", err)
			return
		}
		if methodID.Valid {
			value := methodID.String
			rel.MethodID = &value
		}
		if methodDescription.Valid {
			value := methodDescription.String
			rel.MethodDescription = &value
		}
		releases = append(releases, rel)
	}
	if !h.rowsDone(w, r, rows, "list_releases") {
		return
	}

	httputil.JSON(w, http.StatusOK, releases)
}

func (h *Handler) handleGetRelease(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var rel ReleaseRequest
	var methodID sql.NullString
	var methodDescription sql.NullString
	err = h.db.QueryRowContext(r.Context(), `
		SELECT r.id, r.request_date, r.reference, r.method_id, m.description
		FROM dns_bl_release r
		LEFT JOIN dns_bl_method m ON m.method_id = r.method_id
		WHERE r.id = $1`, id).
		Scan(&rel.ID, &rel.RequestDate, &rel.Reference, &methodID, &methodDescription)
	if h.rowError(w, r, "get_release", err, "release_id", id) {
		return
	}
	if methodID.Valid {
		value := methodID.String
		rel.MethodID = &value
	}
	if methodDescription.Valid {
		value := methodDescription.String
		rel.MethodDescription = &value
	}

	httputil.JSON(w, http.StatusOK, rel)
}

func (h *Handler) handleCreateRelease(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var body CreateReleaseRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Reference = strings.TrimSpace(body.Reference)
	body.MethodID = strings.TrimSpace(body.MethodID)

	if body.RequestDate == "" || body.Reference == "" || body.MethodID == "" {
		httputil.Error(w, http.StatusBadRequest, "request_date, reference, and method_id are required")
		return
	}

	_, invalid := ValidateDomains(body.Domains)
	if len(invalid) > 0 {
		httputil.JSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_domains",
			"message": fmt.Sprintf("%d invalid domain(s)", len(invalid)),
			"invalid": invalid,
		})
		return
	}

	valid, _ := ValidateDomains(body.Domains)
	if len(valid) == 0 {
		httputil.Error(w, http.StatusBadRequest, "at least one domain is required")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "create_release", err)
		return
	}
	defer h.rollbackTx(r, tx, "create_release")

	var releaseID int
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO dns_bl_release (request_date, reference, method_id)
		 VALUES ($1, $2, $3) RETURNING id`,
		body.RequestDate, body.Reference, body.MethodID).Scan(&releaseID)
	if err != nil {
		h.dbFailure(w, r, "create_release", err)
		return
	}

	if err := insertDomains(r.Context(), tx, "dns_bl_release_domain", "release_id", releaseID, valid); err != nil {
		h.dbFailure(w, r, "create_release", err, "release_id", releaseID)
		return
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "create_release", err, "release_id", releaseID)
		return
	}

	httputil.JSON(w, http.StatusCreated, map[string]any{
		"id":            releaseID,
		"domains_count": len(valid),
	})
}

func (h *Handler) handleUpdateRelease(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body UpdateReleaseRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Reference = strings.TrimSpace(body.Reference)
	body.MethodID = strings.TrimSpace(body.MethodID)

	if body.RequestDate == "" || body.Reference == "" || body.MethodID == "" {
		httputil.Error(w, http.StatusBadRequest, "request_date, reference, and method_id are required")
		return
	}

	res, err := h.db.ExecContext(r.Context(),
		`UPDATE dns_bl_release SET request_date = $1, reference = $2, method_id = $3 WHERE id = $4`,
		body.RequestDate, body.Reference, body.MethodID, id)
	if err != nil {
		h.dbFailure(w, r, "update_release", err, "release_id", id)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"id": id})
}

func (h *Handler) handleListReleaseDomains(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, domain FROM dns_bl_release_domain WHERE release_id = $1 ORDER BY id`, id)
	if err != nil {
		h.dbFailure(w, r, "list_release_domains", err, "release_id", id)
		return
	}
	defer rows.Close()

	domains := make([]ReleaseDomain, 0)
	for rows.Next() {
		var d ReleaseDomain
		if err := rows.Scan(&d.ID, &d.Domain); err != nil {
			h.dbFailure(w, r, "list_release_domains", err, "release_id", id)
			return
		}
		domains = append(domains, d)
	}
	if !h.rowsDone(w, r, rows, "list_release_domains", "release_id", id) {
		return
	}

	httputil.JSON(w, http.StatusOK, domains)
}

func (h *Handler) handleAddReleaseDomains(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body AddDomainsRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_, invalid := ValidateDomains(body.Domains)
	if len(invalid) > 0 {
		httputil.JSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_domains",
			"message": fmt.Sprintf("%d invalid domain(s)", len(invalid)),
			"invalid": invalid,
		})
		return
	}

	valid, _ := ValidateDomains(body.Domains)
	if len(valid) == 0 {
		httputil.Error(w, http.StatusBadRequest, "at least one domain is required")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "add_release_domains", err, "release_id", id)
		return
	}
	defer h.rollbackTx(r, tx, "add_release_domains", "release_id", id)

	if err := insertDomains(r.Context(), tx, "dns_bl_release_domain", "release_id", id, valid); err != nil {
		h.dbFailure(w, r, "add_release_domains", err, "release_id", id)
		return
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "add_release_domains", err, "release_id", id)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"added_count": len(valid)})
}

func (h *Handler) handleUpdateReleaseDomain(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	releaseID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid release id")
		return
	}
	domainID, err := pathID(r, "domainId")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid domain id")
		return
	}

	var body UpdateDomainRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	domain := strings.TrimSpace(body.Domain)
	if !ValidateFQDN(domain) {
		httputil.Error(w, http.StatusBadRequest, "invalid FQDN")
		return
	}

	// Ownership check: domain must belong to this release
	res, err := h.db.ExecContext(r.Context(),
		`UPDATE dns_bl_release_domain SET domain = $1 WHERE id = $2 AND release_id = $3`,
		domain, domainID, releaseID)
	if err != nil {
		h.dbFailure(w, r, "update_release_domain", err, "release_id", releaseID, "domain_id", domainID)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"id": domainID, "domain": domain})
}
