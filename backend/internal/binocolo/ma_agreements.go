package binocolo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *SQLStore) ListMACompanyAgreements(ctx context.Context, companyKey string) ([]MACompanyAgreement, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, company_key, kind, signed_on, expires_on, created_at, COALESCE(created_by_email, ''),
       updated_at, COALESCE(updated_by_email, '')
FROM binocolo.ma_company_agreement
WHERE company_key = $1 AND deleted_at IS NULL
ORDER BY signed_on DESC, created_at DESC`, companyKey)
	if err != nil {
		return nil, fmt.Errorf("list ma company agreements: %w", err)
	}
	defer rows.Close()
	agreements := make([]MACompanyAgreement, 0)
	for rows.Next() {
		item, err := scanMACompanyAgreement(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ma company agreement: %w", err)
		}
		agreements = append(agreements, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma company agreements: %w", err)
	}
	return agreements, nil
}

func (s *SQLStore) CreateMACompanyAgreement(ctx context.Context, companyKey string, input MACompanyAgreementWrite, subject, email string) (MACompanyAgreement, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MACompanyAgreement{}, fmt.Errorf("begin create ma company agreement: %w", err)
	}
	defer tx.Rollback()
	if err := lockMAContactCompany(ctx, tx, companyKey); err != nil {
		return MACompanyAgreement{}, err
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_company_agreement (company_key, kind, signed_on, expires_on, created_by_subject, created_by_email)
VALUES ($1, $2, $3::date, NULLIF($4, '')::date, $5, $6)
RETURNING id::text, company_key, kind, signed_on, expires_on, created_at, COALESCE(created_by_email, ''), updated_at, COALESCE(updated_by_email, '')`,
		companyKey, input.Kind, input.SignedOn, input.ExpiresOn, nullString(subject), nullString(email))
	item, err := scanMACompanyAgreement(row)
	if err != nil {
		return MACompanyAgreement{}, fmt.Errorf("insert ma company agreement: %w", translateMACompanyConstraintError(err, false))
	}
	if err := tx.Commit(); err != nil {
		return MACompanyAgreement{}, fmt.Errorf("commit create ma company agreement: %w", err)
	}
	return item, nil
}

func (s *SQLStore) UpdateMACompanyAgreement(ctx context.Context, companyKey, agreementID string, input MACompanyAgreementWrite, subject, email string) (updated MACompanyAgreement, previous MACompanyAgreement, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MACompanyAgreement{}, MACompanyAgreement{}, fmt.Errorf("begin update ma company agreement: %w", err)
	}
	defer tx.Rollback()
	if err := lockMAContactCompany(ctx, tx, companyKey); err != nil {
		return MACompanyAgreement{}, MACompanyAgreement{}, err
	}
	previousRow := tx.QueryRowContext(ctx, `
SELECT id::text, company_key, kind, signed_on, expires_on, created_at, COALESCE(created_by_email, ''), updated_at, COALESCE(updated_by_email, '')
FROM binocolo.ma_company_agreement
WHERE company_key = $1 AND id = $2::uuid AND deleted_at IS NULL
FOR UPDATE`, companyKey, agreementID)
	previous, err = scanMACompanyAgreement(previousRow)
	if errors.Is(err, sql.ErrNoRows) {
		return MACompanyAgreement{}, MACompanyAgreement{}, errMACompanyAgreementNotFound
	}
	if err != nil {
		return MACompanyAgreement{}, MACompanyAgreement{}, fmt.Errorf("select ma company agreement: %w", err)
	}
	updatedRow := tx.QueryRowContext(ctx, `
UPDATE binocolo.ma_company_agreement
SET kind = $3, signed_on = $4::date, expires_on = NULLIF($5, '')::date,
    updated_at = now(), updated_by_subject = $6, updated_by_email = $7
WHERE company_key = $1 AND id = $2::uuid AND deleted_at IS NULL
RETURNING id::text, company_key, kind, signed_on, expires_on, created_at, COALESCE(created_by_email, ''), updated_at, COALESCE(updated_by_email, '')`,
		companyKey, agreementID, input.Kind, input.SignedOn, input.ExpiresOn, nullString(subject), nullString(email))
	updated, err = scanMACompanyAgreement(updatedRow)
	if errors.Is(err, sql.ErrNoRows) {
		return MACompanyAgreement{}, MACompanyAgreement{}, errMACompanyAgreementNotFound
	}
	if err != nil {
		return MACompanyAgreement{}, MACompanyAgreement{}, fmt.Errorf("update ma company agreement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MACompanyAgreement{}, MACompanyAgreement{}, fmt.Errorf("commit update ma company agreement: %w", err)
	}
	return updated, previous, nil
}

func (s *SQLStore) SoftDeleteMACompanyAgreement(ctx context.Context, companyKey, agreementID, subject, email string) (MACompanyAgreement, error) {
	row := s.db.QueryRowContext(ctx, `
UPDATE binocolo.ma_company_agreement
SET deleted_at = now(), deleted_by_subject = $3, deleted_by_email = $4
WHERE company_key = $1 AND id = $2::uuid AND deleted_at IS NULL
RETURNING id::text, company_key, kind, signed_on, expires_on, created_at, COALESCE(created_by_email, ''), updated_at, COALESCE(updated_by_email, '')`,
		companyKey, agreementID, nullString(subject), nullString(email))
	item, err := scanMACompanyAgreement(row)
	if errors.Is(err, sql.ErrNoRows) {
		return MACompanyAgreement{}, errMACompanyAgreementNotFound
	}
	if err != nil {
		return MACompanyAgreement{}, fmt.Errorf("soft delete ma company agreement: %w", err)
	}
	return item, nil
}

// maAgreementRowScanner abstracts *sql.Row and *sql.Rows so create/update/list
// share one scan implementation.
type maAgreementRowScanner interface {
	Scan(dest ...any) error
}

func scanMACompanyAgreement(row maAgreementRowScanner) (MACompanyAgreement, error) {
	var item MACompanyAgreement
	var signedOn time.Time
	var expiresOn sql.NullTime
	if err := row.Scan(&item.ID, &item.CompanyKey, &item.Kind, &signedOn, &expiresOn, &item.CreatedAt, &item.CreatedByEmail, &item.UpdatedAt, &item.UpdatedByEmail); err != nil {
		return MACompanyAgreement{}, err
	}
	item.SignedOn = maDateValue(signedOn)
	if expiresOn.Valid {
		item.ExpiresOn = maDateValue(expiresOn.Time)
	}
	return item, nil
}
