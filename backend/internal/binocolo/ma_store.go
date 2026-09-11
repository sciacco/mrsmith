package binocolo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	maCompanyConstraintFKOnly       = false
	maCompanyConstraintManualInsert = true

	updateMAAnnotationQuery = `
UPDATE binocolo.ma_target_outcome
SET note = $2,
    updated_at = now(),
    updated_by_subject = $3,
    updated_by_email = $4
WHERE id = $1::uuid
  AND event = 'nota'
  AND deleted_at IS NULL
RETURNING id::text`
	softDeleteMAAnnotationQuery = `
UPDATE binocolo.ma_target_outcome
SET deleted_at = now(),
    deleted_by_subject = $2,
    deleted_by_email = $3
WHERE id = $1::uuid
  AND event = 'nota'
  AND deleted_at IS NULL
RETURNING id::text`
)

// translateMACompanyConstraintError keeps database backstops aligned with the
// domain errors produced by the application guards. The mode enables the 23505
// translation only for manual target insertion; callers add operation context.
func translateMACompanyConstraintError(err error, duplicateIsAlreadyPresent bool) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	if pgErr.Code == "23503" && strings.HasSuffix(pgErr.ConstraintName, "_company_key_fkey") {
		return fmt.Errorf("%w (%s: %s)", errMACompanyKeyUnknown, pgErr.ConstraintName, pgErr.Detail)
	}
	if duplicateIsAlreadyPresent && pgErr.Code == "23505" && pgErr.ConstraintName == "ma_target_session_company_key_key" {
		return errMATargetAlreadyPresent
	}
	return err
}

type maWorkspaceStore interface {
	ListMASessions(ctx context.Context, visibility string) ([]MASessionSummary, error)
	CreateMASession(ctx context.Context, input maSessionCreate) (MASessionDetail, error)
	GetMASession(ctx context.Context, id string) (MASessionDetail, error)
	GetMASessionLean(ctx context.Context, id string) (MASessionDetail, error)
	GetMASessionState(ctx context.Context, id string) (MASession, error)
	ListMARuns(ctx context.Context, sessionID string) ([]MAExecutionRun, error)
	ListMATargetRows(ctx context.Context, sessionID string) ([]MATargetRow, error)
	GetMATargetByID(ctx context.Context, sessionID, targetID string) (MATarget, error)
	UpdateMASessionLifecycle(ctx context.Context, sessionID, action, subject, email string) (bool, error)
	AddMAStrategyVersion(ctx context.Context, sessionID string, strategy MAStrategySpec, createdByEmail string) (*MAStrategyVersion, error)
	AddMARescoreStrategyVersion(ctx context.Context, sessionID string, strategy MAStrategySpec, createdByEmail string) (*MAStrategyVersion, error)
	GetMAStrategyVersion(ctx context.Context, sessionID, versionID string) (MAStrategyVersion, error)
	ReplaceMAEstimates(ctx context.Context, sessionID, strategyVersionID, selectedStrategy string, estimates []MAEstimate) error
	EnqueueMAJob(ctx context.Context, input maJobEnqueue) (jobID string, created bool, err error)
	HasActiveMACardDomainJob(ctx context.Context, initiativeID, companyKey string) (bool, error)
	LatestMAManualAddJob(ctx context.Context, sessionID string) (*MAManualAddJobProgress, error)
	SetMASessionEstimateStatus(ctx context.Context, sessionID, strategyVersionID, status string) error
	MarkMASessionExecuting(ctx context.Context, sessionID string) error
	MarkMASessionExecuteFailed(ctx context.Context, sessionID string) error
	HasRunningMAExecution(ctx context.Context, sessionID string) (bool, error)
	AbandonMASessionExecution(ctx context.Context, sessionID, errorCode string) error
	CreateMAExecutionRun(ctx context.Context, input maExecutionRunCreate) (MAExecutionRun, error)
	CompleteMAExecutionRun(ctx context.Context, runID, status string, resultCount int, errorCode string) error
	ReplaceMATargets(ctx context.Context, sessionID, runID string, targets []MATarget) error
	InsertMATarget(ctx context.Context, sessionID, runID string, target MATarget) error
	MarkMATargetAdvancedEnriched(ctx context.Context, targetID string, vendorPayload json.RawMessage) error
	UpsertMATargetRating(ctx context.Context, sessionID string, input MATargetRatingRequest, subject, email string) error
	InsertMATargetOutcome(ctx context.Context, outcome MATargetOutcome) error
	UpdateMAAnnotation(ctx context.Context, id, note, subject, email string) error
	SoftDeleteMAAnnotation(ctx context.Context, id, subject, email string) error
	ListMACompanyContacts(ctx context.Context, companyKey string) ([]MACompanyContact, error)
	CreateMACompanyContact(ctx context.Context, companyKey string, input MACompanyContactWrite, subject, email string) (MACompanyContact, error)
	UpdateMACompanyContact(ctx context.Context, companyKey, contactID string, input MACompanyContactWrite, subject, email string) (MACompanyContact, error)
	SoftDeleteMACompanyContact(ctx context.Context, companyKey, contactID, subject, email string) error
	ListMACompanyAgreements(ctx context.Context, companyKey string) ([]MACompanyAgreement, error)
	CreateMACompanyAgreement(ctx context.Context, companyKey string, input MACompanyAgreementWrite, subject, email string) (MACompanyAgreement, error)
	UpdateMACompanyAgreement(ctx context.Context, companyKey, agreementID string, input MACompanyAgreementWrite, subject, email string) (updated MACompanyAgreement, previous MACompanyAgreement, err error)
	SoftDeleteMACompanyAgreement(ctx context.Context, companyKey, agreementID, subject, email string) (MACompanyAgreement, error)
	// Tag aziendali condivisi (migrazione 148, issue #198): catalogo
	// condiviso + associazioni per company_key. Le scritture sono
	// idempotenti o risolte con riuso; vedi ma_tags.go.
	ListMATags(ctx context.Context) ([]MATag, error)
	RenameMATag(ctx context.Context, tagID, name string) (MATag, error)
	DeleteMATag(ctx context.Context, tagID string) error
	CreateAndAssignMATag(ctx context.Context, companyKey, name string) (MATag, error)
	AssignMATag(ctx context.Context, companyKey, tagID string) error
	UnassignMATag(ctx context.Context, companyKey, tagID string) error
	ListMATagsByCompanyKeys(ctx context.Context, companyKeys []string) (map[string][]MATag, error)
	// Google Drive folder bindings (migration 127, issue #98). The orchestration
	// type-asserts to *SQLStore for the lock-spanning transaction; these accessors
	// are the documented binding surface for read/simple-write paths.
	GetMACompanyDriveFolder(ctx context.Context, companyKey string) (string, error)
	InsertMACompanyDriveFolder(ctx context.Context, companyKey, driveFolderID, subject, email string) error
	GetMACardDriveFolder(ctx context.Context, initiativeID, companyKey string) (string, error)
	InsertMACardDriveFolder(ctx context.Context, initiativeID, companyKey, driveFolderID, subject, email string) error
	ListMACompanyActivity(ctx context.Context, companyKey string, includeDeleted bool) (MACompanyActivity, error)
	LatestMASystemDomainAnnotation(ctx context.Context, companyKey string) (string, error)
	GetMACompanyDomain(ctx context.Context, companyKey, vatCode, taxCode string) (*maCompanyDomain, error)
	UpsertMACompanyDomain(ctx context.Context, record maCompanyDomain) error
	UpsertMAWebValidation(ctx context.Context, input maWebValidationUpsert) (MAWebValidation, error)
	UpsertMASectorEvalLabel(ctx context.Context, sessionID, companyKey, label, note, subject, email string) error
	ListMASectorEvalLabels(ctx context.Context, sessionID string) (map[string]MASectorEvalLabel, error)
	StartMATrace(ctx context.Context, input maTraceStart) (string, error)
	LinkMATrace(ctx context.Context, input maTraceLink) error
	CompleteMATrace(ctx context.Context, input maTraceComplete) error
	RecordMATraceEvent(ctx context.Context, input maTraceEventWrite) error
	RecordMAExport(ctx context.Context, sessionID, format string, rowCount int, createdByEmail string) error
	ListMAParameters(ctx context.Context) ([]MAParameter, error)
	UpdateMAParameter(ctx context.Context, key, value, email string) error
	ListMACompanyLegalForms(ctx context.Context) ([]maCompanyLegalForm, error)
	ListMADeepAnalysis(ctx context.Context, companyKeys []string) (map[string]MADeepAnalysis, error)
	GetMATargetDeepDiveIdentity(ctx context.Context, companyKey string) (*maDeepDiveIdentity, error)
	GetMAInitiativeCardDeepDiveIdentity(ctx context.Context, companyKey string) (*maDeepDiveIdentity, error)
	FindMACompanySnapshotByIdentity(ctx context.Context, vatOrTax string) (*maCompanySnapshot, error)
	FindMACompanySnapshotByKey(ctx context.Context, companyKey string) (*maCompanySnapshot, error)
	EnqueueMADeepAnalysis(ctx context.Context, companyKey, vatCode, taxCode, email string) error
	RefreshMADeepAnalysis(ctx context.Context, companyKey, vatCode, taxCode, email string, staleBefore time.Time) (bool, error)
	EnqueueMADeepAnalysisIfAbsent(ctx context.Context, companyKey, vat, tax, email string) (bool, string, error)
	GetMADeepByFiscalIdentity(ctx context.Context, vat, tax string) (*maDeepVATRecord, error)
	ListMADeepReadyPayloads(ctx context.Context) ([]maDeepPayloadRow, error)
	CountMADeepByStatus(ctx context.Context) (map[string]int, error)
	CountMADeepBriefFormats(ctx context.Context) (map[string]int, []string, error)
	CountMADeepVintage(ctx context.Context) (int, int, error)
	UpdateMADeepScorecard(ctx context.Context, companyKey string, scorecard *MADeepScorecard) error
	UpdateMADeepValuation(ctx context.Context, companyKey string, valuation *MADeepValuation) error
	ResolveSectorMultipleKeys(ctx context.Context, keys []string) (*sectorMultiple, error)
	UpsertMABMFamilySuggestion(ctx context.Context, companyKey, vatCode, taxCode, companyName string, suggestion maBMFamilySuggestion) error
	GetMABMFamily(ctx context.Context, companyKey string) (*MABMFamily, error)
	RatifyMABMFamily(ctx context.Context, companyKey, family, subject, email string) error
	GetMASessionThesisReading(ctx context.Context, sessionID, companyKey string) (*MASessionThesisReading, error)
	UpsertMASessionThesisReading(ctx context.Context, reading *MASessionThesisReading, modelID, promptID, subject string) error
	GetMAWebValidationForCompany(ctx context.Context, sessionID, companyKey string) (*MAWebValidation, error)
	ListMACardIRLItems(ctx context.Context, initiativeID, companyKey string) ([]MACardIRLItem, error)
	MaxMACardIRLPosition(ctx context.Context, initiativeID, companyKey string) (int, error)
	InsertMACardIRLSeed(ctx context.Context, items []MACardIRLItem) (int, map[string]int, error)
	InsertMACardIRLItem(ctx context.Context, item MACardIRLItem) (*MACardIRLItem, error)
	UpdateMACardIRLItem(ctx context.Context, initiativeID, companyKey, itemID string, patch MAIRLItemPatch) (*MACardIRLItem, error)
	DeleteMACardIRLItem(ctx context.Context, initiativeID, companyKey, itemID string) (bool, error)
	ReorderMACardIRLItems(ctx context.Context, initiativeID, companyKey string, itemIDs []string) error
	ListMAIRLTemplates(ctx context.Context, family string) ([]MAIRLTemplate, error)
	GetMADeepByVAT(ctx context.Context, vat string) (*maDeepVATRecord, error)
	ListMADeepReadyForBrief(ctx context.Context) ([]maDeepBriefRow, error)
	UpdateMADeepBrief(ctx context.Context, companyKey string, brief *MADeepBrief, modelID, promptID string) error
	CreateMAInitiative(ctx context.Context, initiative MAInitiative) (MAInitiative, error)
	GetMAInitiative(ctx context.Context, id string) (MAInitiative, error)
	UpdateMAInitiativeInfo(ctx context.Context, id string, title, description *string) (MAInitiative, error)
	ListMAInitiatives(ctx context.Context, visibility string) ([]MAInitiativeSummary, error)
	UpdateMAInitiativeLifecycle(ctx context.Context, id, action, subject, email string) (bool, error)
	CountMAInitiativeAttachedSessions(ctx context.Context, initiativeID, excludeSessionID string) (int, error)
	CountMAInitiativeCards(ctx context.Context, initiativeID string) (int, error)
	SetMASessionInitiative(ctx context.Context, sessionID, initiativeID string) error
	GetMAInitiativeCard(ctx context.Context, initiativeID, companyKey string) (*MAInitiativeCard, error)
	UpsertMAInitiativeCard(ctx context.Context, card MAInitiativeCard) error
	// SaveMAInitiativeCardWithOutcome persiste card ed evento di diario in una
	// sola transazione (issue #201): stato e storico non possono divergere.
	SaveMAInitiativeCardWithOutcome(ctx context.Context, card MAInitiativeCard, outcome MATargetOutcome) error
	ListMAInitiativeCards(ctx context.Context, initiativeID string) ([]MAInitiativeCard, error)
	ListMAActiveCardsByCompany(ctx context.Context, companyKeys []string) (map[string][]MAInitiativeCard, error)
	ListMARatings(ctx context.Context, sessionID string) (map[string]int, error)
	InsertMACompanyFact(ctx context.Context, fact MACompanyFact) (MACompanyFact, error)
	RevokeMACompanyFact(ctx context.Context, id, subject, email, note string) (bool, error)
	GetMACompanyRegistry(ctx context.Context, companyKey string) (MACompanyRegistry, error)
	SearchMACompanies(ctx context.Context, options maCompanySearchOptions) (MACompanySearchResponse, error)
	ListMACompanySearchAreas(ctx context.Context) (MACompanySearchAreas, error)
	GetMACompanyOverviewIdentity(ctx context.Context, companyKey string) (*MACompanyOverviewIdentity, error)
	ListMACompanyOverviewAppearances(ctx context.Context, companyKey string) ([]MACompanyOverviewAppearance, error)
	ListMACompanyOverviewCards(ctx context.Context, companyKey string) ([]MACompanyOverviewCard, error)
	ListMACompanyFactsActive(ctx context.Context, companyKeys []string) (map[string][]string, error)
	ListMASessionsByInitiative(ctx context.Context, initiativeID string) ([]MASessionSummary, error)
	ListMACardProvenances(ctx context.Context, initiativeID string, companyKeys []string) (map[string][]MACardProvenance, error)
	ListMALatestCardEvents(ctx context.Context, initiativeID string, companyKeys []string) (map[string]string, error)
	FindMALatestTargetForCard(ctx context.Context, initiativeID, companyKey string) (sessionID, targetID string, err error)
	ResolveMACompany(ctx context.Context, observation maCompanyObservation) (string, error)
	RecordMACompanyIdentityConflicts(ctx context.Context, conflicts []maCompanyIdentityConflict) error
	MACompanyExists(ctx context.Context, companyKey string) (bool, error)
}

type maCompanyLegalForm struct {
	Code          string
	DescriptionIT string
	DescriptionEN string
}

type maDeepDiveIdentity struct {
	VATCode string
	TaxCode string
}

type maCompanySnapshot struct {
	CompanyKey  string
	CompanyName string
	VATCode     string
	TaxCode     string
	Province    string
}

type maCompanySearchHydration struct {
	ActiveCardState      string
	ActiveCardInitiative string
	ClosedCardState      string
	ClosedCardEsito      string
	ClosedCardInitiative string
	LatestRating         *int
	LatestRatingReason   string
	LatestTarget         *MATargetRow
	LatestTargetThesis   string
}

// maCompanyDomain is one row of the cross-session verified-domain registry
// (binocolo.ma_company_domain, mig 082): the durable company→official-domain
// association written on strong identity verification (on-page P.IVA/CF) or
// manual operator association, consulted by resolution before any retrieval.
type maCompanyDomain struct {
	CompanyKey       string
	VATCode          string
	TaxCode          string
	CompanyName      string
	Domain           string
	Method           string // maDomainMethodAutoVerified | maDomainMethodManual
	GroupSite        bool
	IdentityState    string
	CreatedBySubject string
	CreatedByEmail   string
}

type maWebValidationUpsert struct {
	SessionID              string
	CompanyKey             string
	TargetID               string
	RunID                  string
	PipelineVersion        string
	InputHash              string
	KeywordSetHash         string
	LLMModelID             string
	LLMPromptID            string
	LLMModel               string
	StaleAfter             time.Time
	ExpiresAt              time.Time
	IdentityState          string
	SelectedDomain         string
	DomainConfidence       string
	DomainScore            *int
	WebScore               int
	WebConfidence          string
	WebValidationState     string
	FinalAction            string
	AnalystVerdict         string
	AnalystAction          string
	AnalystConfidence      string
	Summary                json.RawMessage
	KeywordSet             json.RawMessage
	SelectedDomainPayload  json.RawMessage
	DomainResponse         json.RawMessage
	EvidenceRuns           json.RawMessage
	CandidateMatchAnalysis json.RawMessage
	CandidateMatchError    string
	FinalDecision          json.RawMessage
	Subject                string
	Email                  string
}

func (s *SQLStore) ListMASessions(ctx context.Context, visibility string) ([]MASessionSummary, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	where, orderBy := maSessionListClauses(visibility)
	rows, err := s.db.QueryContext(ctx, `
SELECT
  session.id::text,
  session.title,
  session.prompt,
  session.status,
  session.selected_strategy,
  COALESCE((SELECT SUM(estimated_count) FROM binocolo.ma_dry_run_estimate estimate WHERE estimate.session_id = session.id AND estimate.selected), 0) AS estimated_count,
  COALESCE((SELECT SUM(estimated_cost) FROM binocolo.ma_dry_run_estimate estimate WHERE estimate.session_id = session.id AND estimate.selected), 0) AS estimated_cost,
  COALESCE((SELECT COUNT(*) FROM binocolo.ma_target target WHERE target.session_id = session.id), 0) AS result_count,
  session.created_at,
  session.updated_at,
  session.last_executed_at,
  session.archived_at,
  session.archived_by_email,
  session.deleted_at,
  session.deleted_by_email,
  session.initiative_id::text,
  initiative.title
FROM binocolo.ma_session session
LEFT JOIN binocolo.ma_initiative initiative ON initiative.id = session.initiative_id
WHERE `+where+`
ORDER BY `+orderBy+`
LIMIT 80
`)
	if err != nil {
		return nil, fmt.Errorf("list ma sessions: %w", err)
	}
	defer rows.Close()

	out := []MASessionSummary{}
	for rows.Next() {
		var item MASessionSummary
		var selected sql.NullString
		var lastRun sql.NullTime
		var archivedAt sql.NullTime
		var archivedByEmail sql.NullString
		var deletedAt sql.NullTime
		var deletedByEmail sql.NullString
		var initiativeID sql.NullString
		var initiativeTitle sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Prompt,
			&item.Status,
			&selected,
			&item.EstimatedCount,
			&item.EstimatedCost,
			&item.ResultCount,
			&item.CreatedAt,
			&item.UpdatedAt,
			&lastRun,
			&archivedAt,
			&archivedByEmail,
			&deletedAt,
			&deletedByEmail,
			&initiativeID,
			&initiativeTitle,
		); err != nil {
			return nil, fmt.Errorf("scan ma session summary: %w", err)
		}
		item.SelectedStrategy = selected.String
		item.InitiativeID = initiativeID.String
		item.InitiativeTitle = initiativeTitle.String
		if lastRun.Valid {
			item.LastRunAt = &lastRun.Time
		}
		if archivedAt.Valid {
			item.ArchivedAt = &archivedAt.Time
		}
		item.ArchivedByEmail = archivedByEmail.String
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		item.DeletedByEmail = deletedByEmail.String
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma sessions: %w", err)
	}
	return out, nil
}

// CreateMAInitiative inserts a new Iniziativa (mig 087). id is generated by
// the service, not the DB, so callers get it back synchronously without a
// RETURNING round-trip mismatch.
func (s *SQLStore) CreateMAInitiative(ctx context.Context, initiative MAInitiative) (MAInitiative, error) {
	if s == nil || s.db == nil {
		return MAInitiative{}, errors.New("binocolo ma store not configured")
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_initiative (id, title, description, created_by_subject, created_by_email)
VALUES ($1::uuid, $2, $3, $4, $5)
RETURNING id::text, title, description, created_by_subject, created_by_email, created_at, updated_at
`, initiative.ID, initiative.Title, initiative.Description, initiative.CreatedBySubject, initiative.CreatedByEmail)
	var out MAInitiative
	if err := row.Scan(&out.ID, &out.Title, &out.Description, &out.CreatedBySubject, &out.CreatedByEmail, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return MAInitiative{}, fmt.Errorf("create ma initiative: %w", err)
	}
	return out, nil
}

// GetMAInitiative loads a single Iniziativa by id.
func (s *SQLStore) GetMAInitiative(ctx context.Context, id string) (MAInitiative, error) {
	if s == nil || s.db == nil {
		return MAInitiative{}, errors.New("binocolo ma store not configured")
	}
	var out MAInitiative
	var archivedAt sql.NullTime
	var archivedBySubject sql.NullString
	var archivedByEmail sql.NullString
	var deletedAt sql.NullTime
	var deletedBySubject sql.NullString
	var deletedByEmail sql.NullString
	var purgedAt sql.NullTime
	var purgedBySubject sql.NullString
	var purgedByEmail sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT id::text, title, description, created_by_subject, created_by_email, created_at, updated_at,
       archived_at, archived_by_subject, archived_by_email,
       deleted_at, deleted_by_subject, deleted_by_email,
       purged_at, purged_by_subject, purged_by_email
FROM binocolo.ma_initiative
WHERE id = $1::uuid
`, id).Scan(
		&out.ID, &out.Title, &out.Description, &out.CreatedBySubject, &out.CreatedByEmail,
		&out.CreatedAt, &out.UpdatedAt, &archivedAt, &archivedBySubject, &archivedByEmail,
		&deletedAt, &deletedBySubject, &deletedByEmail, &purgedAt, &purgedBySubject, &purgedByEmail,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MAInitiative{}, errMAInitiativeNotFound
	}
	if err != nil {
		return MAInitiative{}, fmt.Errorf("get ma initiative: %w", err)
	}
	if archivedAt.Valid {
		out.ArchivedAt = &archivedAt.Time
	}
	out.ArchivedBySubject = archivedBySubject.String
	out.ArchivedByEmail = archivedByEmail.String
	if deletedAt.Valid {
		out.DeletedAt = &deletedAt.Time
	}
	out.DeletedBySubject = deletedBySubject.String
	out.DeletedByEmail = deletedByEmail.String
	if purgedAt.Valid {
		out.PurgedAt = &purgedAt.Time
	}
	out.PurgedBySubject = purgedBySubject.String
	out.PurgedByEmail = purgedByEmail.String
	return out, nil
}

// UpdateMAInitiativeInfo patches title/description and leaves lifecycle fields untouched.
func (s *SQLStore) UpdateMAInitiativeInfo(ctx context.Context, id string, title, description *string) (MAInitiative, error) {
	if s == nil || s.db == nil {
		return MAInitiative{}, errors.New("binocolo ma store not configured")
	}
	var out MAInitiative
	var archivedAt sql.NullTime
	var archivedBySubject sql.NullString
	var archivedByEmail sql.NullString
	var deletedAt sql.NullTime
	var deletedBySubject sql.NullString
	var deletedByEmail sql.NullString
	var purgedAt sql.NullTime
	var purgedBySubject sql.NullString
	var purgedByEmail sql.NullString
	err := s.db.QueryRowContext(ctx, `
UPDATE binocolo.ma_initiative
SET title = COALESCE($2::text, title),
    description = COALESCE($3::text, description),
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text, title, description, created_by_subject, created_by_email, created_at, updated_at,
       archived_at, archived_by_subject, archived_by_email,
       deleted_at, deleted_by_subject, deleted_by_email,
       purged_at, purged_by_subject, purged_by_email
`, id, nullStringPtr(title), nullStringPtr(description)).Scan(
		&out.ID, &out.Title, &out.Description, &out.CreatedBySubject, &out.CreatedByEmail,
		&out.CreatedAt, &out.UpdatedAt, &archivedAt, &archivedBySubject, &archivedByEmail,
		&deletedAt, &deletedBySubject, &deletedByEmail, &purgedAt, &purgedBySubject, &purgedByEmail,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MAInitiative{}, errMAInitiativeNotFound
	}
	if err != nil {
		return MAInitiative{}, fmt.Errorf("update ma initiative info: %w", err)
	}
	if archivedAt.Valid {
		out.ArchivedAt = &archivedAt.Time
	}
	out.ArchivedBySubject = archivedBySubject.String
	out.ArchivedByEmail = archivedByEmail.String
	if deletedAt.Valid {
		out.DeletedAt = &deletedAt.Time
	}
	out.DeletedBySubject = deletedBySubject.String
	out.DeletedByEmail = deletedByEmail.String
	if purgedAt.Valid {
		out.PurgedAt = &purgedAt.Time
	}
	out.PurgedBySubject = purgedBySubject.String
	out.PurgedByEmail = purgedByEmail.String
	return out, nil
}

