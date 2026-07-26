package binocolo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Resolver del registro identità azienda (issue #86, migrazione 120).
//
// Contratto transazionale, in due passaggi:
//
//  1. RILEVAZIONE — normalizza e rileva TUTTI i conflitti, contro il DB e
//     intra-batch, senza scrivere nulla (planMACompanyResolution).
//  2. SCRITTURA — solo se l'insieme è pulito: crea entità, collega
//     identificatori, promuove lo stato, registra il nome.
//
// Entrambi i passaggi vivono nella transazione del CHIAMANTE: dentro
// ReplaceMATargets la risoluzione è di batch, non una transazione autonoma per
// target — sarebbe lenta e aprirebbe la race su ogni riga.
//
// Un conflitto NON viene registrato qui: il rollback porterebbe via la riga di
// conflitto. Il resolver restituisce un errore tipizzato; il livello service
// intercetta e scrive il ledger in una SECONDA transazione
// (maService.recordMACompanyIdentityConflict).

// newMACompanyKey conia la chiave di un'azienda nuova: un UUID MAIUSCOLO.
//
// Il maiuscolo non è cosmetico. normalizeMACompanyKey (upper+trim) è la forma
// canonica con cui TUTTO il codice — handler, IRL, thesis reading, service —
// tratta una company_key in arrivo, e le chiavi adottate sono ObjectId
// maiuscoli. Un UUID minuscolo verrebbe maiuscolizzato al primo confronto e non
// ritroverebbe più la propria riga: la Scheda azienda di ogni azienda nuova
// darebbe 404.
func newMACompanyKey() string {
	return strings.ToUpper(uuid.NewString())
}

// maResolveRetryAttempts limita i ri-tentativi su race: due resolver che creano
// la stessa azienda nuova nello stesso istante. Il perdente annulla fino al
// savepoint e rilegge il vincitore — senza lasciare ma_company orfane.
const maResolveRetryAttempts = 3

// maCompanyIdentityRaceError distingue la collisione da corsa (l'identificatore
// è stato rivendicato dopo la nostra rilevazione) dal conflitto vero. Non esce
// mai dal resolver: si trasforma in un nuovo tentativo, o — esaurite le prove —
// nel conflitto tipizzato.
type maCompanyIdentityRaceError struct {
	Conflict maCompanyIdentityConflict
}

func (e *maCompanyIdentityRaceError) Error() string {
	return fmt.Sprintf("ma company identity race on %s/%s", e.Conflict.Namespace, e.Conflict.Value)
}

// ResolveMACompany risolve una singola occorrenza nella propria transazione.
// Le scritture di batch usano resolveMACompanyBatchTx dentro la transazione del
// chiamante.
func (s *SQLStore) ResolveMACompany(ctx context.Context, observation maCompanyObservation) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin ma company resolve: %w", err)
	}
	defer tx.Rollback()
	keys, err := resolveMACompanyBatchTx(ctx, tx, []maCompanyObservation{observation})
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit ma company resolve: %w", err)
	}
	return keys[0], nil
}

