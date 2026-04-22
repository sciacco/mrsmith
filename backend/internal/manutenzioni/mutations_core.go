package manutenzioni

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) handleCreateMaintenance(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	var body createMaintenanceRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	body.TitleIT = strings.TrimSpace(body.TitleIT)
	if body.TitleIT == "" || body.MaintenanceKindID <= 0 || body.TechnicalDomainID <= 0 || body.CustomerScopeID <= 0 {
		appError(w, http.StatusBadRequest, "required_fields_missing")
		return
	}

	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "create_maintenance_begin", err)
		return
	}
	defer tx.Rollback()

	var id int64
	var createdAt sql.NullTime
	if err := tx.QueryRowContext(
		r.Context(),
		`INSERT INTO maintenance.maintenance (
			title_it,
			title_en,
			description_it,
			description_en,
			maintenance_kind_id,
			technical_domain_id,
			customer_scope_id,
			site_id,
			reason_it,
			reason_en,
			residual_service_it,
			residual_service_en,
			metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)
		RETURNING maintenance_id, created_at`,
		body.TitleIT,
		nullStringPtr(body.TitleEN),
		nullStringPtr(body.DescriptionIT),
		nullStringPtr(body.DescriptionEN),
		body.MaintenanceKindID,
		body.TechnicalDomainID,
		body.CustomerScopeID,
		body.SiteID,
		nullStringPtr(body.ReasonIT),
		nullStringPtr(body.ReasonEN),
		nullStringPtr(body.ResidualServiceIT),
		nullStringPtr(body.ResidualServiceEN),
		rawJSONOrDefault(body.Metadata),
	).Scan(&id, &createdAt); err != nil {
		h.dbFailure(w, r, "create_maintenance_insert", err)
		return
	}
	year := createdAt.Time.Year()
	if !createdAt.Valid {
		year = time.Now().Year()
	}
	code := formatMaintenanceCode(year, id)
	if _, err := tx.ExecContext(r.Context(), `UPDATE maintenance.maintenance SET code = $1 WHERE maintenance_id = $2`, code, id); err != nil {
		h.dbFailure(w, r, "create_maintenance_code", err, "maintenance_id", id)
		return
	}
	if body.FirstWindow != nil {
		if _, err := h.insertWindow(r, tx, id, *body.FirstWindow); err != nil {
			if errors.Is(err, errBadRequest) {
				appError(w, http.StatusBadRequest, "invalid_window")
				return
			}
			h.dbFailure(w, r, "create_maintenance_window", err, "maintenance_id", id)
			return
		}
	}
	if len(body.InitialServiceTaxonomy) > 0 {
		if err := h.replaceClassificationsTx(r, tx, id, serviceTaxonomyClass, body.InitialServiceTaxonomy); err != nil {
			h.classificationMutationError(w, r, "create_initial_service_taxonomy", err, id)
			return
		}
	}
	if len(body.InitialReasonClasses) > 0 {
		if err := h.replaceClassificationsTx(r, tx, id, reasonClassClass, body.InitialReasonClasses); err != nil {
			h.classificationMutationError(w, r, "create_initial_reason_classes", err, id)
			return
		}
	}
	if len(body.InitialImpactEffects) > 0 {
		if err := h.replaceClassificationsTx(r, tx, id, impactEffectClass, body.InitialImpactEffects); err != nil {
			h.classificationMutationError(w, r, "create_initial_impact_effects", err, id)
			return
		}
	}
	if len(body.InitialQualityFlags) > 0 {
		if err := h.replaceClassificationsTx(r, tx, id, qualityFlagClass, body.InitialQualityFlags); err != nil {
			h.classificationMutationError(w, r, "create_initial_quality_flags", err, id)
			return
		}
	}
	for _, target := range body.InitialTargets {
		if err := h.upsertTargetTx(r, tx, id, 0, target); err != nil {
			if errors.Is(err, errBadRequest) {
				appError(w, http.StatusBadRequest, "invalid_target")
				return
			}
			h.dbFailure(w, r, "create_initial_target", err, "maintenance_id", id)
			return
		}
	}
	if err := writeEvent(r.Context(), tx, id, nil, "created", "Manutenzione creata", claimsActor(r), nil); err != nil {
		h.dbFailure(w, r, "create_maintenance_event", err, "maintenance_id", id)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "create_maintenance_commit", err, "maintenance_id", id)
		return
	}
	respondMutationDetail(h, w, r, id, http.StatusCreated)
}