// ListMAInitiatives returns the index rows (wireframe S1) for the requested
// visibility. Purged tombstones are hidden from every visibility.
func (s *SQLStore) ListMAInitiatives(ctx context.Context, visibility string) ([]MAInitiativeSummary, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	where, orderBy := maInitiativeListClauses(visibility)
	rows, err := s.db.QueryContext(ctx, `
SELECT
  initiative.id::text,
  initiative.title,
  initiative.description,
  initiative.created_by_subject,
  initiative.created_by_email,
  initiative.created_at,
  initiative.updated_at,
  initiative.archived_at,
  initiative.archived_by_subject,
  initiative.archived_by_email,
  initiative.deleted_at,
  initiative.deleted_by_subject,
  initiative.deleted_by_email,
  initiative.purged_at,
  initiative.purged_by_subject,
  initiative.purged_by_email,
  COALESCE((SELECT COUNT(*) FROM binocolo.ma_session session WHERE session.initiative_id = initiative.id), 0) AS session_count,
  (SELECT MAX(session.updated_at) FROM binocolo.ma_session session WHERE session.initiative_id = initiative.id) AS last_activity_at,
  (SELECT
    CASE
      WHEN o.event = 'nota' AND COALESCE(o.note, '') != '' THEN 'Annotazione: ' || o.note
      WHEN o.event = 'nota' THEN 'Annotazione'
      WHEN o.event = 'stato' AND COALESCE(o.note, '') != '' THEN 'Stato aggiornato: ' || o.note
      WHEN o.event = 'stato' THEN 'Stato aggiornato'
      WHEN o.event = 'card_creata' AND COALESCE(o.note, '') != '' THEN 'Card creata: ' || o.note
      WHEN o.event = 'card_creata' THEN 'Card creata'
      WHEN o.event = 'chiusura' AND COALESCE(o.note, '') != '' THEN 'Chiusura: ' || o.note
      WHEN o.event = 'chiusura' THEN 'Chiusura'
      WHEN o.event = 'contattato' AND COALESCE(o.note, '') != '' THEN 'Contattata: ' || o.note
      WHEN o.event = 'contattato' THEN 'Contattata'
      WHEN o.event = 'card_rimossa' THEN 'Card rimossa'
      WHEN o.event = 'card_riaperta' THEN 'Card riaperta'
      WHEN o.event = 'dominio_verificato' AND o.payload->>'esito' = 'confermato' THEN 'Dominio confermato'
      WHEN o.event = 'dominio_verificato' AND o.payload->>'esito' = 'non_confermato' THEN 'Dominio non confermato'
      WHEN o.event = 'dominio_verificato' THEN 'Dominio non verificabile'
      WHEN o.event = 'buon_lead' AND COALESCE(o.note, '') != '' THEN 'Buon lead: ' || o.note
      WHEN o.event = 'buon_lead' THEN 'Buon lead'
      WHEN o.event = 'no_go' AND COALESCE(o.note, '') != '' THEN 'No-go: ' || o.note
      WHEN o.event = 'no_go' THEN 'No-go'
      ELSE COALESCE(o.event, '') || CASE WHEN COALESCE(o.note, '') != '' THEN ': ' || o.note ELSE '' END
    END
    FROM binocolo.ma_target_outcome o
    WHERE o.initiative_id = initiative.id
      AND o.deleted_at IS NULL
    ORDER BY o.created_at DESC
    LIMIT 1
  ) AS last_activity_event,
  COALESCE(
    (SELECT jsonb_object_agg(c.state, c.cnt)
     FROM (SELECT state, COUNT(*) AS cnt
           FROM binocolo.ma_initiative_card
           WHERE initiative_id = initiative.id
           GROUP BY state) c),
    '{}'::jsonb
  ) AS counts
FROM binocolo.ma_initiative initiative
WHERE `+where+`
ORDER BY `+orderBy+`
LIMIT 200
`)
	if err != nil {
		return nil, fmt.Errorf("list ma initiatives: %w", err)
	}
	defer rows.Close()

	out := []MAInitiativeSummary{}
	for rows.Next() {
		var item MAInitiativeSummary
		var archivedAt sql.NullTime
		var archivedBySubject sql.NullString
		var archivedByEmail sql.NullString
		var deletedAt sql.NullTime
		var deletedBySubject sql.NullString
		var deletedByEmail sql.NullString
		var purgedAt sql.NullTime
		var purgedBySubject sql.NullString
		var purgedByEmail sql.NullString
		var lastActivityAt sql.NullTime
		var lastActivityEvent sql.NullString
		var countsJSON json.RawMessage
		if err := rows.Scan(
			&item.ID, &item.Title, &item.Description, &item.CreatedBySubject, &item.CreatedByEmail,
			&item.CreatedAt, &item.UpdatedAt, &archivedAt, &archivedBySubject, &archivedByEmail,
			&deletedAt, &deletedBySubject, &deletedByEmail, &purgedAt, &purgedBySubject, &purgedByEmail,
			&item.SessionCount, &lastActivityAt, &lastActivityEvent, &countsJSON,
		); err != nil {
			return nil, fmt.Errorf("scan ma initiative summary: %w", err)
		}
		if archivedAt.Valid {
			item.ArchivedAt = &archivedAt.Time
		}
		item.ArchivedBySubject = archivedBySubject.String
		item.ArchivedByEmail = archivedByEmail.String
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		item.DeletedBySubject = deletedBySubject.String
		item.DeletedByEmail = deletedByEmail.String
		if purgedAt.Valid {
			item.PurgedAt = &purgedAt.Time
		}
		item.PurgedBySubject = purgedBySubject.String
		item.PurgedByEmail = purgedByEmail.String
		if lastActivityAt.Valid {
			item.LastActivityAt = &lastActivityAt.Time
		}
		if lastActivityEvent.Valid {
			item.LastActivityEvent = lastActivityEvent.String
		}
		item.Counts = map[string]int{}
		if len(countsJSON) > 0 {
			if err := json.Unmarshal(countsJSON, &item.Counts); err != nil {
				return nil, fmt.Errorf("unmarshal initiative counts: %w", err)
			}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma initiatives: %w", err)
	}
	return out, nil
}

func (s *SQLStore) CountMAInitiativeAttachedSessions(ctx context.Context, initiativeID, excludeSessionID string) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("binocolo ma store not configured")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM binocolo.ma_session
WHERE initiative_id = $1::uuid
  AND id <> $2::uuid
  AND purged_at IS NULL
`, initiativeID, excludeSessionID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count ma initiative attached sessions: %w", err)
	}
	return count, nil
}

func (s *SQLStore) CountMAInitiativeCards(ctx context.Context, initiativeID string) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("binocolo ma store not configured")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM binocolo.ma_initiative_card
WHERE initiative_id = $1::uuid
`, initiativeID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count ma initiative cards: %w", err)
	}
	return count, nil
}

func (s *SQLStore) UpdateMAInitiativeLifecycle(ctx context.Context, id, action, subject, email string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	var query string
	switch action {
	case maSessionLifecycleArchive:
		query = `
UPDATE binocolo.ma_initiative
SET archived_at = COALESCE(archived_at, now()),
    archived_by_subject = CASE WHEN archived_at IS NULL THEN $2 ELSE archived_by_subject END,
    archived_by_email = CASE WHEN archived_at IS NULL THEN $3 ELSE archived_by_email END,
    deleted_at = NULL,
    deleted_by_subject = NULL,
    deleted_by_email = NULL,
    updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NULL
  AND purged_at IS NULL
RETURNING id::text
`
	case maSessionLifecycleRestore:
		query = `
UPDATE binocolo.ma_initiative
SET archived_at = NULL,
    archived_by_subject = NULL,
    archived_by_email = NULL,
    deleted_at = NULL,
    deleted_by_subject = NULL,
    deleted_by_email = NULL,
    updated_at = now()
WHERE id = $1::uuid
  AND purged_at IS NULL
  AND (archived_at IS NOT NULL OR deleted_at IS NOT NULL)
RETURNING id::text
`
	case maSessionLifecycleDelete:
		query = `
UPDATE binocolo.ma_initiative
SET deleted_at = COALESCE(deleted_at, now()),
    deleted_by_subject = CASE WHEN deleted_at IS NULL THEN $2 ELSE deleted_by_subject END,
    deleted_by_email = CASE WHEN deleted_at IS NULL THEN $3 ELSE deleted_by_email END,
    archived_at = NULL,
    archived_by_subject = NULL,
    archived_by_email = NULL,
    updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NULL
  AND purged_at IS NULL
RETURNING id::text
`
	case maSessionLifecyclePurge:
		query = `
UPDATE binocolo.ma_initiative
SET purged_at = COALESCE(purged_at, now()),
    purged_by_subject = CASE WHEN purged_at IS NULL THEN $2 ELSE purged_by_subject END,
    purged_by_email = CASE WHEN purged_at IS NULL THEN $3 ELSE purged_by_email END,
    updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NOT NULL
  AND purged_at IS NULL
RETURNING id::text
`
	default:
		return false, fmt.Errorf("unsupported ma initiative lifecycle action: %s", action)
	}
	args := []any{id, subject, email}
	if action == maSessionLifecycleRestore {
		args = []any{id}
	}
	var returnedID string
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&returnedID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("update ma initiative lifecycle: %w", err)
	}
	return true, nil
}

// SetMASessionInitiative moves/anchors a session to an Iniziativa (PRD §3.2).
// Empty initiativeID is retained only for nullable DB transition support and
// must not be exposed by the public API.
func (s *SQLStore) SetMASessionInitiative(ctx context.Context, sessionID, initiativeID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	var res sql.Result
	var err error
	if initiativeID == "" {
		res, err = s.db.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET initiative_id = NULL, updated_at = now()
WHERE id = $1::uuid
`, sessionID)
	} else {
		res, err = s.db.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET initiative_id = $2::uuid, updated_at = now()
WHERE id = $1::uuid
`, sessionID, initiativeID)
	}
	if err != nil {
		return fmt.Errorf("set ma session initiative: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set ma session initiative rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func maSessionListClauses(visibility string) (string, string) {
	switch visibility {
	case maSessionVisibilityArchived:
		return "session.archived_at IS NOT NULL AND session.deleted_at IS NULL", "session.archived_at DESC, session.updated_at DESC"
	case maSessionVisibilityDeleted:
		return "session.deleted_at IS NOT NULL AND session.purged_at IS NULL", "session.deleted_at DESC, session.updated_at DESC"
	default:
		return "session.archived_at IS NULL AND session.deleted_at IS NULL", "session.updated_at DESC, session.created_at DESC"
	}
}

func maInitiativeListClauses(visibility string) (string, string) {
	switch visibility {
	case maSessionVisibilityArchived:
		return "initiative.archived_at IS NOT NULL AND initiative.deleted_at IS NULL AND initiative.purged_at IS NULL", "initiative.archived_at DESC, initiative.updated_at DESC"
	case maSessionVisibilityDeleted:
		return "initiative.deleted_at IS NOT NULL AND initiative.purged_at IS NULL", "initiative.deleted_at DESC, initiative.updated_at DESC"
	default:
		return "initiative.archived_at IS NULL AND initiative.deleted_at IS NULL AND initiative.purged_at IS NULL", "initiative.updated_at DESC, initiative.created_at DESC"
	}
}

func (s *SQLStore) ListMACompanyLegalForms(ctx context.Context) ([]maCompanyLegalForm, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT code, description_it, description_en
FROM binocolo.company_legal_forms
ORDER BY sort_order
`)
	if err != nil {
		return nil, fmt.Errorf("list ma company legal forms: %w", err)
	}
	defer rows.Close()

	out := []maCompanyLegalForm{}
	for rows.Next() {
		var item maCompanyLegalForm
		if err := rows.Scan(&item.Code, &item.DescriptionIT, &item.DescriptionEN); err != nil {
			return nil, fmt.Errorf("scan ma company legal form: %w", err)
		}
		item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
		item.DescriptionIT = cleanText(item.DescriptionIT, 180)
		item.DescriptionEN = cleanText(item.DescriptionEN, 180)
		if item.Code == "" {
			continue
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma company legal forms: %w", err)
	}
	return out, nil
}

func (s *SQLStore) CreateMASession(ctx context.Context, input maSessionCreate) (MASessionDetail, error) {
	if s == nil || s.db == nil {
		return MASessionDetail{}, errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("begin ma session create: %w", err)
	}
	defer tx.Rollback()

	sessionID := input.Session.ID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	strategyID := uuid.NewString()
	rawStrategy, err := strategyToRaw(input.Strategy)
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("marshal ma strategy: %w", err)
	}
	initiativeID := strings.TrimSpace(input.InitiativeID)
	if initiativeID == "" {
		initiativeID = strings.TrimSpace(input.Session.InitiativeID)
	}

	var session MASession
	var estimated sql.NullTime
	var executed sql.NullTime
	var createdInitiativeID sql.NullString
	if err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_session (
  id,
  title,
  prompt,
  status,
  created_by_subject,
  created_by_email,
  initiative_id
) VALUES (
  $1::uuid,
  $2,
  $3,
  $4,
  $5,
  $6,
  NULLIF($7, '')::uuid
)
RETURNING id::text, title, prompt, status, COALESCE(selected_strategy, ''),
          COALESCE(created_by_subject, ''), COALESCE(created_by_email, ''), last_estimated_at,
          last_executed_at, created_at, updated_at, initiative_id::text
`, sessionID,
		input.Session.Title,
		input.Session.Prompt,
		maSessionStatusDraft,
		nullString(input.Session.CreatedBySubject),
		nullString(input.Session.CreatedByEmail),
		initiativeID,
	).Scan(
		&session.ID,
		&session.Title,
		&session.Prompt,
		&session.Status,
		&session.SelectedStrategy,
		&session.CreatedBySubject,
		&session.CreatedByEmail,
		&estimated,
		&executed,
		&session.CreatedAt,
		&session.UpdatedAt,
		&createdInitiativeID,
	); err != nil {
		return MASessionDetail{}, fmt.Errorf("insert ma session: %w", err)
	}
	if estimated.Valid {
		session.LastEstimatedAt = &estimated.Time
	}
	if executed.Valid {
		session.LastExecutedAt = &executed.Time
	}
	session.InitiativeID = createdInitiativeID.String

	strategy, err := insertMAStrategyVersion(ctx, tx, session.ID, strategyID, 1, rawStrategy, input.Session.CreatedByEmail)
	if err != nil {
		return MASessionDetail{}, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET active_strategy_id = $2::uuid
WHERE id = $1::uuid
`, session.ID, strategyID); err != nil {
		return MASessionDetail{}, fmt.Errorf("activate initial ma strategy: %w", err)
	}
	session.ActiveStrategyID = strategyID
	if err := tx.Commit(); err != nil {
		return MASessionDetail{}, fmt.Errorf("commit ma session create: %w", err)
	}
	return MASessionDetail{Session: session, Strategy: &strategy, Estimates: []MAEstimate{}, Runs: []MAExecutionRun{}, Targets: []MATarget{}}, nil
}

func (s *SQLStore) GetMASession(ctx context.Context, id string) (MASessionDetail, error) {
	if s == nil || s.db == nil {
		return MASessionDetail{}, errors.New("binocolo ma store not configured")
	}
	session, err := s.loadMASession(ctx, s.db, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	strategy, err := s.loadActiveMAStrategy(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return MASessionDetail{}, err
	}
	estimates, err := s.loadMAEstimates(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	runs, err := s.loadMARuns(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	targets, err := s.loadMATargets(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	detail := MASessionDetail{Session: session, Estimates: estimates, Runs: runs, Targets: targets}
	if strategy.ID != "" {
		detail.Strategy = &strategy
	}
	return detail, nil
}

func (s *SQLStore) GetMASessionLean(ctx context.Context, id string) (MASessionDetail, error) {
	if s == nil || s.db == nil {
		return MASessionDetail{}, errors.New("binocolo ma store not configured")
	}
	session, err := s.loadMASession(ctx, s.db, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	strategy, err := s.loadActiveMAStrategy(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return MASessionDetail{}, err
	}
	estimates, err := s.loadMAEstimates(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	runs, err := s.loadMARuns(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	detail := MASessionDetail{Session: session, Estimates: estimates, Runs: runs, Targets: []MATarget{}}
	if strategy.ID != "" {
		detail.Strategy = &strategy
	}
	return detail, nil
}

// GetMASessionState loads only the session row (no strategy/estimates/runs/targets),
// for cheap lifecycle checks such as the rating endpoint.
func (s *SQLStore) GetMASessionState(ctx context.Context, id string) (MASession, error) {
	if s == nil || s.db == nil {
		return MASession{}, errors.New("binocolo ma store not configured")
	}
	return s.loadMASession(ctx, s.db, id)
}

func (s *SQLStore) ListMARuns(ctx context.Context, sessionID string) ([]MAExecutionRun, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	return s.loadMARuns(ctx, sessionID)
}

func (s *SQLStore) UpdateMASessionLifecycle(ctx context.Context, sessionID, action, subject, email string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	var query string
	switch action {
	case maSessionLifecycleArchive:
		query = `
UPDATE binocolo.ma_session
SET archived_at = COALESCE(archived_at, now()),
    archived_by_subject = CASE WHEN archived_at IS NULL THEN $2 ELSE archived_by_subject END,
    archived_by_email = CASE WHEN archived_at IS NULL THEN $3 ELSE archived_by_email END,
    deleted_at = NULL,
    deleted_by_subject = NULL,
    deleted_by_email = NULL,
    updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NULL
RETURNING id::text
`
	case maSessionLifecycleRestore:
		query = `
UPDATE binocolo.ma_session
SET archived_at = NULL,
    archived_by_subject = NULL,
    archived_by_email = NULL,
    deleted_at = NULL,
    deleted_by_subject = NULL,
    deleted_by_email = NULL,
    updated_at = now()
WHERE id = $1::uuid
  AND (archived_at IS NOT NULL OR deleted_at IS NOT NULL)
RETURNING id::text
`
	case maSessionLifecycleDelete:
		query = `
UPDATE binocolo.ma_session
SET deleted_at = COALESCE(deleted_at, now()),
    deleted_by_subject = CASE WHEN deleted_at IS NULL THEN $2 ELSE deleted_by_subject END,
    deleted_by_email = CASE WHEN deleted_at IS NULL THEN $3 ELSE deleted_by_email END,
    archived_at = NULL,
    archived_by_subject = NULL,
    archived_by_email = NULL,
    updated_at = now()
WHERE id = $1::uuid
RETURNING id::text
`
	case maSessionLifecyclePurge:
		query = `
UPDATE binocolo.ma_session
SET purged_at = COALESCE(purged_at, now()),
    purged_by_subject = CASE WHEN purged_at IS NULL THEN $2 ELSE purged_by_subject END,
    purged_by_email = CASE WHEN purged_at IS NULL THEN $3 ELSE purged_by_email END,
    updated_at = now()
WHERE id = $1::uuid
  AND deleted_at IS NOT NULL
RETURNING id::text
`
	default:
		return false, fmt.Errorf("invalid ma session lifecycle action: %s", action)
	}
	var id string
	args := []any{sessionID}
	actionsWithSubject := map[string]bool{
		maSessionLifecycleArchive: true,
		maSessionLifecycleDelete:  true,
		maSessionLifecyclePurge:   true,
	}
	if actionsWithSubject[action] {
		args = append(args, nullString(subject), nullString(email))
	}
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("update ma session lifecycle: %w", err)
	}
	return true, nil
}

func (s *SQLStore) AddMAStrategyVersion(ctx context.Context, sessionID string, strategySpec MAStrategySpec, createdByEmail string) (*MAStrategyVersion, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin ma strategy version: %w", err)
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(MAX(version), 0) + 1
FROM binocolo.ma_strategy_version
WHERE session_id = $1::uuid
`, sessionID).Scan(&version); err != nil {
		return nil, fmt.Errorf("next ma strategy version: %w", err)
	}
	strategyID := uuid.NewString()
	rawStrategy, err := strategyToRaw(strategySpec)
	if err != nil {
		return nil, fmt.Errorf("marshal ma strategy: %w", err)
	}
	strategy, err := insertMAStrategyVersion(ctx, tx, sessionID, strategyID, version, rawStrategy, createdByEmail)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM binocolo.ma_dry_run_estimate
WHERE session_id = $1::uuid
`, sessionID); err != nil {
		return nil, fmt.Errorf("clear ma estimates for new strategy: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET active_strategy_id = $2::uuid,
    selected_strategy = NULL,
    status = $3,
    updated_at = now()
WHERE id = $1::uuid
`, sessionID, strategyID, maSessionStatusDraft); err != nil {
		return nil, fmt.Errorf("activate ma strategy version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit ma strategy version: %w", err)
	}
	return &strategy, nil
}

// AddMARescoreStrategyVersion registra una nuova versione di strategia per un
// RESCORE (override di tesi) e la attiva, PRESERVANDO stato sessione, stime e
// strategia selezionata: la tesi muove solo i pesi dello scoring, non la
// superficie di ricerca — azzerare le stime e tornare a draft (come fa
// AddMAStrategyVersion per estimate/execute) butterebbe via stato ancora valido
// e nasconderebbe i risultati appena ri-scorati.
func (s *SQLStore) AddMARescoreStrategyVersion(ctx context.Context, sessionID string, strategySpec MAStrategySpec, createdByEmail string) (*MAStrategyVersion, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin ma rescore strategy version: %w", err)
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(MAX(version), 0) + 1
FROM binocolo.ma_strategy_version
WHERE session_id = $1::uuid
`, sessionID).Scan(&version); err != nil {
		return nil, fmt.Errorf("next ma strategy version: %w", err)
	}
	strategyID := uuid.NewString()
	rawStrategy, err := strategyToRaw(strategySpec)
	if err != nil {
		return nil, fmt.Errorf("marshal ma strategy: %w", err)
	}
	strategy, err := insertMAStrategyVersion(ctx, tx, sessionID, strategyID, version, rawStrategy, createdByEmail)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET active_strategy_id = $2::uuid,
    updated_at = now()
WHERE id = $1::uuid
`, sessionID, strategyID); err != nil {
		return nil, fmt.Errorf("activate ma rescore strategy version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit ma rescore strategy version: %w", err)
	}
	return &strategy, nil
}

