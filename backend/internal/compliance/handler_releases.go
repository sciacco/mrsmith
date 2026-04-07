package compliance

import (
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
		SELECT id, request_date, reference
		FROM dns_bl_release
		ORDER BY request_date DESC, id DESC`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	releases := make([]ReleaseRequest, 0)
	for rows.Next() {
		var r ReleaseRequest
		if err := rows.Scan(&r.ID, &r.RequestDate, &r.Reference); err != nil {
			httputil.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		releases = append(releases, r)
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
	err = h.db.QueryRowContext(r.Context(),
		`SELECT id, request_date, reference FROM dns_bl_release WHERE id = $1`, id).
		Scan(&rel.ID, &rel.RequestDate, &rel.Reference)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
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

	if body.RequestDate == "" || body.Reference == "" {
		httputil.Error(w, http.StatusBadRequest, "request_date and reference are required")
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
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var releaseID int
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO dns_bl_release (request_date, reference)
		 VALUES ($1, $2) RETURNING id`,
		body.RequestDate, body.Reference).Scan(&releaseID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := insertDomains(r.Context(), tx, "dns_bl_release_domain", "release_id", releaseID, valid); err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
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

	res, err := h.db.ExecContext(r.Context(),
		`UPDATE dns_bl_release SET request_date = $1, reference = $2 WHERE id = $3`,
		body.RequestDate, body.Reference, id)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
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
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	domains := make([]ReleaseDomain, 0)
	for rows.Next() {
		var d ReleaseDomain
		if err := rows.Scan(&d.ID, &d.Domain); err != nil {
			httputil.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		domains = append(domains, d)
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
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	if err := insertDomains(r.Context(), tx, "dns_bl_release_domain", "release_id", id, valid); err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
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
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"id": domainID, "domain": domain})
}
