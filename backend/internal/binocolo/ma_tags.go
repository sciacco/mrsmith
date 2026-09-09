package binocolo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Tag aziendali condivisi (issue #198, migrazione 148). Il catalogo è una
// tabella piatta `ma_tag` con unicità case/space-insensitive su
// lower(btrim(name)); le associazioni vivono in `ma_company_tag` con PK
// (company_key, tag_id). Tutte le scritture qui sono idempotenti o
// risolte-con-riuso: la creazione con nome equivalente riusa il tag esistente
// senza rinominarlo, anche sotto concorrenza (ON CONFLICT DO NOTHING su
// indice di espressione + rilettura in READ COMMITTED).

func (s *SQLStore) ListMATags(ctx context.Context) ([]MATag, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, name
FROM binocolo.ma_tag
ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list ma tags: %w", err)
	}
	defer rows.Close()
	tags := make([]MATag, 0)
	for rows.Next() {
		var tag MATag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, fmt.Errorf("scan ma tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma tags: %w", err)
	}
	return tags, nil
}

// ListMATagsByCompanyKeys legge in UNA query i tag di tutte le aziende
// richieste (board e pipeline aggregata: batch per company_key, mai N+1).
// L'ordine per nome è lo stesso del catalogo (ListMATags), così le etichette
// compaiono nello stesso ordine su scheda, card e pipeline. Le chiavi senza
// associazioni restano assenti dalla mappa: è il servizio a proiettarle come
// array vuoto, perché il contratto è che `tags` sia sempre un array JSON.
// Un errore di lettura propaga: nessuna azienda mostra falso «zero tag».
func (s *SQLStore) ListMATagsByCompanyKeys(ctx context.Context, companyKeys []string) (map[string][]MATag, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	out := map[string][]MATag{}
	if len(companyKeys) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(companyKeys))
	args := make([]any, len(companyKeys))
	for i, key := range companyKeys {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = key
	}
	query := fmt.Sprintf(`
SELECT association.company_key, tag.id::text, tag.name
FROM binocolo.ma_company_tag association
JOIN binocolo.ma_tag tag ON tag.id = association.tag_id
WHERE association.company_key IN (%s)
ORDER BY tag.name`, strings.Join(placeholders, ", "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ma tags by company keys: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var companyKey string
		var tag MATag
		if err := rows.Scan(&companyKey, &tag.ID, &tag.Name); err != nil {
			return nil, fmt.Errorf("scan ma tag by company key: %w", err)
		}
		out[companyKey] = append(out[companyKey], tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma tags by company keys: %w", err)
	}
	return out, nil
}

// RenameMATag rinomina un tag preservandone l'UUID. Il guard NOT EXISTS
// rifiuta il nuovo nome se un ALTRO tag è già equivalente; la corsa con una
// rinomina concorrente verso lo stesso nome è chiusa dall'indice univoco,
// tradotta in errMATagNameConflict da translateMATagConstraintError.
func (s *SQLStore) RenameMATag(ctx context.Context, tagID, name string) (MATag, error) {
	res, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_tag
SET name = $2, updated_at = now()
WHERE id = $1::uuid
  AND NOT EXISTS (
    SELECT 1 FROM binocolo.ma_tag other
    WHERE other.id <> $1::uuid AND lower(btrim(other.name)) = lower(btrim($2))
  )`, tagID, name)
	if err != nil {
		return MATag{}, fmt.Errorf("rename ma tag: %w", translateMATagConstraintError(err))
	}
	n, err := res.RowsAffected()
	if err != nil {
		return MATag{}, fmt.Errorf("rename ma tag rows: %w", err)
	}
	if n == 0 {
		// Nessuna riga toccata: il tag manca, oppure il nome è già di un
		// altro tag. La rilettura distingue i due casi.
		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM binocolo.ma_tag WHERE id = $1::uuid)`, tagID).Scan(&exists); err != nil {
			return MATag{}, fmt.Errorf("rename ma tag exists: %w", err)
		}
		if !exists {
			return MATag{}, errMATagNotFound
		}
		return MATag{}, errMATagNameConflict
	}
	var tag MATag
	err = s.db.QueryRowContext(ctx, `
SELECT id::text, name
FROM binocolo.ma_tag
WHERE id = $1::uuid`, tagID).Scan(&tag.ID, &tag.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return MATag{}, errMATagNotFound
	}
	if err != nil {
		return MATag{}, fmt.Errorf("reload ma tag: %w", err)
	}
	return tag, nil
}