func (s *SQLStore) ReplaceMAEstimates(ctx context.Context, sessionID, strategyVersionID, selectedStrategy string, estimates []MAEstimate) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma estimate replace: %w", err)
	}
	defer tx.Rollback()

	// Lock the session row and verify the estimate still targets the active strategy
	// version. If a newer version was activated mid-estimate, abort the whole write
	// (do not clobber the current estimates) and report it superseded so the worker
	// re-runs for the now-active version. FOR UPDATE serializes against the
	// AddMAStrategyVersion that would flip active_strategy_id.
	var activeStrategyID sql.NullString
	if err := tx.QueryRowContext(ctx, `
SELECT active_strategy_id::text
FROM binocolo.ma_session
WHERE id = $1::uuid
FOR UPDATE
`, sessionID).Scan(&activeStrategyID); err != nil {
		return fmt.Errorf("lock ma session for estimate: %w", err)
	}
	if !activeStrategyID.Valid || activeStrategyID.String != strategyVersionID {
		return errMAEstimateSuperseded
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM binocolo.ma_dry_run_estimate
WHERE session_id = $1::uuid
`, sessionID); err != nil {
		return fmt.Errorf("delete ma estimates: %w", err)
	}
	for _, estimate := range estimates {
		id := estimate.ID
		if id == "" {
			id = uuid.NewString()
		}
		params := json.RawMessage(`{}`)
		if len(estimate.Params) > 0 {
			params = estimate.Params
		}
		response := json.RawMessage(`{}`)
		if len(estimate.VendorResponse) > 0 {
			response = estimate.VendorResponse
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_dry_run_estimate (
  id,
  session_id,
  strategy_version_id,
  strategy_type,
  ateco_code,
  ateco_description,
  province,
	  estimated_count,
	  estimated_cost,
	  selected,
	  surface_status,
	  execution_limit,
	  probe_count,
	  params,
	  vendor_response
	) VALUES (
	  $1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15::jsonb
	)
`, id,
			sessionID,
			strategyVersionID,
			estimate.StrategyType,
			nullString(estimate.AtecoCode),
			nullString(estimate.AtecoDescription),
			nullString(estimate.Province),
			estimate.EstimatedCount,
			estimate.EstimatedCost,
			estimate.Selected,
			defaultString(estimate.SurfaceStatus, maEstimateSurfaceExact),
			positiveOrDefault(estimate.ExecutionLimit, maDefaultSearchLimit),
			positiveOrDefault(estimate.ProbeCount, 1),
			[]byte(params),
			[]byte(response),
		); err != nil {
			return fmt.Errorf("insert ma estimate: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = $3,
    selected_strategy = $4,
    last_estimated_at = now(),
    updated_at = now()
WHERE id = $1::uuid
  AND active_strategy_id = $2::uuid
`, sessionID, strategyVersionID, maSessionStatusEstimated, nullString(selectedStrategy)); err != nil {
		return fmt.Errorf("update ma estimate session: %w", err)
	}
	return tx.Commit()
}

func (s *SQLStore) CreateMAExecutionRun(ctx context.Context, input maExecutionRunCreate) (MAExecutionRun, error) {
	if s == nil || s.db == nil {
		return MAExecutionRun{}, errors.New("binocolo ma store not configured")
	}
	id := input.ID
	if id == "" {
		id = uuid.NewString()
	}
	var run MAExecutionRun
	err := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_execution_run (
  id,
  session_id,
  strategy_version_id,
  strategy_type,
  status,
  estimated_count
) VALUES (
  $1::uuid, $2::uuid, $3::uuid, $4, $5, $6
)
RETURNING id::text, session_id::text, strategy_version_id::text, strategy_type, status, estimated_count,
          result_count, COALESCE(error_code, ''), started_at, completed_at
`, id,
		input.SessionID,
		input.StrategyVersionID,
		input.StrategyType,
		maRunStatusRunning,
		input.EstimatedCount,
	).Scan(
		&run.ID,
		&run.SessionID,
		&run.StrategyVersionID,
		&run.StrategyType,
		&run.Status,
		&run.EstimatedCount,
		&run.ResultCount,
		&run.ErrorCode,
		&run.StartedAt,
		&run.CompletedAt,
	)
	if err != nil {
		return MAExecutionRun{}, fmt.Errorf("create ma execution run: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = $2, updated_at = now()
WHERE id = $1::uuid
`, input.SessionID, maSessionStatusRunning); err != nil {
		return MAExecutionRun{}, fmt.Errorf("mark ma session running: %w", err)
	}
	return run, nil
}

func (s *SQLStore) CompleteMAExecutionRun(ctx context.Context, runID, status string, resultCount int, errorCode string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma run complete: %w", err)
	}
	defer tx.Rollback()

	var sessionID string
	if err := tx.QueryRowContext(ctx, `
UPDATE binocolo.ma_execution_run
SET status = $2,
    result_count = $3,
    error_code = $4,
    completed_at = now()
WHERE id = $1::uuid
RETURNING session_id::text
`, runID, status, resultCount, nullString(errorCode)).Scan(&sessionID); err != nil {
		return fmt.Errorf("complete ma run: %w", err)
	}
	sessionStatus := maSessionStatusCompleted
	if status == maRunStatusFailed {
		sessionStatus = maSessionStatusFailed
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_session
SET status = $2,
    last_executed_at = now(),
    updated_at = now()
WHERE id = $1::uuid
`, sessionID, sessionStatus); err != nil {
		return fmt.Errorf("complete ma session: %w", err)
	}
	return tx.Commit()
}

func (s *SQLStore) ReplaceMATargets(ctx context.Context, sessionID, runID string, targets []MATarget) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma target replace: %w", err)
	}
	defer tx.Rollback()
	// Risolvi-poi-scrivi (issue #86): l'intero batch passa dal registro identità
	// PRIMA di qualunque INSERT, così un conflitto abortisce prima che sia stato
	// scritto qualcosa. Un run di ricerca è atomico nella testa dell'analista, e
	// persisterne una parte mostrerebbe un numero di target che non corrisponde
	// a ciò che è stato trovato.
	observations := make([]maCompanyObservation, 0, len(targets))
	for _, target := range targets {
		observations = append(observations, maCompanyObservationFromTarget(target, sessionID, runID))
	}
	companyKeys, err := resolveMACompanyBatchTx(ctx, tx, observations)
	if err != nil {
		return err
	}
	// Deduplica SULLA CHIAVE RISOLTA, che è l'unica che conosce l'identità.
	// dedupeMATargets, a monte, lavora sulla precedenza del payload: due righe
	// della stessa azienda con vendor id diversi — risposte cache e fresche
	// mescolate nello stesso batch, o risultati raccolti a cavallo di una
	// rinumerazione del fornitore — la superano entrambe. Il resolver le manda
	// giustamente sulla stessa company_key (è deduplica, non conflitto), ma
	// senza questo passaggio finirebbero come due target della stessa azienda
	// nella stessa sessione, decorati dallo stesso rating, dallo stesso verdetto
	// e dagli stessi esiti. Vince la prima: nelle liste dei chiamanti i target
	// arricchiti e scorati precedono quelli riportati.
	keptIndexes := make([]int, 0, len(targets))
	firstByKey := make(map[string]int, len(targets))
	droppedByKey := map[string]int{}
	for index := range targets {
		key := companyKeys[index]
		if _, seen := firstByKey[key]; seen {
			droppedByKey[key]++
			continue
		}
		firstByKey[key] = index
		keptIndexes = append(keptIndexes, index)
	}
	if len(droppedByKey) > 0 {
		logging.FromContext(ctx).Warn("binocolo duplicate targets collapsed on resolved company key",
			"component", "binocolo", "operation", "ma_target_replace",
			"session_id", sessionID, "run_id", runID,
			"received", len(targets), "persisted", len(keptIndexes), "companies", len(droppedByKey))
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM binocolo.ma_evidence
WHERE target_id IN (SELECT id FROM binocolo.ma_target WHERE session_id = $1::uuid)
`, sessionID); err != nil {
		return fmt.Errorf("delete ma evidence: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM binocolo.ma_target WHERE session_id = $1::uuid`, sessionID); err != nil {
		return fmt.Errorf("delete ma targets: %w", err)
	}
	for _, index := range keptIndexes {
		target := targets[index]
		companyKey := companyKeys[index]
		targetID := target.ID
		if targetID == "" {
			targetID = uuid.NewString()
		}
		missingRaw, err := json.Marshal(target.MissingCriteria)
		if err != nil {
			return fmt.Errorf("marshal ma target missing criteria: %w", err)
		}
		flagsRaw := []byte("[]")
		if len(target.Flags) > 0 {
			raw, err := json.Marshal(target.Flags)
			if err != nil {
				return fmt.Errorf("marshal ma target flags: %w", err)
			}
			flagsRaw = raw
		}
		payload := json.RawMessage(`{}`)
		if len(target.VendorPayload) > 0 {
			payload = target.VendorPayload
		}
		// Address-stage rows carry identity only: score/match_state are NULL until
		// the enrich_score stage scores the keep/forse survivors. Everything else
		// (execute path, gated advanced stage) is "advanced" and persists both.
		level := target.EnrichmentLevel
		if level == "" {
			level = maEnrichmentAdvanced
		}
		var scoreVal any = target.Score
		var matchVal any = target.MatchState
		var scoreVersionVal any
		if target.ScoreVersion != nil {
			scoreVersionVal = *target.ScoreVersion
		}
		if level == maEnrichmentAddress {
			scoreVal = nil
			matchVal = nil
			scoreVersionVal = nil
		}
		origin := target.Origin
		if origin == "" {
			origin = maTargetOriginSearch
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_target (
  id,
  session_id,
  run_id,
  company_key,
  vendor_id,
  company_name,
  origin,
  vat_code,
  tax_code,
  province,
  town,
  activity_status,
  turnover,
  turnover_year,
  employees,
  ateco_code,
  ateco_description,
  score,
  match_state,
  confidence,
  flags,
  rationale,
  missing_criteria,
  vendor_payload,
  enrichment_level,
  score_version
) VALUES (
  $1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10,
  $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21::jsonb,
  $22, $23::jsonb, $24::jsonb, $25, $26
)
`, targetID,
			sessionID,
			runID,
			companyKey,
			nullString(target.VendorID),
			target.CompanyName,
			origin,
			nullString(target.VATCode),
			nullString(target.TaxCode),
			nullString(target.Province),
			nullString(target.Town),
			nullString(target.ActivityStatus),
			nullInt(target.Turnover),
			nullInt(target.TurnoverYear),
			nullInt(target.Employees),
			nullString(target.AtecoCode),
			nullString(target.AtecoDescription),
			scoreVal,
			matchVal,
			nullString(target.Confidence),
			flagsRaw,
			target.Rationale,
			missingRaw,
			[]byte(payload),
			level,
			scoreVersionVal,
		); err != nil {
			return fmt.Errorf("insert ma target: %w", translateMACompanyConstraintError(err, maCompanyConstraintFKOnly))
		}
		for _, evidence := range target.Evidence {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_evidence (
  id,
  target_id,
  criterion,
  status,
  family,
  label,
  value,
  points,
  weight,
  source_path
) VALUES (
  $1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10
)
`, uuid.NewString(),
				targetID,
				evidence.Criterion,
				evidence.Status,
				nullString(evidence.Family),
				evidence.Label,
				nullString(evidence.Value),
				evidence.Points,
				evidence.Weight,
				nullString(evidence.SourcePath),
			); err != nil {
				return fmt.Errorf("insert ma evidence: %w", err)
			}
		}
	}
	return tx.Commit()
}

// InsertMATarget appends one target to an existing session/run without replacing the
// current set. It serializes per session by locking ma_session and re-checks for a
// duplicate on the RESOLVED company key.
//
// La guardia confrontava i campi grezzi (vendor_id / vat_code / tax_code /
// company_name) replicando la semantica identitaria in Go: un secondo criterio di
// identità che poteva dissentire dal registro. Dopo la 120 confronta la chiave
// risolta, che è l'unico criterio (issue #86, categoria C2).
func (s *SQLStore) InsertMATarget(ctx context.Context, sessionID, runID string, target MATarget) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma target insert: %w", err)
	}
	defer tx.Rollback()
	var locked string
	if err := tx.QueryRowContext(ctx, `SELECT id::text FROM binocolo.ma_session WHERE id = $1::uuid FOR UPDATE`, sessionID).Scan(&locked); err != nil {
		return fmt.Errorf("lock ma session for target insert: %w", err)
	}
	companyKeys, err := resolveMACompanyBatchTx(ctx, tx, []maCompanyObservation{
		maCompanyObservationFromTarget(target, sessionID, runID),
	})
	if err != nil {
		return err
	}
	companyKey := companyKeys[0]
	var duplicates int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM binocolo.ma_target
WHERE session_id = $1::uuid AND company_key = $2
`, sessionID, companyKey).Scan(&duplicates); err != nil {
		return fmt.Errorf("check ma target duplicate: %w", err)
	}
	if duplicates > 0 {
		return errMATargetAlreadyPresent
	}

	targetID := target.ID
	if targetID == "" {
		targetID = uuid.NewString()
	}
	missingRaw, err := json.Marshal(target.MissingCriteria)
	if err != nil {
		return fmt.Errorf("marshal ma target missing criteria: %w", err)
	}
	flagsRaw := []byte("[]")
	if len(target.Flags) > 0 {
		raw, err := json.Marshal(target.Flags)
		if err != nil {
			return fmt.Errorf("marshal ma target flags: %w", err)
		}
		flagsRaw = raw
	}
	payload := json.RawMessage(`{}`)
	if len(target.VendorPayload) > 0 {
		payload = target.VendorPayload
	}
	level := target.EnrichmentLevel
	if level == "" {
		level = maEnrichmentAdvanced
	}
	var scoreVal any = target.Score
	var matchVal any = nullString(target.MatchState)
	var scoreVersionVal any
	if target.ScoreVersion != nil {
		scoreVersionVal = *target.ScoreVersion
	}
	if strings.TrimSpace(target.MatchState) == "" || level == maEnrichmentAddress {
		scoreVal = nil
		matchVal = nil
		scoreVersionVal = nil
	}
	origin := target.Origin
	if origin == "" {
		origin = maTargetOriginSearch
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_target (
  id,
  session_id,
  run_id,
  company_key,
  vendor_id,
  company_name,
  origin,
  vat_code,
  tax_code,
  province,
  town,
  activity_status,
  turnover,
  turnover_year,
  employees,
  ateco_code,
  ateco_description,
  score,
  match_state,
  confidence,
  flags,
  rationale,
  missing_criteria,
  vendor_payload,
  enrichment_level,
  score_version
) VALUES (
  $1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10,
  $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21::jsonb,
  $22, $23::jsonb, $24::jsonb, $25, $26
)
`, targetID,
		sessionID,
		runID,
		companyKey,
		nullString(target.VendorID),
		target.CompanyName,
		origin,
		nullString(target.VATCode),
		nullString(target.TaxCode),
		nullString(target.Province),
		nullString(target.Town),
		nullString(target.ActivityStatus),
		nullInt(target.Turnover),
		nullInt(target.TurnoverYear),
		nullInt(target.Employees),
		nullString(target.AtecoCode),
		nullString(target.AtecoDescription),
		scoreVal,
		matchVal,
		nullString(target.Confidence),
		flagsRaw,
		target.Rationale,
		missingRaw,
		[]byte(payload),
		level,
		scoreVersionVal,
	); err != nil {
		return fmt.Errorf("insert ma target: %w", translateMACompanyConstraintError(err, maCompanyConstraintManualInsert))
	}
	for _, evidence := range target.Evidence {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_evidence (
  id,
  target_id,
  criterion,
  status,
  family,
  label,
  value,
  points,
  weight,
  source_path
) VALUES (
  $1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10
)
`, uuid.NewString(),
			targetID,
			evidence.Criterion,
			evidence.Status,
			nullString(evidence.Family),
			evidence.Label,
			nullString(evidence.Value),
			evidence.Points,
			evidence.Weight,
			nullString(evidence.SourcePath),
		); err != nil {
			return fmt.Errorf("insert ma evidence: %w", err)
		}
	}
	return tx.Commit()
}

// MarkMATargetAdvancedEnriched flips one target to advanced enrichment, storing the
// paid IT-advanced payload. The gated-search enrich stage calls this per company the
// instant its €0.10 fetch returns, BEFORE scoring — so a crashed/reclaimed job re-runs
// only the un-enriched tail (the enrichment_level guard) and never re-charges a company
// already advanced. Score/match_state are left untouched here; the run's final
// ReplaceMATargets writes them once the whole survivor set is scored together.
func (s *SQLStore) MarkMATargetAdvancedEnriched(ctx context.Context, targetID string, vendorPayload json.RawMessage) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	payload := json.RawMessage(`{}`)
	if len(vendorPayload) > 0 {
		payload = vendorPayload
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_target
SET vendor_payload = $2::jsonb,
    enrichment_level = 'advanced'
WHERE id = $1::uuid
`, targetID, []byte(payload)); err != nil {
		return fmt.Errorf("mark ma target advanced enriched: %w", err)
	}
	return nil
}

func (s *SQLStore) StartMATrace(ctx context.Context, input maTraceStart) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	id := input.ID
	if id == "" {
		id = uuid.NewString()
	}
	request := json.RawMessage(`{}`)
	if len(input.Request) > 0 {
		request = input.Request
	}
	startedAt := input.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	var out string
	if err := s.db.QueryRowContext(ctx, `
	INSERT INTO binocolo.ma_operation_trace (
	  id,
	  request_id,
	  operation,
	  method,
	  path,
	  status,
	  session_id,
	  created_by_subject,
	  created_by_email,
	  request,
	  started_at
	) VALUES (
	  $1::uuid, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, $8, $9, $10::jsonb, $11
	)
	RETURNING id::text
	`, id,
		input.RequestID,
		input.Operation,
		input.Method,
		input.Path,
		maTraceStatusRunning,
		input.SessionID,
		input.CreatedBySubject,
		input.CreatedByEmail,
		[]byte(request),
		startedAt,
	).Scan(&out); err != nil {
		return "", fmt.Errorf("start ma trace: %w", err)
	}
	return out, nil
}

func (s *SQLStore) LinkMATrace(ctx context.Context, input maTraceLink) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if input.ID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
	UPDATE binocolo.ma_operation_trace
	SET session_id = COALESCE(NULLIF($2, '')::uuid, session_id),
	    strategy_version_id = COALESCE(NULLIF($3, '')::uuid, strategy_version_id),
	    execution_run_id = COALESCE(NULLIF($4, '')::uuid, execution_run_id)
	WHERE id = $1::uuid
	`, input.ID, input.SessionID, input.StrategyVersionID, input.ExecutionRunID)
	if err != nil {
		return fmt.Errorf("link ma trace: %w", err)
	}
	return nil
}

func (s *SQLStore) CompleteMATrace(ctx context.Context, input maTraceComplete) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if input.ID == "" {
		return nil
	}
	completedAt := input.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	status := input.Status
	if status == "" {
		status = maTraceStatusSucceeded
	}
	_, err := s.db.ExecContext(ctx, `
	UPDATE binocolo.ma_operation_trace
	SET status = $2,
	    http_status = $3,
	    error_code = $4,
	    error_message = $5,
	    completed_at = $6,
	    duration_ms = GREATEST(0, FLOOR(EXTRACT(EPOCH FROM ($6 - started_at)) * 1000)::integer)
	WHERE id = $1::uuid
	`, input.ID,
		status,
		nullIntValue(input.HTTPStatus),
		input.ErrorCode,
		input.ErrorMessage,
		completedAt,
	)
	if err != nil {
		return fmt.Errorf("complete ma trace: %w", err)
	}
	return nil
}

func (s *SQLStore) RecordMATraceEvent(ctx context.Context, input maTraceEventWrite) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if input.TraceID == "" {
		return nil
	}
	request := json.RawMessage(`{}`)
	if len(input.Request) > 0 {
		request = input.Request
	}
	response := json.RawMessage(`{}`)
	if len(input.Response) > 0 {
		response = input.Response
	}
	metadata := json.RawMessage(`{}`)
	if len(input.Metadata) > 0 {
		metadata = input.Metadata
	}
	_, err := s.db.ExecContext(ctx, `
	INSERT INTO binocolo.ma_operation_trace_event (
	  trace_id,
	  event_order,
	  event_type,
	  round,
	  tool_name,
	  external_system,
	  status,
	  duration_ms,
	  request,
	  response,
	  metadata,
	  error
	) VALUES (
	  $1::uuid, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11::jsonb, $12
	)
	`, input.TraceID,
		input.EventOrder,
		input.EventType,
		nullInt(input.Round),
		input.ToolName,
		input.ExternalSystem,
		defaultString(input.Status, maTraceEventInfo),
		nullInt(input.DurationMS),
		[]byte(request),
		[]byte(response),
		[]byte(metadata),
		input.Error,
	)
	if err != nil {
		return fmt.Errorf("record ma trace event: %w", err)
	}
	return nil
}

func (s *SQLStore) RecordMAExport(ctx context.Context, sessionID, format string, rowCount int, createdByEmail string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_export (
  id,
  session_id,
  format,
  row_count,
  created_by_email
) VALUES (
  $1::uuid, $2::uuid, $3, $4, $5
)
`, uuid.NewString(), sessionID, strings.ToLower(strings.TrimSpace(format)), rowCount, nullString(createdByEmail))
	if err != nil {
		return fmt.Errorf("record ma export: %w", err)
	}
	return nil
}

func insertMAStrategyVersion(ctx context.Context, tx *sql.Tx, sessionID, strategyID string, version int, rawStrategy json.RawMessage, createdByEmail string) (MAStrategyVersion, error) {
	var strategy MAStrategyVersion
	var raw []byte
	if err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_strategy_version (
  id,
  session_id,
  version,
  strategy,
  created_by_email
) VALUES (
  $1::uuid, $2::uuid, $3, $4::jsonb, $5
)
RETURNING id::text, session_id::text, version, strategy, COALESCE(created_by_email, ''), created_at
`, strategyID, sessionID, version, []byte(rawStrategy), nullString(createdByEmail)).Scan(
		&strategy.ID,
		&strategy.SessionID,
		&strategy.Version,
		&raw,
		&strategy.CreatedByEmail,
		&strategy.CreatedAt,
	); err != nil {
		return MAStrategyVersion{}, fmt.Errorf("insert ma strategy version: %w", err)
	}
	parsed, err := rawToStrategy(raw)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("decode ma strategy version: %w", err)
	}
	strategy.Strategy = parsed
	return strategy, nil
}

func (s *SQLStore) loadMASession(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (MASession, error) {
	var session MASession
	var selected sql.NullString
	var active sql.NullString
	var subject sql.NullString
	var email sql.NullString
	var estimated sql.NullTime
	var executed sql.NullTime
	var archivedAt sql.NullTime
	var archivedBySubject sql.NullString
	var archivedByEmail sql.NullString
	var deletedAt sql.NullTime
	var deletedBySubject sql.NullString
	var deletedByEmail sql.NullString
	var initiativeID sql.NullString
	var initiativeTitle sql.NullString
	err := q.QueryRowContext(ctx, `
SELECT session.id::text, session.title, session.prompt, session.status, session.selected_strategy, session.active_strategy_id::text,
       session.created_by_subject, session.created_by_email, session.last_estimated_at, session.last_executed_at,
       session.created_at, session.updated_at,
       session.archived_at, session.archived_by_subject, session.archived_by_email,
       session.deleted_at, session.deleted_by_subject, session.deleted_by_email,
       session.initiative_id::text,
       initiative.title
FROM binocolo.ma_session session
LEFT JOIN binocolo.ma_initiative initiative ON initiative.id = session.initiative_id
WHERE session.id = $1::uuid
`, id).Scan(
		&session.ID,
		&session.Title,
		&session.Prompt,
		&session.Status,
		&selected,
		&active,
		&subject,
		&email,
		&estimated,
		&executed,
		&session.CreatedAt,
		&session.UpdatedAt,
		&archivedAt,
		&archivedBySubject,
		&archivedByEmail,
		&deletedAt,
		&deletedBySubject,
		&deletedByEmail,
		&initiativeID,
		&initiativeTitle,
	)
	if err != nil {
		return MASession{}, fmt.Errorf("load ma session: %w", err)
	}
	session.SelectedStrategy = selected.String
	session.ActiveStrategyID = active.String
	session.CreatedBySubject = subject.String
	session.CreatedByEmail = email.String
	if estimated.Valid {
		session.LastEstimatedAt = &estimated.Time
	}
	if executed.Valid {
		session.LastExecutedAt = &executed.Time
	}
	if archivedAt.Valid {
		session.ArchivedAt = &archivedAt.Time
	}
	session.ArchivedBySubject = archivedBySubject.String
	session.ArchivedByEmail = archivedByEmail.String
	if deletedAt.Valid {
		session.DeletedAt = &deletedAt.Time
	}
	session.DeletedBySubject = deletedBySubject.String
	session.DeletedByEmail = deletedByEmail.String
	session.InitiativeID = initiativeID.String
	session.InitiativeTitle = initiativeTitle.String
	return session, nil
}

func (s *SQLStore) loadActiveMAStrategy(ctx context.Context, sessionID string) (MAStrategyVersion, error) {
	var strategy MAStrategyVersion
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
SELECT strategy.id::text, strategy.session_id::text, strategy.version, strategy.strategy,
       COALESCE(strategy.created_by_email, ''), strategy.created_at
FROM binocolo.ma_strategy_version strategy
JOIN binocolo.ma_session session ON session.active_strategy_id = strategy.id
WHERE session.id = $1::uuid
`, sessionID).Scan(
		&strategy.ID,
		&strategy.SessionID,
		&strategy.Version,
		&raw,
		&strategy.CreatedByEmail,
		&strategy.CreatedAt,
	)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("load active ma strategy: %w", err)
	}
	parsed, err := rawToStrategy(raw)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("decode active ma strategy: %w", err)
	}
	strategy.Strategy = parsed
	return strategy, nil
}