// ObserveMACompany registra il nome osservato in una chiamata al fornitore.
// Separata dalla risoluzione perché la regola sul nome richiede un input che
// l'identità non ha — l'ISTANTE della chiamata — e perché rileggere una riga
// già a DB non deve avanzare quella data.
func (s *SQLStore) ObserveMACompany(ctx context.Context, companyKey, companyName, source string, observedAt time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma company observe: %w", err)
	}
	defer tx.Rollback()
	if err := observeMACompanyNameTx(ctx, tx, maCompanyNameObservation{
		CompanyKey: companyKey, CompanyName: companyName, ObservedAt: observedAt, Source: source,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// MACompanyExists dice se una chiave è un'azienda conosciuta dal registro.
//
// Serve ai writer che ricevono la chiave dal CLIENT — voto, esito, etichetta di
// settore — e non da una risoluzione o da una colonna. Senza, «chiave esistente
// fornita dal chiamante» è un'assunzione e non un fatto: un client che ne mandi
// una sbagliata scrive una riga che nessuna lettura ritrova e che il monitor
// scopre solo dopo. La FK verso ma_company è presente dalla migrazione 123; la
// guardia resta perché produce un errore di dominio leggibile prima del 23503.
func (s *SQLStore) MACompanyExists(ctx context.Context, companyKey string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return false, nil
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM binocolo.ma_company WHERE company_key = $1)
`, companyKey).Scan(&exists); err != nil {
		return false, fmt.Errorf("check ma company exists: %w", err)
	}
	return exists, nil
}

// RecordMACompanyIdentityConflicts è la SECONDA transazione del contratto sul
// conflitto. Idempotente: la stessa rilevazione ripetuta aggiorna
// last_detected_at e incrementa il contatore invece di fallire — una scrittura
// diagnostica non deve mai essere ciò che rompe.
func (s *SQLStore) RecordMACompanyIdentityConflicts(ctx context.Context, conflicts []maCompanyIdentityConflict) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if len(conflicts) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma company identity conflict: %w", err)
	}
	defer tx.Rollback()
	for _, conflict := range conflicts {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_company_identity_conflict (
  namespace, value, company_key_existing, company_key_incoming,
  session_id, run_id, target_id, company_name
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (namespace, value) WHERE state = 'open' DO UPDATE SET
  last_detected_at     = now(),
  detections           = binocolo.ma_company_identity_conflict.detections + 1,
  company_key_incoming = EXCLUDED.company_key_incoming,
  session_id           = EXCLUDED.session_id,
  run_id               = EXCLUDED.run_id,
  target_id            = EXCLUDED.target_id,
  company_name         = EXCLUDED.company_name
`, conflict.Namespace, conflict.Value, conflict.ExistingCompanyKey, conflict.IncomingCompanyKey,
			nullString(conflict.SessionID), nullString(conflict.RunID), nullString(conflict.TargetID),
			nullString(conflict.CompanyName)); err != nil {
			return fmt.Errorf("record ma company identity conflict: %w", err)
		}
	}
	return tx.Commit()
}

// resolveMACompanyBatchTx risolve un intero batch dentro la transazione del
// chiamante e restituisce una chiave per osservazione, nello stesso ordine.
//
// Un conflitto abortisce PRIMA di qualunque scrittura: un run di ricerca è atomico
// nella testa dell'analista, e persisterne una parte mostrerebbe un numero di
// target che non corrisponde a ciò che è stato trovato.
func resolveMACompanyBatchTx(ctx context.Context, tx *sql.Tx, observations []maCompanyObservation) ([]string, error) {
	if len(observations) == 0 {
		return nil, nil
	}
	var lastRace *maCompanyIdentityRaceError
	for attempt := 0; attempt < maResolveRetryAttempts; attempt++ {
		// Il savepoint è ciò che rende ri-tentabile una race senza lasciare
		// ma_company orfane: il perdente annulla le proprie scritture e
		// rilegge il vincitore, la transazione del chiamante resta viva.
		savepoint := fmt.Sprintf("ma_resolve_%d", attempt)
		if _, err := tx.ExecContext(ctx, "SAVEPOINT "+savepoint); err != nil {
			return nil, fmt.Errorf("savepoint ma company resolve: %w", err)
		}
		keys, err := resolveMACompanyBatchOnce(ctx, tx, observations)
		if err == nil {
			if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint); err != nil {
				return nil, fmt.Errorf("release savepoint ma company resolve: %w", err)
			}
			return keys, nil
		}
		var race *maCompanyIdentityRaceError
		if !errors.As(err, &race) {
			return nil, err
		}
		lastRace = race
		if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepoint); rollbackErr != nil {
			return nil, fmt.Errorf("rollback to savepoint ma company resolve: %w", rollbackErr)
		}
	}
	return nil, &maCompanyIdentityConflictError{Conflicts: []maCompanyIdentityConflict{lastRace.Conflict}}
}

