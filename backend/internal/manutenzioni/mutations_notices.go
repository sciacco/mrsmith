package manutenzioni

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

func (h *Handler) handleCreateNotice(w http.ResponseWriter, r *http.Request) {
	h.handleNoticeMutation(w, r, 0)
}

func (h *Handler) handleUpdateNotice(w http.ResponseWriter, r *http.Request) {
	noticeID, err := pathInt64(r, "noticeId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_notice_id")
		return
	}
	h.handleNoticeMutation(w, r, noticeID)
}

func (h *Handler) handleNoticeMutation(w http.ResponseWriter, r *http.Request, noticeID int64) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	var body noticeRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if !validNoticeType(body.NoticeType) || !validAudience(body.Audience) || body.NoticeChannelID <= 0 {
		appError(w, http.StatusBadRequest, "invalid_notice")
		return
	}
	generationSource := defaultIfEmpty(body.GenerationSource, "manual")
	if !validGenerationSource(generationSource) {
		appError(w, http.StatusBadRequest, "invalid_notice")
		return
	}
	sendStatus := defaultIfEmpty(body.SendStatus, "draft")
	if !validSendStatus(sendStatus) {
		appError(w, http.StatusBadRequest, "invalid_notice_status")
		return
	}
	scheduledSendAt, err := parseOptionalTime(body.ScheduledSendAt)
	if errors.Is(err, errBadRequest) {
		appError(w, http.StatusBadRequest, "invalid_notice_date")
		return
	}
	sentAt, err := parseOptionalTime(body.SentAt)
	if errors.Is(err, errBadRequest) {
		appError(w, http.StatusBadRequest, "invalid_notice_date")
		return
	}
	if sendStatus == "sent" && sentAt == nil {
		appError(w, http.StatusBadRequest, "sent_at_required")
		return
	}

	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "notice_begin", err, "maintenance_id", maintenanceID)
		return
	}
	defer tx.Rollback()

	if body.MaintenanceWindowID != nil {
		if err := ensureWindowBelongs(r, tx, maintenanceID, *body.MaintenanceWindowID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				appError(w, http.StatusBadRequest, "window_not_found")
				return
			}
			h.dbFailure(w, r, "notice_window_check", err, "maintenance_id", maintenanceID)
			return
		}
	}

	if noticeID == 0 {
		if err := tx.QueryRowContext(
			r.Context(),
			`INSERT INTO maintenance.notice (
				maintenance_id,
				maintenance_window_id,
				notice_type,
				audience,
				notice_channel_id,
				template_code,
				template_version,
				generation_source,
				send_status,
				scheduled_send_at,
				sent_at,
				metadata
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb)
			RETURNING notice_id`,
			maintenanceID,
			body.MaintenanceWindowID,
			body.NoticeType,
			body.Audience,
			body.NoticeChannelID,
			nullStringPtr(body.TemplateCode),
			body.TemplateVersion,
			generationSource,
			sendStatus,
			scheduledSendAt,
			sentAt,
			rawJSONOrDefault(body.Metadata),
		).Scan(&noticeID); err != nil {
			h.dbFailure(w, r, "notice_insert", err, "maintenance_id", maintenanceID)
			return
		}
	} else {
		res, err := tx.ExecContext(
			r.Context(),
			`UPDATE maintenance.notice SET
				maintenance_window_id = $1,
				notice_type = $2,
				audience = $3,
				notice_channel_id = $4,
				template_code = $5,
				template_version = $6,
				generation_source = $7,
				send_status = $8,
				scheduled_send_at = $9,
				sent_at = $10,
				metadata = $11::jsonb
			WHERE maintenance_id = $12 AND notice_id = $13`,
			body.MaintenanceWindowID,
			body.NoticeType,
			body.Audience,
			body.NoticeChannelID,
			nullStringPtr(body.TemplateCode),
			body.TemplateVersion,
			generationSource,
			sendStatus,
			scheduledSendAt,
			sentAt,
			rawJSONOrDefault(body.Metadata),
			maintenanceID,
			noticeID,
		)
		if err != nil {
			h.dbFailure(w, r, "notice_update", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
			return
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			appError(w, http.StatusNotFound, "notice_not_found")
			return
		}
	}
	for _, locale := range body.Locales {
		if err := upsertNoticeLocaleTx(r, tx, noticeID, locale); err != nil {
			if errors.Is(err, errBadRequest) {
				appError(w, http.StatusBadRequest, "invalid_notice_locale")
				return
			}
			h.dbFailure(w, r, "notice_locale_upsert", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
			return
		}
	}
	if sendStatus != "draft" {
		if err := validateNoticeContent(r, tx, noticeID, body.Audience); err != nil {
			if errors.Is(err, errBadRequest) {
				appError(w, http.StatusBadRequest, "notice_content_required")
				return
			}
			h.dbFailure(w, r, "notice_content_validate", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
			return
		}
	}
	if err := writeEvent(r.Context(), tx, maintenanceID, body.MaintenanceWindowID, noticeEventType(body.NoticeType, sendStatus), "Comunicazione aggiornata", claimsActor(r), map[string]any{"notice_id": noticeID, "send_status": sendStatus}); err != nil {
		h.dbFailure(w, r, "notice_event", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "notice_commit", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	status := http.StatusOK
	if r.Method == http.MethodPost {
		status = http.StatusCreated
	}
	respondMutationDetail(h, w, r, maintenanceID, status)
}

func ensureWindowBelongs(r *http.Request, q queryer, maintenanceID int64, windowID int64) error {
	var exists bool
	if err := q.QueryRowContext(
		r.Context(),
		`SELECT EXISTS (
			SELECT 1 FROM maintenance.maintenance_window
			WHERE maintenance_id = $1 AND maintenance_window_id = $2
		)`,
		maintenanceID,
		windowID,
	).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}

func (h *Handler) handleUpsertNoticeLocale(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	noticeID, err := pathInt64(r, "noticeId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_notice_id")
		return
	}
	var body localeRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	body.Locale = r.PathValue("locale")
	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "notice_locale_begin", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	defer tx.Rollback()
	if err := ensureNoticeBelongs(r, tx, maintenanceID, noticeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			appError(w, http.StatusNotFound, "notice_not_found")
			return
		}
		h.dbFailure(w, r, "notice_locale_check", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	if err := upsertNoticeLocaleTx(r, tx, noticeID, body); err != nil {
		if errors.Is(err, errBadRequest) {
			appError(w, http.StatusBadRequest, "invalid_notice_locale")
			return
		}
		h.dbFailure(w, r, "notice_locale_upsert", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "notice_locale_commit", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusOK)
}

func upsertNoticeLocaleTx(r *http.Request, tx *sql.Tx, noticeID int64, body localeRequest) error {
	body.Locale = strings.TrimSpace(body.Locale)
	body.Subject = strings.TrimSpace(body.Subject)
	if (body.Locale != "it" && body.Locale != "en") || body.Subject == "" {
		return errBadRequest
	}
	_, err := tx.ExecContext(
		r.Context(),
		`INSERT INTO maintenance.notice_locale (notice_id, locale, subject, body_html, body_text)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (notice_id, locale) DO UPDATE SET
			subject = excluded.subject,
			body_html = excluded.body_html,
			body_text = excluded.body_text`,
		noticeID,
		body.Locale,
		body.Subject,
		nullStringPtr(body.BodyHTML),
		nullStringPtr(body.BodyText),
	)
	return err
}

func ensureNoticeBelongs(r *http.Request, q queryer, maintenanceID int64, noticeID int64) error {
	var exists bool
	if err := q.QueryRowContext(
		r.Context(),
		`SELECT EXISTS (
			SELECT 1 FROM maintenance.notice
			WHERE maintenance_id = $1 AND notice_id = $2
		)`,
		maintenanceID,
		noticeID,
	).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}

func validateNoticeContent(r *http.Request, tx *sql.Tx, noticeID int64, audience string) error {
	required := []string{"it"}
	if audience == "external" {
		required = append(required, "en")
	}
	for _, locale := range required {
		var ok bool
		if err := tx.QueryRowContext(
			r.Context(),
			`SELECT EXISTS (
				SELECT 1 FROM maintenance.notice_locale
				WHERE notice_id = $1
				AND locale = $2
				AND btrim(subject) <> ''
				AND (COALESCE(btrim(body_text), '') <> '' OR COALESCE(btrim(body_html), '') <> '')
			)`,
			noticeID,
			locale,
		).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return errBadRequest
		}
	}
	return nil
}

func (h *Handler) handleNoticeStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	noticeID, err := pathInt64(r, "noticeId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_notice_id")
		return
	}
	var body noticeStatusRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	body.SendStatus = strings.TrimSpace(body.SendStatus)
	if !validSendStatus(body.SendStatus) {
		appError(w, http.StatusBadRequest, "invalid_notice_status")
		return
	}
	sentAt, err := parseOptionalTime(body.SentAt)
	if errors.Is(err, errBadRequest) {
		appError(w, http.StatusBadRequest, "invalid_notice_date")
		return
	}
	if body.SendStatus == "sent" && sentAt == nil {
		appError(w, http.StatusBadRequest, "sent_at_required")
		return
	}
	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "notice_status_begin", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	defer tx.Rollback()
	var noticeType, audience string
	var windowID sql.NullInt64
	if err := tx.QueryRowContext(
		r.Context(),
		`SELECT notice_type, audience, maintenance_window_id
		FROM maintenance.notice
		WHERE maintenance_id = $1 AND notice_id = $2
		FOR UPDATE`,
		maintenanceID,
		noticeID,
	).Scan(&noticeType, &audience, &windowID); errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "notice_not_found")
		return
	} else if err != nil {
		h.dbFailure(w, r, "notice_status_load", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	if body.SendStatus != "draft" {
		if err := validateNoticeContent(r, tx, noticeID, audience); err != nil {
			if errors.Is(err, errBadRequest) {
				appError(w, http.StatusBadRequest, "notice_content_required")
				return
			}
			h.dbFailure(w, r, "notice_status_content", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
			return
		}
	}
	if _, err := tx.ExecContext(
		r.Context(),
		`UPDATE maintenance.notice SET send_status = $1, sent_at = $2 WHERE maintenance_id = $3 AND notice_id = $4`,
		body.SendStatus,
		sentAt,
		maintenanceID,
		noticeID,
	); err != nil {
		h.dbFailure(w, r, "notice_status_update", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	eventWindowID := nullInt64Value(windowID)
	if err := writeEvent(r.Context(), tx, maintenanceID, eventWindowID, noticeEventType(noticeType, body.SendStatus), "Stato comunicazione aggiornato", claimsActor(r), map[string]any{"notice_id": noticeID, "send_status": body.SendStatus}); err != nil {
		h.dbFailure(w, r, "notice_status_event", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "notice_status_commit", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusOK)
}

func (h *Handler) handleReplaceNoticeQualityFlags(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	noticeID, err := pathInt64(r, "noticeId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_notice_id")
		return
	}
	var body noticeQualityFlagsRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "notice_flags_begin", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	defer tx.Rollback()
	if err := ensureNoticeBelongs(r, tx, maintenanceID, noticeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			appError(w, http.StatusNotFound, "notice_not_found")
			return
		}
		h.dbFailure(w, r, "notice_flags_check", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM maintenance.notice_quality_flag WHERE notice_id = $1`, noticeID); err != nil {
		h.dbFailure(w, r, "notice_flags_delete", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	seen := map[int64]struct{}{}
	for _, item := range body.Items {
		if item.ReferenceID <= 0 {
			appError(w, http.StatusBadRequest, "invalid_notice_quality_flag")
			return
		}
		if _, ok := seen[item.ReferenceID]; ok {
			appError(w, http.StatusBadRequest, "invalid_notice_quality_flag")
			return
		}
		seen[item.ReferenceID] = struct{}{}
		source := normalizeSource(item.Source)
		if !validClassificationSource(qualityFlagClass, source) || (item.Confidence != nil && (*item.Confidence < 0 || *item.Confidence > 1)) {
			appError(w, http.StatusBadRequest, "invalid_notice_quality_flag")
			return
		}
		if _, err := tx.ExecContext(
			r.Context(),
			`INSERT INTO maintenance.notice_quality_flag (notice_id, quality_flag_id, source, confidence, metadata)
			VALUES ($1, $2, $3, $4, $5::jsonb)`,
			noticeID,
			item.ReferenceID,
			source,
			item.Confidence,
			rawJSONOrDefault(item.Metadata),
		); err != nil {
			h.dbFailure(w, r, "notice_flags_insert", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "notice_flags_commit", err, "maintenance_id", maintenanceID, "notice_id", noticeID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusOK)
}

func validNoticeType(value string) bool {
	switch value {
	case "announcement", "reminder", "reschedule", "cancellation", "start", "completion", "internal_update":
		return true
	default:
		return false
	}
}

func validAudience(value string) bool {
	return value == "internal" || value == "external"
}

func validGenerationSource(value string) bool {
	switch value {
	case "manual", "ai", "system", "import":
		return true
	default:
		return false
	}
}

func validSendStatus(value string) bool {
	switch value {
	case "draft", "ready", "sent", "failed", "suppressed":
		return true
	default:
		return false
	}
}

func noticeEventType(noticeType string, sendStatus string) string {
	if noticeType == "announcement" && (sendStatus == "ready" || sendStatus == "sent") {
		return "announced"
	}
	if noticeType == "reminder" && sendStatus == "sent" {
		return "reminder_sent"
	}
	return "updated"
}
