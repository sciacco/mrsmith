package compliance

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT b.id, b.request_date, b.reference, b.method_id, m.description
		FROM dns_bl_block b
		JOIN dns_bl_method m ON m.method_id = b.method_id
		ORDER BY b.request_date DESC, b.id DESC`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	blocks := make([]BlockRequest, 0)
	for rows.Next() {
		var b BlockRequest
		if err := rows.Scan(&b.ID, &b.RequestDate, &b.Reference, &b.MethodID, &b.MethodDescription); err != nil {
			httputil.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		blocks = append(blocks, b)
	}

	httputil.JSON(w, http.StatusOK, blocks)
}

func (h *Handler) handleGetBlock(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var b BlockRequest
	err = h.db.QueryRowContext(r.Context(), `
		SELECT b.id, b.request_date, b.reference, b.method_id, m.description
		FROM dns_bl_block b
		JOIN dns_bl_method m ON m.method_id = b.method_id
		WHERE b.id = $1`, id).
		Scan(&b.ID, &b.RequestDate, &b.Reference, &b.MethodID, &b.MethodDescription)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "not_found")
		return
	}

	httputil.JSON(w, http.StatusOK, b)
}

func (h *Handler) handleCreateBlock(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var body CreateBlockRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

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
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var blockID int
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO dns_bl_block (request_date, reference, method_id)
		 VALUES ($1, $2, $3) RETURNING id`,
		body.RequestDate, body.Reference, body.MethodID).Scan(&blockID)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := insertDomains(r.Context(), tx, "dns_bl_block_domain", "block_id", blockID, valid); err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.JSON(w, http.StatusCreated, map[string]any{
		"id":            blockID,
		"domains_count": len(valid),
	})
}

func (h *Handler) handleUpdateBlock(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body UpdateBlockRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := h.db.ExecContext(r.Context(),
		`UPDATE dns_bl_block SET request_date = $1, reference = $2, method_id = $3 WHERE id = $4`,
		body.RequestDate, body.Reference, body.MethodID, id)
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

func (h *Handler) handleListBlockDomains(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, domain FROM dns_bl_block_domain WHERE block_id = $1 ORDER BY id`, id)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	domains := make([]BlockDomain, 0)
	for rows.Next() {
		var d BlockDomain
		if err := rows.Scan(&d.ID, &d.Domain); err != nil {
			httputil.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		domains = append(domains, d)
	}

	httputil.JSON(w, http.StatusOK, domains)
}

func (h *Handler) handleAddBlockDomains(w http.ResponseWriter, r *http.Request) {
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

	if err := insertDomains(r.Context(), tx, "dns_bl_block_domain", "block_id", id, valid); err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		httputil.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"added_count": len(valid)})
}

func (h *Handler) handleUpdateBlockDomain(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	blockID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid block id")
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

	// Ownership check: domain must belong to this block
	res, err := h.db.ExecContext(r.Context(),
		`UPDATE dns_bl_block_domain SET domain = $1 WHERE id = $2 AND block_id = $3`,
		domain, domainID, blockID)
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
