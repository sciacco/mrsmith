package grappadcim

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListRackUnits(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	items, err := h.listUnitsForRack(r, id)
	if err != nil {
		h.dbFailure(w, r, "list_rack_units", err, "rack_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleListRackMedia(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	items, err := h.listMediaForRack(r, id)
	if err != nil {
		h.dbFailure(w, r, "list_rack_media", err, "rack_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleReplaceRackMedia(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rackID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_rack_id")
		return
	}
	var body RackMediaInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_media_payload")
		return
	}
	if len(body.Items) > 200 {
		invalidRequest(w, "too_many_media_items")
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		for _, item := range body.Items {
			if item.UnitID <= 0 || strings.TrimSpace(item.Side) == "" {
				return errBadRequest
			}
			var exists int
			if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM units WHERE id = ? AND racks_id = ?`, item.UnitID, rackID).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				return errBadRequest
			}
			if strings.TrimSpace(item.Path) == "" {
				if _, err := tx.ExecContext(r.Context(), `DELETE FROM media WHERE unit_id = ? AND side = ?`, item.UnitID, strings.TrimSpace(item.Side)); err != nil {
					return err
				}
				continue
			}
			var mediaID int
			err := tx.QueryRowContext(r.Context(), `SELECT id FROM media WHERE unit_id = ? AND side = ? LIMIT 1`, item.UnitID, strings.TrimSpace(item.Side)).Scan(&mediaID)
			if err == sql.ErrNoRows {
				_, err = tx.ExecContext(r.Context(), `INSERT INTO media (path, unit_id, side) VALUES (?, ?, ?)`, strings.TrimSpace(item.Path), item.UnitID, strings.TrimSpace(item.Side))
			} else if err == nil {
				_, err = tx.ExecContext(r.Context(), `UPDATE media SET path = ?, updated_at = NOW() WHERE id = ?`, strings.TrimSpace(item.Path), mediaID)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if err == errBadRequest {
			invalidRequest(w, "invalid_media_payload")
			return
		}
		h.dbFailure(w, r, "replace_rack_media", err, "rack_id", rackID)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: rackID, Message: "Media aggiornati."})
}

func (h *Handler) listUnitsForRack(r *http.Request, rackID int) ([]RackUnit, error) {
	rows, err := h.grappa.QueryContext(r.Context(), `SELECT id, num, racks_id, device_id FROM units WHERE racks_id = ? ORDER BY num ASC, id ASC`, rackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RackUnit{}
	for rows.Next() {
		var item RackUnit
		var num, rack, device sql.NullInt64
		if err := rows.Scan(&item.ID, &num, &rack, &device); err != nil {
			return nil, err
		}
		item.Num = nullableInt(num)
		item.RackID = nullableInt(rack)
		item.DeviceID = nullableInt(device)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *Handler) listMediaForRack(r *http.Request, rackID int) ([]RackMedia, error) {
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT m.id, m.path, m.unit_id, m.side, m.updated_at
		FROM media m
		JOIN units u ON u.id = m.unit_id
		WHERE u.racks_id = ?
		ORDER BY u.num ASC, m.side ASC, m.id ASC`, rackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RackMedia{}
	for rows.Next() {
		var item RackMedia
		var path, side sql.NullString
		var unitID sql.NullInt64
		var updatedAt sql.NullTime
		if err := rows.Scan(&item.ID, &path, &unitID, &side, &updatedAt); err != nil {
			return nil, err
		}
		item.Path = nullableString(path)
		item.UnitID = nullableInt(unitID)
		item.Side = nullableString(side)
		item.UpdatedAt = nullableTime(updatedAt)
		items = append(items, item)
	}
	return items, rows.Err()
}
