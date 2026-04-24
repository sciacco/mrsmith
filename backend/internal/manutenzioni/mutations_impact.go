package manutenzioni

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

func (h *Handler) handleReplaceServiceTaxonomy(w http.ResponseWriter, r *http.Request) {
	h.handleReplaceClassifications(w, r, serviceTaxonomyClass)
}

func (h *Handler) handleReplaceReasonClasses(w http.ResponseWriter, r *http.Request) {
	h.handleReplaceClassifications(w, r, reasonClassClass)
}

func (h *Handler) handleReplaceImpactEffects(w http.ResponseWriter, r *http.Request) {
	h.handleReplaceClassifications(w, r, impactEffectClass)
}

func (h *Handler) handleReplaceQualityFlags(w http.ResponseWriter, r *http.Request) {
	h.handleReplaceClassifications(w, r, qualityFlagClass)
}

func (h *Handler) handleReplaceClassifications(w http.ResponseWriter, r *http.Request, meta classificationMeta) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	var body classificationRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "classification_begin", err, "maintenance_id", maintenanceID)
		return
	}
	defer tx.Rollback()
	if err := h.replaceClassificationsTx(r, tx, maintenanceID, meta, body.Items); err != nil {
		h.classificationMutationError(w, r, "classification_replace", err, maintenanceID)
		return
	}
	if err := writeEvent(r.Context(), tx, maintenanceID, nil, "classified", meta.EventLabel, claimsActor(r), map[string]any{"resource": meta.Resource.Key}); err != nil {
		h.dbFailure(w, r, "classification_event", err, "maintenance_id", maintenanceID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "classification_commit", err, "maintenance_id", maintenanceID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusOK)
}

