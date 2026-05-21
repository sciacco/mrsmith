package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *SQLStore) CreateAward(ctx context.Context, principal Principal, input AwardInput) (ActionResponse, error) {
	if !principal.IsPeopleAdmin && strings.TrimSpace(input.EmployeeID) != "" {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		employeeID := strings.TrimSpace(input.EmployeeID)
		if employeeID == "" {
			var err error
			employeeID, err = s.employeeIDByEmail(ctx, tx, principal.Email)
			if err != nil {
				return err
			}
		}
		if strings.TrimSpace(input.CertificationID) == "" || strings.TrimSpace(input.Outcome) == "" || strings.TrimSpace(input.AwardedOn) == "" {
			return validationError("missing_required_fields", "certificazione, esito e data sono obbligatori")
		}
		if strings.TrimSpace(input.EnrollmentID) != "" {
			enrollment, err := s.enrollmentGuard(ctx, tx, principal, input.EnrollmentID, false)
			if err != nil {
				return err
			}
			if enrollment.EmployeeID != employeeID {
				return validationError("enrollment_employee_mismatch", "iscrizione e certificazione devono riferirsi alla stessa persona")
			}
		}
		actor := actorForPrincipal(principal)
		result := AttemptAwardTransition(AwardInProgress, AwardIssue, TransitionContext{Actor: actor, Reason: input.Reason})
		if !result.OK {
			return conflictError(result.Code, result.Message)
		}
		source := strings.TrimSpace(input.ValidationSource)
		if source == "" {
			source = "document_verified"
		}
		const stmt = `
INSERT INTO training.certification_award (
  employee_id,
  certification_id,
  enrollment_id,
  outcome,
  awarded_on,
  expires_on,
  validation_source,
  external_credential_id,
  external_credential_url,
  notes
) VALUES (
  $1::uuid,
  $2::uuid,
  $3::uuid,
  $4::training.award_outcome,
  $5::date,
  NULLIF($6, '')::date,
  $7::training.validation_source,
  NULLIF($8, ''),
  NULLIF($9, ''),
  NULLIF($10, '')
) RETURNING id::text, outcome::text`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			employeeID,
			input.CertificationID,
			nullableUUID(input.EnrollmentID),
			input.Outcome,
			input.AwardedOn,
			strings.TrimSpace(input.ExpiresOn),
			source,
			input.ExternalCredentialID,
			input.ExternalCredentialURL,
			input.Notes,
		).Scan(&response.ID, &response.Status); err != nil {
			return fmt.Errorf("create training award: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "certification_award", response.ID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "certification_award", response.ID, "create", nil, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) awardGuard(ctx context.Context, q sqlRunner, principal Principal, id string, lock bool) (awardGuard, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF ca"
	}
	query := `
SELECT
  ca.id::text,
  e.id::text,
  e.email::text,
  ca.outcome::text
FROM training.certification_award ca
JOIN training.employee e ON e.id = ca.employee_id
WHERE ca.id = $1::uuid` + lockClause
	var guard awardGuard
	err := q.QueryRowContext(ctx, query, id).Scan(&guard.ID, &guard.EmployeeID, &guard.EmployeeEmail, &guard.Outcome)
	if errors.Is(err, sql.ErrNoRows) {
		return awardGuard{}, notFoundError("award_not_found", "certificazione non trovata")
	}
	if err != nil {
		return awardGuard{}, fmt.Errorf("load training award guard: %w", err)
	}
	if !principalCanAccessEmployee(principal, guard.EmployeeEmail) {
		return awardGuard{}, forbiddenError("not_owner", "certificazione non accessibile")
	}
	return guard, nil
}

func (s *SQLStore) TransitionAward(ctx context.Context, principal Principal, id string, input AwardTransitionInput) (ActionResponse, error) {
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		guard, err := s.awardGuard(ctx, tx, principal, id, true)
		if err != nil {
			return err
		}
		before, err := entitySnapshot(ctx, tx, "certification_award", id)
		if err != nil {
			return err
		}
		actor := actorForPrincipal(principal)
		transition := AwardTransition(strings.TrimSpace(input.Transition))
		result := AttemptAwardTransition(AwardOutcome(guard.Outcome), transition, TransitionContext{Actor: actor, Reason: input.Reason})
		if !result.OK {
			return conflictError(result.Code, result.Message)
		}
		outcome := result.Target
		if transition == AwardCorrect && strings.TrimSpace(input.Outcome) != "" {
			outcome = strings.TrimSpace(input.Outcome)
		}
		if outcome == "" {
			return validationError("invalid_outcome", "esito non valido")
		}
		source := strings.TrimSpace(input.ValidationSource)
		if source == "" {
			source = "document_verified"
		}
		const stmt = `
UPDATE training.certification_award
SET outcome = $2::training.award_outcome,
    awarded_on = COALESCE(NULLIF($3, '')::date, awarded_on),
    expires_on = NULLIF($4, '')::date,
    validation_source = $5::training.validation_source,
    notes = COALESCE(NULLIF($6, ''), notes)
WHERE id = $1::uuid
RETURNING id::text, outcome::text`
		if err := tx.QueryRowContext(ctx, stmt, id, outcome, input.AwardedOn, input.ExpiresOn, source, input.Reason).Scan(&response.ID, &response.Status); err != nil {
			return fmt.Errorf("transition training award: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "certification_award", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "certification_award", id, "transition:"+input.Transition, before, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}

func (s *SQLStore) InsertDocument(ctx context.Context, principal Principal, enrollmentID string, awardID string, filename string, mime string, stored StoredObject) (DocumentMetadata, error) {
	var result DocumentMetadata
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		var uploadedBy any
		if principal.IsPeopleAdmin {
			uploadedBy = nullableUUIDPtr(s.actorEmployeeID(ctx, tx, principal))
		} else {
			employeeID, err := s.employeeIDByEmail(ctx, tx, principal.Email)
			if err != nil {
				return err
			}
			uploadedBy = employeeID
		}
		if strings.TrimSpace(enrollmentID) != "" {
			if _, err := s.enrollmentGuard(ctx, tx, principal, enrollmentID, false); err != nil {
				return err
			}
		}
		if strings.TrimSpace(awardID) != "" {
			if _, err := s.awardGuard(ctx, tx, principal, awardID, false); err != nil {
				return err
			}
		}
		const stmt = `
INSERT INTO training.document (
  enrollment_id,
  certification_award_id,
  filename,
  storage_key,
  sha256,
  mime,
  size_bytes,
  uploaded_by
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::uuid)
RETURNING id::text, COALESCE(enrollment_id::text, ''), COALESCE(certification_award_id::text, ''), filename, sha256, mime, size_bytes, uploaded_at::text, is_validated`
		if err := tx.QueryRowContext(
			ctx,
			stmt,
			nullableUUID(enrollmentID),
			nullableUUID(awardID),
			filename,
			stored.Key,
			stored.SHA256,
			mime,
			stored.SizeBytes,
			uploadedBy,
		).Scan(
			&result.ID,
			&result.EnrollmentID,
			&result.CertificationAwardID,
			&result.Filename,
			&result.SHA256,
			&result.MIME,
			&result.SizeBytes,
			&result.UploadedAt,
			&result.Validated,
		); err != nil {
			return fmt.Errorf("insert training document: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "document", result.ID)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "document", result.ID, "create", nil, after)
	})
	return result, err
}

func (s *SQLStore) DocumentForAccess(ctx context.Context, principal Principal, id string) (documentGuard, error) {
	const q = `
SELECT
  d.id::text,
  COALESCE(d.enrollment_id::text, ''),
  COALESCE(d.certification_award_id::text, ''),
  d.filename,
  d.storage_key,
  d.sha256,
  d.mime,
  d.size_bytes,
  d.uploaded_at::text,
  d.is_validated,
  e.email::text
FROM training.document d
LEFT JOIN training.enrollment en ON en.id = d.enrollment_id
LEFT JOIN training.certification_award ca ON ca.id = d.certification_award_id
JOIN training.employee e ON e.id = COALESCE(en.employee_id, ca.employee_id)
WHERE d.id = $1::uuid`
	var doc documentGuard
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&doc.ID,
		&doc.EnrollmentID,
		&doc.CertificationAwardID,
		&doc.Filename,
		&doc.StorageKey,
		&doc.SHA256,
		&doc.MIME,
		&doc.SizeBytes,
		&doc.UploadedAt,
		&doc.Validated,
		&doc.EmployeeEmail,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return documentGuard{}, notFoundError("document_not_found", "documento non trovato")
	}
	if err != nil {
		return documentGuard{}, fmt.Errorf("load training document: %w", err)
	}
	if !principalCanAccessEmployee(principal, doc.EmployeeEmail) {
		return documentGuard{}, forbiddenError("not_owner", "documento non accessibile")
	}
	return doc, nil
}

func (s *SQLStore) ValidateDocument(ctx context.Context, principal Principal, id string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	var response ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "document", id)
		if err != nil {
			return err
		}
		validator := s.actorEmployeeID(ctx, tx, principal)
		const stmt = `
UPDATE training.document
SET is_validated = true,
    validated_by = $2::uuid,
    validated_at = now()
WHERE id = $1::uuid
RETURNING id::text`
		if err := tx.QueryRowContext(ctx, stmt, id, nullableUUIDPtr(validator)).Scan(&response.ID); err != nil {
			return fmt.Errorf("validate training document: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "document", id)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "document", id, "validate", before, after); err != nil {
			return err
		}
		response.OK = true
		return nil
	})
	return response, err
}
