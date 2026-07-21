package binocolo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Store for the nota-integrativa reading pipeline (issue #78, Fase 6). database/sql on the
// shared Anisetta DB, same conventions as ma_filing_store.go: methods on *SQLStore, $n
// params, ::uuid/::date casts, nullString/nullInt/nullFloatArg for NULLs (numeric columns
// take a plain float parameter, as cost_eur does), columns tracking migration 114 exactly.
// A reading run is one LLM pass over a processing run; its
// proposals are IMMUTABLE (dedup by fingerprint within the run) and its decisions APPEND-ONLY
// (the latest decision per proposal is the current state; the reducer lives in ma_ni_reading.go).

// nullFloatArg carries a nullable numeric column value: nil (SQL NULL) for a nil pointer,
// else the float verbatim. Mirrors nullInt/nullString for *float64 columns.
func nullFloatArg(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

// ---------------------------------------------------------------------------
// ma_ni_reading_run — one LLM reading pass over a processing run.
// ---------------------------------------------------------------------------

// maNIReadingRun mirrors binocolo.ma_ni_reading_run. prompt_id/model_id are uuid soft-refs
// to the mrsmith registry (nullable, no FK — pattern 105), surfaced as strings ("" for NULL).
type maNIReadingRun struct {
	ID              string
	FilingID        string
	ProcessingRunID string
	PromptID        string
	ModelID         string
	RequestID       string
	Status          string
	CreatedAt       time.Time
}

const maNIReadingRunColumns = `id::text, filing_id::text, processing_run_id::text,
	COALESCE(prompt_id::text, ''), COALESCE(model_id::text, ''), COALESCE(request_id, ''),
	status, created_at`

func scanMANIReadingRun(scanner interface{ Scan(...any) error }) (maNIReadingRun, error) {
	var r maNIReadingRun
	if err := scanner.Scan(
		&r.ID, &r.FilingID, &r.ProcessingRunID,
		&r.PromptID, &r.ModelID, &r.RequestID,
		&r.Status, &r.CreatedAt,
	); err != nil {
		return maNIReadingRun{}, err
	}
	return r, nil
}

// CreateMANIReadingRun opens a new immutable NI reading run for a filing: in one transaction
// it supersedes the filing's prior active reading run(s) and inserts the new run as active.
// model_id/prompt_id are the uuid soft-refs of the resolved registry Model/Prompt (contract
// QA-F1); requestID correlates the run to the ingest trace + the per-section llm_call_audit
// rows. Returns the new run id.
func (s *SQLStore) CreateMANIReadingRun(ctx context.Context, filingID, processingRunID, promptID, modelID, requestID string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin create ma ni reading run: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_ni_reading_run
SET status = 'superseded'
WHERE filing_id = $1::uuid AND status = 'active'
`, filingID); err != nil {
		return "", fmt.Errorf("supersede ma ni reading runs: %w", err)
	}

	var id string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_ni_reading_run
  (filing_id, processing_run_id, prompt_id, model_id, request_id, status)
VALUES
  ($1::uuid, $2::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, NULLIF($5, ''), 'active')
RETURNING id::text
`, filingID, processingRunID, promptID, modelID, requestID).Scan(&id); err != nil {
		return "", fmt.Errorf("insert ma ni reading run: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit create ma ni reading run: %w", err)
	}
	return id, nil
}

// GetActiveMANIReadingRun returns the current active reading run for a filing, or (nil, nil)
// when none. Application-enforced one-active invariant (the table has no unique constraint,
// coherent with ma_filing_processing_run); ORDER BY created_at DESC guards against a
// transient double-active.
func (s *SQLStore) GetActiveMANIReadingRun(ctx context.Context, filingID string) (*maNIReadingRun, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	r, err := scanMANIReadingRun(s.db.QueryRowContext(ctx, `SELECT `+maNIReadingRunColumns+`
FROM binocolo.ma_ni_reading_run
WHERE filing_id = $1::uuid AND status = 'active'
ORDER BY created_at DESC
LIMIT 1`, filingID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active ma ni reading run: %w", err)
	}
	return &r, nil
}

// ---------------------------------------------------------------------------
// ma_ni_proposal — immutable adjustment proposals (dedup by fingerprint per run).
// ---------------------------------------------------------------------------

// maNIProposal mirrors binocolo.ma_ni_proposal. Nullable numerics surface as *float64,
// exercise_date/page_no as *time.Time / *int (nil when NULL).
type maNIProposal struct {
	ID                   string
	RunID                string
	ExerciseDate         *time.Time
	Fingerprint          string
	FattoOsservato       string
	ImportoLordo         *float64
	TrattamentoCandidato string
	Direction            string
	ImportoRettifica     *float64
	Incertezza           string
	Quote                string
	PageNo               *int
	Section              string
	Label                string
	Rationale            string
	CreatedAt            time.Time
}

const maNIProposalColumns = `id::text, run_id::text, exercise_date, fingerprint,
	COALESCE(fatto_osservato, ''), importo_lordo, COALESCE(trattamento_candidato, ''),
	COALESCE(direction, ''), importo_rettifica, COALESCE(incertezza, ''),
	COALESCE(quote, ''), page_no, COALESCE(section, ''), COALESCE(label, ''),
	COALESCE(rationale, ''), created_at`

func scanMANIProposal(scanner interface{ Scan(...any) error }) (maNIProposal, error) {
	var p maNIProposal
	var exerciseDate sql.NullTime
	var importoLordo, importoRettifica sql.NullFloat64
	var pageNo sql.NullInt64
	if err := scanner.Scan(
		&p.ID, &p.RunID, &exerciseDate, &p.Fingerprint,
		&p.FattoOsservato, &importoLordo, &p.TrattamentoCandidato,
		&p.Direction, &importoRettifica, &p.Incertezza,
		&p.Quote, &pageNo, &p.Section, &p.Label,
		&p.Rationale, &p.CreatedAt,
	); err != nil {
		return maNIProposal{}, err
	}
	if exerciseDate.Valid {
		p.ExerciseDate = &exerciseDate.Time
	}
	if importoLordo.Valid {
		v := importoLordo.Float64
		p.ImportoLordo = &v
	}
	if importoRettifica.Valid {
		v := importoRettifica.Float64
		p.ImportoRettifica = &v
	}
	if pageNo.Valid {
		v := int(pageNo.Int64)
		p.PageNo = &v
	}
	return p, nil
}

// InsertMANIProposals batch-inserts a run's proposals, IMMUTABLE and deduplicated by the
// (run_id, fingerprint) unique index: ON CONFLICT DO NOTHING makes a duplicate fact within
// the run a no-op and a retry of the same batch idempotent. Returns the number of rows
// actually inserted.
func (s *SQLStore) InsertMANIProposals(ctx context.Context, runID string, proposals []maNIProposal) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("binocolo ma store not configured")
	}
	if len(proposals) == 0 {
		return 0, nil
	}
	var b strings.Builder
	b.WriteString(`INSERT INTO binocolo.ma_ni_proposal
  (run_id, exercise_date, fingerprint, fatto_osservato, importo_lordo, trattamento_candidato,
   direction, importo_rettifica, incertezza, quote, page_no, section, label, rationale) VALUES `)
	args := []any{runID}
	for i, p := range proposals {
		if i > 0 {
			b.WriteString(", ")
		}
		base := len(args)
		fmt.Fprintf(&b, "($1::uuid, $%d::date, $%d, NULLIF($%d, ''), $%d, NULLIF($%d, ''), NULLIF($%d, ''), $%d, NULLIF($%d, ''), NULLIF($%d, ''), $%d, NULLIF($%d, ''), NULLIF($%d, ''), NULLIF($%d, ''))",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10, base+11, base+12, base+13)
		args = append(args,
			maDateArg(p.ExerciseDate), p.Fingerprint, p.FattoOsservato, nullFloatArg(p.ImportoLordo),
			p.TrattamentoCandidato, p.Direction, nullFloatArg(p.ImportoRettifica), p.Incertezza,
			p.Quote, nullInt(p.PageNo), p.Section, p.Label, p.Rationale)
	}
	b.WriteString(" ON CONFLICT (run_id, fingerprint) DO NOTHING")
	res, err := s.db.ExecContext(ctx, b.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("insert ma ni proposals: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("insert ma ni proposals rows: %w", err)
	}
	return int(affected), nil
}

// ListMANIProposalsByRun returns the proposals of a reading run, oldest first.
func (s *SQLStore) ListMANIProposalsByRun(ctx context.Context, runID string) ([]maNIProposal, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+maNIProposalColumns+`
FROM binocolo.ma_ni_proposal
WHERE run_id = $1::uuid
ORDER BY created_at, id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list ma ni proposals by run: %w", err)
	}
	defer rows.Close()
	out := []maNIProposal{}
	for rows.Next() {
		p, err := scanMANIProposal(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ma ni proposal: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma ni proposals: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// ma_ni_decision — append-only analyst decisions (ratify / reject / revoke).
// ---------------------------------------------------------------------------

// maNIDecision mirrors binocolo.ma_ni_decision. auto_reconfirmed_from links an
// auto-reconfirmed decision to the human decision it clones ("" when not a clone).
type maNIDecision struct {
	ID                  string
	ProposalID          string
	Action              string
	RatifiedAmount      *float64
	RatifiedTreatment   string
	Reason              string
	ActorSubject        string
	ActorEmail          string
	CreatedAt           time.Time
	AutoReconfirmedFrom string
}

const maNIDecisionColumns = `id::text, proposal_id::text, action, ratified_amount,
	COALESCE(ratified_treatment, ''), COALESCE(reason, ''), COALESCE(actor_subject, ''),
	COALESCE(actor_email, ''), created_at, COALESCE(auto_reconfirmed_from::text, '')`

func scanMANIDecision(scanner interface{ Scan(...any) error }) (maNIDecision, error) {
	var d maNIDecision
	var ratifiedAmount sql.NullFloat64
	if err := scanner.Scan(
		&d.ID, &d.ProposalID, &d.Action, &ratifiedAmount,
		&d.RatifiedTreatment, &d.Reason, &d.ActorSubject,
		&d.ActorEmail, &d.CreatedAt, &d.AutoReconfirmedFrom,
	); err != nil {
		return maNIDecision{}, err
	}
	if ratifiedAmount.Valid {
		v := ratifiedAmount.Float64
		d.RatifiedAmount = &v
	}
	return d, nil
}

// maNIDecisionCreate is the input to InsertMANIDecision. ratified_amount is the SIGNED
// effect (valued only on 'ratify'); ratified_treatment promotes a dd_only proposal; the
// enforcement of "verso incerto senza importo+segno ⇒ mai effetto" lives in the reducer
// (the DB is permissive). AutoReconfirmedFrom is set only for cloned decisions.
type maNIDecisionCreate struct {
	ProposalID          string
	Action              string
	RatifiedAmount      *float64
	RatifiedTreatment   string
	Reason              string
	ActorSubject        string
	ActorEmail          string
	AutoReconfirmedFrom string
}

// InsertMANIDecision appends a decision to a proposal (append-only: the latest row is the
// current state). Validates the proposal exists (the INSERT ... SELECT ... WHERE EXISTS
// returns no row otherwise) so a stale proposal id fails legibly rather than on the FK.
func (s *SQLStore) InsertMANIDecision(ctx context.Context, in maNIDecisionCreate) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var id string
	err := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_ni_decision
  (proposal_id, action, ratified_amount, ratified_treatment, reason, actor_subject, actor_email, auto_reconfirmed_from)
SELECT $1::uuid, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, '')::uuid
WHERE EXISTS (SELECT 1 FROM binocolo.ma_ni_proposal WHERE id = $1::uuid)
RETURNING id::text
`, in.ProposalID, in.Action, nullFloatArg(in.RatifiedAmount), in.RatifiedTreatment,
		in.Reason, in.ActorSubject, in.ActorEmail, in.AutoReconfirmedFrom).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("insert ma ni decision: proposal %s not found", in.ProposalID)
	}
	if err != nil {
		return "", fmt.Errorf("insert ma ni decision: %w", err)
	}
	return id, nil
}

