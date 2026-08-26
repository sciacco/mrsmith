package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

func (s *SQLStore) CreateEventExpense(ctx context.Context, principal Principal, eventID string, poID int64, enrollmentIDs []string) (eventExpenseLocal, error) {
	if !principal.IsPeopleAdmin {
		return eventExpenseLocal{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	var err error
	eventID, err = normalizeExpenseUUID(eventID, "id evento obbligatorio")
	if err != nil {
		return eventExpenseLocal{}, err
	}
	if poID <= 0 {
		return eventExpenseLocal{}, validationError("invalid_po_reference", "ID PO non valido")
	}
	ids, err := normalizeExpenseEnrollmentIDs(enrollmentIDs)
	if err != nil {
		return eventExpenseLocal{}, err
	}

	var expense eventExpenseLocal
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if err := ensureEventExists(ctx, tx, eventID); err != nil {
			return err
		}
		actorID := s.actorEmployeeID(ctx, tx, principal)
		if err := tx.QueryRowContext(ctx, `
INSERT INTO training.event_expense (event_id, rda_id, created_by)
VALUES ($1::uuid, $2, $3::uuid)
RETURNING id::text, event_id::text, rda_id, created_at::text, updated_at::text`, eventID, poID, nullableUUIDPtr(actorID)).Scan(
			&expense.ID, &expense.EventID, &expense.POID, &expense.CreatedAt, &expense.UpdatedAt,
		); err != nil {
			if isUniqueViolation(err, "uq_event_expense_event_rda") {
				return conflictError("event_expense_po_duplicate", "il PO e gia collegato a questo evento")
			}
			return fmt.Errorf("create training event expense: %w", err)
		}
		if err := s.replaceExpenseEnrollments(ctx, tx, expense.ID, expense.EventID, ids); err != nil {
			return err
		}
		expense.EnrollmentIDs = ids
		after, err := s.eventExpenseSnapshot(ctx, tx, expense.ID)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "event_expense", expense.ID, "create", nil, after)
	})
	return expense, err
}

func (s *SQLStore) ReplaceEventExpensePO(ctx context.Context, principal Principal, expenseID string, poID int64) (eventExpenseLocal, error) {
	if !principal.IsPeopleAdmin {
		return eventExpenseLocal{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	var err error
	expenseID, err = normalizeExpenseUUID(expenseID, "id voce obbligatorio")
	if err != nil {
		return eventExpenseLocal{}, err
	}
	if poID <= 0 {
		return eventExpenseLocal{}, validationError("invalid_po_reference", "ID PO non valido")
	}
	var expense eventExpenseLocal
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		var err error
		expense, err = s.lockEventExpenseForMutation(ctx, tx, expenseID)
		if err != nil {
			return err
		}
		before, err := s.eventExpenseSnapshot(ctx, tx, expenseID)
		if err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `
UPDATE training.event_expense
SET rda_id = $2, updated_at = now()
WHERE id = $1::uuid
RETURNING updated_at::text`, expenseID, poID).Scan(&expense.UpdatedAt); err != nil {
			if isUniqueViolation(err, "uq_event_expense_event_rda") {
				return conflictError("event_expense_po_duplicate", "il PO e gia collegato a questo evento")
			}
			return fmt.Errorf("replace training event expense PO: %w", err)
		}
		expense.POID = poID
		expense.EnrollmentIDs, err = s.expenseEnrollmentIDs(ctx, tx, expenseID)
		if err != nil {
			return err
		}
		after, err := s.eventExpenseSnapshot(ctx, tx, expenseID)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "event_expense", expenseID, "replace_po", before, after)
	})
	return expense, err
}

