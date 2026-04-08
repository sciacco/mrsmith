package listini

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// handleListCustomerGroups returns all customer groups.
func (h *Handler) handleListCustomerGroups(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT id, name FROM customers.customer_group ORDER BY name`)
	if err != nil {
		h.dbFailure(w, r, "list_customer_groups", err)
		return
	}
	defer rows.Close()

	type group struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	var result []group
	for rows.Next() {
		var g group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			h.dbFailure(w, r, "list_customer_groups_scan", err)
			return
		}
		result = append(result, g)
	}
	if !h.rowsDone(w, r, rows, "list_customer_groups") {
		return
	}
	if result == nil {
		result = []group{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleGetCustomerGroups returns group IDs for a specific customer.
func (h *Handler) handleGetCustomerGroups(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	// Do NOT filter on deprecated `active` column — see compatibility note in IMPL plan
	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT group_id FROM customers.group_association WHERE customer_id = $1`,
		customerID)
	if err != nil {
		h.dbFailure(w, r, "get_customer_groups", err)
		return
	}
	defer rows.Close()

	var groupIDs []int
	for rows.Next() {
		var gid int
		if err := rows.Scan(&gid); err != nil {
			h.dbFailure(w, r, "get_customer_groups_scan", err)
			return
		}
		groupIDs = append(groupIDs, gid)
	}
	if !h.rowsDone(w, r, rows, "get_customer_groups") {
		return
	}
	if groupIDs == nil {
		groupIDs = []int{}
	}

	httputil.JSON(w, http.StatusOK, map[string][]int{"groupIds": groupIDs})
}

// handleSyncCustomerGroups performs a diff-based sync of group associations.
func (h *Handler) handleSyncCustomerGroups(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	customerID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_customer_id")
		return
	}

	var req GroupSyncRequest
	if err := decodeBody(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_body")
		return
	}

	tx, err := h.mistraDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "sync_customer_groups_begin", err)
		return
	}
	defer h.rollbackTx(r, tx, "sync_customer_groups")

	// Get current associations (do NOT filter on deprecated `active` column)
	rows, err := tx.QueryContext(r.Context(),
		`SELECT group_id FROM customers.group_association WHERE customer_id = $1`,
		customerID)
	if err != nil {
		h.dbFailure(w, r, "sync_customer_groups_query", err)
		return
	}

	currentSet := make(map[int]bool)
	for rows.Next() {
		var gid int
		if err := rows.Scan(&gid); err != nil {
			rows.Close()
			h.dbFailure(w, r, "sync_customer_groups_scan", err)
			return
		}
		currentSet[gid] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "sync_customer_groups_rows", err)
		return
	}

	desiredSet := make(map[int]bool)
	for _, gid := range req.GroupIDs {
		desiredSet[gid] = true
	}

	// Remove associations not in desired set
	for gid := range currentSet {
		if !desiredSet[gid] {
			_, err := tx.ExecContext(r.Context(),
				`DELETE FROM customers.group_association WHERE customer_id = $1 AND group_id = $2`,
				customerID, gid)
			if err != nil {
				h.dbFailure(w, r, "sync_customer_groups_delete", err)
				return
			}
		}
	}

	// Add associations in desired set but not current
	// Set active = true for Appsmith coexistence (see compatibility note in IMPL plan)
	for gid := range desiredSet {
		if !currentSet[gid] {
			_, err := tx.ExecContext(r.Context(), `
				INSERT INTO customers.group_association (customer_id, group_id, active)
				VALUES ($1, $2, true)
				ON CONFLICT DO NOTHING`,
				customerID, gid)
			if err != nil {
				h.dbFailure(w, r, "sync_customer_groups_insert", err)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "sync_customer_groups_commit", err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleListKitDiscountsByGroup returns kit discounts for a specific group.
func (h *Handler) handleListKitDiscountsByGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	groupID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_group_id")
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
		SELECT kcg.kit_id, k.internal_name AS kit_name,
		       kcg.discount_mrc, kcg.discount_nrc
		FROM products.kit_customer_group kcg
		JOIN products.kit k ON k.id = kcg.kit_id
		WHERE kcg.group_id = $1
		  AND k.is_active = true
		ORDER BY k.internal_name`, groupID)
	if err != nil {
		h.dbFailure(w, r, "list_kit_discounts_by_group", err)
		return
	}
	defer rows.Close()

	type discount struct {
		KitID       int     `json:"kit_id"`
		KitName     string  `json:"kit_name"`
		DiscountMRC float64 `json:"discount_mrc"`
		DiscountNRC float64 `json:"discount_nrc"`
	}

	var result []discount
	for rows.Next() {
		var d discount
		if err := rows.Scan(&d.KitID, &d.KitName, &d.DiscountMRC, &d.DiscountNRC); err != nil {
			h.dbFailure(w, r, "list_kit_discounts_by_group_scan", err)
			return
		}
		result = append(result, d)
	}
	if !h.rowsDone(w, r, rows, "list_kit_discounts_by_group") {
		return
	}
	if result == nil {
		result = []discount{}
	}

	httputil.JSON(w, http.StatusOK, result)
}