func (h *Handler) handleUpdateMaintenance(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	var body updateMaintenanceRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	args := []any{}
	sets := []string{}
	addString := func(column string, value *string, required bool) bool {
		if value == nil {
			return true
		}
		trimmed := strings.TrimSpace(*value)
		if required && trimmed == "" {
			return false
		}
		sets = append(sets, column+" = "+placeholder(&args, nullIfEmpty(trimmed)))
		return true
	}
	if !addString("title_it", body.TitleIT, true) ||
		!addString("title_en", body.TitleEN, false) ||
		!addString("description_it", body.DescriptionIT, false) ||
		!addString("description_en", body.DescriptionEN, false) ||
		!addString("reason_it", body.ReasonIT, false) ||
		!addString("reason_en", body.ReasonEN, false) ||
		!addString("residual_service_it", body.ResidualServiceIT, false) ||
		!addString("residual_service_en", body.ResidualServiceEN, false) {
		appError(w, http.StatusBadRequest, "required_fields_missing")
		return
	}
	if body.MaintenanceKindID != nil {
		if *body.MaintenanceKindID <= 0 {
			appError(w, http.StatusBadRequest, "invalid_maintenance_kind")
			return
		}
		sets = append(sets, "maintenance_kind_id = "+placeholder(&args, *body.MaintenanceKindID))
	}
	if body.TechnicalDomainID != nil {
		if *body.TechnicalDomainID <= 0 {
			appError(w, http.StatusBadRequest, "invalid_technical_domain")
			return
		}
		sets = append(sets, "technical_domain_id = "+placeholder(&args, *body.TechnicalDomainID))
	}
	if body.CustomerScopeID != nil {
		if *body.CustomerScopeID <= 0 {
			appError(w, http.StatusBadRequest, "invalid_customer_scope")
			return
		}
		sets = append(sets, "customer_scope_id = "+placeholder(&args, *body.CustomerScopeID))
	}
	if body.ClearSite {
		sets = append(sets, "site_id = NULL")
	} else if body.SiteID != nil {
		if *body.SiteID <= 0 {
			appError(w, http.StatusBadRequest, "invalid_site")
			return
		}
		sets = append(sets, "site_id = "+placeholder(&args, *body.SiteID))
	}
	if len(body.Metadata) > 0 {
		sets = append(sets, "metadata = "+placeholder(&args, rawJSONOrDefault(body.Metadata))+"::jsonb")
	}
	if len(sets) == 0 {
		respondMutationDetail(h, w, r, id, http.StatusOK)
		return
	}
	sets = append(sets, "updated_at = now()")
	args = append(args, id)
	query := `UPDATE maintenance.maintenance SET ` + strings.Join(sets, ", ") + ` WHERE maintenance_id = $` + strconv.Itoa(len(args)) + ` RETURNING maintenance_id`
	var updatedID int64
	if err := h.maintenance.QueryRowContext(r.Context(), query, args...).Scan(&updatedID); errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "maintenance_not_found")
		return
	} else if err != nil {
		h.dbFailure(w, r, "update_maintenance", err, "maintenance_id", id)
		return
	}
	if err := writeEvent(r.Context(), h.maintenance, id, nil, "updated", "Riepilogo aggiornato", claimsActor(r), nil); err != nil {
		h.dbFailure(w, r, "update_maintenance_event", err, "maintenance_id", id)
		return
	}
	respondMutationDetail(h, w, r, id, http.StatusOK)
}

