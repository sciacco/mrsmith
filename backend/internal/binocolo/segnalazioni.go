package binocolo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Segnalazione is a quick, free-text report of a company or an opportunity
// entered by a Binocolo user and shared with everyone on the Segnalazioni
// Kanban. It has no link to ma_company or to initiatives: it lives on its own
// with a four-state lifecycle.
type Segnalazione struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Website        string    `json:"website"`
	Location       string    `json:"location"`
	FiscalID       string    `json:"fiscalId"`
	Contacts       string    `json:"contacts"`
	Notes          string    `json:"notes"`
	State          string    `json:"state"`
	CreatedAt      time.Time `json:"createdAt"`
	CreatedByEmail string    `json:"createdByEmail,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
	UpdatedByEmail string    `json:"updatedByEmail,omitempty"`
}

// SegnalazioneWrite carries the six free-text form fields. Every field is
// optional on its own; the service enforces the minimum-content rule.
type SegnalazioneWrite struct {
	Name     string `json:"name"`
	Website  string `json:"website"`
	Location string `json:"location"`
	FiscalID string `json:"fiscalId"`
	Contacts string `json:"contacts"`
	Notes    string `json:"notes"`
}

type SegnalazioneStateRequest struct {
	State string `json:"state"`
}

const (
	segnalazioneStateDaGestire  = "da_gestire"
	segnalazioneStateInGestione = "in_gestione"
	segnalazioneStateChiusa     = "chiusa"
	segnalazioneStateAnnullata  = "annullata"
)

var segnalazioneStates = map[string]struct{}{
	segnalazioneStateDaGestire:  {},
	segnalazioneStateInGestione: {},
	segnalazioneStateChiusa:     {},
	segnalazioneStateAnnullata:  {},
}

var (
	errSegnalazioneNotFound     = errors.New("segnalazione not found")
	errSegnalazioneChiusa       = errors.New("segnalazione chiusa: contents are read-only")
	errSegnalazioneVuota        = errors.New("segnalazione requires at least one of name, website, fiscalId, notes")
	errSegnalazioneStateInvalid = errors.New("invalid segnalazione state")
)

// segnalazioneStore is the persistence the segnalazioni service needs. Kept
// separate from maWorkspaceStore so the M&A fakes stay untouched; *SQLStore
// satisfies it.
type segnalazioneStore interface {
	ListSegnalazioni(ctx context.Context) ([]Segnalazione, error)
	GetSegnalazione(ctx context.Context, id string) (Segnalazione, error)
	CreateSegnalazione(ctx context.Context, input SegnalazioneWrite, subject, email string) (Segnalazione, error)
	UpdateSegnalazioneContent(ctx context.Context, id string, input SegnalazioneWrite, subject, email string) (Segnalazione, error)
	SetSegnalazioneState(ctx context.Context, id, state, subject, email string) (Segnalazione, error)
}

type segnalazioneService struct {
	store segnalazioneStore
}

func newSegnalazioneService(store segnalazioneStore) *segnalazioneService {
	return &segnalazioneService{store: store}
}

func (s *segnalazioneService) list(ctx context.Context) ([]Segnalazione, error) {
	if s == nil || s.store == nil {
		return nil, errMAStoreUnavailable
	}
	return s.store.ListSegnalazioni(ctx)
}

func (s *segnalazioneService) get(ctx context.Context, id string) (Segnalazione, error) {
	if s == nil || s.store == nil {
		return Segnalazione{}, errMAStoreUnavailable
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Segnalazione{}, errSegnalazioneNotFound
	}
	return s.store.GetSegnalazione(ctx, id)
}

func (s *segnalazioneService) create(ctx context.Context, input SegnalazioneWrite, subject, email string) (Segnalazione, error) {
	if s == nil || s.store == nil {
		return Segnalazione{}, errMAStoreUnavailable
	}
	clean, err := validateSegnalazioneWrite(input)
	if err != nil {
		return Segnalazione{}, err
	}
	return s.store.CreateSegnalazione(ctx, clean, subject, email)
}

func (s *segnalazioneService) updateContent(ctx context.Context, id string, input SegnalazioneWrite, subject, email string) (Segnalazione, error) {
	if s == nil || s.store == nil {
		return Segnalazione{}, errMAStoreUnavailable
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Segnalazione{}, errSegnalazioneNotFound
	}
	clean, err := validateSegnalazioneWrite(input)
	if err != nil {
		return Segnalazione{}, err
	}
	return s.store.UpdateSegnalazioneContent(ctx, id, clean, subject, email)
}

func (s *segnalazioneService) setState(ctx context.Context, id, state, subject, email string) (Segnalazione, error) {
	if s == nil || s.store == nil {
		return Segnalazione{}, errMAStoreUnavailable
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Segnalazione{}, errSegnalazioneNotFound
	}
	state = strings.TrimSpace(state)
	if _, ok := segnalazioneStates[state]; !ok {
		return Segnalazione{}, fmt.Errorf("%w: %q", errSegnalazioneStateInvalid, state)
	}
	return s.store.SetSegnalazioneState(ctx, id, state, subject, email)
}

// validateSegnalazioneWrite trims the outer whitespace of every field (no other
// normalisation: leading zeros, line breaks and foreign characters are kept
// verbatim) and enforces the minimum-content rule.
func validateSegnalazioneWrite(input SegnalazioneWrite) (SegnalazioneWrite, error) {
	clean := SegnalazioneWrite{
		Name:     strings.TrimSpace(input.Name),
		Website:  strings.TrimSpace(input.Website),
		Location: strings.TrimSpace(input.Location),
		FiscalID: strings.TrimSpace(input.FiscalID),
		Contacts: strings.TrimSpace(input.Contacts),
		Notes:    strings.TrimSpace(input.Notes),
	}
	if clean.Name == "" && clean.Website == "" && clean.FiscalID == "" && clean.Notes == "" {
		return SegnalazioneWrite{}, errSegnalazioneVuota
	}
	return clean, nil
}

// ---------------------------------------------------------------------------
// SQL store
// ---------------------------------------------------------------------------

const segnalazioneColumns = `id::text, name, website, location, fiscal_id, contacts, notes, state,
       created_at, COALESCE(created_by_email, ''), updated_at, COALESCE(updated_by_email, '')`

func (s *SQLStore) ListSegnalazioni(ctx context.Context) ([]Segnalazione, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT `+segnalazioneColumns+`
FROM binocolo.segnalazione
ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list segnalazioni: %w", err)
	}
	defer rows.Close()
	items := make([]Segnalazione, 0)
	for rows.Next() {
		item, err := scanSegnalazione(rows)
		if err != nil {
			return nil, fmt.Errorf("scan segnalazione: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate segnalazioni: %w", err)
	}
	return items, nil
}

func (s *SQLStore) GetSegnalazione(ctx context.Context, id string) (Segnalazione, error) {
	if !isUUIDLike(id) {
		return Segnalazione{}, errSegnalazioneNotFound
	}
	row := s.db.QueryRowContext(ctx, `