// GetMAStrategyVersion loads a specific strategy version by id (scoped to its
// session). The execute job pins the version the user confirmed, so the worker
// executes exactly that — not whatever is active when it later runs.
func (s *SQLStore) GetMAStrategyVersion(ctx context.Context, sessionID, versionID string) (MAStrategyVersion, error) {
	if s == nil || s.db == nil {
		return MAStrategyVersion{}, errors.New("binocolo ma store not configured")
	}
	var strategy MAStrategyVersion
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
SELECT strategy.id::text, strategy.session_id::text, strategy.version, strategy.strategy,
       COALESCE(strategy.created_by_email, ''), strategy.created_at
FROM binocolo.ma_strategy_version strategy
WHERE strategy.id = $1::uuid AND strategy.session_id = $2::uuid
`, versionID, sessionID).Scan(
		&strategy.ID,
		&strategy.SessionID,
		&strategy.Version,
		&raw,
		&strategy.CreatedByEmail,
		&strategy.CreatedAt,
	)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("get ma strategy version: %w", err)
	}
	parsed, err := rawToStrategy(raw)
	if err != nil {
		return MAStrategyVersion{}, fmt.Errorf("decode ma strategy version: %w", err)
	}
	strategy.Strategy = parsed
	return strategy, nil
}

func (s *SQLStore) loadMAEstimates(ctx context.Context, sessionID string) ([]MAEstimate, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, session_id::text, strategy_version_id::text, strategy_type,
       COALESCE(ateco_code, ''), COALESCE(ateco_description, ''), COALESCE(province, ''),
       estimated_count, estimated_cost, selected,
       COALESCE(surface_status, 'exact'), execution_limit, probe_count,
       params, vendor_response, created_at
FROM binocolo.ma_dry_run_estimate
WHERE session_id = $1::uuid
ORDER BY selected DESC, strategy_type, province, ateco_code
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma estimates: %w", err)
	}
	defer rows.Close()
	out := []MAEstimate{}
	for rows.Next() {
		var item MAEstimate
		if err := rows.Scan(
			&item.ID,
			&item.SessionID,
			&item.StrategyVersionID,
			&item.StrategyType,
			&item.AtecoCode,
			&item.AtecoDescription,
			&item.Province,
			&item.EstimatedCount,
			&item.EstimatedCost,
			&item.Selected,
			&item.SurfaceStatus,
			&item.ExecutionLimit,
			&item.ProbeCount,
			&item.Params,
			&item.VendorResponse,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ma estimate: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma estimates: %w", err)
	}
	return out, nil
}

func (s *SQLStore) loadMARuns(ctx context.Context, sessionID string) ([]MAExecutionRun, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, session_id::text, strategy_version_id::text, strategy_type, status,
       estimated_count, result_count, COALESCE(error_code, ''), started_at, completed_at
FROM binocolo.ma_execution_run
WHERE session_id = $1::uuid
ORDER BY started_at DESC
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma runs: %w", err)
	}
	defer rows.Close()
	out := []MAExecutionRun{}
	for rows.Next() {
		var item MAExecutionRun
		if err := rows.Scan(
			&item.ID,
			&item.SessionID,
			&item.StrategyVersionID,
			&item.StrategyType,
			&item.Status,
			&item.EstimatedCount,
			&item.ResultCount,
			&item.ErrorCode,
			&item.StartedAt,
			&item.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ma run: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma runs: %w", err)
	}
	return out, nil
}

type maTargetScanner interface {
	Scan(dest ...any) error
}

func scanMATargetBase(row maTargetScanner) (MATarget, error) {
	var item MATarget
	var turnover sql.NullInt64
	var turnoverYear sql.NullInt64
	var employees sql.NullInt64
	var score sql.NullInt64
	var scoreVersion sql.NullInt64
	var missingRaw []byte
	var flagsRaw []byte
	if err := row.Scan(
		&item.ID,
		&item.SessionID,
		&item.RunID,
		&item.CompanyKey,
		&item.VendorID,
		&item.CompanyName,
		&item.Origin,
		&item.VATCode,
		&item.TaxCode,
		&item.Province,
		&item.Town,
		&item.ActivityStatus,
		&turnover,
		&turnoverYear,
		&employees,
		&item.AtecoCode,
		&item.AtecoDescription,
		&score,
		&item.MatchState,
		&item.Confidence,
		&flagsRaw,
		&item.Rationale,
		&missingRaw,
		&item.VendorPayload,
		&item.EnrichmentLevel,
		&scoreVersion,
		&item.CreatedAt,
	); err != nil {
		return MATarget{}, fmt.Errorf("scan ma target: %w", err)
	}
	// Address-stage rows are unscored (score NULL); leave item.Score at 0.
	if score.Valid {
		item.Score = int(score.Int64)
	}
	if scoreVersion.Valid {
		value := int(scoreVersion.Int64)
		item.ScoreVersion = &value
	}
	if len(flagsRaw) > 0 {
		_ = json.Unmarshal(flagsRaw, &item.Flags)
	}
	if turnover.Valid {
		value := int(turnover.Int64)
		item.Turnover = &value
	}
	if turnoverYear.Valid {
		value := int(turnoverYear.Int64)
		item.TurnoverYear = &value
	}
	if employees.Valid {
		value := int(employees.Int64)
		item.Employees = &value
	}
	if len(missingRaw) > 0 {
		_ = json.Unmarshal(missingRaw, &item.MissingCriteria)
	}
	// La chiave si LEGGE, non si ricalcola dal payload del vendor (issue #86).
	// La guardia erra invece di derivare: una riga senza chiave è una riga
	// scritta da un binario pre-cutover, e restituirla con chiave vuota
	// orfanerebbe in silenzio rating, gate e dossier. Il backfill della
	// migrazione 121 è lo strumento per ripararla.
	if strings.TrimSpace(item.CompanyKey) == "" {
		return MATarget{}, fmt.Errorf("ma target %s: company_key assente (rieseguire il backfill della migrazione 121)", item.ID)
	}
	return item, nil
}

func (s *SQLStore) loadMATargets(ctx context.Context, sessionID string) ([]MATarget, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, session_id::text, run_id::text, COALESCE(company_key, ''), COALESCE(vendor_id, ''), company_name,
       COALESCE(origin, 'search'), COALESCE(vat_code, ''), COALESCE(tax_code, ''), COALESCE(province, ''), COALESCE(town, ''),
       COALESCE(activity_status, ''), turnover, turnover_year, employees, COALESCE(ateco_code, ''),
       COALESCE(ateco_description, ''), score, COALESCE(match_state, ''), COALESCE(confidence, ''), flags,
       rationale, missing_criteria, vendor_payload, COALESCE(enrichment_level, 'advanced'), score_version, created_at
FROM binocolo.ma_target
WHERE session_id = $1::uuid
ORDER BY score DESC NULLS LAST, company_name
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma targets: %w", err)
	}
	defer rows.Close()
	targets := []MATarget{}
	for rows.Next() {
		item, err := scanMATargetBase(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma targets: %w", err)
	}
	if len(targets) == 0 {
		return targets, nil
	}
	evidence, err := s.loadMAEvidence(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	ratings, err := s.loadMARatings(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	outcomes, err := s.loadMAOutcomes(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	webValidations, err := s.loadMAWebValidations(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(targets))
	for index := range targets {
		keys = append(keys, targets[index].CompanyKey)
	}
	deep, err := s.ListMADeepAnalysis(ctx, keys)
	if err != nil {
		return nil, err
	}
	for index := range targets {
		targets[index].Evidence = evidence[targets[index].ID]
		if rating, ok := ratings[targets[index].CompanyKey]; ok {
			value := rating
			targets[index].Rating = &value
		}
		if events, ok := outcomes[targets[index].CompanyKey]; ok {
			targets[index].Outcomes = events
		}
		if validation, ok := webValidations[targets[index].CompanyKey]; ok {
			record := validation
			targets[index].WebValidation = &record
		}
		if analysis, ok := deep[targets[index].CompanyKey]; ok {
			record := analysis
			targets[index].Deep = &record
		}
	}
	sortMATargetsByRating(targets)
	return targets, nil
}

func (s *SQLStore) ListMATargetRows(ctx context.Context, sessionID string) ([]MATargetRow, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
WITH target_rows AS (
  -- Colonne esplicite, non t.*: la CTE espone una company_key, e un t.* che ne
  -- porti dentro un'altra con lo stesso nome rende ambiguo ogni riferimento
  -- successivo. È il difetto che la migrazione 120 avrebbe innescato sul binario
  -- precedente (issue #86), e non deve poter tornare da questo lato.
  SELECT
    t.id,
    t.session_id,
    t.run_id,
    t.company_key,
    t.company_name,
    t.origin,
    t.vat_code,
    t.province,
    t.town,
    t.ateco_code,
    t.score,
    t.score_version,
    t.match_state,
    t.confidence,
    t.flags,
    t.enrichment_level,
    t.turnover
  FROM binocolo.ma_target t
  WHERE t.session_id = $1::uuid
)
SELECT
  t.id::text,
  t.run_id::text,
  COALESCE(t.company_key, ''),
  t.company_name,
  COALESCE(t.origin, 'search'),
  COALESCE(t.vat_code, ''),
  COALESCE(t.province, ''),
  COALESCE(t.town, ''),
  COALESCE(t.ateco_code, ''),
  t.score,
  t.score_version,
  COALESCE(t.match_state, ''),
  COALESCE(t.confidence, ''),
  t.flags,
  COALESCE(t.enrichment_level, 'advanced'),
  t.turnover,
  r.rating,
  wv.company_key IS NOT NULL AS has_validation,
  COALESCE(wv.web_validation_state, ''),
  COALESCE(wv.final_action, ''),
  COALESCE(wv.selected_domain, ''),
  COALESCE(wv.final_decision->>'reason', ''),
  COALESCE(wv.domain_response->'groupSiteHint'->>'domain', ''),
  COALESCE(wv.domain_response->'groupSiteHint'->>'identifier', ''),
  COALESCE(jsonb_array_length(
    CASE
      WHEN jsonb_typeof(wv.domain_response->'candidates') = 'array' THEN wv.domain_response->'candidates'
      ELSE '[]'::jsonb
    END
  ), 0),
  outside_filter.revenue_per_employee_value,
  outside_filter.max_shareholders_value
FROM target_rows t
LEFT JOIN LATERAL (
  SELECT
    MAX(COALESCE(evidence.value, '')) FILTER (WHERE evidence.criterion = $3) AS revenue_per_employee_value,
    MAX(COALESCE(evidence.value, '')) FILTER (WHERE evidence.criterion = $4) AS max_shareholders_value
  FROM binocolo.ma_evidence evidence
  WHERE evidence.target_id = t.id
    AND evidence.status = $2
    AND evidence.criterion IN ($3, $4)
) outside_filter ON TRUE
LEFT JOIN binocolo.ma_target_web_validation wv
  ON wv.session_id = t.session_id AND wv.company_key = t.company_key
LEFT JOIN binocolo.ma_target_rating r
  ON r.session_id = t.session_id AND r.company_key = t.company_key
ORDER BY t.score DESC NULLS LAST, t.company_name
`, sessionID, maEvidenceOutside, maPostFilterRevenuePerEmployeeMin, maPostFilterMaxShareholders)
	if err != nil {
		return nil, fmt.Errorf("list ma target rows: %w", err)
	}
	defer rows.Close()

	out := []MATargetRow{}
	for rows.Next() {
		// La stessa guardia di scanMATargetBase, e per lo stesso motivo: senza,
		// questa lista serviva la riga con chiave VUOTA mentre GetMASession
		// errava sulla stessa riga. Due contratti diversi sulla stessa colonna,
		// ed è lo scarto che rendeva raggiungibili i fallback del client — la UI
		// riceveva un target senza chiave e se ne inventava una dalla P.IVA.
		var item MATargetRow
		var score sql.NullInt64
		var scoreVersion sql.NullInt64
		var turnover sql.NullInt64
		var rating sql.NullInt64
		var flagsRaw []byte
		var hasValidation bool
		var webState, finalAction, selectedDomain, reason string
		var groupDomain, groupIdentifier string
		var candidateCount int
		var outsideRevenue, outsideShareholders sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.RunID,
			&item.CompanyKey,
			&item.CompanyName,
			&item.Origin,
			&item.VATCode,
			&item.Province,
			&item.Town,
			&item.AtecoCode,
			&score,
			&scoreVersion,
			&item.MatchState,
			&item.Confidence,
			&flagsRaw,
			&item.EnrichmentLevel,
			&turnover,
			&rating,
			&hasValidation,
			&webState,
			&finalAction,
			&selectedDomain,
			&reason,
			&groupDomain,
			&groupIdentifier,
			&candidateCount,
			&outsideRevenue,
			&outsideShareholders,
		); err != nil {
			return nil, fmt.Errorf("scan ma target row: %w", err)
		}
		if score.Valid {
			item.Score = int(score.Int64)
		}
		if scoreVersion.Valid {
			value := int(scoreVersion.Int64)
			item.ScoreVersion = &value
		}
		if turnover.Valid {
			value := int(turnover.Int64)
			item.SortTurnover = &value
		}
		if rating.Valid {
			value := int(rating.Int64)
			item.Rating = &value
		}
		if outsideRevenue.Valid {
			value := outsideRevenue.String
			item.OutsideRevenuePerEmployeeValue = &value
		}
		if outsideShareholders.Valid {
			value := outsideShareholders.String
			item.OutsideMaxShareholdersValue = &value
		}
		if len(flagsRaw) > 0 {
			_ = json.Unmarshal(flagsRaw, &item.Flags)
		}
		if hasValidation {
			item.WebValidation = &MATargetRowWeb{
				WebValidationState:  webState,
				FinalAction:         finalAction,
				SelectedDomain:      selectedDomain,
				FinalDecision:       MATargetRowFinalDecision{Reason: reason},
				GroupSiteDomain:     groupDomain,
				GroupSiteIdentifier: groupIdentifier,
				CandidateCount:      candidateCount,
			}
		}
		if strings.TrimSpace(item.CompanyKey) == "" {
			return nil, fmt.Errorf("ma target %s: company_key assente (rieseguire il backfill della migrazione 121)", item.ID)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma target rows: %w", err)
	}
	sortMATargetRowsByRating(out)
	return out, nil
}

// FindMALatestTargetForCard resolves the most recent target row for
// (initiative, companyKey) among the sessions anchored to the initiative,
// so the card-dossier page can hydrate the full MATarget (deep/web
// validation included) via GetMATargetByID. La chiave è persistita
// (mig 120): la ricerca la confronta, non la ricalcola dal payload.
func (s *SQLStore) FindMALatestTargetForCard(ctx context.Context, initiativeID, companyKey string) (string, string, error) {
	if s == nil || s.db == nil {
		return "", "", errors.New("binocolo ma store not configured")
	}
	row := s.db.QueryRowContext(ctx, `
SELECT t.session_id::text, t.id::text
FROM binocolo.ma_target t
JOIN binocolo.ma_session s ON s.id = t.session_id
WHERE s.initiative_id = $1::uuid
  AND t.company_key = $2
ORDER BY t.created_at DESC
LIMIT 1
`, initiativeID, companyKey)
	var sessionID, targetID string
	if err := row.Scan(&sessionID, &targetID); err != nil {
		return "", "", err
	}
	return sessionID, targetID, nil
}

func (s *SQLStore) GetMATargetByID(ctx context.Context, sessionID, targetID string) (MATarget, error) {
	if s == nil || s.db == nil {
		return MATarget{}, errors.New("binocolo ma store not configured")
	}
	row := s.db.QueryRowContext(ctx, `
SELECT id::text, session_id::text, run_id::text, COALESCE(company_key, ''), COALESCE(vendor_id, ''), company_name,
       COALESCE(origin, 'search'), COALESCE(vat_code, ''), COALESCE(tax_code, ''), COALESCE(province, ''), COALESCE(town, ''),
       COALESCE(activity_status, ''), turnover, turnover_year, employees, COALESCE(ateco_code, ''),
       COALESCE(ateco_description, ''), score, COALESCE(match_state, ''), COALESCE(confidence, ''), flags,
       rationale, missing_criteria, vendor_payload, COALESCE(enrichment_level, 'advanced'), score_version, created_at
FROM binocolo.ma_target
WHERE session_id = $1::uuid AND id = $2::uuid
`, sessionID, targetID)
	target, err := scanMATargetBase(row)
	if err != nil {
		return MATarget{}, err
	}
	target.Evidence, err = s.loadMAEvidenceForTarget(ctx, sessionID, target.ID)
	if err != nil {
		return MATarget{}, err
	}
	rating, err := s.loadMARating(ctx, sessionID, target.CompanyKey)
	if err != nil {
		return MATarget{}, err
	}
	target.Rating = rating
	target.Outcomes, err = s.loadMAOutcomesForCompany(ctx, sessionID, target.CompanyKey)
	if err != nil {
		return MATarget{}, err
	}
	validation, err := s.loadMAWebValidationForCompany(ctx, sessionID, target.CompanyKey)
	if err != nil {
		return MATarget{}, err
	}
	if validation != nil {
		target.WebValidation = validation
	}
	deep, err := s.ListMADeepAnalysis(ctx, []string{target.CompanyKey})
	if err != nil {
		return MATarget{}, err
	}
	if analysis, ok := deep[target.CompanyKey]; ok {
		record := analysis
		target.Deep = &record
	}
	return target, nil
}

// sortMATargetsByRating ordina i target con la classifica utente in testa
// (rating DESC), poi per punteggio dello scoring, poi — a parità di punteggio —
// per sostanza invece che per alfabeto: confidence (documentato batte rado),
// fatturato, e solo alla fine il nome. I non valutati (rating nil -> 0) stanno
// tra le stelle e gli esclusi (-1).
func sortMATargetsByRating(targets []MATarget) {
	sort.SliceStable(targets, func(i, j int) bool {
		ri, rj := maRatingValue(targets[i].Rating), maRatingValue(targets[j].Rating)
		if ri != rj {
			return ri > rj
		}
		if targets[i].Score != targets[j].Score {
			return targets[i].Score > targets[j].Score
		}
		if ci, cj := maConfidenceRank(targets[i].Confidence), maConfidenceRank(targets[j].Confidence); ci != cj {
			return ci > cj
		}
		if ti, tj := intValue(targets[i].Turnover), intValue(targets[j].Turnover); ti != tj {
			return ti > tj
		}
		return targets[i].CompanyName < targets[j].CompanyName
	})
}

func sortMATargetRowsByRating(rows []MATargetRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		ri, rj := maRatingValue(rows[i].Rating), maRatingValue(rows[j].Rating)
		if ri != rj {
			return ri > rj
		}
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		if ci, cj := maConfidenceRank(rows[i].Confidence), maConfidenceRank(rows[j].Confidence); ci != cj {
			return ci > cj
		}
		if ti, tj := intValue(rows[i].SortTurnover), intValue(rows[j].SortTurnover); ti != tj {
			return ti > tj
		}
		return rows[i].CompanyName < rows[j].CompanyName
	})
}

func maConfidenceRank(confidence string) int {
	switch confidence {
	case "alta":
		return 3
	case "media":
		return 2
	case "bassa":
		return 1
	default:
		return 0
	}
}

func maRatingValue(rating *int) int {
	if rating == nil {
		return 0
	}
	return *rating
}

// ListMARatings exposes loadMARatings on the store interface (backfill on
// retro-anchor, PRD §3.2): every rated company of a session, keyed by
// company_key.
func (s *SQLStore) ListMARatings(ctx context.Context, sessionID string) (map[string]int, error) {
	return s.loadMARatings(ctx, sessionID)
}

func (s *SQLStore) loadMARatings(ctx context.Context, sessionID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key, rating
FROM binocolo.ma_target_rating
WHERE session_id = $1::uuid
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma ratings: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var key string
		var rating int
		if err := rows.Scan(&key, &rating); err != nil {
			return nil, fmt.Errorf("scan ma rating: %w", err)
		}
		out[key] = rating
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma ratings: %w", err)
	}
	return out, nil
}

func (s *SQLStore) loadMARating(ctx context.Context, sessionID, companyKey string) (*int, error) {
	if companyKey == "" {
		return nil, nil
	}
	var rating int
	err := s.db.QueryRowContext(ctx, `
SELECT rating
FROM binocolo.ma_target_rating
WHERE session_id = $1::uuid AND company_key = $2
`, sessionID, companyKey).Scan(&rating)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load ma rating: %w", err)
	}
	return &rating, nil
}

// UpsertMATargetRating salva (o azzera) il voto su un'azienda nella sessione.
// rating == 0 cancella la riga (torna "non valutato"); -1/1..3 fanno upsert.
// Con il voto persiste la ground truth (migrazione 080): il motivo (compilato
// sull'esclusione) e lo snapshot di score/confidence che la UI mostrava al
// momento del giudizio.
func (s *SQLStore) UpsertMATargetRating(ctx context.Context, sessionID string, input MATargetRatingRequest, subject, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if input.Rating == 0 {
		if _, err := s.db.ExecContext(ctx, `
DELETE FROM binocolo.ma_target_rating
WHERE session_id = $1::uuid AND company_key = $2
`, sessionID, input.CompanyKey); err != nil {
			return fmt.Errorf("clear ma target rating: %w", err)
		}
		return nil
	}
	var scoreAt any
	if input.ScoreAtRating != nil {
		scoreAt = *input.ScoreAtRating
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_target_rating (session_id, company_key, rating, reason, score_at_rating, confidence_at_rating, rated_by_subject, rated_by_email, rated_at)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (session_id, company_key) DO UPDATE
SET rating = EXCLUDED.rating,
    reason = EXCLUDED.reason,
    score_at_rating = EXCLUDED.score_at_rating,
    confidence_at_rating = EXCLUDED.confidence_at_rating,
    rated_by_subject = EXCLUDED.rated_by_subject,
    rated_by_email = EXCLUDED.rated_by_email,
    rated_at = now()
`, sessionID, input.CompanyKey, input.Rating, nullString(input.Reason), scoreAt, nullString(input.ConfidenceAtRating), nullString(subject), nullString(email)); err != nil {
		return fmt.Errorf("upsert ma target rating: %w", err)
	}
	return nil
}

// InsertMATargetOutcome appends an event. Ordinary events remain append-only;
// annotations (`nota`) are the sole event kind mutable through the guarded
// methods below.
func (s *SQLStore) InsertMATargetOutcome(ctx context.Context, outcome MATargetOutcome) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	return insertMATargetOutcome(ctx, s.db, outcome)
}

// insertMATargetOutcome is the shared body of the plain append and the
// card-state transaction, so both paths write the exact same row.
func insertMATargetOutcome(ctx context.Context, exec maCardEventExec, outcome MATargetOutcome) error {
	var sessionID any
	if outcome.SessionID != "" {
		sessionID = outcome.SessionID
	}
	var initiativeID any
	if outcome.InitiativeID != "" {
		initiativeID = outcome.InitiativeID
	}
	payload := outcome.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	id := outcome.ID
	if id == "" {
		id = uuid.NewString()
	}
	var createdAt any
	if !outcome.CreatedAt.IsZero() {
		createdAt = outcome.CreatedAt
	}
	if _, err := exec.ExecContext(ctx, `
INSERT INTO binocolo.ma_target_outcome (id, session_id, initiative_id, company_key, event, note, payload, created_by_subject, created_by_email, created_at)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7::jsonb, $8, $9, COALESCE($10::timestamptz, now()))
`, id, sessionID, initiativeID, outcome.CompanyKey, outcome.Event, nullString(outcome.Note), string(payload), nullString(outcome.CreatedBySubject), nullString(outcome.CreatedByEmail), createdAt); err != nil {
		return fmt.Errorf("insert ma target outcome: %w", err)
	}
	return nil
}

func (s *SQLStore) UpdateMAAnnotation(ctx context.Context, id, note, subject, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	var returnedID string
	if err := s.db.QueryRowContext(ctx, updateMAAnnotationQuery, id, note, nullString(subject), nullString(email)).Scan(&returnedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errMAAnnotationNotFound
		}
		return fmt.Errorf("update ma annotation: %w", err)
	}
	return nil
}

func (s *SQLStore) SoftDeleteMAAnnotation(ctx context.Context, id, subject, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	var returnedID string
	if err := s.db.QueryRowContext(ctx, softDeleteMAAnnotationQuery, id, nullString(subject), nullString(email)).Scan(&returnedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errMAAnnotationNotFound
		}
		return fmt.Errorf("soft delete ma annotation: %w", err)
	}
	return nil
}

func maCompanyActivityQuery(includeDeleted bool) string {
	deletedClause := "\n  AND o.deleted_at IS NULL"
	if includeDeleted {
		deletedClause = " /* includes-deleted */"
	}
	return `
SELECT o.id::text, COALESCE(o.session_id::text, ''), COALESCE(o.initiative_id::text, ''), o.company_key,
       o.event, COALESCE(o.note, ''), COALESCE(o.payload::text, '{}'),
       COALESCE(o.created_by_email, ''), o.created_at,
       o.updated_at, COALESCE(o.updated_by_email, ''),
       o.deleted_at, COALESCE(o.deleted_by_email, '')
FROM binocolo.ma_target_outcome o /* activity-includes-deleted */
WHERE o.company_key = $1` + deletedClause + `
ORDER BY o.created_at DESC`
}

func (s *SQLStore) ListMACompanyActivity(ctx context.Context, companyKey string, includeDeleted bool) (MACompanyActivity, error) {
	if s == nil || s.db == nil {
		return MACompanyActivity{}, errors.New("binocolo ma store not configured")
	}
	out := MACompanyActivity{Items: []MATargetOutcome{}, Initiatives: []MACompanyActivityLookup{}, Sessions: []MACompanyActivitySession{}}
	rows, err := s.db.QueryContext(ctx, maCompanyActivityQuery(includeDeleted), companyKey)
	if err != nil {
		return out, fmt.Errorf("list ma company activity: %w", err)
	}
	for rows.Next() {
		var item MATargetOutcome
		var payload string
		var updatedAt, deletedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.SessionID, &item.InitiativeID, &item.CompanyKey, &item.Event, &item.Note, &payload,
			&item.CreatedByEmail, &item.CreatedAt, &updatedAt, &item.UpdatedByEmail, &deletedAt, &item.DeletedByEmail); err != nil {
			rows.Close()
			return out, fmt.Errorf("scan ma company activity: %w", err)
		}
		item.Payload = json.RawMessage(payload)
		if updatedAt.Valid {
			item.UpdatedAt = &updatedAt.Time
		}
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Close(); err != nil {
		return out, fmt.Errorf("close ma company activity: %w", err)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("iterate ma company activity: %w", err)
	}

	initiativeRows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT initiative.id::text, initiative.title
FROM binocolo.ma_target_outcome outcome /* activity-includes-deleted */
LEFT JOIN binocolo.ma_session session ON session.id = outcome.session_id
JOIN binocolo.ma_initiative initiative ON initiative.id = COALESCE(outcome.initiative_id, session.initiative_id)
WHERE outcome.company_key = $1
ORDER BY initiative.title, initiative.id::text`, companyKey)
	if err != nil {
		return out, fmt.Errorf("list ma company activity initiatives: %w", err)
	}
	for initiativeRows.Next() {
		var item MACompanyActivityLookup
		if err := initiativeRows.Scan(&item.ID, &item.Title); err != nil {
			initiativeRows.Close()
			return out, fmt.Errorf("scan ma company activity initiative: %w", err)
		}
		out.Initiatives = append(out.Initiatives, item)
	}
	if err := initiativeRows.Err(); err != nil {
		initiativeRows.Close()
		return out, fmt.Errorf("iterate ma company activity initiatives: %w", err)
	}
	if err := initiativeRows.Close(); err != nil {
		return out, fmt.Errorf("close ma company activity initiatives: %w", err)
	}

	sessionRows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT session.id::text, COALESCE(session.title, ''), COALESCE(session.initiative_id::text, '')
FROM binocolo.ma_session session
JOIN binocolo.ma_target_outcome outcome ON outcome.session_id = session.id
WHERE outcome.company_key = $1
ORDER BY COALESCE(session.title, ''), session.id::text`, companyKey)
	if err != nil {
		return out, fmt.Errorf("list ma company activity sessions: %w", err)
	}
	defer sessionRows.Close()
	for sessionRows.Next() {
		var item MACompanyActivitySession
		if err := sessionRows.Scan(&item.ID, &item.Title, &item.InitiativeID); err != nil {
			return out, fmt.Errorf("scan ma company activity session: %w", err)
		}
		out.Sessions = append(out.Sessions, item)
	}
	if err := sessionRows.Err(); err != nil {
		return out, fmt.Errorf("iterate ma company activity sessions: %w", err)
	}
	return out, nil
}

func (s *SQLStore) LatestMASystemDomainAnnotation(ctx context.Context, companyKey string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("binocolo ma store not configured")
	}
	var note string
	err := s.db.QueryRowContext(ctx, `
SELECT COALESCE(note, '')
FROM binocolo.ma_target_outcome
WHERE company_key = $1 AND event = 'nota' AND deleted_at IS NULL
  AND payload->>'annotationOrigin' = 'system_domain'
ORDER BY created_at DESC
LIMIT 1`, companyKey).Scan(&note)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("latest ma system domain annotation: %w", err)
	}
	return note, nil
}

// GetMACompanyDomain looks up the verified-domain registry by any of the
// company's stable identities. Manual entries win over auto-verified ones, an
// exact company_key match wins over an identifier match, freshest last-resort.
// A miss is (nil, nil) — only real DB failures return an error.
//
// Il confronto fiscale è sui valori NORMALIZZATI da entrambi i lati (issue
// #86, F0): confrontare i valori grezzi era un difetto latente destinato a
// manifestarsi il giorno in cui un valore fosse arrivato da fonte non-vendor,
// con una punteggiatura o un prefisso IT diversi.
func (s *SQLStore) GetMACompanyDomain(ctx context.Context, companyKey, vatCode, taxCode string) (*maCompanyDomain, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	companyKey = normalizeMACompanyKey(companyKey)
	vatCode = maStableVAT(vatCode)
	taxCode = normalizeMAFiscalValue(taxCode)
	if companyKey == "" && vatCode == "" && taxCode == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
SELECT company_key, vat_code, tax_code, company_name, domain, method, COALESCE(group_site, false), COALESCE(identity_state, '')
FROM binocolo.ma_company_domain
WHERE ($1 <> '' AND company_key = $1)
   OR ($2 <> '' AND binocolo.ma_stable_vat(vat_code) = $2)
   OR ($3 <> '' AND binocolo.ma_normalize_fiscal(tax_code) = $3)
ORDER BY (method IN ('manual', 'no_website')) DESC, (company_key = $1) DESC, verified_at DESC
LIMIT 1
`, companyKey, vatCode, taxCode)
	var rec maCompanyDomain
	if err := row.Scan(&rec.CompanyKey, &rec.VATCode, &rec.TaxCode, &rec.CompanyName, &rec.Domain, &rec.Method, &rec.GroupSite, &rec.IdentityState); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ma company domain: %w", err)
	}
	return &rec, nil
}

