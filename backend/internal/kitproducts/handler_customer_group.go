package kitproducts

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type CustomerGroup struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	IsDefault    bool     `json:"is_default"`
	IsPartner    bool     `json:"is_partner"`
	ReadOnly     bool     `json:"read_only"`
	BaseDiscount *float64 `json:"base_discount"`
}

type CustomerGroupCreateRequest struct {
	Name      string `json:"name"`
	IsPartner bool   `json:"is_partner"`
}

type CustomerGroupUpdateItem struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsPartner bool   `json:"is_partner"`
}

type CustomerGroupBatchUpdateRequest struct {
	Items []CustomerGroupUpdateItem `json:"items"`
}

func (h *Handler) handleListCustomerGroups(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
SELECT id, name, is_default, is_partner, read_only, base_discount
FROM customers.customer_group
ORDER BY name
`)
	if err != nil {
		h.dbFailure(w, r, "list_customer_groups", err)
		return
	}
	defer rows.Close()

	groups := make([]CustomerGroup, 0)
	for rows.Next() {
		group, err := scanCustomerGroup(rows)
		if err != nil {
			h.dbFailure(w, r, "list_customer_groups", err)
			return
		}
		groups = append(groups, group)
	}
	if !h.rowsDone(w, r, rows, "list_customer_groups") {
		return
	}

	httputil.JSON(w, http.StatusOK, groups)
}

func (h *Handler) handleCreateCustomerGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var req CustomerGroupCreateRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httputil.Error(w, http.StatusBadRequest, "name is required")
		return
	}

	var (
		group        CustomerGroup
		baseDiscount sql.NullFloat64
	)
	err := h.mistraDB.QueryRowContext(r.Context(), `
INSERT INTO customers.customer_group (name, is_partner)
VALUES ($1, $2)
RETURNING id, name, is_default, is_partner, read_only, base_discount
`, req.Name, req.IsPartner).Scan(
		&group.ID,
		&group.Name,
		&group.IsDefault,
		&group.IsPartner,
		&group.ReadOnly,
		&baseDiscount,
	)
	if h.rowError(w, r, "create_customer_group", err) {
		return
	}
	group.BaseDiscount = nullFloat(baseDiscount)

	httputil.JSON(w, http.StatusCreated, group)
}

func (h *Handler) handleBatchUpdateCustomerGroups(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	var req CustomerGroupBatchUpdateRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Items) == 0 {
		httputil.Error(w, http.StatusBadRequest, "items are required")
		return
	}

	tx, err := h.mistraDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "batch_update_customer_groups_begin", err)
		return
	}
	defer h.rollbackTx(r, tx, "batch_update_customer_groups")

	for _, item := range req.Items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			httputil.Error(w, http.StatusBadRequest, "name is required")
			return
		}

		var readOnly bool
		if err := tx.QueryRowContext(r.Context(), `
SELECT read_only
FROM customers.customer_group
WHERE id = $1
`, item.ID).Scan(&readOnly); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				httputil.Error(w, http.StatusNotFound, "not_found")
				return
			}
			h.dbFailure(w, r, "batch_update_customer_groups_lookup", err, "group_id", item.ID)
			return
		}
		if readOnly {
			httputil.Error(w, http.StatusBadRequest, "read_only_group")
			return
		}

		if _, err := tx.ExecContext(r.Context(), `
UPDATE customers.customer_group
SET name = $1, is_partner = $2
WHERE id = $3
`, name, item.IsPartner, item.ID); err != nil {
			h.dbFailure(w, r, "batch_update_customer_groups_exec", err, "group_id", item.ID)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "batch_update_customer_groups_commit", err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]int{"updated": len(req.Items)})
}

func scanCustomerGroup(scanner interface{ Scan(dest ...any) error }) (CustomerGroup, error) {
	var (
		group        CustomerGroup
		baseDiscount sql.NullFloat64
	)
	if err := scanner.Scan(
		&group.ID,
		&group.Name,
		&group.IsDefault,
		&group.IsPartner,
		&group.ReadOnly,
		&baseDiscount,
	); err != nil {
		return CustomerGroup{}, err
	}
	group.BaseDiscount = nullFloat(baseDiscount)
	return group, nil
}

func nullFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}