SELECT `+segnalazioneColumns+`
FROM binocolo.segnalazione
WHERE id = $1::uuid`, id)
	item, err := scanSegnalazione(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Segnalazione{}, errSegnalazioneNotFound
	}
	if err != nil {
		return Segnalazione{}, fmt.Errorf("select segnalazione: %w", err)
	}
	return item, nil
}

func (s *SQLStore) CreateSegnalazione(ctx context.Context, input SegnalazioneWrite, subject, email string) (Segnalazione, error) {
	row := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.segnalazione (name, website, location, fiscal_id, contacts, notes,
    created_by_subject, created_by_email, updated_by_subject, updated_by_email)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $7, $8)
RETURNING `+segnalazioneColumns,
		input.Name, input.Website, input.Location, input.FiscalID, input.Contacts, input.Notes,
		nullString(subject), nullString(email))
	item, err := scanSegnalazione(row)
	if err != nil {
		return Segnalazione{}, fmt.Errorf("insert segnalazione: %w", err)
	}
	return item, nil
}

// UpdateSegnalazioneContent writes the six fields only when the persisted state
// is not `chiusa`; the guard lives in the UPDATE itself so a closure that lands
// after the form was opened is still honoured. When no row is touched, the state
// is re-read to tell "not found" from "closed".
func (s *SQLStore) UpdateSegnalazioneContent(ctx context.Context, id string, input SegnalazioneWrite, subject, email string) (Segnalazione, error) {
	if !isUUIDLike(id) {
		return Segnalazione{}, errSegnalazioneNotFound
	}
	row := s.db.QueryRowContext(ctx, `
UPDATE binocolo.segnalazione
SET name = $2, website = $3, location = $4, fiscal_id = $5, contacts = $6, notes = $7,
    updated_at = now(), updated_by_subject = $8, updated_by_email = $9
WHERE id = $1::uuid AND state <> 'chiusa'
RETURNING `+segnalazioneColumns,
		id, input.Name, input.Website, input.Location, input.FiscalID, input.Contacts, input.Notes,
		nullString(subject), nullString(email))
	item, err := scanSegnalazione(row)
	if errors.Is(err, sql.ErrNoRows) {
		var state string
		lookupErr := s.db.QueryRowContext(ctx, `SELECT state FROM binocolo.segnalazione WHERE id = $1::uuid`, id).Scan(&state)
		if errors.Is(lookupErr, sql.ErrNoRows) {
			return Segnalazione{}, errSegnalazioneNotFound
		}
		if lookupErr != nil {
			return Segnalazione{}, fmt.Errorf("select segnalazione state: %w", lookupErr)
		}
		if state == segnalazioneStateChiusa {
			return Segnalazione{}, errSegnalazioneChiusa
		}
		return Segnalazione{}, errSegnalazioneNotFound
	}
	if err != nil {
		return Segnalazione{}, fmt.Errorf("update segnalazione: %w", err)
	}
	return item, nil
}

func (s *SQLStore) SetSegnalazioneState(ctx context.Context, id, state, subject, email string) (Segnalazione, error) {
	if !isUUIDLike(id) {
		return Segnalazione{}, errSegnalazioneNotFound
	}
	row := s.db.QueryRowContext(ctx, `
UPDATE binocolo.segnalazione
SET state = $2, updated_at = now(), updated_by_subject = $3, updated_by_email = $4
WHERE id = $1::uuid
RETURNING `+segnalazioneColumns,
		id, state, nullString(subject), nullString(email))
	item, err := scanSegnalazione(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Segnalazione{}, errSegnalazioneNotFound
	}
	if err != nil {
		return Segnalazione{}, fmt.Errorf("update segnalazione state: %w", err)
	}
	return item, nil
}

type segnalazioneRowScanner interface {
	Scan(dest ...any) error
}

func scanSegnalazione(row segnalazioneRowScanner) (Segnalazione, error) {
	var item Segnalazione
	if err := row.Scan(&item.ID, &item.Name, &item.Website, &item.Location, &item.FiscalID, &item.Contacts, &item.Notes, &item.State,
		&item.CreatedAt, &item.CreatedByEmail, &item.UpdatedAt, &item.UpdatedByEmail); err != nil {
		return Segnalazione{}, err
	}
	return item, nil
}

// isUUIDLike is a cheap shape check so a malformed id becomes "not found"
// instead of a Postgres cast error surfacing as 500.
func isUUIDLike(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
