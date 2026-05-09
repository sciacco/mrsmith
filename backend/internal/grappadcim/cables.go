package grappadcim

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

var errCableFibersAssigned = errors.New("cable fibers assigned")

func (h *Handler) handleListCables(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(c.name LIKE ? OR c.description LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	rows, err := h.grappa.QueryContext(r.Context(), cableSelectSQL()+" WHERE "+strings.Join(where, " AND ")+" GROUP BY c.id, c.name, c.description, c.fibers_num, c.status ORDER BY c.name ASC, c.id ASC", args...)
	if err != nil {
		h.dbFailure(w, r, "list_cables", err)
		return
	}
	defer rows.Close()
	items := []Cable{}
	for rows.Next() {
		item, err := scanCable(rows)
		if err != nil {
			h.dbFailure(w, r, "list_cables_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_cables") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetCable(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_cable_id")
		return
	}
	item, found, err := h.getCable(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_cable", err, "cable_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "cable_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateCable(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body CableInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_cable_payload")
		return
	}
	if strings.TrimSpace(body.Name) == "" || body.FibersNum <= 0 {
		invalidRequest(w, "cable_name_fibers_required")
		return
	}
	status := strings.TrimSpace(body.Status)
	if status == "" {
		status = "Attivo"
	}
	description := strings.TrimSpace(body.Description)
	var id int64
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(r.Context(), `
			INSERT INTO cables (name, description, fibers_num, status)
			VALUES (?, ?, ?, ?)`, strings.TrimSpace(body.Name), description, body.FibersNum, status)
		if err != nil {
			return err
		}
		id, _ = result.LastInsertId()
		for fiber := 1; fiber <= body.FibersNum; fiber++ {
			if _, err := tx.ExecContext(r.Context(), `INSERT INTO fibers (num, status, cable_id) VALUES (?, 'Libera', ?)`, fiber, id); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		h.dbFailure(w, r, "create_cable", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Cavo creato con fibre libere."})
}

func (h *Handler) handleUpdateCable(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_cable_id")
		return
	}
	var body CablePatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_cable_payload")
		return
	}
	sets := []string{}
	args := []any{}
	if body.Name != nil {
		if strings.TrimSpace(*body.Name) == "" {
			invalidRequest(w, "cable_name_required")
			return
		}
		sets = append(sets, "name = ?")
		args = append(args, strings.TrimSpace(*body.Name))
	}
	if body.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, strings.TrimSpace(*body.Description))
	}
	if body.Status != nil {
		if strings.TrimSpace(*body.Status) == "" {
			invalidRequest(w, "cable_status_required")
			return
		}
		sets = append(sets, "status = ?")
		args = append(args, strings.TrimSpace(*body.Status))
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), "UPDATE cables SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	if err != nil {
		h.dbFailure(w, r, "update_cable", err, "cable_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "cable_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Cavo aggiornato."})
}

func (h *Handler) handleDeleteCable(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_cable_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(r.Context(), `
			SELECT f.id, f.status, f.left_port_id, f.right_port_id
			FROM fibers f
			WHERE f.cable_id = ?
			FOR UPDATE`, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		found := false
		fiberIDs := []int{}
		for rows.Next() {
			var fiberID int
			var status string
			var left, right sql.NullInt64
			if err := rows.Scan(&fiberID, &status, &left, &right); err != nil {
				return err
			}
			found = true
			fiberIDs = append(fiberIDs, fiberID)
			if !strings.EqualFold(strings.TrimSpace(status), "Libera") || left.Valid || right.Valid {
				return errCableFibersAssigned
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if !found {
			var exists int
			if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM cables WHERE id = ?`, id).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				return sql.ErrNoRows
			}
		}
		if len(fiberIDs) > 0 {
			fiberPlaceholders := placeholders(len(fiberIDs))
			query := fmt.Sprintf(`
				SELECT id
				FROM ports
				WHERE cable_fiber_id IN (%s)
				   OR fo_in_id IN (%s)
				   OR fo_out_id IN (%s)
				FOR UPDATE`, fiberPlaceholders, fiberPlaceholders, fiberPlaceholders)
			args := make([]any, 0, len(fiberIDs)*3)
			for repeat := 0; repeat < 3; repeat++ {
				for _, fiberID := range fiberIDs {
					args = append(args, fiberID)
				}
			}
			portRows, err := tx.QueryContext(r.Context(), query, args...)
			if err != nil {
				return err
			}
			hasPortRef := portRows.Next()
			if err := portRows.Err(); err != nil {
				_ = portRows.Close()
				return err
			}
			if err := portRows.Close(); err != nil {
				return err
			}
			if hasPortRef {
				return errCableFibersAssigned
			}
		}
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM fibers WHERE cable_id = ?`, id); err != nil {
			return err
		}
		result, err := tx.ExecContext(r.Context(), `DELETE FROM cables WHERE id = ?`, id)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return sql.ErrNoRows
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "cable_not_found")
			return
		}
		if err == errCableFibersAssigned {
			httputil.Error(w, http.StatusConflict, "cable_fibers_assigned")
			return
		}
		h.dbFailure(w, r, "delete_cable", err, "cable_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Cavo eliminato."})
}

func (h *Handler) handleListCableFibers(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_cable_id")
		return
	}
	rows, err := h.grappa.QueryContext(r.Context(), fiberSelectSQL()+` WHERE f.cable_id = ? ORDER BY f.num ASC, f.id ASC`, id)
	if err != nil {
		h.dbFailure(w, r, "list_cable_fibers", err, "cable_id", id)
		return
	}
	defer rows.Close()
	items := []Fiber{}
	for rows.Next() {
		item, err := scanFiber(rows)
		if err != nil {
			h.dbFailure(w, r, "list_cable_fibers_scan", err, "cable_id", id)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_cable_fibers") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) getCable(r *http.Request, id int) (Cable, bool, error) {
	row := h.grappa.QueryRowContext(r.Context(), cableSelectSQL()+` WHERE c.id = ? GROUP BY c.id, c.name, c.description, c.fibers_num, c.status`, id)
	item, err := scanCable(row)
	if err == sql.ErrNoRows {
		return Cable{}, false, nil
	}
	return item, err == nil, err
}

func cableSelectSQL() string {
	return `
		SELECT c.id, c.name, c.description, c.fibers_num, c.status,
		       COUNT(DISTINCT CASE WHEN f.left_port_id IS NOT NULL OR f.right_port_id IS NOT NULL OR LOWER(TRIM(f.status)) <> 'libera' THEN f.id END) AS assigned_fibers
		FROM cables c
		LEFT JOIN fibers f ON f.cable_id = c.id`
}

type cableScanner interface {
	Scan(dest ...any) error
}

func scanCable(scanner cableScanner) (Cable, error) {
	var item Cable
	if err := scanner.Scan(&item.ID, &item.Name, &item.Description, &item.FibersNum, &item.Status, &item.AssignedFibers); err != nil {
		return item, err
	}
	return item, nil
}

func fiberSelectSQL() string {
	return `
		SELECT f.id, f.num, f.status, f.cable_id, f.left_port_id, f.right_port_id,
		       lp.name, rp.name
		FROM fibers f
		LEFT JOIN ports lp ON lp.id = f.left_port_id
		LEFT JOIN ports rp ON rp.id = f.right_port_id`
}

type fiberScanner interface {
	Scan(dest ...any) error
}

func scanFiber(scanner fiberScanner) (Fiber, error) {
	var item Fiber
	var left, right sql.NullInt64
	var leftLabel, rightLabel sql.NullString
	if err := scanner.Scan(&item.ID, &item.Number, &item.Status, &item.CableID, &left, &right, &leftLabel, &rightLabel); err != nil {
		return item, err
	}
	item.LeftPortID = nullableInt(left)
	item.RightPortID = nullableInt(right)
	item.LeftLabel = nullableString(leftLabel)
	item.RightLabel = nullableString(rightLabel)
	return item, nil
}

func fiberLabel(id int, name sql.NullString, num sql.NullInt64) string {
	if name.Valid && strings.TrimSpace(name.String) != "" {
		return strings.TrimSpace(name.String)
	}
	if num.Valid {
		return fmt.Sprintf("Porta %d", num.Int64)
	}
	return fmt.Sprintf("Porta #%d", id)
}
