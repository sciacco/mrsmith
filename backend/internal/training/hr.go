package training

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type HRProvider interface {
	ListEmployees(ctx context.Context) ([]HREmployee, error)
	GetEmployee(ctx context.Context, source string, externalID string) (*HREmployee, error)
	HandleWebhook(ctx context.Context, payload []byte) ([]HREmployee, error)
}

type HREmployee struct {
	ExternalID      string
	ExternalSource  string
	FirstName       string
	LastName        string
	Email           string
	HireDate        string
	TerminationDate string
	Status          string
	Notes           string
}

type HRSyncResult struct {
	OK      bool `json:"ok"`
	Created int  `json:"created"`
	Updated int  `json:"updated"`
	Skipped int  `json:"skipped"`
}

func (s *SQLStore) SyncEmployeesFromProvider(ctx context.Context, principal Principal, provider HRProvider) (HRSyncResult, error) {
	if !principal.IsPeopleAdmin {
		return HRSyncResult{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if provider == nil {
		return HRSyncResult{}, serviceUnavailableError("hr_provider_not_configured", "sorgente HR non configurata")
	}
	employees, err := provider.ListEmployees(ctx)
	if err != nil {
		return HRSyncResult{}, fmt.Errorf("list HR employees: %w", err)
	}

	result := HRSyncResult{OK: true}
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		for _, employee := range employees {
			normalized, ok := normalizeHREmployee(employee)
			if !ok {
				result.Skipped++
				continue
			}
			created, err := s.upsertHREmployee(ctx, tx, normalized)
			if err != nil {
				return err
			}
			if created {
				result.Created++
			} else {
				result.Updated++
			}
		}
		return nil
	})
	return result, err
}

func normalizeHREmployee(employee HREmployee) (HREmployee, bool) {
	employee.ExternalID = strings.TrimSpace(employee.ExternalID)
	employee.ExternalSource = strings.TrimSpace(employee.ExternalSource)
	if employee.ExternalSource == "" {
		employee.ExternalSource = "factorial"
	}
	employee.FirstName = strings.TrimSpace(employee.FirstName)
	employee.LastName = strings.TrimSpace(employee.LastName)
	employee.Email = normalizeEmail(employee.Email)
	employee.Status = strings.TrimSpace(employee.Status)
	if employee.Status == "" {
		employee.Status = "active"
	}
	employee.HireDate = strings.TrimSpace(employee.HireDate)
	employee.TerminationDate = strings.TrimSpace(employee.TerminationDate)
	employee.Notes = strings.TrimSpace(employee.Notes)
	return employee, employee.ExternalID != "" && employee.FirstName != "" && employee.LastName != "" && employee.Email != ""
}

func (s *SQLStore) upsertHREmployee(ctx context.Context, tx *sql.Tx, employee HREmployee) (bool, error) {
	const stmt = `
WITH updated AS (
  UPDATE training.employee
  SET first_name = $3,
      last_name = $4,
      email = $5,
      hire_date = NULLIF($6, '')::date,
      termination_date = NULLIF($7, '')::date,
      status = $8::training.employee_status,
      notes = NULLIF($9, '')
  WHERE external_source = $1::training.hr_source
    AND external_id = $2
  RETURNING false AS inserted
), inserted AS (
  INSERT INTO training.employee (
    external_source,
    external_id,
    first_name,
    last_name,
    email,
    hire_date,
    termination_date,
    status,
    notes
  )
  SELECT
    $1::training.hr_source,
    $2,
    $3,
    $4,
    $5,
    NULLIF($6, '')::date,
    NULLIF($7, '')::date,
    $8::training.employee_status,
    NULLIF($9, '')
  WHERE NOT EXISTS (SELECT 1 FROM updated)
  RETURNING true AS inserted
)
SELECT inserted FROM inserted
UNION ALL
SELECT inserted FROM updated
LIMIT 1`
	var inserted bool
	if err := tx.QueryRowContext(
		ctx,
		stmt,
		employee.ExternalSource,
		employee.ExternalID,
		employee.FirstName,
		employee.LastName,
		employee.Email,
		employee.HireDate,
		employee.TerminationDate,
		employee.Status,
		employee.Notes,
	).Scan(&inserted); err != nil {
		return false, fmt.Errorf("upsert HR employee: %w", err)
	}
	return inserted, nil
}