func resolveMACompanyBatchOnce(ctx context.Context, tx *sql.Tx, observations []maCompanyObservation) ([]string, error) {
	refs := map[maCompanyIdentifierRef]struct{}{}
	priors := map[string]struct{}{}
	for _, observation := range observations {
		for _, identifier := range extractMACompanyIdentifiers(observation.VATCode, observation.TaxCode, observation.VendorID) {
			refs[identifier.maCompanyIdentifierRef] = struct{}{}
		}
		if prior := normalizeMACompanyKey(observation.PriorCompanyKey); prior != "" {
			priors[prior] = struct{}{}
		}
	}

	// Una sola query per batch, non una per target: un run di ricerca può
	// portare centinaia di target, e N round-trip sarebbero il costo dominante.
	existing, err := loadMACompanyIdentifierOwnersTx(ctx, tx, refs)
	if err != nil {
		return nil, err
	}
	knownKeys, err := loadMACompanyKeysTx(ctx, tx, priors)
	if err != nil {
		return nil, err
	}

	plan := planMACompanyResolution(observations, existing, knownKeys, newMACompanyKey)
	if len(plan.MissingIndex) > 0 {
		first := observations[plan.MissingIndex[0]]
		return nil, &maCompanyIdentityMissingError{CompanyName: first.CompanyName, TargetID: first.TargetID}
	}
	if len(plan.Conflicts) > 0 {
		return nil, &maCompanyIdentityConflictError{Conflicts: plan.Conflicts}
	}

	for _, draft := range plan.NewEntities {
		var observedAt any
		var source any
		if draft.ObservedAt != nil && draft.CompanyName != "" {
			observedAt = draft.ObservedAt.UTC()
			source = nullString(draft.Source)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_company (company_key, company_name, name_observed_at, name_source, identity_state, origin)
VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6)
`, draft.CompanyKey, draft.CompanyName, observedAt, source, draft.IdentityState, maCompanyOriginAssigned); err != nil {
			return nil, fmt.Errorf("insert ma company: %w", err)
		}
	}

	for _, attachment := range plan.Attachments {
		if err := attachMACompanyIdentifierTx(ctx, tx, attachment); err != nil {
			return nil, err
		}
	}

	// La promozione vendor_only -> fiscal è APPLICATIVA e sta in questa
	// transazione: un trigger che aggiornasse ma_company a ogni inserimento di
	// identificatore sarebbe un effetto collaterale nascosto. La transizione
	// inversa, invece, è vietata dal trigger — è una regola che non deve
	// dipendere da chi scrive.
	for _, key := range plan.Promotions {
		if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_company
SET identity_state = 'fiscal'
WHERE company_key = $1 AND identity_state = 'vendor_only'
`, key); err != nil {
			return nil, fmt.Errorf("promote ma company identity state: %w", err)
		}
	}

	for _, name := range plan.Names {
		if err := observeMACompanyNameTx(ctx, tx, name); err != nil {
			return nil, err
		}
	}
	return plan.Keys, nil
}

// attachMACompanyIdentifierTx collega un identificatore aggiornando i ruoli in
// OR. Il WHERE sul DO UPDATE è la guardia di corsa: se nel frattempo un altro
// resolver ha rivendicato lo stesso valore per un'altra azienda, la UPDATE non
// tocca nulla e RETURNING non restituisce righe.
//
// first_seen_at/last_seen_at datano l'OSSERVAZIONE DEL FORNITORE, non il
// passaggio nel resolver, con la stessa semantica del backfill (MIN/MAX su
// seen_at): LEAST arretra la prima osservazione quando ne arriva una più antica
// — una risposta di cache elaborata dopo una fresca — e GREATEST impedisce a
// quella stessa risposta vecchia di far avanzare l'ultima. Una ri-persistenza
// (rescore, ObservedAt nil) non muove né l'una né l'altra. I ruoli, invece, si
// aggiornano sempre: sono conoscenza acquisita, indipendente da quando è
// arrivata.
//
// Un INSERT senza istante di osservazione data la riga a now(): non abbiamo di
// meglio, ed è il momento in cui l'identificatore è entrato nel registro. LEAST
// lo arretra alla prima osservazione vera appena ne arriva una.
func attachMACompanyIdentifierTx(ctx context.Context, tx *sql.Tx, attachment maCompanyIdentifierAttachment) error {
	var observedAt any
	if attachment.ObservedAt != nil && !attachment.ObservedAt.IsZero() {
		observedAt = attachment.ObservedAt.UTC()
	}
	var owner string
	err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_company_identifier (namespace, value, company_key, is_vat, is_tax, first_seen_at, last_seen_at)
VALUES ($1, $2, $3, $4, $5, COALESCE($6::timestamptz, now()), COALESCE($6::timestamptz, now()))
ON CONFLICT (namespace, value) DO UPDATE SET
  is_vat        = binocolo.ma_company_identifier.is_vat OR EXCLUDED.is_vat,
  is_tax        = binocolo.ma_company_identifier.is_tax OR EXCLUDED.is_tax,
  first_seen_at = LEAST(
                    binocolo.ma_company_identifier.first_seen_at,
                    COALESCE($6::timestamptz, binocolo.ma_company_identifier.first_seen_at)
                  ),
  last_seen_at  = GREATEST(
                    binocolo.ma_company_identifier.last_seen_at,
                    COALESCE($6::timestamptz, binocolo.ma_company_identifier.last_seen_at)
                  )
WHERE binocolo.ma_company_identifier.company_key = EXCLUDED.company_key
RETURNING company_key
`, attachment.Namespace, attachment.Value, attachment.CompanyKey, attachment.IsVAT, attachment.IsTax, observedAt).Scan(&owner)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("attach ma company identifier: %w", err)
	}
	existing := ""
	if err := tx.QueryRowContext(ctx, `
SELECT company_key FROM binocolo.ma_company_identifier WHERE namespace = $1 AND value = $2
`, attachment.Namespace, attachment.Value).Scan(&existing); err != nil {
		return fmt.Errorf("read ma company identifier owner: %w", err)
	}
	return &maCompanyIdentityRaceError{Conflict: maCompanyIdentityConflict{
		Namespace:          attachment.Namespace,
		Value:              attachment.Value,
		ExistingCompanyKey: existing,
		IncomingCompanyKey: attachment.CompanyKey,
	}}
}

// observeMACompanyNameTx applica la regola «vince la lettura più recente». Il
// confronto è sull'istante dell'osservazione, non su quello della scrittura:
// una rilettura del DB non porta ObservedAt e non arriva mai qui.
func observeMACompanyNameTx(ctx context.Context, tx *sql.Tx, observation maCompanyNameObservation) error {
	key := normalizeMACompanyKey(observation.CompanyKey)
	name := strings.TrimSpace(observation.CompanyName)
	if key == "" || name == "" || observation.ObservedAt.IsZero() {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_company
SET company_name = $2, name_observed_at = $3, name_source = NULLIF($4, '')
WHERE company_key = $1
  AND (name_observed_at IS NULL OR name_observed_at < $3)
`, key, name, observation.ObservedAt.UTC(), strings.TrimSpace(observation.Source)); err != nil {
		return fmt.Errorf("observe ma company name: %w", err)
	}
	return nil
}