// ListMANIDecisionsByProposals returns every decision for the given proposals, ordered by
// created_at then id (the reducer's tie-break). Empty input ⇒ no query.
func (s *SQLStore) ListMANIDecisionsByProposals(ctx context.Context, proposalIDs []string) ([]maNIDecision, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	if len(proposalIDs) == 0 {
		return []maNIDecision{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+maNIDecisionColumns+`
FROM binocolo.ma_ni_decision
WHERE proposal_id = ANY(string_to_array($1, ',')::uuid[])
ORDER BY created_at, id`, strings.Join(proposalIDs, ","))
	if err != nil {
		return nil, fmt.Errorf("list ma ni decisions by proposals: %w", err)
	}
	defer rows.Close()
	out := []maNIDecision{}
	for rows.Next() {
		d, err := scanMANIDecision(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ma ni decision: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma ni decisions: %w", err)
	}
	return out, nil
}

// maNIProposalWithDecisions is one proposal of the active reading run plus its decisions
// (oldest first). Consumed by F7 (the effective-adjustment reducer) and F8 (the scheda).
type maNIProposalWithDecisions struct {
	Proposal  maNIProposal
	Decisions []maNIDecision
}

// ListMANIProposalsWithDecisions returns, for a filing's ACTIVE reading run, each proposal
// with its decisions ordered by created_at. Empty (not an error) when the filing has no
// active reading run yet.
func (s *SQLStore) ListMANIProposalsWithDecisions(ctx context.Context, filingID string) ([]maNIProposalWithDecisions, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	run, err := s.GetActiveMANIReadingRun(ctx, filingID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return []maNIProposalWithDecisions{}, nil
	}
	proposals, err := s.ListMANIProposalsByRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	proposalIDs := make([]string, 0, len(proposals))
	for _, p := range proposals {
		proposalIDs = append(proposalIDs, p.ID)
	}
	decisions, err := s.ListMANIDecisionsByProposals(ctx, proposalIDs)
	if err != nil {
		return nil, err
	}
	byProposal := make(map[string][]maNIDecision, len(proposals))
	for _, d := range decisions {
		byProposal[d.ProposalID] = append(byProposal[d.ProposalID], d)
	}
	out := make([]maNIProposalWithDecisions, 0, len(proposals))
	for _, p := range proposals {
		out = append(out, maNIProposalWithDecisions{Proposal: p, Decisions: byProposal[p.ID]})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Cross-run auto-reconfirmation.
// ---------------------------------------------------------------------------

// maNIAutoReconfirm is one decision to clone onto a new-run proposal from the analyst's
// standing decision on the same fact (identical fingerprint) in a prior run.
type maNIAutoReconfirm struct {
	NewProposalID        string
	HistoricalDecisionID string
	Action               string
	RatifiedAmount       *float64
	RatifiedTreatment    string
	Reason               string
	ActorSubject         string
	ActorEmail           string
}

// maNINewProposal / maNIPriorProposal are the raw inputs to the pure matcher
// (matchMANIAutoReconfirm), loaded by ListMANIAutoReconfirmCandidates.
type maNINewProposal struct {
	ProposalID  string
	Fingerprint string
}

type maNIPriorProposal struct {
	ProposalID  string
	Fingerprint string
	CreatedAt   time.Time
	Decision    *maNIDecision // the LATEST decision on this prior proposal (nil if none)
}

// ListMANIAutoReconfirmCandidates loads the inputs and computes (in Go, via
// matchMANIAutoReconfirm) which of the new run's still-undecided proposals should inherit a
// standing decision from a prior run of the SAME filing with an identical fingerprint.
func (s *SQLStore) ListMANIAutoReconfirmCandidates(ctx context.Context, filingID, runID string) ([]maNIAutoReconfirm, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	// New-run proposals that have NO decision yet (idempotent: a re-run never re-clones).
	newRows, err := s.db.QueryContext(ctx, `
SELECT p.id::text, p.fingerprint
FROM binocolo.ma_ni_proposal p
WHERE p.run_id = $1::uuid
  AND NOT EXISTS (SELECT 1 FROM binocolo.ma_ni_decision d WHERE d.proposal_id = p.id)
`, runID)
	if err != nil {
		return nil, fmt.Errorf("list ma ni auto-reconfirm new proposals: %w", err)
	}
	defer newRows.Close()
	var newProps []maNINewProposal
	for newRows.Next() {
		var np maNINewProposal
		if err := newRows.Scan(&np.ProposalID, &np.Fingerprint); err != nil {
			return nil, fmt.Errorf("scan ma ni auto-reconfirm new proposal: %w", err)
		}
		newProps = append(newProps, np)
	}
	if err := newRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma ni auto-reconfirm new proposals: %w", err)
	}
	if len(newProps) == 0 {
		return nil, nil
	}

	// Prior-run proposals of the same filing, each with its LATEST decision (LEFT JOIN
	// LATERAL over the append-only decisions).
	priorRows, err := s.db.QueryContext(ctx, `
SELECT p.id::text, p.fingerprint, p.created_at,
       COALESCE(d.id::text, ''), COALESCE(d.action, ''), d.ratified_amount,
       COALESCE(d.ratified_treatment, ''), COALESCE(d.reason, ''),
       COALESCE(d.actor_subject, ''), COALESCE(d.actor_email, ''),
       COALESCE(d.auto_reconfirmed_from::text, '')
FROM binocolo.ma_ni_proposal p
JOIN binocolo.ma_ni_reading_run r ON r.id = p.run_id
LEFT JOIN LATERAL (
  SELECT dd.id, dd.action, dd.ratified_amount, dd.ratified_treatment, dd.reason,
         dd.actor_subject, dd.actor_email, dd.auto_reconfirmed_from
  FROM binocolo.ma_ni_decision dd
  WHERE dd.proposal_id = p.id
  ORDER BY dd.created_at DESC, dd.id DESC
  LIMIT 1
) d ON true
WHERE r.filing_id = $1::uuid AND p.run_id <> $2::uuid
`, filingID, runID)
	if err != nil {
		return nil, fmt.Errorf("list ma ni auto-reconfirm prior proposals: %w", err)
	}
	defer priorRows.Close()
	var prior []maNIPriorProposal
	for priorRows.Next() {
		var pp maNIPriorProposal
		var decID, action, ratifiedTreatment, reason, actorSubject, actorEmail, autoFrom string
		var ratifiedAmount sql.NullFloat64
		if err := priorRows.Scan(&pp.ProposalID, &pp.Fingerprint, &pp.CreatedAt,
			&decID, &action, &ratifiedAmount, &ratifiedTreatment, &reason,
			&actorSubject, &actorEmail, &autoFrom); err != nil {
			return nil, fmt.Errorf("scan ma ni auto-reconfirm prior proposal: %w", err)
		}
		if decID != "" {
			d := maNIDecision{
				ID:                  decID,
				ProposalID:          pp.ProposalID,
				Action:              action,
				RatifiedTreatment:   ratifiedTreatment,
				Reason:              reason,
				ActorSubject:        actorSubject,
				ActorEmail:          actorEmail,
				AutoReconfirmedFrom: autoFrom,
			}
			if ratifiedAmount.Valid {
				v := ratifiedAmount.Float64
				d.RatifiedAmount = &v
			}
			pp.Decision = &d
		}
		prior = append(prior, pp)
	}
	if err := priorRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma ni auto-reconfirm prior proposals: %w", err)
	}
	return matchMANIAutoReconfirm(newProps, prior), nil
}
