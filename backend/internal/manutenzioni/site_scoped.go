package manutenzioni

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// validateGlobalSiteTx verifica che il site_id esista ed abbia scope='global'.
// Usato in POST maintenance: il sito deve essere permanente (niente scoped,
// dato che la manutenzione ancora non esiste e quindi non può possederne uno).
func validateGlobalSiteTx(ctx context.Context, tx *sql.Tx, siteID int64) error {
	if siteID <= 0 {
		return errBadRequest
	}
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM maintenance.site WHERE site_id = $1 AND scope = 'global')`,
		siteID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errBadRequest
	}
	return nil
}

// validateAssignableSiteTx verifica che il site_id sia assegnabile alla manutenzione:
// deve essere un sito globale, oppure uno scoped posseduto da QUESTA manutenzione.
// Impedisce di collegare una manutenzione a un sito scoped che appartiene ad altra
// manutenzione (invariante non esprimibile via CHECK di DB).
func validateAssignableSiteTx(ctx context.Context, tx *sql.Tx, siteID, maintenanceID int64) error {
	if siteID <= 0 {
		return errBadRequest
	}
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM maintenance.site
			WHERE site_id = $1
			  AND (scope = 'global' OR (scope = 'scoped' AND owner_maintenance_id = $2))
		)`,
		siteID, maintenanceID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errBadRequest
	}
	return nil
}

// insertScopedSiteTx crea un sito scoped legato alla manutenzione indicata.
// Il codice viene auto-generato come SCOPED-{maintenance_id} se non fornito:
// l'unicità è garantita dall'indice parziale site_code_scoped_unique per (owner, code).
func insertScopedSiteTx(ctx context.Context, tx *sql.Tx, maintenanceID int64, input adhocSiteInput) (int64, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return 0, errBadRequest
	}
	code := ""
	if input.Code != nil {
		code = strings.TrimSpace(*input.Code)
	}
	if code == "" {
		code = fmt.Sprintf("SCOPED-%d", maintenanceID)
	}
	var city, country *string
	if input.City != nil {
		trimmed := strings.TrimSpace(*input.City)
		if trimmed != "" {
			city = &trimmed
		}
	}
	if input.CountryCode != nil {
		trimmed := strings.TrimSpace(*input.CountryCode)
		if trimmed != "" {
			country = &trimmed
		}
	}

	var siteID int64
	err := tx.QueryRowContext(ctx,
		`INSERT INTO maintenance.site (code, name, city, country_code, is_active, scope, owner_maintenance_id)
		 VALUES ($1, $2, $3, $4, true, 'scoped', $5)
		 RETURNING site_id`,
		code, name, city, country, maintenanceID).Scan(&siteID)
	if err != nil {
		// Violazione unicità code per-owner: probabile race con un altro
		// scoped della stessa manutenzione. Segnaliamo come bad request per
		// far rigenerare un code diverso lato client.
		if isUniqueViolation(err) {
			return 0, errBadRequest
		}
		return 0, err
	}
	return siteID, nil
}

// isUniqueViolation rileva l'errore di violazione di un indice unico in
// PostgreSQL senza importare driver-specific packages: match sul testo,
// sufficiente per il nostro caso d'uso.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return errors.Is(err, sql.ErrNoRows) == false &&
		(strings.Contains(msg, "SQLSTATE 23505") ||
			strings.Contains(msg, "duplicate key") ||
			strings.Contains(msg, "unique constraint"))
}