// UpsertMACompanyDomain writes a registry entry. The WHERE guard on conflict is
// the precedence rule: an automatic verification never overwrites an operator's
// manual association (manual overwrites anything, auto refreshes auto).
func (s *SQLStore) UpsertMACompanyDomain(ctx context.Context, record maCompanyDomain) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if record.CompanyKey == "" || record.Method == "" || (record.Method != maDomainMethodNoWebsite && record.Domain == "") {
		return errors.New("ma company domain: missing key, domain or method")
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_company_domain (
    company_key, vat_code, tax_code, company_name, domain, method, group_site, identity_state,
    verified_at, created_by_subject, created_by_email, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), now(), $9, $10, now(), now())
ON CONFLICT (company_key) DO UPDATE SET
    domain       = EXCLUDED.domain,
    method       = EXCLUDED.method,
    group_site   = EXCLUDED.group_site,
    identity_state = EXCLUDED.identity_state,
    vat_code     = CASE WHEN EXCLUDED.vat_code <> '' THEN EXCLUDED.vat_code ELSE binocolo.ma_company_domain.vat_code END,
    tax_code     = CASE WHEN EXCLUDED.tax_code <> '' THEN EXCLUDED.tax_code ELSE binocolo.ma_company_domain.tax_code END,
    company_name = CASE WHEN EXCLUDED.company_name <> '' THEN EXCLUDED.company_name ELSE binocolo.ma_company_domain.company_name END,
    verified_at  = now(),
    updated_at   = now()
WHERE NOT (binocolo.ma_company_domain.method IN ('manual', 'no_website') AND EXCLUDED.method = 'auto_verified')
`, record.CompanyKey, record.VATCode, record.TaxCode, record.CompanyName, record.Domain, record.Method,
		record.GroupSite, record.IdentityState, record.CreatedBySubject, record.CreatedByEmail); err != nil {
		return fmt.Errorf("upsert ma company domain: %w", err)
	}
	return nil
}

func (s *SQLStore) loadMAOutcomes(ctx context.Context, sessionID string) (map[string][]MATargetOutcome, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, company_key, event, COALESCE(note, ''), COALESCE(created_by_email, ''), created_at
FROM binocolo.ma_target_outcome
WHERE session_id = $1::uuid
  AND deleted_at IS NULL
ORDER BY created_at
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma outcomes: %w", err)
	}
	defer rows.Close()
	out := map[string][]MATargetOutcome{}
	for rows.Next() {
		var item MATargetOutcome
		if err := rows.Scan(&item.ID, &item.CompanyKey, &item.Event, &item.Note, &item.CreatedByEmail, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ma outcome: %w", err)
		}
		out[item.CompanyKey] = append(out[item.CompanyKey], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma outcomes: %w", err)
	}
	return out, nil
}

func (s *SQLStore) loadMAOutcomesForCompany(ctx context.Context, sessionID, companyKey string) ([]MATargetOutcome, error) {
	if companyKey == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, company_key, event, COALESCE(note, ''), COALESCE(created_by_email, ''), created_at
FROM binocolo.ma_target_outcome
WHERE session_id = $1::uuid AND company_key = $2
  AND deleted_at IS NULL
ORDER BY created_at
`, sessionID, companyKey)
	if err != nil {
		return nil, fmt.Errorf("load ma outcomes for company: %w", err)
	}
	defer rows.Close()
	out := []MATargetOutcome{}
	for rows.Next() {
		var item MATargetOutcome
		if err := rows.Scan(&item.ID, &item.CompanyKey, &item.Event, &item.Note, &item.CreatedByEmail, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ma outcome: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma outcomes for company: %w", err)
	}
	return out, nil
}

// GetMAInitiativeCard loads a single card by its (initiative, company) key. A
// miss is (nil, nil) — only real DB failures return an error, mirroring
// GetMACompanyDomain.
func (s *SQLStore) GetMAInitiativeCard(ctx context.Context, initiativeID, companyKey string) (*MAInitiativeCard, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	row := s.db.QueryRowContext(ctx, `
SELECT initiative_id::text, company_key, company_name, vat_code, tax_code, province,
       origin, state, COALESCE(esito, ''), COALESCE(created_from_session::text, ''),
       created_at, updated_at, closed_at, recontact_on, visit_on
FROM binocolo.ma_initiative_card
WHERE initiative_id = $1::uuid AND company_key = $2
`, initiativeID, companyKey)
	var card MAInitiativeCard
	var closedAt, recontactOn, visitOn sql.NullTime
	if err := row.Scan(&card.InitiativeID, &card.CompanyKey, &card.CompanyName, &card.VATCode, &card.TaxCode,
		&card.Province, &card.Origin, &card.State, &card.Esito, &card.CreatedFromSession, &card.CreatedAt, &card.UpdatedAt, &closedAt, &recontactOn, &visitOn); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ma initiative card: %w", err)
	}
	if closedAt.Valid {
		card.ClosedAt = &closedAt.Time
	}
	if recontactOn.Valid {
		card.RecontactOn = &recontactOn.Time
	}
	if visitOn.Valid {
		card.VisitOn = &visitOn.Time
	}
	return &card, nil
}

// maCardEventExec è soddisfatta da *sql.DB e *sql.Tx: la coppia di scritture
// card+evento gira o in autonomia (upsert/append semplici) o dentro un'unica
// transazione (SaveMAInitiativeCardWithOutcome, issue #201).
type maCardEventExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// UpsertMAInitiativeCard inserts or updates a card. Callers own the state
// machine (ensureInitiativeCard, B4 transitions) — this is a plain write.
func (s *SQLStore) UpsertMAInitiativeCard(ctx context.Context, card MAInitiativeCard) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	return upsertMAInitiativeCard(ctx, s.db, card)
}

// upsertMAInitiativeCard is the shared body of the plain write and of the
// transactional card+event save.
func upsertMAInitiativeCard(ctx context.Context, exec maCardEventExec, card MAInitiativeCard) error {
	if card.InitiativeID == "" || card.CompanyKey == "" {
		return errors.New("ma initiative card: missing initiative or company key")
	}
	var createdFromSession any
	if card.CreatedFromSession != "" {
		createdFromSession = card.CreatedFromSession
	}
	if _, err := exec.ExecContext(ctx, `
INSERT INTO binocolo.ma_initiative_card (
    initiative_id, company_key, company_name, vat_code, tax_code, province, origin,
    state, esito, created_from_session, created_at, updated_at, closed_at, recontact_on, visit_on)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), $10::uuid, now(), now(), $11, $12, $13)
ON CONFLICT (initiative_id, company_key) DO UPDATE SET
    company_name = CASE WHEN EXCLUDED.company_name <> '' THEN EXCLUDED.company_name ELSE binocolo.ma_initiative_card.company_name END,
    vat_code     = CASE WHEN EXCLUDED.vat_code <> '' THEN EXCLUDED.vat_code ELSE binocolo.ma_initiative_card.vat_code END,
    tax_code     = CASE WHEN EXCLUDED.tax_code <> '' THEN EXCLUDED.tax_code ELSE binocolo.ma_initiative_card.tax_code END,
    province     = CASE WHEN EXCLUDED.province <> '' THEN EXCLUDED.province ELSE binocolo.ma_initiative_card.province END,
    origin       = EXCLUDED.origin,
    state        = EXCLUDED.state,
    esito        = EXCLUDED.esito,
    updated_at   = now(),
    closed_at    = EXCLUDED.closed_at,
    recontact_on = EXCLUDED.recontact_on,
    visit_on     = EXCLUDED.visit_on
`, card.InitiativeID, card.CompanyKey, card.CompanyName, card.VATCode, card.TaxCode, card.Province,
		card.Origin, card.State, card.Esito, createdFromSession, card.ClosedAt, card.RecontactOn, card.VisitOn); err != nil {
		return fmt.Errorf("upsert ma initiative card: %w", err)
	}
	return nil
}

// SaveMAInitiativeCardWithOutcome persiste la card e l'evento di diario che la
// racconta in un'unica transazione (issue #201): o riescono entrambi o nessuno
// dei due. È il confine transazionale delle transizioni di stato card (state,
// close, remove, reopen e cambio della sola data visita).
func (s *SQLStore) SaveMAInitiativeCardWithOutcome(ctx context.Context, card MAInitiativeCard, outcome MATargetOutcome) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma card state save: %w", err)
	}
	defer tx.Rollback()
	if err := upsertMAInitiativeCard(ctx, tx, card); err != nil {
		return err
	}
	if err := insertMATargetOutcome(ctx, tx, outcome); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ma card state save: %w", err)
	}
	return nil
}

// ListMAInitiativeCards loads every card of an Iniziativa (board rows, B4).
func (s *SQLStore) ListMAInitiativeCards(ctx context.Context, initiativeID string) ([]MAInitiativeCard, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT initiative_id::text, company_key, company_name, vat_code, tax_code, province,
       origin, state, COALESCE(esito, ''), COALESCE(created_from_session::text, ''),
       created_at, updated_at, closed_at, recontact_on, visit_on
FROM binocolo.ma_initiative_card
WHERE initiative_id = $1::uuid
ORDER BY updated_at DESC
`, initiativeID)
	if err != nil {
		return nil, fmt.Errorf("list ma initiative cards: %w", err)
	}
	defer rows.Close()
	out := []MAInitiativeCard{}
	for rows.Next() {
		var card MAInitiativeCard
		var closedAt, recontactOn, visitOn sql.NullTime
		if err := rows.Scan(&card.InitiativeID, &card.CompanyKey, &card.CompanyName, &card.VATCode, &card.TaxCode,
			&card.Province, &card.Origin, &card.State, &card.Esito, &card.CreatedFromSession, &card.CreatedAt, &card.UpdatedAt, &closedAt, &recontactOn, &visitOn); err != nil {
			return nil, fmt.Errorf("scan ma initiative card: %w", err)
		}
		if closedAt.Valid {
			card.ClosedAt = &closedAt.Time
		}
		if recontactOn.Valid {
			card.RecontactOn = &recontactOn.Time
		}
		if visitOn.Valid {
			card.VisitOn = &visitOn.Time
		}
		out = append(out, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma initiative cards: %w", err)
	}
	return out, nil
}

// ListMALatestCardEvents returns the latest human-readable event label for
// each card (companyKey) in the initiative. Used by Q1 (semantic last
// activity) on the board table and drawer.
func (s *SQLStore) ListMALatestCardEvents(ctx context.Context, initiativeID string, companyKeys []string) (map[string]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	out := map[string]string{}
	if len(companyKeys) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(companyKeys))
	args := make([]any, len(companyKeys)+1)
	args[0] = initiativeID
	for i, key := range companyKeys {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = key
	}
	query := fmt.Sprintf(`
SELECT DISTINCT ON (o.company_key) o.company_key,
  CASE
    WHEN o.event = 'nota' AND COALESCE(o.note, '') != '' THEN 'Annotazione: ' || o.note
    WHEN o.event = 'nota' THEN 'Annotazione'
    WHEN o.event = 'stato' AND COALESCE(o.note, '') != '' THEN 'Stato aggiornato: ' || o.note
    WHEN o.event = 'stato' THEN 'Stato aggiornato'
    WHEN o.event = 'card_creata' AND COALESCE(o.note, '') != '' THEN 'Card creata: ' || o.note
    WHEN o.event = 'card_creata' THEN 'Card creata'
    WHEN o.event = 'chiusura' AND COALESCE(o.note, '') != '' THEN 'Chiusura: ' || o.note
    WHEN o.event = 'chiusura' THEN 'Chiusura'
    WHEN o.event = 'contattato' AND COALESCE(o.note, '') != '' THEN 'Contattata: ' || o.note
    WHEN o.event = 'contattato' THEN 'Contattata'
    WHEN o.event = 'card_rimossa' THEN 'Card rimossa'
    WHEN o.event = 'card_riaperta' THEN 'Card riaperta'
    WHEN o.event = 'dominio_verificato' AND o.payload->>'esito' = 'confermato' THEN 'Dominio confermato'
    WHEN o.event = 'dominio_verificato' AND o.payload->>'esito' = 'non_confermato' THEN 'Dominio non confermato'
    WHEN o.event = 'dominio_verificato' THEN 'Dominio non verificabile'
    WHEN o.event = 'buon_lead' AND COALESCE(o.note, '') != '' THEN 'Buon lead: ' || o.note
    WHEN o.event = 'buon_lead' THEN 'Buon lead'
    WHEN o.event = 'no_go' AND COALESCE(o.note, '') != '' THEN 'No-go: ' || o.note
    WHEN o.event = 'no_go' THEN 'No-go'
    ELSE COALESCE(o.event, '') || CASE WHEN COALESCE(o.note, '') != '' THEN ': ' || o.note ELSE '' END
  END AS event_label
FROM binocolo.ma_target_outcome o
WHERE o.initiative_id = $1::uuid
  AND o.company_key IN (%s)
  AND o.deleted_at IS NULL
  AND o.event NOT IN ('card_creata', 'dominio_verificato')
ORDER BY o.company_key, o.created_at DESC
`, strings.Join(placeholders, ", "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ma latest card events: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var companyKey, label string
		if err := rows.Scan(&companyKey, &label); err != nil {
			return nil, fmt.Errorf("scan ma latest card event: %w", err)
		}
		out[companyKey] = label
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma latest card events: %w", err)
	}
	return out, nil
}

// ListMAActiveCardsByCompany batches the collision/badge lookup (PRD §6.1,
// B5): for each company key, every ACTIVE card (state NOT IN chiusa/rimossa)
// across all operational initiatives (archived/deleted/purged initiatives are
// out of the working scene — their cards must not surface as collision markers).
func (s *SQLStore) ListMAActiveCardsByCompany(ctx context.Context, companyKeys []string) (map[string][]MAInitiativeCard, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	out := map[string][]MAInitiativeCard{}
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
SELECT c.initiative_id::text, c.company_key, c.company_name, c.vat_code, c.tax_code, c.province,
       c.origin, c.state, COALESCE(c.esito, ''), COALESCE(c.created_from_session::text, ''),
       c.created_at, c.updated_at, c.closed_at
FROM binocolo.ma_initiative_card c
JOIN binocolo.ma_initiative i ON i.id = c.initiative_id
WHERE c.company_key IN (%s)
  AND c.state NOT IN ('won', 'ko_nostro', 'ko_target', 'rimossa')
  AND i.archived_at IS NULL
  AND i.deleted_at IS NULL
  AND i.purged_at IS NULL
`, strings.Join(placeholders, ", "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ma active cards by company: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var card MAInitiativeCard
		var closedAt sql.NullTime
		if err := rows.Scan(&card.InitiativeID, &card.CompanyKey, &card.CompanyName, &card.VATCode, &card.TaxCode,
			&card.Province, &card.Origin, &card.State, &card.Esito, &card.CreatedFromSession, &card.CreatedAt, &card.UpdatedAt, &closedAt); err != nil {
			return nil, fmt.Errorf("scan ma active card: %w", err)
		}
		if closedAt.Valid {
			card.ClosedAt = &closedAt.Time
		}
		out[card.CompanyKey] = append(out[card.CompanyKey], card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma active cards by company: %w", err)
	}
	return out, nil
}

// errMACompanyFactActive signals a violation of ma_company_fact_active_idx
// (mig 090): a fact of this kind is already active for the company. The
// service maps it to a 400 (INIZIATIVE-PRD.md §6, B3 done-when).
var errMACompanyFactActive = errors.New("ma company fact already active")

// InsertMACompanyFact records a new typed fact in the company registry (mig
// 090). Violating the (company_key, kind) active-uniqueness returns
// errMACompanyFactActive.
func (s *SQLStore) InsertMACompanyFact(ctx context.Context, fact MACompanyFact) (MACompanyFact, error) {
	if s == nil || s.db == nil {
		return MACompanyFact{}, errors.New("binocolo ma store not configured")
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_company_fact (
    id, company_key, vat_code, tax_code, company_name, kind, note,
    created_by_subject, created_by_email)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id::text, company_key, vat_code, tax_code, company_name, kind, note,
          created_by_subject, created_by_email, created_at
`, fact.ID, fact.CompanyKey, fact.VATCode, fact.TaxCode, fact.CompanyName, fact.Kind, fact.Note,
		fact.CreatedBySubject, fact.CreatedByEmail)
	var out MACompanyFact
	if err := row.Scan(&out.ID, &out.CompanyKey, &out.VATCode, &out.TaxCode, &out.CompanyName, &out.Kind, &out.Note,
		&out.CreatedBySubject, &out.CreatedByEmail, &out.CreatedAt); err != nil {
		if strings.Contains(err.Error(), "ma_company_fact_active_idx") || strings.Contains(err.Error(), "duplicate key") {
			return MACompanyFact{}, errMACompanyFactActive
		}
		return MACompanyFact{}, fmt.Errorf("insert ma company fact: %w", err)
	}
	return out, nil
}

// RevokeMACompanyFact revokes an active fact by id. Returns false when no
// active fact with that id exists (already revoked or unknown id).
func (s *SQLStore) RevokeMACompanyFact(ctx context.Context, id, subject, email, note string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	var returnedID string
	err := s.db.QueryRowContext(ctx, `
UPDATE binocolo.ma_company_fact
SET revoked_at = now(),
    revoked_by_subject = $2,
    revoked_by_email = $3,
    revoke_note = $4
WHERE id = $1::uuid
  AND revoked_at IS NULL
RETURNING id::text
`, id, subject, email, note).Scan(&returnedID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("revoke ma company fact: %w", err)
	}
	return true, nil
}

// GetMACompanyRegistry loads registry facts (active and revoked) for the
// dossier §6 section and the F4/board reads.
func (s *SQLStore) GetMACompanyRegistry(ctx context.Context, companyKey string) (MACompanyRegistry, error) {
	if s == nil || s.db == nil {
		return MACompanyRegistry{}, errors.New("binocolo ma store not configured")
	}
	factRows, err := s.db.QueryContext(ctx, `
SELECT id::text, company_key, vat_code, tax_code, company_name, kind, note,
       created_by_subject, created_by_email, created_at,
       revoked_at, COALESCE(revoked_by_subject, ''), COALESCE(revoked_by_email, ''), revoke_note
FROM binocolo.ma_company_fact
WHERE company_key = $1
ORDER BY created_at DESC
`, companyKey)
	if err != nil {
		return MACompanyRegistry{}, fmt.Errorf("list ma company facts: %w", err)
	}
	defer factRows.Close()
	out := MACompanyRegistry{Facts: []MACompanyFact{}}
	for factRows.Next() {
		var fact MACompanyFact
		var revokedAt sql.NullTime
		if err := factRows.Scan(&fact.ID, &fact.CompanyKey, &fact.VATCode, &fact.TaxCode, &fact.CompanyName, &fact.Kind, &fact.Note,
			&fact.CreatedBySubject, &fact.CreatedByEmail, &fact.CreatedAt,
			&revokedAt, &fact.RevokedBySubject, &fact.RevokedByEmail, &fact.RevokeNote); err != nil {
			return MACompanyRegistry{}, fmt.Errorf("scan ma company fact: %w", err)
		}
		if revokedAt.Valid {
			fact.RevokedAt = &revokedAt.Time
		}
		out.Facts = append(out.Facts, fact)
	}
	if err := factRows.Err(); err != nil {
		return MACompanyRegistry{}, fmt.Errorf("iterate ma company facts: %w", err)
	}

	return out, nil
}

func resolveMACompanySearchStatus(row maCompanySearchHydration) MACompanySearchStatus {
	if row.ActiveCardState != "" {
		return MACompanySearchStatus{Kind: "working", Value: row.ActiveCardState, ContextTitle: row.ActiveCardInitiative}
	}
	if row.ClosedCardState != "" {
		// v2: il "closed" porta lo STATO terminale come Value (won/ko_nostro/
		// ko_target) e l'esito in Reason — più informativo dell'esito solo (che
		// per WON è vuoto). KANBAN-V2-PLAN.md §B1.6.
		return MACompanySearchStatus{Kind: "closed", Value: row.ClosedCardState, Reason: row.ClosedCardEsito, ContextTitle: row.ClosedCardInitiative}
	}
	if row.LatestRating != nil {
		if *row.LatestRating == -1 {
			return MACompanySearchStatus{Kind: "excluded", Reason: row.LatestRatingReason}
		}
		return MACompanySearchStatus{Kind: "preferred", Value: fmt.Sprintf("%d", *row.LatestRating)}
	}
	if row.LatestTarget != nil {
		decision := maRouteTarget(rowAsTarget(*row.LatestTarget), row.LatestTargetThesis)
		status := MACompanySearchStatus{Value: decision.Bucket}
		switch decision.Bucket {
		case maBucketPrincipale:
			status.Kind = "thesis"
		case maBucketDaVerificare:
			status.Kind = "review"
		case maBucketAzionabile:
			status.Kind = "actionable"
		case maBucketSoppresso:
			status.Kind = "suppressed"
		default:
			status.Kind = "review"
		}
		if decision.Reason != nil {
			status.Reason = decision.Reason.Label
		} else if decision.SuppressedReason != nil {
			status.Reason = decision.SuppressedReason.Label
		}
		return status
	}
	return MACompanySearchStatus{Kind: "registry"}
}

func (s *SQLStore) GetMACompanyOverviewIdentity(ctx context.Context, companyKey string) (*MACompanyOverviewIdentity, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
WITH identities AS (
  SELECT
    COALESCE(company_key, '') AS company_key,
    COALESCE(company_name, '') AS company_name, COALESCE(vat_code, '') AS vat_code,
    COALESCE(tax_code, '') AS tax_code, COALESCE(province, '') AS province,
    COALESCE(town, '') AS town, COALESCE(ateco_code, '') AS ateco_code,
    COALESCE(ateco_description, '') AS ateco_description, created_at, 1 AS priority
  FROM binocolo.ma_target
  UNION ALL
  SELECT company_key, company_name, vat_code, tax_code, province, '', '', '', updated_at, 2
  FROM binocolo.ma_initiative_card
)
SELECT company_key, company_name, vat_code, tax_code, province, town, ateco_code, ateco_description
FROM identities
WHERE company_key = $1
ORDER BY priority, created_at DESC
LIMIT 1
`, companyKey)
	var identity MACompanyOverviewIdentity
	if err := row.Scan(&identity.CompanyKey, &identity.CompanyName, &identity.VATCode, &identity.TaxCode, &identity.Province, &identity.Town, &identity.AtecoCode, &identity.AtecoDescription); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ma company overview identity: %w", err)
	}
	domain, err := s.GetMACompanyDomain(ctx, identity.CompanyKey, identity.VATCode, identity.TaxCode)
	if err != nil {
		return nil, err
	}
	if domain != nil {
		identity.Domain = domain.Domain
		identity.DomainMethod = domain.Method
		identity.IdentityState = domain.IdentityState
	}
	return &identity, nil
}