func (h *Handler) replaceClassificationsTx(r *http.Request, tx *sql.Tx, maintenanceID int64, meta classificationMeta, items []classificationInput) error {
	if err := ensureMaintenanceExists(r.Context(), tx, maintenanceID); err != nil {
		return err
	}
	primaryCount := 0
	seen := map[int64]struct{}{}
	for i := range items {
		items[i].ReferenceID = resolveClassificationReferenceID(meta, items[i])
		if items[i].ReferenceID <= 0 {
			return errBadRequest
		}
		if _, ok := seen[items[i].ReferenceID]; ok {
			return errBadRequest
		}
		seen[items[i].ReferenceID] = struct{}{}
		if items[i].Confidence != nil && (*items[i].Confidence < 0 || *items[i].Confidence > 1) {
			return errBadRequest
		}
		if meta.Resource.Kind == resourceService {
			if !validServiceRole(defaultIfEmpty(items[i].Role, "operated")) {
				return errBadRequest
			}
			if !validSeverity(defaultIfEmpty(items[i].ExpectedSeverity, "unavailable")) {
				return errBadRequest
			}
			if items[i].ExpectedAudience != nil && !validExpectedAudience(*items[i].ExpectedAudience) {
				return errBadRequest
			}
		}
		if items[i].IsPrimary {
			primaryCount++
		}
	}
	if meta.HasPrimary && primaryCount > 1 {
		return errBadRequest
	}
	if meta.Resource.Kind == resourceService {
		maintenanceDomainID, err := h.loadMaintenanceTechnicalDomainTx(r, tx, maintenanceID)
		if err != nil {
			return err
		}
		serviceDomains, err := h.loadServiceTechnicalDomainsTx(r, tx, items)
		if err != nil {
			return err
		}
		if err := validateOperatedServiceDomains(maintenanceDomainID, serviceDomains, items); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM `+meta.RelationTable+` WHERE maintenance_id = $1`, maintenanceID); err != nil {
		return err
	}
	for _, item := range items {
		source := normalizeSource(item.Source)
		if !validClassificationSource(meta, source) {
			return errBadRequest
		}
		if meta.Resource.Kind == resourceService {
			_, err := tx.ExecContext(
				r.Context(),
				`INSERT INTO `+meta.RelationTable+` (
					maintenance_id, `+meta.ReferenceIDColumn+`, source, confidence, is_primary,
					role, expected_severity, expected_audience, metadata
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)`,
				maintenanceID,
				item.ReferenceID,
				source,
				item.Confidence,
				item.IsPrimary,
				defaultIfEmpty(item.Role, "operated"),
				defaultIfEmpty(item.ExpectedSeverity, "unavailable"),
				cleanExpectedAudiencePtr(item.ExpectedAudience),
				rawJSONOrDefault(item.Metadata),
			)
			if err != nil {
				return err
			}
		} else if meta.HasPrimary {
			_, err := tx.ExecContext(
				r.Context(),
				`INSERT INTO `+meta.RelationTable+` (
					maintenance_id, `+meta.ReferenceIDColumn+`, source, confidence, is_primary, metadata
				) VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
				maintenanceID,
				item.ReferenceID,
				source,
				item.Confidence,
				item.IsPrimary,
				rawJSONOrDefault(item.Metadata),
			)
			if err != nil {
				return err
			}
		} else {
			_, err := tx.ExecContext(
				r.Context(),
				`INSERT INTO `+meta.RelationTable+` (
					maintenance_id, `+meta.ReferenceIDColumn+`, source, confidence, metadata
				) VALUES ($1, $2, $3, $4, $5::jsonb)`,
				maintenanceID,
				item.ReferenceID,
				source,
				item.Confidence,
				rawJSONOrDefault(item.Metadata),
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Handler) loadMaintenanceTechnicalDomainTx(r *http.Request, tx *sql.Tx, maintenanceID int64) (int64, error) {
	var domainID int64
	if err := tx.QueryRowContext(
		r.Context(),
		`SELECT technical_domain_id FROM maintenance.maintenance WHERE maintenance_id = $1`,
		maintenanceID,
	).Scan(&domainID); err != nil {
		return 0, err
	}
	return domainID, nil
}

func (h *Handler) loadServiceTechnicalDomainsTx(r *http.Request, tx *sql.Tx, items []classificationInput) (map[int64]int64, error) {
	ids := make([]int64, 0, len(items))
	seen := map[int64]struct{}{}
	for _, item := range items {
		if _, ok := seen[item.ReferenceID]; ok {
			continue
		}
		seen[item.ReferenceID] = struct{}{}
		ids = append(ids, item.ReferenceID)
	}
	if len(ids) == 0 {
		return map[int64]int64{}, nil
	}
	args := []any{}
	holders := make([]string, 0, len(ids))
	for _, id := range ids {
		holders = append(holders, placeholder(&args, id))
	}
	rows, err := tx.QueryContext(
		r.Context(),
		`SELECT service_taxonomy_id, technical_domain_id
		FROM maintenance.service_taxonomy
		WHERE service_taxonomy_id IN (`+strings.Join(holders, ", ")+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	domains := map[int64]int64{}
	for rows.Next() {
		var serviceID int64
		var domainID int64
		if err := rows.Scan(&serviceID, &domainID); err != nil {
			return nil, err
		}
		domains[serviceID] = domainID
	}
	return domains, rows.Err()
}

func validateOperatedServiceDomains(maintenanceDomainID int64, serviceDomains map[int64]int64, items []classificationInput) error {
	for _, item := range items {
		role := defaultIfEmpty(item.Role, "operated")
		serviceDomainID, ok := serviceDomains[item.ReferenceID]
		if !ok {
			return errBadRequest
		}
		if role == "operated" && serviceDomainID != maintenanceDomainID {
			return errBadRequest
		}
	}
	return nil
}

func resolveClassificationReferenceID(meta classificationMeta, item classificationInput) int64 {
	if meta.Resource.Kind == resourceService && item.ServiceTaxonomyID > 0 {
		return item.ServiceTaxonomyID
	}
	return item.ReferenceID
}

func validClassificationSource(meta classificationMeta, source string) bool {
	switch source {
	case "manual", "import", "rule", "ai_extracted", "catalog_mapping":
		return true
	case "dependency_graph":
		return meta.Resource.Kind == resourceService
	default:
		return false
	}
}

func validServiceRole(value string) bool {
	switch value {
	case "operated", "dependent":
		return true
	default:
		return false
	}
}

func validSeverity(value string) bool {
	switch value {
	case "none", "degraded", "unavailable":
		return true
	default:
		return false
	}
}

func validExpectedAudience(value string) bool {
	switch strings.TrimSpace(value) {
	case "internal", "external", "both":
		return true
	default:
		return false
	}
}

func cleanExpectedAudiencePtr(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func (h *Handler) classificationMutationError(w http.ResponseWriter, r *http.Request, operation string, err error, maintenanceID int64) {
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "maintenance_not_found")
		return
	}
	if errors.Is(err, errBadRequest) {
		appError(w, http.StatusBadRequest, "invalid_classification")
		return
	}
	h.dbFailure(w, r, operation, err, "maintenance_id", maintenanceID)
}

func (h *Handler) handleCreateTarget(w http.ResponseWriter, r *http.Request) {
	h.handleTargetMutation(w, r, 0)
}

func (h *Handler) handleUpdateTarget(w http.ResponseWriter, r *http.Request) {
	targetID, err := pathInt64(r, "targetId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_target_id")
		return
	}
	h.handleTargetMutation(w, r, targetID)
}