// DeleteMATag elimina globalmente il tag; le associazioni seguono per
// CASCADE, le aziende non vengono toccate.
func (s *SQLStore) DeleteMATag(ctx context.Context, tagID string) error {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM binocolo.ma_tag
WHERE id = $1::uuid`, tagID)
	if err != nil {
		return fmt.Errorf("delete ma tag: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete ma tag rows: %w", err)
	}
	if n == 0 {
		return errMATagNotFound
	}
	return nil
}

// CreateAndAssignMATag crea il tag (o riusa quello equivalente, senza
// rinominarlo) e lo assegna all'azienda nella STESSA transazione. L'azienda
// è già stata validata dal servizio; una sparizione concorrente esplode
// sulla FK RESTRICT e viene tradotta in errMACompanyKeyUnknown.
func (s *SQLStore) CreateAndAssignMATag(ctx context.Context, companyKey, name string) (MATag, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MATag{}, fmt.Errorf("begin create ma tag: %w", err)
	}
	defer tx.Rollback()
	var tag MATag
	err = tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_tag (name)
VALUES ($1)
ON CONFLICT (lower(btrim(name))) DO NOTHING
RETURNING id::text, name`, name).Scan(&tag.ID, &tag.Name)
	if errors.Is(err, sql.ErrNoRows) {
		// Nome equivalente già presente, anche per inserimento concorrente:
		// si riusa il tag esistente con il SUO nome. La rilettura aspetta il
		// commit dell'altro inserimento (attesa sull'indice speculative) e
		// in READ COMMITTED ne vede la riga.
		err = tx.QueryRowContext(ctx, `
SELECT id::text, name
FROM binocolo.ma_tag
WHERE lower(btrim(name)) = lower(btrim($1))`, name).Scan(&tag.ID, &tag.Name)
	}
	if err != nil {
		return MATag{}, fmt.Errorf("insert ma tag: %w", translateMATagConstraintError(err))
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_company_tag (company_key, tag_id)
VALUES ($1, $2::uuid)
ON CONFLICT (company_key, tag_id) DO NOTHING`, companyKey, tag.ID); err != nil {
		return MATag{}, fmt.Errorf("insert ma company tag: %w", translateMATagConstraintError(translateMACompanyConstraintError(err, false)))
	}
	if err := tx.Commit(); err != nil {
		return MATag{}, fmt.Errorf("commit create ma tag: %w", err)
	}
	return tag, nil
}

// AssignMATag assegna un tag esistente a un'azienda; l'associazione
// duplicata è un no-op (idempotente), il vincolo PK è il backstop.
func (s *SQLStore) AssignMATag(ctx context.Context, companyKey, tagID string) error {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM binocolo.ma_tag WHERE id = $1::uuid)`, tagID).Scan(&exists); err != nil {
		return fmt.Errorf("exists ma tag: %w", err)
	}
	if !exists {
		return errMATagNotFound
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_company_tag (company_key, tag_id)
VALUES ($1, $2::uuid)
ON CONFLICT (company_key, tag_id) DO NOTHING`, companyKey, tagID); err != nil {
		return fmt.Errorf("assign ma company tag: %w", translateMATagConstraintError(translateMACompanyConstraintError(err, false)))
	}
	return nil
}

// UnassignMATag rimuove la sola associazione: il tag resta nel catalogo e
// sulle altre aziende. Un tag inesistente è errMATagNotFound (riferimento
// sconosciuto, non caso idempotente); con il tag esistente, l'assenza
// dell'associazione è obiettivo già raggiunto e non è un errore (idempotente).
func (s *SQLStore) UnassignMATag(ctx context.Context, companyKey, tagID string) error {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM binocolo.ma_tag WHERE id = $1::uuid)`, tagID).Scan(&exists); err != nil {
		return fmt.Errorf("exists ma tag: %w", err)
	}
	if !exists {
		return errMATagNotFound
	}
	if _, err := s.db.ExecContext(ctx, `
DELETE FROM binocolo.ma_company_tag
WHERE company_key = $1 AND tag_id = $2::uuid`, companyKey, tagID); err != nil {
		return fmt.Errorf("unassign ma company tag: %w", err)
	}
	return nil
}

// translateMATagConstraintError traduce nel dominio le violazioni di vincolo
// del modulo tag: 23505 su ma_tag_name_normalized_key è la corsa tra
// rinomine/creazioni equivalenti (conflitto di nome); 23503 su
// ma_company_tag_tag_id_fkey è la corsa con l'eliminazione globale del tag
// durante un'assegnazione, che emerge come tag inesistente. La FK verso
// l'azienda (_company_key_fkey) resta gestita da translateMACompanyConstraintError.
func translateMATagConstraintError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	if pgErr.Code == "23505" && pgErr.ConstraintName == "ma_tag_name_normalized_key" {
		return errMATagNameConflict
	}
	if pgErr.Code == "23503" && pgErr.ConstraintName == "ma_company_tag_tag_id_fkey" {
		return errMATagNotFound
	}
	return err
}