func (s *SQLStore) ReplaceEventExpenseEnrollments(ctx context.Context, principal Principal, expenseID string, enrollmentIDs []string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	var err error
	expenseID, err = normalizeExpenseUUID(expenseID, "id voce obbligatorio")
	if err != nil {
		return ActionResponse{}, err
	}
	ids, err := normalizeExpenseEnrollmentIDs(enrollmentIDs)
	if err != nil {
		return ActionResponse{}, err
	}
	response := ActionResponse{OK: true, ID: expenseID}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		expense, err := s.lockEventExpenseForMutation(ctx, tx, expenseID)
		if err != nil {
			return err
		}
		before, err := s.eventExpenseSnapshot(ctx, tx, expenseID)
		if err != nil {
			return err
		}
		if err := s.replaceExpenseEnrollments(ctx, tx, expenseID, expense.EventID, ids); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE training.event_expense SET updated_at = now() WHERE id = $1::uuid`, expenseID); err != nil {
			return fmt.Errorf("touch training event expense: %w", err)
		}
		after, err := s.eventExpenseSnapshot(ctx, tx, expenseID)
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, principal, "event_expense", expenseID, "replace_enrollments", before, after)
	})
	return response, err
}

func (s *SQLStore) DeleteEventExpense(ctx context.Context, principal Principal, expenseID string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	var err error
	expenseID, err = normalizeExpenseUUID(expenseID, "id voce obbligatorio")
	if err != nil {
		return ActionResponse{}, err
	}
	response := ActionResponse{OK: true, ID: expenseID}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := s.lockEventExpenseForMutation(ctx, tx, expenseID); err != nil {
			return err
		}
		before, err := s.eventExpenseSnapshot(ctx, tx, expenseID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM training.event_expense WHERE id = $1::uuid`, expenseID); err != nil {
			return fmt.Errorf("delete training event expense: %w", err)
		}
		return s.audit(ctx, tx, principal, "event_expense", expenseID, "delete", before, nil)
	})
	return response, err
}