func (h *Handler) handleTargetMutation(w http.ResponseWriter, r *http.Request, targetID int64) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	var body targetRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	tx, err := h.maintenance.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "target_begin", err, "maintenance_id", maintenanceID)
		return
	}
	defer tx.Rollback()
	if err := h.upsertTargetTx(r, tx, maintenanceID, targetID, body); errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "target_not_found")
		return
	} else if errors.Is(err, errBadRequest) {
		appError(w, http.StatusBadRequest, "invalid_target")
		return
	} else if err != nil {
		h.dbFailure(w, r, "target_save", err, "maintenance_id", maintenanceID, "target_id", targetID)
		return
	}
	if err := writeEvent(r.Context(), tx, maintenanceID, nil, "updated", "Target aggiornati", claimsActor(r), map[string]any{"action": "target_saved"}); err != nil {
		h.dbFailure(w, r, "target_event", err, "maintenance_id", maintenanceID)
		return
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "target_commit", err, "maintenance_id", maintenanceID)
		return
	}
	status := http.StatusOK
	if targetID == 0 {
		status = http.StatusCreated
	}
	respondMutationDetail(h, w, r, maintenanceID, status)
}

func (h *Handler) upsertTargetTx(r *http.Request, tx *sql.Tx, maintenanceID int64, targetID int64, body targetRequest) error {
	if body.TargetTypeID <= 0 || strings.TrimSpace(body.DisplayName) == "" {
		return errBadRequest
	}
	source := normalizeSource(body.Source)
	if !validTargetSource(source) {
		return errBadRequest
	}
	if body.ServiceTaxonomyID != nil && *body.ServiceTaxonomyID <= 0 {
		return errBadRequest
	}
	if body.Confidence != nil && (*body.Confidence < 0 || *body.Confidence > 1) {
		return errBadRequest
	}
	if err := ensureMaintenanceExists(r.Context(), tx, maintenanceID); err != nil {
		return err
	}
	if body.IsPrimary {
		query := `UPDATE maintenance.maintenance_target SET is_primary = false WHERE maintenance_id = $1`
		args := []any{maintenanceID}
		if targetID > 0 {
			query += ` AND maintenance_target_id <> $2`
			args = append(args, targetID)
		}
		if _, err := tx.ExecContext(r.Context(), query, args...); err != nil {
			return err
		}
	}
	if targetID == 0 {
		_, err := tx.ExecContext(
			r.Context(),
			`INSERT INTO maintenance.maintenance_target (
				maintenance_id, target_type_id, service_taxonomy_id, ref_table, ref_id, external_key,
				display_name, source, confidence, is_primary, metadata
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)`,
			maintenanceID,
			body.TargetTypeID,
			body.ServiceTaxonomyID,
			nullStringPtr(body.ReferenceTable),
			body.ReferenceID,
			nullStringPtr(body.ExternalKey),
			strings.TrimSpace(body.DisplayName),
			source,
			body.Confidence,
			body.IsPrimary,
			rawJSONOrDefault(body.Metadata),
		)
		return err
	}
	res, err := tx.ExecContext(
		r.Context(),
		`UPDATE maintenance.maintenance_target SET
			target_type_id = $1,
			service_taxonomy_id = $2,
			ref_table = $3,
			ref_id = $4,
			external_key = $5,
			display_name = $6,
			source = $7,
			confidence = $8,
			is_primary = $9,
			metadata = $10::jsonb
		WHERE maintenance_id = $11 AND maintenance_target_id = $12`,
		body.TargetTypeID,
		body.ServiceTaxonomyID,
		nullStringPtr(body.ReferenceTable),
		body.ReferenceID,
		nullStringPtr(body.ExternalKey),
		strings.TrimSpace(body.DisplayName),
		source,
		body.Confidence,
		body.IsPrimary,
		rawJSONOrDefault(body.Metadata),
		maintenanceID,
		targetID,
	)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func validTargetSource(source string) bool {
	switch source {
	case "manual", "import", "rule", "ai_extracted", "catalog_mapping":
		return true
	default:
		return false
	}
}

func (h *Handler) handleDeleteTarget(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	targetID, err := pathInt64(r, "targetId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_target_id")
		return
	}
	res, err := h.maintenance.ExecContext(r.Context(), `DELETE FROM maintenance.maintenance_target WHERE maintenance_id = $1 AND maintenance_target_id = $2`, maintenanceID, targetID)
	if err != nil {
		h.dbFailure(w, r, "target_delete", err, "maintenance_id", maintenanceID, "target_id", targetID)
		return
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		appError(w, http.StatusNotFound, "target_not_found")
		return
	}
	if err := writeEvent(r.Context(), h.maintenance, maintenanceID, nil, "updated", "Target rimosso", claimsActor(r), map[string]any{"action": "target_removed"}); err != nil {
		h.dbFailure(w, r, "target_delete_event", err, "maintenance_id", maintenanceID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusOK)
}

func (h *Handler) handleCreateImpactedCustomer(w http.ResponseWriter, r *http.Request) {
	h.handleImpactedCustomerMutation(w, r, 0)
}

func (h *Handler) handleUpdateImpactedCustomer(w http.ResponseWriter, r *http.Request) {
	customerImpactID, err := pathInt64(r, "customerImpactId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_customer_impact_id")
		return
	}
	h.handleImpactedCustomerMutation(w, r, customerImpactID)
}