func (s *SQLStore) ListMACompanyOverviewAppearances(ctx context.Context, companyKey string) ([]MACompanyOverviewAppearance, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return []MACompanyOverviewAppearance{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `
WITH target_rows AS (
  SELECT t.*, COALESCE(t.company_key, '') AS resolved_company_key
  FROM binocolo.ma_target t
), outcome_rows AS (
  SELECT session_id, company_key, jsonb_agg(jsonb_build_object(
    'id', id::text,
    'sessionId', COALESCE(session_id::text, ''),
    'initiativeId', COALESCE(initiative_id::text, ''),
    'companyKey', company_key,
    'event', event,
    'note', COALESCE(note, ''),
    'payload', COALESCE(payload, '{}'::jsonb),
    'createdByEmail', COALESCE(created_by_email, ''),
    'createdAt', created_at,
    'updatedAt', updated_at,
    'updatedByEmail', COALESCE(updated_by_email, ''),
    'deletedAt', deleted_at,
    'deletedByEmail', COALESCE(deleted_by_email, '')
  ) ORDER BY created_at DESC) AS outcomes
  FROM binocolo.ma_target_outcome
  WHERE company_key = $1 AND session_id IS NOT NULL
    AND deleted_at IS NULL
  GROUP BY session_id, company_key
)
SELECT
  session.id::text, COALESCE(session.title, ''), session.status,
  COALESCE(session.initiative_id::text, ''), COALESCE(initiative.title, ''),
  t.id::text, t.run_id::text, t.score, t.score_version, COALESCE(t.match_state, ''), COALESCE(t.confidence, ''), COALESCE(t.origin, 'search'),
  COALESCE(t.company_name, ''), COALESCE(t.vat_code, ''), COALESCE(t.province, ''), COALESCE(t.town, ''), COALESCE(t.ateco_code, ''),
  t.flags, COALESCE(t.enrichment_level, 'advanced'), r.rating, r.score_at_rating, COALESCE(r.confidence_at_rating, ''),
  r.rated_at, COALESCE(r.reason, ''), COALESCE(outcome_rows.outcomes, '[]'::jsonb), t.created_at,
  COALESCE(strategy.strategy->>'thesis', ''),
  wv.company_key IS NOT NULL AS has_validation,
  COALESCE(wv.web_validation_state, ''),
  COALESCE(wv.final_action, ''),
  COALESCE(wv.selected_domain, ''),
  COALESCE(wv.final_decision->>'reason', ''),
  COALESCE(wv.domain_response->'groupSiteHint'->>'domain', ''),
  COALESCE(wv.domain_response->'groupSiteHint'->>'identifier', ''),
  COALESCE(jsonb_array_length(
    CASE
      WHEN jsonb_typeof(wv.domain_response->'candidates') = 'array' THEN wv.domain_response->'candidates'
      ELSE '[]'::jsonb
    END
  ), 0),
  outside_filter.revenue_per_employee_value,
  outside_filter.max_shareholders_value
FROM target_rows t
JOIN binocolo.ma_session session ON session.id = t.session_id
LEFT JOIN LATERAL (
  SELECT
    MAX(COALESCE(evidence.value, '')) FILTER (WHERE evidence.criterion = $3) AS revenue_per_employee_value,
    MAX(COALESCE(evidence.value, '')) FILTER (WHERE evidence.criterion = $4) AS max_shareholders_value
  FROM binocolo.ma_evidence evidence
  WHERE evidence.target_id = t.id
    AND evidence.status = $2
    AND evidence.criterion IN ($3, $4)
) outside_filter ON TRUE
LEFT JOIN binocolo.ma_strategy_version strategy ON strategy.id = session.active_strategy_id
LEFT JOIN binocolo.ma_initiative initiative ON initiative.id = session.initiative_id
LEFT JOIN binocolo.ma_target_web_validation wv ON wv.session_id = t.session_id AND wv.company_key = t.resolved_company_key
LEFT JOIN binocolo.ma_target_rating r ON r.session_id = t.session_id AND r.company_key = t.resolved_company_key
LEFT JOIN outcome_rows ON outcome_rows.session_id = t.session_id AND outcome_rows.company_key = t.resolved_company_key
WHERE t.resolved_company_key = $1
ORDER BY t.created_at DESC, session.id DESC, t.id DESC
`, companyKey, maEvidenceOutside, maPostFilterRevenuePerEmployeeMin, maPostFilterMaxShareholders)
	if err != nil {
		return nil, fmt.Errorf("list ma company overview appearances: %w", err)
	}
	defer rows.Close()
	out := []MACompanyOverviewAppearance{}
	for rows.Next() {
		var item MACompanyOverviewAppearance
		var targetRow MATargetRow
		var score sql.NullInt64
		var scoreVersion sql.NullInt64
		var rating sql.NullInt64
		var scoreAt sql.NullInt64
		var ratedAt sql.NullTime
		var thesis string
		var flagsRaw []byte
		var outcomesRaw []byte
		var hasValidation bool
		var webState, finalAction, selectedDomain, reason string
		var groupDomain, groupIdentifier string
		var candidateCount int
		var outsideRevenue, outsideShareholders sql.NullString
		if err := rows.Scan(&item.SessionID, &item.SessionTitle, &item.SessionStatus, &item.InitiativeID, &item.InitiativeTitle,
			&item.TargetID, &targetRow.RunID, &score, &scoreVersion, &targetRow.MatchState, &targetRow.Confidence, &targetRow.Origin,
			&targetRow.CompanyName, &targetRow.VATCode, &targetRow.Province, &targetRow.Town, &targetRow.AtecoCode,
			&flagsRaw, &targetRow.EnrichmentLevel, &rating, &scoreAt,
			&item.ConfidenceAtRating, &ratedAt, &item.ExclusionReason, &outcomesRaw, &item.CreatedAt,
			&thesis, &hasValidation, &webState, &finalAction, &selectedDomain, &reason, &groupDomain, &groupIdentifier,
			&candidateCount, &outsideRevenue, &outsideShareholders); err != nil {
			return nil, fmt.Errorf("scan ma company overview appearance: %w", err)
		}
		targetRow.ID = item.TargetID
		if score.Valid {
			item.Score = int(score.Int64)
			targetRow.Score = int(score.Int64)
		}
		if scoreVersion.Valid {
			value := int(scoreVersion.Int64)
			targetRow.ScoreVersion = &value
		}
		if rating.Valid {
			v := int(rating.Int64)
			item.Rating = &v
			targetRow.Rating = &v
		}
		if outsideRevenue.Valid {
			value := outsideRevenue.String
			targetRow.OutsideRevenuePerEmployeeValue = &value
		}
		if outsideShareholders.Valid {
			value := outsideShareholders.String
			targetRow.OutsideMaxShareholdersValue = &value
		}
		if scoreAt.Valid {
			v := int(scoreAt.Int64)
			item.ScoreAtRating = &v
		}
		if ratedAt.Valid {
			item.RatedAt = &ratedAt.Time
		}
		if len(flagsRaw) > 0 {
			_ = json.Unmarshal(flagsRaw, &targetRow.Flags)
		}
		if hasValidation {
			targetRow.WebValidation = &MATargetRowWeb{
				WebValidationState:  webState,
				FinalAction:         finalAction,
				SelectedDomain:      selectedDomain,
				FinalDecision:       MATargetRowFinalDecision{Reason: reason},
				GroupSiteDomain:     groupDomain,
				GroupSiteIdentifier: groupIdentifier,
				CandidateCount:      candidateCount,
			}
		}
		decision := maRouteTarget(rowAsTarget(targetRow), thesis)
		item.Bucket = decision.Bucket
		item.BucketReason = decision.Reason
		item.SuppressedReason = decision.SuppressedReason
		if len(outcomesRaw) > 0 {
			_ = json.Unmarshal(outcomesRaw, &item.Outcomes)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma company overview appearances: %w", err)
	}
	return out, nil
}

func (s *SQLStore) ListMACompanyOverviewCards(ctx context.Context, companyKey string) ([]MACompanyOverviewCard, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return []MACompanyOverviewCard{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT c.initiative_id::text, COALESCE(i.title, ''), c.company_key, c.company_name, c.origin, c.state, COALESCE(c.esito, ''),
       COALESCE(c.created_from_session::text, ''), c.created_at, c.updated_at, c.closed_at
FROM binocolo.ma_initiative_card c
JOIN binocolo.ma_initiative i ON i.id = c.initiative_id
WHERE c.company_key = $1
  AND i.deleted_at IS NULL
ORDER BY c.updated_at DESC, c.created_at DESC
`, companyKey)
	if err != nil {
		return nil, fmt.Errorf("list ma company overview cards: %w", err)
	}
	defer rows.Close()
	out := []MACompanyOverviewCard{}
	for rows.Next() {
		var item MACompanyOverviewCard
		var closedAt sql.NullTime
		if err := rows.Scan(&item.InitiativeID, &item.InitiativeTitle, &item.CompanyKey, &item.CompanyName, &item.Origin, &item.State, &item.Esito,
			&item.CreatedFromSession, &item.CreatedAt, &item.UpdatedAt, &closedAt); err != nil {
			return nil, fmt.Errorf("scan ma company overview card: %w", err)
		}
		if closedAt.Valid {
			item.ClosedAt = &closedAt.Time
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma company overview cards: %w", err)
	}
	return out, nil
}

// ListMACompanyFactsActive batches the badge lookup (B5): for each company
// key, the kinds of every currently active fact.
func (s *SQLStore) ListMACompanyFactsActive(ctx context.Context, companyKeys []string) (map[string][]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	out := map[string][]string{}
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
SELECT company_key, kind
FROM binocolo.ma_company_fact
WHERE company_key IN (%s)
  AND revoked_at IS NULL
`, strings.Join(placeholders, ", "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ma company facts active: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var companyKey, kind string
		if err := rows.Scan(&companyKey, &kind); err != nil {
			return nil, fmt.Errorf("scan ma company fact active: %w", err)
		}
		out[companyKey] = append(out[companyKey], kind)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma company facts active: %w", err)
	}
	return out, nil
}

// ListMASessionsByInitiative loads the summaries of every session anchored to
// an Iniziativa (B4 board header: chips). Reuses the same shape as
// ListMASessions so the client renders both with one component.
func (s *SQLStore) ListMASessionsByInitiative(ctx context.Context, initiativeID string) ([]MASessionSummary, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT
  session.id::text,
  session.title,
  session.prompt,
  session.status,
  session.selected_strategy,
  COALESCE((SELECT SUM(estimated_count) FROM binocolo.ma_dry_run_estimate estimate WHERE estimate.session_id = session.id AND estimate.selected), 0) AS estimated_count,
  COALESCE((SELECT SUM(estimated_cost) FROM binocolo.ma_dry_run_estimate estimate WHERE estimate.session_id = session.id AND estimate.selected), 0) AS estimated_cost,
  COALESCE((SELECT COUNT(*) FROM binocolo.ma_target target WHERE target.session_id = session.id), 0) AS result_count,
  session.created_at,
  session.updated_at,
  session.last_executed_at,
  session.archived_at,
  session.archived_by_email,
  session.deleted_at,
  session.deleted_by_email,
  session.initiative_id::text,
  initiative.title
FROM binocolo.ma_session session
LEFT JOIN binocolo.ma_initiative initiative ON initiative.id = session.initiative_id
WHERE session.initiative_id = $1::uuid AND session.deleted_at IS NULL
ORDER BY session.updated_at DESC, session.created_at DESC
`, initiativeID)
	if err != nil {
		return nil, fmt.Errorf("list ma sessions by initiative: %w", err)
	}
	defer rows.Close()

	out := []MASessionSummary{}
	for rows.Next() {
		var item MASessionSummary
		var selected sql.NullString
		var lastRun sql.NullTime
		var archivedAt sql.NullTime
		var archivedByEmail sql.NullString
		var deletedAt sql.NullTime
		var deletedByEmail sql.NullString
		var initID sql.NullString
		var initTitle sql.NullString
		if err := rows.Scan(
			&item.ID, &item.Title, &item.Prompt, &item.Status, &selected,
			&item.EstimatedCount, &item.EstimatedCost, &item.ResultCount,
			&item.CreatedAt, &item.UpdatedAt, &lastRun,
			&archivedAt, &archivedByEmail, &deletedAt, &deletedByEmail,
			&initID, &initTitle,
		); err != nil {
			return nil, fmt.Errorf("scan ma session by initiative: %w", err)
		}
		item.SelectedStrategy = selected.String
		item.InitiativeID = initID.String
		item.InitiativeTitle = initTitle.String
		if lastRun.Valid {
			item.LastRunAt = &lastRun.Time
		}
		if archivedAt.Valid {
			item.ArchivedAt = &archivedAt.Time
		}
		item.ArchivedByEmail = archivedByEmail.String
		if deletedAt.Valid {
			item.DeletedAt = &deletedAt.Time
		}
		item.DeletedByEmail = deletedByEmail.String
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma sessions by initiative: %w", err)
	}
	return out, nil
}

// ListMACardProvenances batches the provenance lookup for the board (B4
// passo 1): every ≥1★ rating on the sessions of this Iniziativa, for the
// given company keys, in ONE query — not N. Ordered newest-first so callers
// that want "most recent" just take index 0.
func (s *SQLStore) ListMACardProvenances(ctx context.Context, initiativeID string, companyKeys []string) (map[string][]MACardProvenance, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	out := map[string][]MACardProvenance{}
	if len(companyKeys) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(companyKeys))
	args := make([]any, 0, len(companyKeys)+1)
	args = append(args, initiativeID)
	for i, key := range companyKeys {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, key)
	}
	query := fmt.Sprintf(`
SELECT r.session_id::text, COALESCE(session.title, ''), r.company_key, r.rating, r.score_at_rating, r.confidence_at_rating, r.rated_at
FROM binocolo.ma_target_rating r
JOIN binocolo.ma_session session ON session.id = r.session_id
WHERE session.initiative_id = $1::uuid
  AND r.company_key IN (%s)
  AND r.rating >= 1
ORDER BY r.rated_at DESC
`, strings.Join(placeholders, ", "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ma card provenances: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item MACardProvenance
		var companyKey string
		var scoreAt sql.NullInt64
		var confidenceAt sql.NullString
		if err := rows.Scan(&item.SessionID, &item.SessionTitle, &companyKey, &item.Rating, &scoreAt, &confidenceAt, &item.RatedAt); err != nil {
			return nil, fmt.Errorf("scan ma card provenance: %w", err)
		}
		if scoreAt.Valid {
			v := int(scoreAt.Int64)
			item.ScoreAtRating = &v
		}
		item.ConfidenceAtRating = confidenceAt.String
		out[companyKey] = append(out[companyKey], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma card provenances: %w", err)
	}
	return out, nil
}

// ListMASectorEvalLabels loads the human ground-truth sector labels for a session,
// keyed by company_key. Used by the sector-eval harness to compare against predictions.
func (s *SQLStore) ListMASectorEvalLabels(ctx context.Context, sessionID string) (map[string]MASectorEvalLabel, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key, label, COALESCE(note, ''), COALESCE(labeled_by_email, ''), updated_at
FROM binocolo.ma_sector_eval_label
WHERE session_id = $1::uuid
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma sector eval labels: %w", err)
	}
	defer rows.Close()
	out := map[string]MASectorEvalLabel{}
	for rows.Next() {
		var v MASectorEvalLabel
		if err := rows.Scan(&v.CompanyKey, &v.Label, &v.Note, &v.LabeledByEmail, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ma sector eval label: %w", err)
		}
		out[v.CompanyKey] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma sector eval labels: %w", err)
	}
	return out, nil
}

// UpsertMASectorEvalLabel saves (or, with an empty label, clears) the human ground-truth
// sector label for a company in a session.
func (s *SQLStore) UpsertMASectorEvalLabel(ctx context.Context, sessionID, companyKey, label, note, subject, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if label == "" {
		if _, err := s.db.ExecContext(ctx, `
DELETE FROM binocolo.ma_sector_eval_label
WHERE session_id = $1::uuid AND company_key = $2
`, sessionID, companyKey); err != nil {
			return fmt.Errorf("clear ma sector eval label: %w", err)
		}
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_sector_eval_label (session_id, company_key, label, note, labeled_by_subject, labeled_by_email, updated_at)
VALUES ($1::uuid, $2, $3, $4, $5, $6, now())
ON CONFLICT (session_id, company_key) DO UPDATE
SET label = EXCLUDED.label,
    note = EXCLUDED.note,
    labeled_by_subject = EXCLUDED.labeled_by_subject,
    labeled_by_email = EXCLUDED.labeled_by_email,
    updated_at = now()
`, sessionID, companyKey, label, nullString(note), nullString(subject), nullString(email)); err != nil {
		return fmt.Errorf("upsert ma sector eval label: %w", err)
	}
	return nil
}

func (s *SQLStore) loadMAWebValidations(ctx context.Context, sessionID string) (map[string]MAWebValidation, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT
  session_id::text,
  company_key,
  COALESCE(target_id::text, ''),
  COALESCE(run_id::text, ''),
  COALESCE(selected_domain, ''),
  COALESCE(domain_confidence, ''),
  domain_score,
  web_score,
  COALESCE(web_confidence, ''),
  web_validation_state,
  final_action,
  COALESCE(analyst_verdict, ''),
  COALESCE(analyst_action, ''),
  COALESCE(analyst_confidence, ''),
  pipeline_version,
  input_hash,
  keyword_set_hash,
  COALESCE(llm_model_id, ''),
  COALESCE(llm_prompt_id, ''),
  COALESCE(llm_model, ''),
  stale_after,
  expires_at,
  summary,
  keyword_set,
  selected_domain_payload,
  domain_response,
  evidence_runs,
  candidate_match_analysis,
  COALESCE(candidate_match_error, ''),
  final_decision,
  COALESCE(identity_state, ''),
  COALESCE(updated_by_email, ''),
  updated_at
FROM binocolo.ma_target_web_validation
WHERE session_id = $1::uuid
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma web validations: %w", err)
	}
	defer rows.Close()
	out := map[string]MAWebValidation{}
	for rows.Next() {
		item, err := scanMAWebValidation(rows)
		if err != nil {
			return nil, err
		}
		out[item.CompanyKey] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma web validations: %w", err)
	}
	return out, nil
}

func (s *SQLStore) loadMAWebValidationForCompany(ctx context.Context, sessionID, companyKey string) (*MAWebValidation, error) {
	if companyKey == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
SELECT
  session_id::text,
  company_key,
  COALESCE(target_id::text, ''),
  COALESCE(run_id::text, ''),
  COALESCE(selected_domain, ''),
  COALESCE(domain_confidence, ''),
  domain_score,
  web_score,
  COALESCE(web_confidence, ''),
  web_validation_state,
  final_action,
  COALESCE(analyst_verdict, ''),
  COALESCE(analyst_action, ''),
  COALESCE(analyst_confidence, ''),
  pipeline_version,
  input_hash,
  keyword_set_hash,
  COALESCE(llm_model_id, ''),
  COALESCE(llm_prompt_id, ''),
  COALESCE(llm_model, ''),
  stale_after,
  expires_at,
  summary,
  keyword_set,
  selected_domain_payload,
  domain_response,
  evidence_runs,
  candidate_match_analysis,
  COALESCE(candidate_match_error, ''),
  final_decision,
  COALESCE(identity_state, ''),
  COALESCE(updated_by_email, ''),
  updated_at
FROM binocolo.ma_target_web_validation
WHERE session_id = $1::uuid AND company_key = $2
`, sessionID, companyKey)
	item, err := scanMAWebValidation(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *SQLStore) UpsertMAWebValidation(ctx context.Context, input maWebValidationUpsert) (MAWebValidation, error) {
	if s == nil || s.db == nil {
		return MAWebValidation{}, errors.New("binocolo ma store not configured")
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_target_web_validation (
  session_id,
  company_key,
  target_id,
  run_id,
  selected_domain,
  domain_confidence,
  domain_score,
  web_score,
  web_confidence,
  web_validation_state,
  final_action,
  analyst_verdict,
  analyst_action,
  analyst_confidence,
  pipeline_version,
  input_hash,
  keyword_set_hash,
  llm_model_id,
  llm_prompt_id,
  llm_model,
  stale_after,
  expires_at,
  summary,
  keyword_set,
  selected_domain_payload,
  domain_response,
  evidence_runs,
  candidate_match_analysis,
  candidate_match_error,
  final_decision,
  identity_state,
  created_by_subject,
  created_by_email,
  updated_by_subject,
  updated_by_email
) VALUES (
  $1::uuid, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, $6, $7, $8, $9, $10,
  $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23::jsonb, $24::jsonb,
  $25::jsonb, $26::jsonb, $27::jsonb, $28::jsonb, $29, $30::jsonb, $33, $31, $32, $31, $32
)
ON CONFLICT (session_id, company_key) DO UPDATE
SET target_id = EXCLUDED.target_id,
    run_id = EXCLUDED.run_id,
    selected_domain = EXCLUDED.selected_domain,
    domain_confidence = EXCLUDED.domain_confidence,
    domain_score = EXCLUDED.domain_score,
    web_score = EXCLUDED.web_score,
    web_confidence = EXCLUDED.web_confidence,
    web_validation_state = EXCLUDED.web_validation_state,
    final_action = EXCLUDED.final_action,
    analyst_verdict = EXCLUDED.analyst_verdict,
    analyst_action = EXCLUDED.analyst_action,
    analyst_confidence = EXCLUDED.analyst_confidence,
    pipeline_version = EXCLUDED.pipeline_version,
    input_hash = EXCLUDED.input_hash,
    keyword_set_hash = EXCLUDED.keyword_set_hash,
    llm_model_id = EXCLUDED.llm_model_id,
    llm_prompt_id = EXCLUDED.llm_prompt_id,
    llm_model = EXCLUDED.llm_model,
    stale_after = EXCLUDED.stale_after,
    expires_at = EXCLUDED.expires_at,
    summary = EXCLUDED.summary,
    keyword_set = EXCLUDED.keyword_set,
    selected_domain_payload = EXCLUDED.selected_domain_payload,
    domain_response = EXCLUDED.domain_response,
    evidence_runs = EXCLUDED.evidence_runs,
    candidate_match_analysis = EXCLUDED.candidate_match_analysis,
    candidate_match_error = EXCLUDED.candidate_match_error,
    final_decision = EXCLUDED.final_decision,
    identity_state = EXCLUDED.identity_state,
    updated_by_subject = EXCLUDED.updated_by_subject,
    updated_by_email = EXCLUDED.updated_by_email,
    updated_at = now()
RETURNING
  session_id::text,
  company_key,
  COALESCE(target_id::text, ''),
  COALESCE(run_id::text, ''),
  COALESCE(selected_domain, ''),
  COALESCE(domain_confidence, ''),
  domain_score,
  web_score,
  COALESCE(web_confidence, ''),
  web_validation_state,
  final_action,
  COALESCE(analyst_verdict, ''),
  COALESCE(analyst_action, ''),
  COALESCE(analyst_confidence, ''),
  pipeline_version,
  input_hash,
  keyword_set_hash,
  COALESCE(llm_model_id, ''),
  COALESCE(llm_prompt_id, ''),
  COALESCE(llm_model, ''),
  stale_after,
  expires_at,
  summary,
  keyword_set,
  selected_domain_payload,
  domain_response,
  evidence_runs,
  candidate_match_analysis,
  COALESCE(candidate_match_error, ''),
  final_decision,
  COALESCE(identity_state, ''),
  COALESCE(updated_by_email, ''),
  updated_at
`, input.SessionID,
		input.CompanyKey,
		input.TargetID,
		input.RunID,
		nullString(input.SelectedDomain),
		nullString(input.DomainConfidence),
		nullInt(input.DomainScore),
		input.WebScore,
		nullString(input.WebConfidence),
		input.WebValidationState,
		input.FinalAction,
		nullString(input.AnalystVerdict),
		nullString(input.AnalystAction),
		nullString(input.AnalystConfidence),
		input.PipelineVersion,
		input.InputHash,
		input.KeywordSetHash,
		nullString(input.LLMModelID),
		nullString(input.LLMPromptID),
		nullString(input.LLMModel),
		input.StaleAfter,
		input.ExpiresAt,
		[]byte(input.Summary),
		[]byte(input.KeywordSet),
		[]byte(input.SelectedDomainPayload),
		[]byte(input.DomainResponse),
		[]byte(input.EvidenceRuns),
		[]byte(input.CandidateMatchAnalysis),
		nullString(input.CandidateMatchError),
		[]byte(input.FinalDecision),
		nullString(input.Subject),
		nullString(input.Email),
		nullString(input.IdentityState),
	)
	return scanMAWebValidation(row)
}

type maWebValidationScanner interface {
	Scan(dest ...any) error
}

func scanMAWebValidation(row maWebValidationScanner) (MAWebValidation, error) {
	var item MAWebValidation
	var domainScore sql.NullInt64
	var summaryRaw, keywordSetRaw, selectedDomainRaw, domainResponseRaw, evidenceRunsRaw, analysisRaw, finalDecisionRaw []byte
	if err := row.Scan(
		&item.SessionID,
		&item.CompanyKey,
		&item.TargetID,
		&item.RunID,
		&item.SelectedDomain,
		&item.DomainConfidence,
		&domainScore,
		&item.WebScore,
		&item.WebConfidence,
		&item.WebValidationState,
		&item.FinalAction,
		&item.AnalystVerdict,
		&item.AnalystAction,
		&item.AnalystConfidence,
		&item.PipelineVersion,
		&item.InputHash,
		&item.KeywordSetHash,
		&item.LLMModelID,
		&item.LLMPromptID,
		&item.LLMModel,
		&item.StaleAfter,
		&item.ExpiresAt,
		&summaryRaw,
		&keywordSetRaw,
		&selectedDomainRaw,
		&domainResponseRaw,
		&evidenceRunsRaw,
		&analysisRaw,
		&item.CandidateMatchError,
		&finalDecisionRaw,
		&item.IdentityState,
		&item.UpdatedByEmail,
		&item.UpdatedAt,
	); err != nil {
		return MAWebValidation{}, fmt.Errorf("scan ma web validation: %w", err)
	}
	if domainScore.Valid {
		value := int(domainScore.Int64)
		item.DomainScore = &value
	}
	item.Freshness = maWebValidationFreshness(time.Now(), item.StaleAfter, item.ExpiresAt)
	_ = json.Unmarshal(summaryRaw, &item.Summary)
	_ = json.Unmarshal(keywordSetRaw, &item.KeywordSet)
	if len(selectedDomainRaw) > 0 && string(selectedDomainRaw) != "null" && string(selectedDomainRaw) != "{}" {
		var candidate DomainResolutionCandidate
		if err := json.Unmarshal(selectedDomainRaw, &candidate); err == nil && candidate.Domain != "" {
			item.SelectedDomainPayload = &candidate
		}
	}
	_ = json.Unmarshal(domainResponseRaw, &item.DomainResponse)
	_ = json.Unmarshal(evidenceRunsRaw, &item.EvidenceRuns)
	if len(analysisRaw) > 0 && string(analysisRaw) != "null" && string(analysisRaw) != "{}" {
		var analysis CandidateMatchAnalysisResponse
		if err := json.Unmarshal(analysisRaw, &analysis); err == nil && analysis.Verdict != "" {
			item.CandidateMatchAnalysis = &analysis
		}
	}
	_ = json.Unmarshal(finalDecisionRaw, &item.FinalDecision)
	return item, nil
}

func (s *SQLStore) loadMAEvidence(ctx context.Context, sessionID string) (map[string][]MATargetEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT evidence.target_id::text, evidence.criterion, evidence.status,
       COALESCE(evidence.family, ''), evidence.label, COALESCE(evidence.value, ''),
       COALESCE(evidence.points, 0)::float8, COALESCE(evidence.weight, 0)::float8,
       COALESCE(evidence.source_path, '')
FROM binocolo.ma_evidence evidence
JOIN binocolo.ma_target target ON target.id = evidence.target_id
WHERE target.session_id = $1::uuid
ORDER BY evidence.created_at, evidence.id
`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load ma evidence: %w", err)
	}
	defer rows.Close()
	out := map[string][]MATargetEvidence{}
	for rows.Next() {
		var targetID string
		var item MATargetEvidence
		if err := rows.Scan(&targetID, &item.Criterion, &item.Status, &item.Family, &item.Label, &item.Value, &item.Points, &item.Weight, &item.SourcePath); err != nil {
			return nil, fmt.Errorf("scan ma evidence: %w", err)
		}
		out[targetID] = append(out[targetID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma evidence: %w", err)
	}
	return out, nil
}

func (s *SQLStore) loadMAEvidenceForTarget(ctx context.Context, sessionID, targetID string) ([]MATargetEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT evidence.criterion, evidence.status,
       COALESCE(evidence.family, ''), evidence.label, COALESCE(evidence.value, ''),
       COALESCE(evidence.points, 0)::float8, COALESCE(evidence.weight, 0)::float8,
       COALESCE(evidence.source_path, '')
FROM binocolo.ma_evidence evidence
JOIN binocolo.ma_target target ON target.id = evidence.target_id
WHERE target.session_id = $1::uuid
  AND evidence.target_id = $2::uuid
ORDER BY evidence.created_at, evidence.id
`, sessionID, targetID)
	if err != nil {
		return nil, fmt.Errorf("load ma evidence for target: %w", err)
	}
	defer rows.Close()
	out := []MATargetEvidence{}
	for rows.Next() {
		var item MATargetEvidence
		if err := rows.Scan(&item.Criterion, &item.Status, &item.Family, &item.Label, &item.Value, &item.Points, &item.Weight, &item.SourcePath); err != nil {
			return nil, fmt.Errorf("scan ma evidence: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma evidence for target: %w", err)
	}
	return out, nil
}