func ensureEventExists(ctx context.Context, tx *sql.Tx, eventID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id::text FROM training.training_event WHERE id = $1::uuid FOR UPDATE`, eventID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFoundError("event_not_found", "evento non trovato")
	}
	if err != nil {
		return fmt.Errorf("load training event for expense: %w", err)
	}
	return nil
}

// lockEventExpenseForMutation takes an event's aggregate lock before the
// expense lock. It reloads the expense afterwards: a concurrent delete while
// waiting is reported as event_expense_not_found and no pre-lock event ID is
// used to mutate coverage.
func (s *SQLStore) lockEventExpenseForMutation(ctx context.Context, tx *sql.Tx, expenseID string) (eventExpenseLocal, error) {
	var eventID string
	err := tx.QueryRowContext(ctx, `
SELECT event_id::text
FROM training.event_expense
WHERE id = $1::uuid`, expenseID).Scan(&eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return eventExpenseLocal{}, notFoundError("event_expense_not_found", "voce di spesa non trovata")
	}
	if err != nil {
		return eventExpenseLocal{}, fmt.Errorf("load training event for expense mutation: %w", err)
	}
	if err := ensureEventExists(ctx, tx, eventID); err != nil {
		return eventExpenseLocal{}, err
	}

	expense, err := s.lockEventExpense(ctx, tx, expenseID)
	if err != nil {
		return eventExpenseLocal{}, err
	}
	if expense.EventID != eventID {
		return eventExpenseLocal{}, fmt.Errorf("event expense %s changed event while acquiring aggregate lock", expenseID)
	}
	return expense, nil
}

func (s *SQLStore) lockEventExpense(ctx context.Context, tx *sql.Tx, expenseID string) (eventExpenseLocal, error) {
	var expense eventExpenseLocal
	err := tx.QueryRowContext(ctx, `
SELECT id::text, event_id::text, rda_id, created_at::text, updated_at::text
FROM training.event_expense
WHERE id = $1::uuid
FOR UPDATE`, expenseID).Scan(&expense.ID, &expense.EventID, &expense.POID, &expense.CreatedAt, &expense.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return eventExpenseLocal{}, notFoundError("event_expense_not_found", "voce di spesa non trovata")
	}
	if err != nil {
		return eventExpenseLocal{}, fmt.Errorf("load training event expense: %w", err)
	}
	return expense, nil
}

func (s *SQLStore) replaceExpenseEnrollments(ctx context.Context, tx *sql.Tx, expenseID, eventID string, enrollmentIDs []string) error {
	for _, enrollmentID := range enrollmentIDs {
		var enrollmentEventID string
		err := tx.QueryRowContext(ctx, `
SELECT event_id::text
FROM training.enrollment
WHERE id = $1::uuid
FOR UPDATE`, enrollmentID).Scan(&enrollmentEventID)
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("enrollment_not_found", "iscrizione non trovata")
		}
		if err != nil {
			return fmt.Errorf("load training enrollment for expense: %w", err)
		}
		if enrollmentEventID != eventID {
			return conflictError("expense_enrollment_event_mismatch", "l'iscrizione non appartiene all'evento della voce")
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM training.event_expense_enrollment WHERE expense_id = $1::uuid`, expenseID); err != nil {
		return fmt.Errorf("delete training event expense enrollments: %w", err)
	}
	for _, enrollmentID := range enrollmentIDs {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO training.event_expense_enrollment (expense_id, enrollment_id)
VALUES ($1::uuid, $2::uuid)`, expenseID, enrollmentID); err != nil {
			return fmt.Errorf("insert training event expense enrollment: %w", err)
		}
	}
	return nil
}

func (s *SQLStore) expenseEnrollmentIDs(ctx context.Context, tx *sql.Tx, expenseID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT enrollment_id::text
FROM training.event_expense_enrollment
WHERE expense_id = $1::uuid
ORDER BY enrollment_id`, expenseID)
	if err != nil {
		return nil, fmt.Errorf("list training event expense enrollments: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan training event expense enrollment: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *SQLStore) eventExpenseSnapshot(ctx context.Context, tx *sql.Tx, expenseID string) (json.RawMessage, error) {
	expense, err := s.lockEventExpense(ctx, tx, expenseID)
	if err != nil {
		return nil, err
	}
	expense.EnrollmentIDs, err = s.expenseEnrollmentIDs(ctx, tx, expenseID)
	if err != nil {
		return nil, err
	}
	snapshot, err := json.Marshal(map[string]any{
		"id":            expense.ID,
		"eventId":       expense.EventID,
		"poId":          expense.POID,
		"enrollmentIds": expense.EnrollmentIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal training event expense audit: %w", err)
	}
	return snapshot, nil
}

func normalizeExpenseEnrollmentIDs(input []string) ([]string, error) {
	seen := make(map[string]struct{}, len(input))
	ids := make([]string, 0, len(input))
	for _, rawID := range input {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return nil, validationError("missing_id", "id iscrizione obbligatorio")
		}
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, validationError("invalid_enrollment_id", "id iscrizione non valido")
		}
		id = parsed.String()
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func normalizeExpenseUUID(rawID, missingMessage string) (string, error) {
	id := strings.TrimSpace(rawID)
	if id == "" {
		return "", validationError("missing_id", missingMessage)
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return "", validationError("invalid_id", "id non valido")
	}
	return parsed.String(), nil
}

// EventExpenses loads the local event/PO links and their explicit enrollment
// coverage in deterministic order. Arak data is intentionally not read or
// persisted here.
func (s *SQLStore) EventExpenses(ctx context.Context, eventID string) ([]eventExpenseLocal, error) {
	const query = `
SELECT
  ee.id::text,
  ee.event_id::text,
  ee.rda_id,
  ee.created_at::text,
  ee.updated_at::text,
  COALESCE(eee.enrollment_id::text, '')
FROM training.event_expense ee
LEFT JOIN training.event_expense_enrollment eee ON eee.expense_id = ee.id
WHERE ee.event_id = $1::uuid
ORDER BY ee.created_at, ee.id, eee.enrollment_id`
	rows, err := s.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("list training event expenses: %w", err)
	}
	defer rows.Close()

	result := make([]eventExpenseLocal, 0)
	byID := make(map[string]int)
	for rows.Next() {
		var (
			local        eventExpenseLocal
			enrollmentID string
		)
		if err := rows.Scan(&local.ID, &local.EventID, &local.POID, &local.CreatedAt, &local.UpdatedAt, &enrollmentID); err != nil {
			return nil, fmt.Errorf("scan training event expense: %w", err)
		}
		index, exists := byID[local.ID]
		if !exists {
			local.EnrollmentIDs = make([]string, 0)
			index = len(result)
			byID[local.ID] = index
			result = append(result, local)
		}
		if enrollmentID != "" {
			result[index].EnrollmentIDs = append(result[index].EnrollmentIDs, enrollmentID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