func (h *Handler) handleImpactedCustomerMutation(w http.ResponseWriter, r *http.Request, customerImpactID int64) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	var body impactedCustomerRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if body.CustomerID <= 0 || !validImpactScope(body.ImpactScope) || !validDerivationSource(body.DerivationSource) {
		appError(w, http.StatusBadRequest, "invalid_customer_impact")
		return
	}
	if body.Confidence != nil && (*body.Confidence < 0 || *body.Confidence > 1) {
		appError(w, http.StatusBadRequest, "invalid_confidence")
		return
	}
	var res sql.Result
	if customerImpactID == 0 {
		res, err = h.maintenance.ExecContext(
			r.Context(),
			`INSERT INTO maintenance.maintenance_impacted_customer (
				maintenance_id, customer_id, order_id, service_id, impact_scope,
				derivation_source, confidence, reason, metadata
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)`,
			maintenanceID,
			body.CustomerID,
			body.OrderID,
			body.ServiceID,
			body.ImpactScope,
			body.DerivationSource,
			body.Confidence,
			nullStringPtr(body.Reason),
			rawJSONOrDefault(body.Metadata),
		)
	} else {
		res, err = h.maintenance.ExecContext(
			r.Context(),
			`UPDATE maintenance.maintenance_impacted_customer SET
				customer_id = $1,
				order_id = $2,
				service_id = $3,
				impact_scope = $4,
				derivation_source = $5,
				confidence = $6,
				reason = $7,
				metadata = $8::jsonb
			WHERE maintenance_id = $9 AND maintenance_impacted_customer_id = $10`,
			body.CustomerID,
			body.OrderID,
			body.ServiceID,
			body.ImpactScope,
			body.DerivationSource,
			body.Confidence,
			nullStringPtr(body.Reason),
			rawJSONOrDefault(body.Metadata),
			maintenanceID,
			customerImpactID,
		)
	}
	if err != nil {
		h.dbFailure(w, r, "customer_impact_save", err, "maintenance_id", maintenanceID, "customer_impact_id", customerImpactID)
		return
	}
	if customerImpactID > 0 {
		if affected, _ := res.RowsAffected(); affected == 0 {
			appError(w, http.StatusNotFound, "customer_impact_not_found")
			return
		}
	}
	if err := writeEvent(r.Context(), h.maintenance, maintenanceID, nil, "impact_recomputed", "Clienti impattati aggiornati", claimsActor(r), map[string]any{"action": "customer_impact_saved"}); err != nil {
		h.dbFailure(w, r, "customer_impact_event", err, "maintenance_id", maintenanceID)
		return
	}
	status := http.StatusOK
	if customerImpactID == 0 {
		status = http.StatusCreated
	}
	respondMutationDetail(h, w, r, maintenanceID, status)
}

func (h *Handler) handleDeleteImpactedCustomer(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	maintenanceID, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	customerImpactID, err := pathInt64(r, "customerImpactId")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_customer_impact_id")
		return
	}
	res, err := h.maintenance.ExecContext(r.Context(), `DELETE FROM maintenance.maintenance_impacted_customer WHERE maintenance_id = $1 AND maintenance_impacted_customer_id = $2`, maintenanceID, customerImpactID)
	if err != nil {
		h.dbFailure(w, r, "customer_impact_delete", err, "maintenance_id", maintenanceID, "customer_impact_id", customerImpactID)
		return
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		appError(w, http.StatusNotFound, "customer_impact_not_found")
		return
	}
	if err := writeEvent(r.Context(), h.maintenance, maintenanceID, nil, "impact_recomputed", "Cliente impattato rimosso", claimsActor(r), map[string]any{"action": "customer_impact_removed"}); err != nil {
		h.dbFailure(w, r, "customer_impact_delete_event", err, "maintenance_id", maintenanceID)
		return
	}
	respondMutationDetail(h, w, r, maintenanceID, http.StatusOK)
}

func validImpactScope(value string) bool {
	switch value {
	case "direct", "indirect", "possible":
		return true
	default:
		return false
	}
}

func validDerivationSource(value string) bool {
	switch value {
	case "manual", "rule", "ai", "hybrid":
		return true
	default:
		return false
	}
}