func loadMACompanyIdentifierOwnersTx(ctx context.Context, tx *sql.Tx, refs map[maCompanyIdentifierRef]struct{}) (map[maCompanyIdentifierRef]string, error) {
	out := map[maCompanyIdentifierRef]string{}
	if len(refs) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(refs))
	args := make([]any, 0, len(refs)*2)
	for ref := range refs {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", len(args)+1, len(args)+2))
		args = append(args, ref.Namespace, ref.Value)
	}
	rows, err := tx.QueryContext(ctx, `
SELECT namespace, value, company_key
FROM binocolo.ma_company_identifier
WHERE (namespace, value) IN (`+strings.Join(placeholders, ", ")+`)
`, args...)
	if err != nil {
		return nil, fmt.Errorf("load ma company identifiers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ref maCompanyIdentifierRef
		var key string
		if err := rows.Scan(&ref.Namespace, &ref.Value, &key); err != nil {
			return nil, fmt.Errorf("scan ma company identifier: %w", err)
		}
		out[ref] = key
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma company identifiers: %w", err)
	}
	return out, nil
}

func loadMACompanyKeysTx(ctx context.Context, tx *sql.Tx, keys map[string]struct{}) (map[string]bool, error) {
	out := map[string]bool{}
	if len(keys) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))
	for key := range keys {
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
		args = append(args, key)
	}
	rows, err := tx.QueryContext(ctx, `
SELECT company_key FROM binocolo.ma_company WHERE company_key IN (`+strings.Join(placeholders, ", ")+`)
`, args...)
	if err != nil {
		return nil, fmt.Errorf("load ma companies: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan ma company: %w", err)
		}
		out[key] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma companies: %w", err)
	}
	return out, nil
}

// maCompanyObservationFromTarget costruisce l'osservazione a partire da un
// target. VendorObservedAt è non-nil solo quando la riga viene da una chiamata
// al fornitore appena fatta (lo stampiglia parseMATargetsFromVendorData): una
// riga riletta dal DB non deve avanzare la data di osservazione del nome.
func maCompanyObservationFromTarget(target MATarget, sessionID, runID string) maCompanyObservation {
	return maCompanyObservation{
		PriorCompanyKey: target.CompanyKey,
		VendorID:        target.VendorID,
		VATCode:         target.VATCode,
		TaxCode:         target.TaxCode,
		CompanyName:     target.CompanyName,
		ObservedAt:      target.VendorObservedAt,
		Source:          maCompanyNameSourceVendor,
		SessionID:       sessionID,
		RunID:           runID,
		TargetID:        target.ID,
	}
}