func (h *Handler) handleMaintenanceStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	var body statusActionRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	body.Action = strings.TrimSpace(body.Action)
	if body.Action == "approve" && !canApprove(r) {
		appError(w, http.StatusForbidden, "approval_role_required")
		return
	}
	if body.Action != "approve" && !canManage(r) {
		appError(w, http.StatusForbidden, "manager_role_required")
		return
	}

	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "status_begin", err, "maintenance_id", id)
		return
	}
	defer tx.Rollback()

	var current string
	if err := tx.QueryRowContext(r.Context(), `SELECT status FROM maintenance.maintenance WHERE maintenance_id = $1 FOR UPDATE`, id).Scan(&current); errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "maintenance_not_found")
		return
	} else if err != nil {
		h.dbFailure(w, r, "status_load", err, "maintenance_id", id)
		return
	}

	next, eventType, summary, ok := nextStatus(current, body.Action)
	if !ok {
		appError(w, http.StatusBadRequest, "status_transition_not_allowed")
		return
	}
	if (body.Action == "schedule" || body.Action == "announce" || body.Action == "start") && !h.hasUsableWindowTx(r, tx, id) {
		appError(w, http.StatusBadRequest, "maintenance_window_required")
		return
	}
	if body.Action == "cancel" && (current == StatusScheduled || current == StatusAnnounced) && strings.TrimSpace(stringValue(body.ReasonIT)) == "" {
		appError(w, http.StatusBadRequest, "cancellation_reason_required")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE maintenance.maintenance SET status = $1, updated_at = now() WHERE maintenance_id = $2`, next, id); err != nil {
		h.dbFailure(w, r, "status_update", err, "maintenance_id", id)
		return
	}
	payload := map[string]any{
		"action": body.Action,
		"from":   current,
		"to":     next,
	}
	if body.ReasonIT != nil {
		payload["reason_it"] = strings.TrimSpace(*body.ReasonIT)
	}
	if body.ReasonEN != nil {
		payload["reason_en"] = strings.TrimSpace(*body.ReasonEN)
	}
	if err := writeEvent(r.Context(), tx, id, nil, eventType, summary, claimsActor(r), payload); err != nil {
		h.dbFailure(w, r, "status_event", err, "maintenance_id", id)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "status_commit", err, "maintenance_id", id)
		return
	}
	respondMutationDetail(h, w, r, id, http.StatusOK)
}

func nextStatus(current string, action string) (string, string, string, bool) {
	switch action {
	case "approve":
		if current == StatusDraft {
			return StatusApproved, "updated", "Manutenzione approvata", true
		}
	case "schedule":
		if current == StatusApproved || current == StatusAnnounced {
			return StatusScheduled, "updated", "Manutenzione pianificata", true
		}
	case "announce":
		if current == StatusApproved || current == StatusScheduled {
			return StatusAnnounced, "announced", "Manutenzione annunciata", true
		}
	case "start":
		if current == StatusScheduled || current == StatusAnnounced {
			return StatusInProgress, "started", "Manutenzione avviata", true
		}
	case "complete":
		if current == StatusInProgress {
			return StatusCompleted, "completed", "Manutenzione completata", true
		}
	case "cancel":
		if current == StatusDraft || current == StatusApproved || current == StatusScheduled || current == StatusAnnounced {
			return StatusCancelled, "cancelled", "Manutenzione annullata", true
		}
	}
	return "", "", "", false
}

func (h *Handler) hasUsableWindowTx(r *http.Request, tx *sql.Tx, maintenanceID int64) bool {
	var exists bool
	err := tx.QueryRowContext(
		r.Context(),
		`SELECT EXISTS (
			SELECT 1 FROM maintenance.maintenance_window
			WHERE maintenance_id = $1 AND window_status = 'planned'
		)`,
		maintenanceID,
	).Scan(&exists)
	return err == nil && exists
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