func (s *SQLStore) ListMAParameters(ctx context.Context) ([]MAParameter, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT key, value, value_type, label, COALESCE(description, ''), COALESCE(updated_by_email, ''), updated_at
FROM binocolo.ma_parameter
ORDER BY key
`)
	if err != nil {
		return nil, fmt.Errorf("list ma parameters: %w", err)
	}
	defer rows.Close()
	out := []MAParameter{}
	for rows.Next() {
		var item MAParameter
		var updatedAt time.Time
		if err := rows.Scan(&item.Key, &item.Value, &item.ValueType, &item.Label, &item.Description, &item.UpdatedByEmail, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan ma parameter: %w", err)
		}
		ts := updatedAt
		item.UpdatedAt = &ts
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma parameters: %w", err)
	}
	return out, nil
}

// UpdateMAParameter updates one existing parameter. It returns sql.ErrNoRows when
// the key does not exist, so callers can reject unknown keys (no implicit insert).
func (s *SQLStore) UpdateMAParameter(ctx context.Context, key, value, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_parameter
SET value = $2, updated_by_email = $3, updated_at = now()
WHERE key = $1
`, key, value, nullString(email))
	if err != nil {
		return fmt.Errorf("update ma parameter: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update ma parameter rows: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// --- Deep analysis (veryshort IT-full) -------------------------------------

type maDeepJob struct {
	CompanyKey      string
	VATCode         string
	TaxCode         string
	Status          string
	VendorRequestID string
	Attempts        int
}

func (s *SQLStore) ListMADeepAnalysis(ctx context.Context, companyKeys []string) (map[string]MADeepAnalysis, error) {
	out := map[string]MADeepAnalysis{}
	if s == nil || s.db == nil || len(companyKeys) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(companyKeys))
	args := make([]any, len(companyKeys))
	for i, key := range companyKeys {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = key
	}
	query := `
SELECT company_key, status, scorecard, valuation, brief, cost_eur, COALESCE(error_code, ''), updated_at,
       COALESCE(requested_at, created_at)
FROM binocolo.ma_deep_analysis
WHERE company_key IN (` + strings.Join(placeholders, ", ") + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ma deep analysis: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item MADeepAnalysis
		var scorecardRaw, valuationRaw, briefRaw []byte
		var updatedAt, fetchedAt time.Time
		if err := rows.Scan(&item.CompanyKey, &item.Status, &scorecardRaw, &valuationRaw, &briefRaw, &item.CostEUR, &item.ErrorCode, &updatedAt, &fetchedAt); err != nil {
			return nil, fmt.Errorf("scan ma deep analysis: %w", err)
		}
		if len(scorecardRaw) > 0 {
			var scorecard MADeepScorecard
			if json.Unmarshal(scorecardRaw, &scorecard) == nil {
				item.Scorecard = &scorecard
			}
		}
		if len(valuationRaw) > 0 {
			var valuation MADeepValuation
			if json.Unmarshal(valuationRaw, &valuation) == nil {
				item.Valuation = &valuation
			}
		}
		if len(briefRaw) > 0 {
			var brief MADeepBrief
			if json.Unmarshal(briefRaw, &brief) == nil {
				item.Brief = &brief
			}
		}
		updated := updatedAt
		item.UpdatedAt = &updated
		fetched := fetchedAt
		item.FetchedAt = &fetched
		out[item.CompanyKey] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma deep analysis: %w", err)
	}
	return out, nil
}

func (s *SQLStore) GetMATargetDeepDiveIdentity(ctx context.Context, companyKey string) (*maDeepDiveIdentity, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
SELECT COALESCE(NULLIF(btrim(vat_code), ''), ''), COALESCE(NULLIF(btrim(tax_code), ''), '')
FROM binocolo.ma_target
WHERE company_key = $1
  AND (COALESCE(NULLIF(btrim(vat_code), ''), '') <> '' OR COALESCE(NULLIF(btrim(tax_code), ''), '') <> '')
ORDER BY created_at DESC
LIMIT 1
`, companyKey)
	var identity maDeepDiveIdentity
	if err := row.Scan(&identity.VATCode, &identity.TaxCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ma target deep dive identity: %w", err)
	}
	return &identity, nil
}

func (s *SQLStore) FindMACompanySnapshotByIdentity(ctx context.Context, vatOrTax string) (*maCompanySnapshot, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	vatOrTax = normalizeMAVATOrTax(vatOrTax)
	if vatOrTax == "" {
		return nil, nil
	}
	// Confronto NORMALIZZATO su entrambi i lati, come GetMACompanyDomain: con
	// `upper(btrim(...))` una P.IVA scritta con il prefisso IT o con i punti non
	// trovava lo snapshot, e la card diretta ripartiva da una probe Address a
	// pagamento invece di riusare l'azienda che avevamo già.
	row := s.db.QueryRowContext(ctx, `
WITH snapshots AS (
  SELECT COALESCE(company_key, '') AS company_key,
         COALESCE(company_name, '') AS company_name, COALESCE(vat_code, '') AS vat_code,
         COALESCE(tax_code, '') AS tax_code, COALESCE(province, '') AS province,
         created_at AS seen_at, 1 AS priority
  FROM binocolo.ma_target
  WHERE binocolo.ma_stable_vat(vat_code) = $1 OR binocolo.ma_normalize_fiscal(tax_code) = $2
  UNION ALL
  SELECT company_key, company_name, vat_code, tax_code, province, updated_at, 2
  FROM binocolo.ma_initiative_card
  WHERE binocolo.ma_stable_vat(vat_code) = $1 OR binocolo.ma_normalize_fiscal(tax_code) = $2
)
SELECT company_key, company_name, vat_code, tax_code, province
FROM snapshots
ORDER BY priority, seen_at DESC
LIMIT 1
`, maStableVAT(vatOrTax), normalizeMAFiscalValue(vatOrTax))
	var out maCompanySnapshot
	if err := row.Scan(&out.CompanyKey, &out.CompanyName, &out.VATCode, &out.TaxCode, &out.Province); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find ma company snapshot by identity: %w", err)
	}
	out.CompanyKey = normalizeMACompanyKey(out.CompanyKey)
	return &out, nil
}

func (s *SQLStore) FindMACompanySnapshotByKey(ctx context.Context, companyKey string) (*maCompanySnapshot, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return nil, nil
	}
	if row, err := s.GetMACompanyOverviewIdentity(ctx, companyKey); err != nil {
		return nil, err
	} else if row != nil {
		return &maCompanySnapshot{CompanyKey: row.CompanyKey, CompanyName: row.CompanyName, VATCode: row.VATCode, TaxCode: row.TaxCode, Province: row.Province}, nil
	}
	return nil, nil
}

func (s *SQLStore) GetMAInitiativeCardDeepDiveIdentity(ctx context.Context, companyKey string) (*maDeepDiveIdentity, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
SELECT COALESCE(NULLIF(btrim(vat_code), ''), ''), COALESCE(NULLIF(btrim(tax_code), ''), '')
FROM binocolo.ma_initiative_card
WHERE company_key = $1
  AND (COALESCE(NULLIF(btrim(vat_code), ''), '') <> '' OR COALESCE(NULLIF(btrim(tax_code), ''), '') <> '')
ORDER BY updated_at DESC
LIMIT 1
`, companyKey)
	var identity maDeepDiveIdentity
	if err := row.Scan(&identity.VATCode, &identity.TaxCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get ma initiative card deep dive identity: %w", err)
	}
	return &identity, nil
}

// EnqueueMADeepAnalysis queues a company for IT-full. New companies are inserted
// queued; previously failed ones are re-queued; ready/running/queued rows are left
// untouched (the ON CONFLICT WHERE only matches failed) so paid analyses are reused.
func (s *SQLStore) EnqueueMADeepAnalysis(ctx context.Context, companyKey, vatCode, taxCode, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_deep_analysis (company_key, vat_code, tax_code, status, refreshed_by_email)
VALUES ($1, $2, $3, 'queued', $4)
ON CONFLICT (company_key) DO UPDATE
SET status = 'queued', attempts = 0, error_code = NULL, vendor_request_id = NULL, requested_at = NULL,
    lease_until = NULL, locked_by = NULL,
    vat_code = EXCLUDED.vat_code, tax_code = EXCLUDED.tax_code,
    refreshed_by_email = EXCLUDED.refreshed_by_email, updated_at = now()
WHERE binocolo.ma_deep_analysis.status = 'failed'
`, companyKey, nullString(vatCode), nullString(taxCode), nullString(email))
	if err != nil {
		return fmt.Errorf("enqueue ma deep analysis: %w", translateMACompanyConstraintError(err, maCompanyConstraintFKOnly))
	}
	return nil
}

// RefreshMADeepAnalysis requeues a ready IT-full record only when its vendor
// request predates staleBefore. The conditional update is the concurrency guard:
// simultaneous deep-dive requests can refresh the same company only once.
func (s *SQLStore) RefreshMADeepAnalysis(ctx context.Context, companyKey, vatCode, taxCode, email string, staleBefore time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET status = 'queued', attempts = 0, error_code = NULL, vendor_request_id = NULL, requested_at = NULL,
    lease_until = NULL, locked_by = NULL,
    vat_code = COALESCE($2, vat_code), tax_code = COALESCE($3, tax_code),
    refreshed_by_email = $4, updated_at = now()
WHERE company_key = $1
  AND status = 'ready'
  AND COALESCE(requested_at, created_at) <= $5
`, companyKey, nullString(vatCode), nullString(taxCode), nullString(email), staleBefore)
	if err != nil {
		return false, fmt.Errorf("refresh ma deep analysis: %w", translateMACompanyConstraintError(err, maCompanyConstraintFKOnly))
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("refresh ma deep analysis rows: %w", err)
	}
	return affected == 1, nil
}

func (s *SQLStore) ListMADeepJobs(ctx context.Context, limit int, workerID string) ([]maDeepJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	if limit <= 0 {
		limit = 16
	}
	// Only surface rows that are free or already owned by this worker, so workers do
	// not even fetch rows another worker is actively processing.
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key, COALESCE(vat_code, ''), COALESCE(tax_code, ''), status, COALESCE(vendor_request_id, ''), attempts
FROM binocolo.ma_deep_analysis
WHERE status IN ('queued', 'running')
  AND (lease_until IS NULL OR lease_until < now() OR locked_by = $2)
ORDER BY updated_at
LIMIT $1
`, limit, workerID)
	if err != nil {
		return nil, fmt.Errorf("list ma deep jobs: %w", err)
	}
	defer rows.Close()
	out := []maDeepJob{}
	for rows.Next() {
		var job maDeepJob
		if err := rows.Scan(&job.CompanyKey, &job.VATCode, &job.TaxCode, &job.Status, &job.VendorRequestID, &job.Attempts); err != nil {
			return nil, fmt.Errorf("scan ma deep job: %w", err)
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma deep jobs: %w", err)
	}
	return out, nil
}

// ClaimMADeepQueued atomically transitions a row from queued to running. It returns
// true only for the worker/replica that won the claim (rows affected == 1), so with
// multiple replicas the IT-full POST — and its €0.30 charge — happens exactly once.
// Resets the poll attempt budget for the running phase.
func (s *SQLStore) ClaimMADeepQueued(ctx context.Context, companyKey string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET status = 'running', attempts = 0, error_code = NULL, requested_at = now(), updated_at = now()
WHERE company_key = $1 AND status = 'queued'
`, companyKey)
	if err != nil {
		return false, fmt.Errorf("claim ma deep queued: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim ma deep queued rows: %w", err)
	}
	return affected == 1, nil
}

// AcquireMADeepLease grants the calling worker exclusive ownership of a row for
// leaseSeconds (measured on the DB clock, so it is immune to clock skew across worker
// machines). Returns true only for the worker that wins the atomic update; others skip.
// A worker renews its own lease (locked_by match); a crashed worker's lease simply
// expires and the row becomes reclaimable. This is what makes the worker correct with
// multiple instances on one DB (shared staging, k8s replicas, rolling-deploy overlap).
func (s *SQLStore) AcquireMADeepLease(ctx context.Context, companyKey, workerID string, leaseSeconds int) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET lease_until = now() + ($3::int * interval '1 second'), locked_by = $2
WHERE company_key = $1 AND status IN ('queued', 'running')
  AND (lease_until IS NULL OR lease_until < now() OR locked_by = $2)
`, companyKey, workerID, leaseSeconds)
	if err != nil {
		return false, fmt.Errorf("acquire ma deep lease: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("acquire ma deep lease rows: %w", err)
	}
	return affected == 1, nil
}

func (s *SQLStore) SetMADeepVendorRequest(ctx context.Context, companyKey, vendorRequestID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET vendor_request_id = $2, updated_at = now()
WHERE company_key = $1
`, companyKey, vendorRequestID)
	if err != nil {
		return fmt.Errorf("set ma deep vendor request: %w", err)
	}
	return nil
}

func (s *SQLStore) SaveMADeepReady(ctx context.Context, companyKey string, result maDeepResult) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	marshalJSON := func(value any, label string) ([]byte, error) {
		if value == nil {
			return []byte("null"), nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("marshal ma deep %s: %w", label, err)
		}
		return raw, nil
	}
	scorecardRaw, err := marshalJSON(result.Scorecard, "scorecard")
	if err != nil {
		return err
	}
	valuationRaw, err := marshalJSON(result.Valuation, "valuation")
	if err != nil {
		return err
	}
	briefRaw, err := marshalJSON(result.Brief, "brief")
	if err != nil {
		return err
	}
	payloadRaw := json.RawMessage("null")
	if len(result.Payload) > 0 {
		payloadRaw = result.Payload
	}
	// brief_generated_at stamps the brief-write moment: SaveMADeepReady and UpdateMADeepBrief are
	// the ONLY two points that write the brief column, so they are the only two that stamp it. The
	// stamp backs the on-read brief staleness (maDeepBriefStale) vs newer aligned NI decisions.
	_, err = s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET status = 'ready', itfull_payload = $2::jsonb, scorecard = $3::jsonb, valuation = $4::jsonb, brief = $5::jsonb,
    model_id = $6::uuid, prompt_id = $7::uuid, cost_eur = $8, error_code = NULL,
    brief_generated_at = now(), updated_at = now()
WHERE company_key = $1
`, companyKey, []byte(payloadRaw), scorecardRaw, valuationRaw, briefRaw, nullString(result.ModelID), nullString(result.PromptID), result.CostEUR)
	if err != nil {
		return fmt.Errorf("save ma deep ready: %w", err)
	}
	return nil
}

func (s *SQLStore) MarkMADeepFailed(ctx context.Context, companyKey, errorCode string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET status = 'failed', error_code = $2, updated_at = now()
WHERE company_key = $1
`, companyKey, nullString(errorCode))
	if err != nil {
		return fmt.Errorf("mark ma deep failed: %w", err)
	}
	return nil
}

func (s *SQLStore) BumpMADeepAttempt(ctx context.Context, companyKey string) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("binocolo ma store not configured")
	}
	var attempts int
	if err := s.db.QueryRowContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET attempts = attempts + 1, updated_at = now()
WHERE company_key = $1
RETURNING attempts
`, companyKey).Scan(&attempts); err != nil {
		return 0, fmt.Errorf("bump ma deep attempt: %w", err)
	}
	return attempts, nil
}

// InsertMADeepVintage archives one (company, balance-sheet date) payload vintage
// (migration 091). ON CONFLICT DO NOTHING: re-fetching the same filing is not a new
// vintage; a new filing is a new row. Called best-effort by the worker BEFORE the
// analysis pipeline, so a vintage failure (e.g. migration not yet applied on the
// shared DB) never loses the paid payload processing.
func (s *SQLStore) InsertMADeepVintage(ctx context.Context, companyKey, balanceSheetDate string, turnoverYear *int, payload json.RawMessage) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if strings.TrimSpace(companyKey) == "" || strings.TrimSpace(balanceSheetDate) == "" || len(payload) == 0 {
		return errors.New("ma deep vintage: missing key or payload")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_deep_payload_vintage (company_key, balance_sheet_date, turnover_year, payload)
VALUES ($1, $2::date, $3, $4::jsonb)
ON CONFLICT (company_key, balance_sheet_date) DO NOTHING
`, companyKey, balanceSheetDate, nullInt(turnoverYear), []byte(payload))
	if err != nil {
		return fmt.Errorf("insert ma deep vintage: %w", err)
	}
	return nil
}

// CountMADeepByStatus returns the row count of ma_deep_analysis per status (inspect).
func (s *SQLStore) CountMADeepByStatus(ctx context.Context) (map[string]int, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT status, count(*) FROM binocolo.ma_deep_analysis GROUP BY status
`)
	if err != nil {
		return nil, fmt.Errorf("count ma deep by status: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan ma deep status count: %w", err)
		}
		out[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma deep status counts: %w", err)
	}
	return out, nil
}

// CountMADeepBriefFormats classifies the cached ready briefs by payload shape —
// 'financialReading' (v3), 'legacy' (pre-v3), 'none' — and returns the company keys
// still NOT on the current format, so a prompt rollout is verifiable senza rigenerare.
func (s *SQLStore) CountMADeepBriefFormats(ctx context.Context) (map[string]int, []string, error) {
	if s == nil || s.db == nil {
		return nil, nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key,
       CASE WHEN brief IS NULL OR brief = 'null'::jsonb THEN 'none'
            WHEN brief ? 'financialReading' THEN 'financialReading'
            ELSE 'legacy' END
FROM binocolo.ma_deep_analysis
WHERE status = 'ready'
ORDER BY company_key
`)
	if err != nil {
		return nil, nil, fmt.Errorf("count ma deep brief formats: %w", err)
	}
	defer rows.Close()
	counts := map[string]int{}
	stale := []string{}
	for rows.Next() {
		var companyKey, format string
		if err := rows.Scan(&companyKey, &format); err != nil {
			return nil, nil, fmt.Errorf("scan ma deep brief format: %w", err)
		}
		counts[format]++
		if format != "financialReading" {
			stale = append(stale, companyKey)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate ma deep brief formats: %w", err)
	}
	return counts, stale, nil
}

// CountMADeepVintage returns (rows, distinct companies) of the vintage archive.
func (s *SQLStore) CountMADeepVintage(ctx context.Context) (int, int, error) {
	if s == nil || s.db == nil {
		return 0, 0, errors.New("binocolo ma store not configured")
	}
	var rows, companies int
	if err := s.db.QueryRowContext(ctx, `
SELECT count(*), count(DISTINCT company_key) FROM binocolo.ma_deep_payload_vintage
`).Scan(&rows, &companies); err != nil {
		return 0, 0, fmt.Errorf("count ma deep vintage: %w", err)
	}
	return rows, companies, nil
}

type maDeepPayloadRow struct {
	CompanyKey string
	VATCode    string
	TaxCode    string
	Payload    json.RawMessage
}

// ListMADeepReadyPayloads returns the cached IT-full payload of every ready deep
// analysis, so the scorecard can be rebuilt deterministically without a vendor call.
func (s *SQLStore) ListMADeepReadyPayloads(ctx context.Context) ([]maDeepPayloadRow, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key, COALESCE(vat_code, ''), COALESCE(tax_code, ''), itfull_payload
FROM binocolo.ma_deep_analysis
WHERE status = 'ready' AND itfull_payload IS NOT NULL AND itfull_payload <> 'null'::jsonb
ORDER BY updated_at
`)
	if err != nil {
		return nil, fmt.Errorf("list ma deep ready payloads: %w", err)
	}
	defer rows.Close()
	out := []maDeepPayloadRow{}
	for rows.Next() {
		var row maDeepPayloadRow
		var payload []byte
		if err := rows.Scan(&row.CompanyKey, &row.VATCode, &row.TaxCode, &payload); err != nil {
			return nil, fmt.Errorf("scan ma deep ready payload: %w", err)
		}
		row.Payload = json.RawMessage(payload)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma deep ready payloads: %w", err)
	}
	return out, nil
}

// UpdateMADeepScorecard overwrites only the scorecard column; valuation and brief
// are untouched. Used by the deterministic recompute after an engine calibration
// (the scorecard fixes do not change the valuation inputs).
func (s *SQLStore) UpdateMADeepScorecard(ctx context.Context, companyKey string, scorecard *MADeepScorecard) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	raw := json.RawMessage("null")
	if scorecard != nil {
		b, err := json.Marshal(scorecard)
		if err != nil {
			return fmt.Errorf("marshal ma deep scorecard: %w", err)
		}
		raw = b
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET scorecard = $2::jsonb, updated_at = now()
WHERE company_key = $1
`, companyKey, []byte(raw)); err != nil {
		return fmt.Errorf("update ma deep scorecard: %w", err)
	}
	return nil
}

// UpdateMADeepValuation overwrites only the valuation column (recompute con
// {"valuation": true}, Fase 2: bridge e banda asimmetrica cambiano la semantica
// della valuation e il refresh sulle righe cached è voluto).
func (s *SQLStore) UpdateMADeepValuation(ctx context.Context, companyKey string, valuation *MADeepValuation) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	raw := json.RawMessage("null")
	if valuation != nil {
		b, err := json.Marshal(valuation)
		if err != nil {
			return fmt.Errorf("marshal ma deep valuation: %w", err)
		}
		raw = b
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET valuation = $2::jsonb, updated_at = now()
WHERE company_key = $1
`, companyKey, []byte(raw)); err != nil {
		return fmt.Errorf("update ma deep valuation: %w", err)
	}
	return nil
}

// maDeepVATRecord is a single cached deep analysis resolved by partita IVA, carrying
// the raw IT-full payload (facts layer) alongside the derived scorecard/valuation/brief.
type maDeepVATRecord struct {
	CompanyKey string
	Status     string
	Scorecard  *MADeepScorecard
	Valuation  *MADeepValuation
	Brief      *MADeepBrief
	Payload    json.RawMessage
	ErrorCode  string
	UpdatedAt  time.Time
	// FetchedAt tracks the vendor request rather than subsequent derived writes.
	FetchedAt time.Time
	// BriefGeneratedAt is the brief-write stamp (nil when never generated), consumed by the
	// on-read brief-staleness check (maDeepBriefStale) against the newest aligned NI decision.
	BriefGeneratedAt *time.Time
}

// GetMADeepByVAT resolves a deep analysis by vat_code regardless of how its row was
// keyed (the funnel keys by vendor id, the standalone lookup by VAT). A ready row
// wins over an in-flight/failed one, then most recent. Returns (nil, nil) when absent.
func (s *SQLStore) GetMADeepByVAT(ctx context.Context, vat string) (*maDeepVATRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	// Confronto sull'identità fiscale NORMALIZZATA, non sul valore grezzo.
	// `WHERE vat_code = $1` mancava la riga per una differenza di forma — prefisso
	// IT, punti, spazi — e il buco non era cosmetico: questa è la lettura che
	// impedisce a /azienda di ricomprare un dossier già pagato. Mancarla
	// significava un secondo IT-full a €0.30 E una seconda riga
	// ma_deep_analysis per la stessa azienda sotto una chiave diversa.
	// Cerca anche sul codice fiscale, perché l'input di /azienda può essere un CF.
	fiscalKey := buildMAFiscalKey(vat, vat)
	if fiscalKey == "" {
		return nil, nil
	}
	var rec maDeepVATRecord
	var scorecardRaw, valuationRaw, briefRaw, payloadRaw []byte
	var briefGeneratedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
SELECT company_key, status, scorecard, valuation, brief, itfull_payload, COALESCE(error_code, ''), updated_at,
       COALESCE(requested_at, created_at), brief_generated_at
FROM binocolo.ma_deep_analysis
WHERE `+maDeepFiscalIdentityMatch+`
ORDER BY (status = 'ready') DESC, updated_at DESC
LIMIT 1
`, fiscalKey).Scan(&rec.CompanyKey, &rec.Status, &scorecardRaw, &valuationRaw, &briefRaw, &payloadRaw, &rec.ErrorCode, &rec.UpdatedAt, &rec.FetchedAt, &briefGeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ma deep by vat: %w", err)
	}
	if briefGeneratedAt.Valid {
		rec.BriefGeneratedAt = &briefGeneratedAt.Time
	}
	if len(scorecardRaw) > 0 {
		var v MADeepScorecard
		if json.Unmarshal(scorecardRaw, &v) == nil {
			rec.Scorecard = &v
		}
	}
	if len(valuationRaw) > 0 {
		var v MADeepValuation
		if json.Unmarshal(valuationRaw, &v) == nil {
			rec.Valuation = &v
		}
	}
	if len(briefRaw) > 0 {
		var v MADeepBrief
		if json.Unmarshal(briefRaw, &v) == nil {
			rec.Brief = &v
		}
	}
	if len(payloadRaw) > 0 && string(payloadRaw) != "null" {
		rec.Payload = json.RawMessage(payloadRaw)
	}
	return &rec, nil
}

// maRowQuerier is satisfied by both *sql.DB and *sql.Tx, so the fiscal-identity lookup can
// run either standalone or inside the identity-aware enqueue transaction.
type maRowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// maDeepVATRecordColumns backs maDeepVATRecord (same shape scanned by GetMADeepByVAT),
// reused by the fiscal-identity lookups.
const maDeepVATRecordColumns = `company_key, status, scorecard, valuation, brief, itfull_payload, COALESCE(error_code, ''), updated_at, COALESCE(requested_at, created_at), brief_generated_at`

