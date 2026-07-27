package binocolo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *SQLStore) ListMACompanyContacts(ctx context.Context, companyKey string) ([]MACompanyContact, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, company_key, name, COALESCE(relationship, ''), COALESCE(contact_details, ''),
       COALESCE(note, ''), is_primary, created_at, updated_at
FROM binocolo.ma_company_contact
WHERE company_key = $1 AND deleted_at IS NULL
ORDER BY is_primary DESC, lower(name), created_at`, companyKey)
	if err != nil {
		return nil, fmt.Errorf("list ma company contacts: %w", err)
	}
	defer rows.Close()
	contacts := make([]MACompanyContact, 0)
	for rows.Next() {
		var item MACompanyContact
		if err := rows.Scan(&item.ID, &item.CompanyKey, &item.Name, &item.Relationship, &item.ContactDetails, &item.Note, &item.IsPrimary, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ma company contact: %w", err)
		}
		contacts = append(contacts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma company contacts: %w", err)
	}
	return contacts, nil
}

func (s *SQLStore) CreateMACompanyContact(ctx context.Context, companyKey string, input MACompanyContactWrite, subject, email string) (MACompanyContact, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MACompanyContact{}, fmt.Errorf("begin create ma company contact: %w", err)
	}
	defer tx.Rollback()
	if err := lockMAContactCompany(ctx, tx, companyKey); err != nil {
		return MACompanyContact{}, err
	}
	if input.IsPrimary {
		if _, err := tx.ExecContext(ctx, `UPDATE binocolo.ma_company_contact SET is_primary = false, updated_at = now(), updated_by_subject = $2, updated_by_email = $3 WHERE company_key = $1 AND is_primary AND deleted_at IS NULL`, companyKey, nullString(subject), nullString(email)); err != nil {
			return MACompanyContact{}, fmt.Errorf("clear ma company primary contact: %w", err)
		}
	}
	var item MACompanyContact
	err = tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_company_contact (company_key, name, relationship, contact_details, note, is_primary, created_by_subject, created_by_email)
VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8)
RETURNING id::text, company_key, name, COALESCE(relationship, ''), COALESCE(contact_details, ''), COALESCE(note, ''), is_primary, created_at, updated_at`,
		companyKey, input.Name, input.Relationship, input.ContactDetails, input.Note, input.IsPrimary, nullString(subject), nullString(email)).Scan(
		&item.ID, &item.CompanyKey, &item.Name, &item.Relationship, &item.ContactDetails, &item.Note, &item.IsPrimary, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return MACompanyContact{}, fmt.Errorf("insert ma company contact: %w", translateMACompanyConstraintError(err, false))
	}
	if err := tx.Commit(); err != nil {
		return MACompanyContact{}, fmt.Errorf("commit create ma company contact: %w", err)
	}
	return item, nil
}

func (s *SQLStore) UpdateMACompanyContact(ctx context.Context, companyKey, contactID string, input MACompanyContactWrite, subject, email string) (MACompanyContact, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MACompanyContact{}, fmt.Errorf("begin update ma company contact: %w", err)
	}
	defer tx.Rollback()
	if err := lockMAContactCompany(ctx, tx, companyKey); err != nil {
		return MACompanyContact{}, err
	}
	if input.IsPrimary {
		if _, err := tx.ExecContext(ctx, `UPDATE binocolo.ma_company_contact SET is_primary = false, updated_at = now(), updated_by_subject = $3, updated_by_email = $4 WHERE company_key = $1 AND id <> $2::uuid AND is_primary AND deleted_at IS NULL`, companyKey, contactID, nullString(subject), nullString(email)); err != nil {
			return MACompanyContact{}, fmt.Errorf("clear ma company primary contact: %w", err)
		}
	}
	var item MACompanyContact
	err = tx.QueryRowContext(ctx, `
UPDATE binocolo.ma_company_contact
SET name = $3, relationship = NULLIF($4, ''), contact_details = NULLIF($5, ''), note = NULLIF($6, ''), is_primary = $7,
    updated_at = now(), updated_by_subject = $8, updated_by_email = $9
WHERE company_key = $1 AND id = $2::uuid AND deleted_at IS NULL
RETURNING id::text, company_key, name, COALESCE(relationship, ''), COALESCE(contact_details, ''), COALESCE(note, ''), is_primary, created_at, updated_at`,
		companyKey, contactID, input.Name, input.Relationship, input.ContactDetails, input.Note, input.IsPrimary, nullString(subject), nullString(email)).Scan(
		&item.ID, &item.CompanyKey, &item.Name, &item.Relationship, &item.ContactDetails, &item.Note, &item.IsPrimary, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return MACompanyContact{}, errMACompanyContactNotFound
	}
	if err != nil {
		return MACompanyContact{}, fmt.Errorf("update ma company contact: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MACompanyContact{}, fmt.Errorf("commit update ma company contact: %w", err)
	}
	return item, nil
}

func (s *SQLStore) SoftDeleteMACompanyContact(ctx context.Context, companyKey, contactID, subject, email string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_company_contact SET deleted_at = now(), deleted_by_subject = $3, deleted_by_email = $4
WHERE company_key = $1 AND id = $2::uuid AND deleted_at IS NULL`, companyKey, contactID, nullString(subject), nullString(email))
	if err != nil {
		return fmt.Errorf("soft delete ma company contact: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("soft delete ma company contact rows: %w", err)
	}
	if count == 0 {
		return errMACompanyContactNotFound
	}
	return nil
}

func lockMAContactCompany(ctx context.Context, tx *sql.Tx, companyKey string) error {
	var key string
	if err := tx.QueryRowContext(ctx, `SELECT company_key FROM binocolo.ma_company WHERE company_key = $1 FOR UPDATE`, companyKey).Scan(&key); errors.Is(err, sql.ErrNoRows) {
		return errMACompanyKeyUnknown
	} else if err != nil {
		return fmt.Errorf("lock ma contact company: %w", err)
	}
	return nil
}
