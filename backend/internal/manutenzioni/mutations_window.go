package manutenzioni

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) handleCreateWindow(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	var body windowRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "create_window_begin", err, "maintenance_id", maintenanceID)
		return
	}
	defer tx.Rollback()
	if err := ensureMaintenanceExists(r.Context(), tx, maintenanceID); errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "maintenance_not_found")
		return
	} else if err != nil {
		h.dbFailure(w, r, "create_window_check", err, "maintenance_id", maintenanceID)
		return
	}
	windowID, err := h.insertWindow(r, tx, maintenanceID, body)
	if errors.Is(err, errBadRequest) {
		appError(w, http.StatusBadRequest, windowRequestErrorCode(err))
		return
	}
	if err != nil {
		h.dbFailure(w, r, "create_window_insert", err, "maintenance_id", maintenanceID)
		return
	}
	if err := writeEvent(r.Context(), tx, maintenanceID, &windowID, "updated", "Finestra aggiunta", claimsActor(r), map[string]any{"action": "window_created"}); err != nil {
		h.dbFailure(w, r, "create_window_event", err, "maintenance_id", maintenanceID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "create_window_commit", err, "maintenance_id", maintenanceID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusCreated)
}

func (h *Handler) insertWindow(r *http.Request, tx *sql.Tx, maintenanceID int64, body windowRequest) (int64, error) {
	start, end, actualStart, actualEnd, announcedAt, lastNoticeAt, err := validateWindowRequest(body)
	if err != nil {
		return 0, err
	}
	var seqNo int
	if err := tx.QueryRowContext(
		r.Context(),
		`SELECT COALESCE(MAX(seq_no), 0) + 1 FROM maintenance.maintenance_window WHERE maintenance_id = $1`,
		maintenanceID,
	).Scan(&seqNo); err != nil {
		return 0, err
	}
	var id int64
	err = tx.QueryRowContext(
		r.Context(),
		`INSERT INTO maintenance.maintenance_window (
			maintenance_id,
			seq_no,
			scheduled_start_at,
			scheduled_end_at,
			expected_downtime_minutes,
			actual_start_at,
			actual_end_at,
			actual_downtime_minutes,
			cancellation_reason_it,
			cancellation_reason_en,
			announced_at,
			last_notice_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING maintenance_window_id`,
		maintenanceID,
		seqNo,
		start,
		end,
		body.ExpectedDowntimeMinutes,
		actualStart,
		actualEnd,
		body.ActualDowntimeMinutes,
		nullStringPtr(body.CancellationReasonIT),
		nullStringPtr(body.CancellationReasonEN),
		announcedAt,
		lastNoticeAt,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (h *Handler) handleUpdateWindow(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	windowID, err := pathInt64(r, "windowId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_window_id")
		return
	}
	var body windowRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	start, end, actualStart, actualEnd, announcedAt, lastNoticeAt, err := validateWindowRequest(body)
	if errors.Is(err, errBadRequest) {
		appError(w, http.StatusBadRequest, windowRequestErrorCode(err))
		return
	}
	if err != nil {
		h.dbFailure(w, r, "update_window_validate", err, "maintenance_id", maintenanceID, "window_id", windowID)
		return
	}
	res, err := h.maintenance.ExecContext(
		r.Context(),
		`UPDATE maintenance.maintenance_window SET
			scheduled_start_at = $1,
			scheduled_end_at = $2,
			expected_downtime_minutes = $3,
			actual_start_at = $4,
			actual_end_at = $5,
			actual_downtime_minutes = $6,
			cancellation_reason_it = $7,
			cancellation_reason_en = $8,
			announced_at = $9,
			last_notice_at = $10,
			window_status = CASE WHEN $5::timestamptz IS NOT NULL THEN 'executed' ELSE window_status END
		WHERE maintenance_id = $11 AND maintenance_window_id = $12`,
		start,
		end,
		body.ExpectedDowntimeMinutes,
		actualStart,
		actualEnd,
		body.ActualDowntimeMinutes,
		nullStringPtr(body.CancellationReasonIT),
		nullStringPtr(body.CancellationReasonEN),
		announcedAt,
		lastNoticeAt,
		maintenanceID,
		windowID,
	)
	if err != nil {
		h.dbFailure(w, r, "update_window", err, "maintenance_id", maintenanceID, "window_id", windowID)
		return
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		appError(w, http.StatusNotFound, "window_not_found")
		return
	}
	if err := writeEvent(r.Context(), h.maintenance, maintenanceID, &windowID, "updated", "Finestra aggiornata", claimsActor(r), map[string]any{"action": "window_updated"}); err != nil {
		h.dbFailure(w, r, "update_window_event", err, "maintenance_id", maintenanceID, "window_id", windowID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusOK)
}

func (h *Handler) handleCancelWindow(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	windowID, err := pathInt64(r, "windowId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_window_id")
		return
	}
	var body cancelWindowRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "cancel_window_begin", err, "maintenance_id", maintenanceID, "window_id", windowID)
		return
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(r.Context(), `SELECT status FROM maintenance.maintenance WHERE maintenance_id = $1 FOR UPDATE`, maintenanceID).Scan(&status); errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "maintenance_not_found")
		return
	} else if err != nil {
		h.dbFailure(w, r, "cancel_window_status", err, "maintenance_id", maintenanceID)
		return
	}
	if (status == StatusScheduled || status == StatusAnnounced) && strings.TrimSpace(body.ReasonIT) == "" {
		appError(w, http.StatusBadRequest, "cancellation_reason_required")
		return
	}
	res, err := tx.ExecContext(
		r.Context(),
		`UPDATE maintenance.maintenance_window
		SET window_status = 'cancelled', cancellation_reason_it = $1, cancellation_reason_en = $2
		WHERE maintenance_id = $3 AND maintenance_window_id = $4`,
		nullIfEmpty(body.ReasonIT),
		nullStringPtr(body.ReasonEN),
		maintenanceID,
		windowID,
	)
	if err != nil {
		h.dbFailure(w, r, "cancel_window_update", err, "maintenance_id", maintenanceID, "window_id", windowID)
		return
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		appError(w, http.StatusNotFound, "window_not_found")
		return
	}
	if err := writeEvent(r.Context(), tx, maintenanceID, &windowID, "cancelled", "Finestra annullata", claimsActor(r), map[string]any{"reason_it": strings.TrimSpace(body.ReasonIT)}); err != nil {
		h.dbFailure(w, r, "cancel_window_event", err, "maintenance_id", maintenanceID, "window_id", windowID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "cancel_window_commit", err, "maintenance_id", maintenanceID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusOK)
}