// maDeepFiscalIdentityMatch is the WHERE predicate matching a ma_deep_analysis row to a
// canonical fiscal_key ($1): vat_code IT-stripped alla forma stabile, tax_code pulito, e la
// riga corrisponde se UNO dei due eguaglia la chiave. Un'azienda ancorata al solo CF è
// raggiunta dal ramo tax_code, perché buildMAFiscalKey ripiega sul codice fiscale pulito.
//
// Chiama le primitive della migrazione 120 invece di ricopiarne l'espressione: prima erano
// tre implementazioni parallele della stessa normalizzazione — questa, la CTE del finder e
// le funzioni SQL — bit-identiche ma tenute in sincronia da un commento (issue #86, F0).
const maDeepFiscalIdentityMatch = `(
  binocolo.ma_stable_vat(vat_code) = $1
  OR binocolo.ma_normalize_fiscal(tax_code) = $1
)`

func scanMADeepVATRecord(scanner interface{ Scan(...any) error }) (*maDeepVATRecord, error) {
	var rec maDeepVATRecord
	var scorecardRaw, valuationRaw, briefRaw, payloadRaw []byte
	var briefGeneratedAt sql.NullTime
	if err := scanner.Scan(&rec.CompanyKey, &rec.Status, &scorecardRaw, &valuationRaw, &briefRaw, &payloadRaw, &rec.ErrorCode, &rec.UpdatedAt, &rec.FetchedAt, &briefGeneratedAt); err != nil {
		return nil, err
	}
	if briefGeneratedAt.Valid {
		rec.BriefGeneratedAt = &briefGeneratedAt.Time
	}
	if len(scorecardRaw) > 0 {
		var v MADeepScorecard
		if json.Unmarshal(scorecardRaw, &v) == nil {
			rec.Scorecard = &v
		}
	}
	if len(valuationRaw) > 0 {
		var v MADeepValuation
		if json.Unmarshal(valuationRaw, &v) == nil {
			rec.Valuation = &v
		}
	}
	if len(briefRaw) > 0 {
		var v MADeepBrief
		if json.Unmarshal(briefRaw, &v) == nil {
			rec.Brief = &v
		}
	}
	if len(payloadRaw) > 0 && string(payloadRaw) != "null" {
		rec.Payload = json.RawMessage(payloadRaw)
	}
	return &rec, nil
}

// lookupMADeepByFiscalKey resolves the most significant deep analysis for a canonical
// fiscal_key: a ready row wins over an in-flight/failed one, then most recent. A failed row
// is included (the caller decides what to do with it). Returns (nil, nil) when absent.
func lookupMADeepByFiscalKey(ctx context.Context, q maRowQuerier, fiscalKey string) (*maDeepVATRecord, error) {
	rec, err := scanMADeepVATRecord(q.QueryRowContext(ctx, `
SELECT `+maDeepVATRecordColumns+`
FROM binocolo.ma_deep_analysis
WHERE `+maDeepFiscalIdentityMatch+`
ORDER BY (status = 'ready') DESC, updated_at DESC
LIMIT 1
`, fiscalKey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup ma deep by fiscal identity: %w", err)
	}
	return rec, nil
}

// GetMADeepByFiscalIdentity resolves a deep analysis by fiscal identity, matching against
// BOTH vat_code (stable, IT-stripped) and tax_code with the finder's normalization — so a
// company anchored only by CF is still found. Returns (nil, nil) when the identity is empty
// or absent.
func (s *SQLStore) GetMADeepByFiscalIdentity(ctx context.Context, vat, tax string) (*maDeepVATRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	fiscalKey := buildMAFiscalKey(vat, tax)
	if fiscalKey == "" {
		return nil, nil
	}
	return lookupMADeepByFiscalKey(ctx, s.db, fiscalKey)
}

// EnqueueMADeepAnalysisIfAbsent queues an IT-full deep analysis for a company ONLY when no
// analysis already exists for its fiscal identity — the filing pipeline's automatic
// baseline trigger. Unlike EnqueueMADeepAnalysis (the explicit deep-dive button, which
// re-queues failed rows), this NEVER re-charges: any existing row — ready/queued/running AND
// failed — is a no-op returning that row's status.
//
// One transaction: (1) an advisory transaction lock on the canonical fiscal_key serializes
// concurrent callers so two of them can't both pass the "absent" check and double-insert
// (their company_key differs, so the company_key unique index would not catch it); (2) the
// identity-aware lookup runs inside the lock; (3) an existing row short-circuits; (4)
// otherwise INSERT ON CONFLICT (company_key) DO NOTHING. companyKey is the caller's context
// reference (non-identifying); the fiscal identity comes from vat/tax.
func (s *SQLStore) EnqueueMADeepAnalysisIfAbsent(ctx context.Context, companyKey, vat, tax, email string) (bool, string, error) {
	if s == nil || s.db == nil {
		return false, "", errors.New("binocolo ma store not configured")
	}
	// Guard (QA-F5 pendenza): an empty company_key must NEVER be inserted — it is the
	// conflict key, so a blank value would collide across distinct fiscal identities (a sweeper
	// re-driving an orphan upload can carry an empty context). Skip, best-effort (no error).
	companyKey = strings.TrimSpace(companyKey)
	if companyKey == "" {
		logging.FromContext(ctx).Warn("binocolo deep enqueue-if-absent skipped: empty company key",
			"component", "binocolo", "operation", "ma_deep_analysis")
		return false, "", nil
	}
	fiscalKey := buildMAFiscalKey(vat, tax)
	if fiscalKey == "" {
		return false, "", errors.New("enqueue ma deep analysis if absent: empty fiscal identity (vat and tax both blank)")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", fmt.Errorf("begin enqueue ma deep analysis if absent: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, fiscalKey); err != nil {
		return false, "", fmt.Errorf("advisory lock ma deep fiscal key: %w", err)
	}

	existing, err := lookupMADeepByFiscalKey(ctx, tx, fiscalKey)
	if err != nil {
		return false, "", err
	}
	if existing != nil {
		return false, existing.Status, nil
	}

	res, err := tx.ExecContext(ctx, `
INSERT INTO binocolo.ma_deep_analysis (company_key, vat_code, tax_code, status, refreshed_by_email)
VALUES ($1, $2, $3, 'queued', $4)
ON CONFLICT (company_key) DO NOTHING
`, companyKey, nullString(vat), nullString(tax), nullString(email))
	if err != nil {
		return false, "", fmt.Errorf("insert ma deep analysis if absent: %w", translateMACompanyConstraintError(err, maCompanyConstraintFKOnly))
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, "", fmt.Errorf("insert ma deep analysis if absent rows: %w", err)
	}
	if affected == 0 {
		// company_key already present under a row whose fiscal identity didn't match the key
		// (blank/other vat/tax): surface its status without creating a duplicate.
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM binocolo.ma_deep_analysis WHERE company_key = $1`, companyKey).Scan(&status); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, "", fmt.Errorf("read existing ma deep status: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return false, "", fmt.Errorf("commit enqueue ma deep analysis if absent: %w", err)
		}
		return false, status, nil
	}
	if err := tx.Commit(); err != nil {
		return false, "", fmt.Errorf("commit enqueue ma deep analysis if absent: %w", err)
	}
	return true, "", nil
}

type maDeepBriefRow struct {
	CompanyKey string
	Payload    json.RawMessage
	Valuation  *MADeepValuation
	// VATCode/TaxCode carry the fiscal identity so brief regeneration can resolve the canonical
	// filing for the optional filing context (F7). Empty when the row was keyed without them.
	VATCode string
	TaxCode string
}

// ListMADeepReadyForBrief returns ready rows with their cached payload + valuation, for
// LLM brief regeneration (the scorecard is rebuilt from the payload). No vendor call.
func (s *SQLStore) ListMADeepReadyForBrief(ctx context.Context) ([]maDeepBriefRow, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT company_key, itfull_payload, valuation, COALESCE(vat_code, ''), COALESCE(tax_code, '')
FROM binocolo.ma_deep_analysis
WHERE status = 'ready' AND itfull_payload IS NOT NULL AND itfull_payload <> 'null'::jsonb
ORDER BY updated_at
`)
	if err != nil {
		return nil, fmt.Errorf("list ma deep ready for brief: %w", err)
	}
	defer rows.Close()
	out := []maDeepBriefRow{}
	for rows.Next() {
		var row maDeepBriefRow
		var payload, valuationRaw []byte
		if err := rows.Scan(&row.CompanyKey, &payload, &valuationRaw, &row.VATCode, &row.TaxCode); err != nil {
			return nil, fmt.Errorf("scan ma deep brief row: %w", err)
		}
		row.Payload = json.RawMessage(payload)
		if len(valuationRaw) > 0 {
			var v MADeepValuation
			if json.Unmarshal(valuationRaw, &v) == nil {
				row.Valuation = &v
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma deep brief rows: %w", err)
	}
	return out, nil
}

// UpdateMADeepBrief overwrites the brief column and its model/prompt provenance, leaving
// scorecard/valuation/payload untouched. Used by brief regeneration.
func (s *SQLStore) UpdateMADeepBrief(ctx context.Context, companyKey string, brief *MADeepBrief, modelID, promptID string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	raw := json.RawMessage("null")
	if brief != nil {
		b, err := json.Marshal(brief)
		if err != nil {
			return fmt.Errorf("marshal ma deep brief: %w", err)
		}
		raw = b
	}
	// brief_generated_at is re-stamped here too (the second and last brief-write point) so a
	// regenerated brief resets the staleness clock.
	if _, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_deep_analysis
SET brief = $2::jsonb, model_id = $3::uuid, prompt_id = $4::uuid,
    brief_generated_at = now(), updated_at = now()
WHERE company_key = $1
`, companyKey, []byte(raw), nullString(modelID), nullString(promptID)); err != nil {
		return fmt.Errorf("update ma deep brief: %w", err)
	}
	return nil
}

type sectorMultiple struct {
	Prefix     string
	Industry   string
	EVEbitda   *float64
	EVSales    *float64
	NFirms     int
	Source     string
	SourceDate string
}

// ResolveSectorMultipleKeys tries the lookup keys in order (dal redesign Fase 3:
// famiglia di business model → prefissi ATECO 4→3→2 → riga TOTAL; l'ordine è
// costruito da sectorMultipleLookupKeys). Nil when nothing matches.
func (s *SQLStore) ResolveSectorMultipleKeys(ctx context.Context, keys []string) (*sectorMultiple, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	for _, key := range keys {
		multiple, err := s.loadSectorMultiple(ctx, key)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		return multiple, nil
	}
	return nil, nil
}

// UpsertMABMFamilySuggestion salva il suggerimento deterministico di famiglia
// (mig 093) senza MAI toccare la ratifica dell'analista; gli snapshot anagrafici
// si aggiornano solo se valorizzati.
func (s *SQLStore) UpsertMABMFamilySuggestion(ctx context.Context, companyKey, vatCode, taxCode, companyName string, suggestion maBMFamilySuggestion) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if strings.TrimSpace(companyKey) == "" || !maBMFamilies[suggestion.Family] {
		return errors.New("ma bm family suggestion: invalid input")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_company_bm_family
  (company_key, vat_code, tax_code, company_name, suggested_family, suggested_source, suggested_evidence)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (company_key) DO UPDATE SET
  vat_code = CASE WHEN EXCLUDED.vat_code <> '' THEN EXCLUDED.vat_code ELSE ma_company_bm_family.vat_code END,
  tax_code = CASE WHEN EXCLUDED.tax_code <> '' THEN EXCLUDED.tax_code ELSE ma_company_bm_family.tax_code END,
  company_name = CASE WHEN EXCLUDED.company_name <> '' THEN EXCLUDED.company_name ELSE ma_company_bm_family.company_name END,
  suggested_family = EXCLUDED.suggested_family,
  suggested_source = EXCLUDED.suggested_source,
  suggested_evidence = EXCLUDED.suggested_evidence,
  updated_at = now()
`, companyKey, vatCode, taxCode, companyName, suggestion.Family, suggestion.Source, suggestion.Evidence)
	if err != nil {
		return fmt.Errorf("upsert ma bm family suggestion: %w", err)
	}
	return nil
}

// GetMABMFamily ritorna la classificazione di famiglia (nil se mai suggerita né
// ratificata).
func (s *SQLStore) GetMABMFamily(ctx context.Context, companyKey string) (*MABMFamily, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	var out MABMFamily
	var suggestedFamily, suggestedSource, suggestedEvidence, family, ratifiedEmail sql.NullString
	var ratifiedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
SELECT company_key, suggested_family, suggested_source, suggested_evidence, family, ratified_by_email, ratified_at
FROM binocolo.ma_company_bm_family
WHERE company_key = $1
`, companyKey).Scan(&out.CompanyKey, &suggestedFamily, &suggestedSource, &suggestedEvidence, &family, &ratifiedEmail, &ratifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ma bm family: %w", err)
	}
	out.SuggestedFamily = suggestedFamily.String
	out.SuggestedSource = suggestedSource.String
	out.SuggestedEvidence = suggestedEvidence.String
	out.Family = family.String
	out.RatifiedByEmail = ratifiedEmail.String
	if ratifiedAt.Valid {
		ts := ratifiedAt.Time
		out.RatifiedAt = &ts
	}
	return &out, nil
}

// GetMAWebValidationForCompany esposizione read-only della web validation per
// (sessione, azienda): la lettura di tesi (Fase 5) la usa come evidenza
// qualitativa, con la sua data dichiarata.
func (s *SQLStore) GetMAWebValidationForCompany(ctx context.Context, sessionID, companyKey string) (*MAWebValidation, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	return s.loadMAWebValidationForCompany(ctx, sessionID, companyKey)
}

// GetMASessionThesisReading ritorna la lettura di tesi persistita (nil se assente).
func (s *SQLStore) GetMASessionThesisReading(ctx context.Context, sessionID, companyKey string) (*MASessionThesisReading, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	var out MASessionThesisReading
	var webDate sql.NullTime
	var readingRaw []byte
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
SELECT session_id::text, company_key, thesis_snapshot, reading,
       web_evidence_date, generated_by_email, updated_at
FROM binocolo.ma_session_thesis_reading
WHERE session_id = $1::uuid AND company_key = $2
`, sessionID, companyKey).Scan(&out.SessionID, &out.CompanyKey, &out.ThesisSnapshot,
		&readingRaw, &webDate, &out.GeneratedByEmail, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ma session thesis reading: %w", err)
	}
	if webDate.Valid {
		ts := webDate.Time
		out.WebEvidenceDate = &ts
	}
	out.UpdatedAt = &updatedAt
	if len(readingRaw) > 0 {
		var reading MAThesisReading
		if err := json.Unmarshal(readingRaw, &reading); err == nil {
			out.Reading = &reading
		}
	}
	return &out, nil
}

// UpsertMASessionThesisReading persiste la lettura generata (rigenerazione =
// sovrascrittura consapevole: l'azione è esplicita dell'analista).
func (s *SQLStore) UpsertMASessionThesisReading(ctx context.Context, reading *MASessionThesisReading, modelID, promptID, subject string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if reading == nil || reading.Reading == nil {
		return errors.New("ma session thesis reading: missing reading")
	}
	raw, err := json.Marshal(reading.Reading)
	if err != nil {
		return fmt.Errorf("marshal ma thesis reading: %w", err)
	}
	var webDate sql.NullTime
	if reading.WebEvidenceDate != nil {
		webDate = sql.NullTime{Time: *reading.WebEvidenceDate, Valid: true}
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_session_thesis_reading
  (session_id, company_key, thesis_snapshot, reading, web_evidence_date,
   model_id, prompt_id, generated_by_subject, generated_by_email)
VALUES ($1::uuid, $2, $3, $4::jsonb, $5, NULLIF($6, '')::uuid, NULLIF($7, '')::uuid, $8, $9)
ON CONFLICT (session_id, company_key) DO UPDATE SET
  thesis_snapshot = EXCLUDED.thesis_snapshot,
  reading = EXCLUDED.reading,
  web_evidence_date = EXCLUDED.web_evidence_date,
  model_id = EXCLUDED.model_id,
  prompt_id = EXCLUDED.prompt_id,
  generated_by_subject = EXCLUDED.generated_by_subject,
  generated_by_email = EXCLUDED.generated_by_email,
  updated_at = now()
`, reading.SessionID, reading.CompanyKey, reading.ThesisSnapshot, raw, webDate,
		modelID, promptID, subject, reading.GeneratedByEmail)
	if err != nil {
		return fmt.Errorf("upsert ma session thesis reading: %w", err)
	}
	return nil
}

// RatifyMABMFamily registra la ratifica/override dell'analista; family vuota =
// revoca (si torna al solo suggerimento, con caveat in valuation).
func (s *SQLStore) RatifyMABMFamily(ctx context.Context, companyKey, family, subject, email string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	if strings.TrimSpace(companyKey) == "" {
		return errors.New("ma bm family ratify: missing company key")
	}
	if family == "" {
		_, err := s.db.ExecContext(ctx, `
UPDATE binocolo.ma_company_bm_family
SET family = NULL, ratified_by_subject = NULL, ratified_by_email = NULL, ratified_at = NULL, updated_at = now()
WHERE company_key = $1
`, companyKey)
		if err != nil {
			return fmt.Errorf("revoke ma bm family: %w", err)
		}
		return nil
	}
	if !maBMFamilies[family] {
		return errors.New("ma bm family ratify: unknown family")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO binocolo.ma_company_bm_family (company_key, family, ratified_by_subject, ratified_by_email, ratified_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (company_key) DO UPDATE SET
  family = EXCLUDED.family,
  ratified_by_subject = EXCLUDED.ratified_by_subject,
  ratified_by_email = EXCLUDED.ratified_by_email,
  ratified_at = now(),
  updated_at = now()
`, companyKey, family, nullString(subject), nullString(email))
	if err != nil {
		return fmt.Errorf("ratify ma bm family: %w", err)
	}
	return nil
}

func (s *SQLStore) loadSectorMultiple(ctx context.Context, prefix string) (*sectorMultiple, error) {
	var multiple sectorMultiple
	var evEbitda, evSales sql.NullFloat64
	var nFirms sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT ateco_prefix, damodaran_industry, ev_ebitda, ev_sales, n_firms, source, source_date
FROM binocolo.sector_valuation_multiple
WHERE ateco_prefix = $1
`, prefix).Scan(&multiple.Prefix, &multiple.Industry, &evEbitda, &evSales, &nFirms, &multiple.Source, &multiple.SourceDate)
	if err != nil {
		return nil, err
	}
	if evEbitda.Valid {
		v := evEbitda.Float64
		multiple.EVEbitda = &v
	}
	if evSales.Valid {
		v := evSales.Float64
		multiple.EVSales = &v
	}
	if nFirms.Valid {
		multiple.NFirms = int(nFirms.Int64)
	}
	return &multiple, nil
}

func nullInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullIntValue(value int) sql.NullInt64 {
	if value <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}

const maCardIRLItemColumns = `id::text, initiative_id::text, company_key, category, question, source, source_ref, status, position, created_by_email, created_at, updated_at`

func scanMACardIRLItem(scanner interface{ Scan(...any) error }) (MACardIRLItem, error) {
	var item MACardIRLItem
	err := scanner.Scan(
		&item.ID,
		&item.InitiativeID,
		&item.CompanyKey,
		&item.Category,
		&item.Question,
		&item.Source,
		&item.SourceRef,
		&item.Status,
		&item.Position,
		&item.CreatedByEmail,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

// ListMACardIRLItems lista le voci IRL della card in ordine di posizione.
func (s *SQLStore) ListMACardIRLItems(ctx context.Context, initiativeID, companyKey string) ([]MACardIRLItem, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT `+maCardIRLItemColumns+`
FROM binocolo.ma_card_irl_item
WHERE initiative_id = $1::uuid AND company_key = $2
ORDER BY position, created_at
`, initiativeID, companyKey)
	if err != nil {
		return nil, fmt.Errorf("list ma card irl items: %w", err)
	}
	defer rows.Close()
	out := []MACardIRLItem{}
	for rows.Next() {
		item, err := scanMACardIRLItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ma card irl item: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma card irl items: %w", err)
	}
	return out, nil
}

func (s *SQLStore) MaxMACardIRLPosition(ctx context.Context, initiativeID, companyKey string) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("binocolo ma store not configured")
	}
	var max int
	if err := s.db.QueryRowContext(ctx, `
SELECT COALESCE(MAX(position), 0) FROM binocolo.ma_card_irl_item
WHERE initiative_id = $1::uuid AND company_key = $2
`, initiativeID, companyKey).Scan(&max); err != nil {
		return 0, fmt.Errorf("max ma card irl position: %w", err)
	}
	return max, nil
}

// InsertMACardIRLSeed inserisce le proposte del seed SALTANDO i source_ref già
// presenti (indice parziale ma_card_irl_item_seed_unique): è il lucchetto del
// re-seed additivo — la curatela esistente non viene mai toccata. Ritorna il
// totale inserito e lo spaccato per fonte.
func (s *SQLStore) InsertMACardIRLSeed(ctx context.Context, items []MACardIRLItem) (int, map[string]int, error) {
	bySource := map[string]int{}
	if s == nil || s.db == nil {
		return 0, bySource, errors.New("binocolo ma store not configured")
	}
	if len(items) == 0 {
		return 0, bySource, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, bySource, fmt.Errorf("begin ma card irl seed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	inserted := 0
	for _, item := range items {
		var id string
		err := tx.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_card_irl_item (initiative_id, company_key, category, question, source, source_ref, status, position, created_by_email)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (initiative_id, company_key, source, source_ref) WHERE source_ref <> '' DO NOTHING
RETURNING id::text
`, item.InitiativeID, item.CompanyKey, item.Category, item.Question, item.Source, item.SourceRef, item.Status, item.Position, item.CreatedByEmail).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			continue // source_ref già presente: voce curata, non si tocca
		}
		if err != nil {
			return 0, bySource, fmt.Errorf("insert ma card irl seed item: %w", err)
		}
		inserted++
		bySource[item.Source]++
	}
	if err := tx.Commit(); err != nil {
		return 0, bySource, fmt.Errorf("commit ma card irl seed: %w", err)
	}
	return inserted, bySource, nil
}

func (s *SQLStore) InsertMACardIRLItem(ctx context.Context, item MACardIRLItem) (*MACardIRLItem, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	saved, err := scanMACardIRLItem(s.db.QueryRowContext(ctx, `
INSERT INTO binocolo.ma_card_irl_item (initiative_id, company_key, category, question, source, source_ref, status, position, created_by_email)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING `+maCardIRLItemColumns+`
`, item.InitiativeID, item.CompanyKey, item.Category, item.Question, item.Source, item.SourceRef, item.Status, item.Position, item.CreatedByEmail))
	if err != nil {
		return nil, fmt.Errorf("insert ma card irl item: %w", err)
	}
	return &saved, nil
}

// UpdateMACardIRLItem applica la patch parziale (campi nil invariati). Nil
// senza errore quando la voce non appartiene alla card.
func (s *SQLStore) UpdateMACardIRLItem(ctx context.Context, initiativeID, companyKey, itemID string, patch MAIRLItemPatch) (*MACardIRLItem, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	item, err := scanMACardIRLItem(s.db.QueryRowContext(ctx, `
UPDATE binocolo.ma_card_irl_item
SET category   = COALESCE($4, category),
    question   = COALESCE($5, question),
    status     = COALESCE($6, status),
    updated_at = now()
WHERE id = $3::uuid AND initiative_id = $1::uuid AND company_key = $2
RETURNING `+maCardIRLItemColumns+`
`, initiativeID, companyKey, itemID, nullStringPtr(patch.Category), nullStringPtr(patch.Question), nullStringPtr(patch.Status)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update ma card irl item: %w", err)
	}
	return &item, nil
}

func (s *SQLStore) DeleteMACardIRLItem(ctx context.Context, initiativeID, companyKey, itemID string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("binocolo ma store not configured")
	}
	res, err := s.db.ExecContext(ctx, `
DELETE FROM binocolo.ma_card_irl_item
WHERE id = $3::uuid AND initiative_id = $1::uuid AND company_key = $2
`, initiativeID, companyKey, itemID)
	if err != nil {
		return false, fmt.Errorf("delete ma card irl item: %w", err)
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

// ReorderMACardIRLItems riassegna le posizioni secondo l'ordine dell'array
// (passo 10); voci non elencate mantengono la posizione corrente.
func (s *SQLStore) ReorderMACardIRLItems(ctx context.Context, initiativeID, companyKey string, itemIDs []string) error {
	if s == nil || s.db == nil {
		return errors.New("binocolo ma store not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ma card irl reorder: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for index, itemID := range itemIDs {
		if _, err := tx.ExecContext(ctx, `
UPDATE binocolo.ma_card_irl_item
SET position = $4, updated_at = now()
WHERE id = $3::uuid AND initiative_id = $1::uuid AND company_key = $2
`, initiativeID, companyKey, itemID, (index+1)*10); err != nil {
			return fmt.Errorf("reorder ma card irl item: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ma card irl reorder: %w", err)
	}
	return nil
}

func (s *SQLStore) ListMAIRLTemplates(ctx context.Context, family string) ([]MAIRLTemplate, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id::text, family, category, question, position
FROM binocolo.ma_irl_template
WHERE family = $1
ORDER BY position, question
`, family)
	if err != nil {
		return nil, fmt.Errorf("list ma irl templates: %w", err)
	}
	defer rows.Close()
	out := []MAIRLTemplate{}
	for rows.Next() {
		var t MAIRLTemplate
		if err := rows.Scan(&t.ID, &t.Family, &t.Category, &t.Question, &t.Position); err != nil {
			return nil, fmt.Errorf("scan ma irl template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ma irl templates: %w", err)
	}
	return out, nil
}

// nullStringPtr: NULL quando il puntatore è nil (patch parziale).
func nullStringPtr(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}