func (h *Handler) handleRescheduleWindow(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	var targetWindowID int64
	if rawWindowID := strings.TrimSpace(r.PathValue("windowId")); rawWindowID != "" {
		targetWindowID, err = strconv.ParseInt(rawWindowID, 10, 64)
		if err != nil || targetWindowID <= 0 {
			appError(w, http.StatusBadRequest, "invalid_window_id")
			return
		}
	}
	var body windowRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "reschedule_begin", err, "maintenance_id", maintenanceID)
		return
	}
	defer tx.Rollback()

	var previousID sql.NullInt64
	if targetWindowID > 0 {
		if err := tx.QueryRowContext(
			r.Context(),
			`SELECT maintenance_window_id
			FROM maintenance.maintenance_window
			WHERE maintenance_id = $1 AND maintenance_window_id = $2 AND window_status = 'planned'
			FOR UPDATE`,
			maintenanceID,
			targetWindowID,
		).Scan(&previousID); errors.Is(err, sql.ErrNoRows) {
			appError(w, http.StatusNotFound, "window_not_found")
			return
		} else if err != nil {
			h.dbFailure(w, r, "reschedule_previous", err, "maintenance_id", maintenanceID, "window_id", targetWindowID)
			return
		}
	} else {
		if err := tx.QueryRowContext(
			r.Context(),
			`SELECT maintenance_window_id
			FROM maintenance.maintenance_window
			WHERE maintenance_id = $1 AND window_status = 'planned'
			ORDER BY seq_no DESC
			LIMIT 1
			FOR UPDATE`,
			maintenanceID,
		).Scan(&previousID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			h.dbFailure(w, r, "reschedule_previous", err, "maintenance_id", maintenanceID)
			return
		}
	}
	if previousID.Valid {
		if _, err := tx.ExecContext(r.Context(), `UPDATE maintenance.maintenance_window SET window_status = 'superseded' WHERE maintenance_window_id = $1`, previousID.Int64); err != nil {
			h.dbFailure(w, r, "reschedule_supersede", err, "maintenance_id", maintenanceID, "window_id", previousID.Int64)
			return
		}
	}
	newID, err := h.insertWindow(r, tx, maintenanceID, body)
	if errors.Is(err, errBadRequest) {
		appError(w, http.StatusBadRequest, windowRequestErrorCode(err))
		return
	}
	if err != nil {
		h.dbFailure(w, r, "reschedule_insert", err, "maintenance_id", maintenanceID)
		return
	}
	payload := map[string]any{"new_window_id": newID}
	if previousID.Valid {
		payload["previous_window_id"] = previousID.Int64
	}
	if err := writeEvent(r.Context(), tx, maintenanceID, &newID, "rescheduled", windowRescheduledLabel, claimsActor(r), payload); err != nil {
		h.dbFailure(w, r, "reschedule_event", err, "maintenance_id", maintenanceID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "reschedule_commit", err, "maintenance_id", maintenanceID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusCreated)
}

func updateSetPlaceholder(args *[]any, value any) string {
	*args = append(*args, value)
	return "$" + strconv.Itoa(len(*args))
}
