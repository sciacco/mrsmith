package binocolo

import (
	"encoding/json"
	"time"
)

const (
	maStrategyTypeATECO    = "ateco"
	maStrategyTypeExpanded = "expanded"

	maSessionStatusDraft      = "draft"
	maSessionStatusEstimating = "estimating"
	maSessionStatusEstimated  = "estimated"
	maSessionStatusRunning    = "running"
	maSessionStatusCompleted  = "completed"
	maSessionStatusFailed     = "failed"

	maSessionVisibilityActive   = "active"
	maSessionVisibilityArchived = "archived"
	maSessionVisibilityDeleted  = "deleted"

	maSessionLifecycleArchive = "archive"
	maSessionLifecycleRestore = "restore"
	maSessionLifecycleDelete  = "delete"
	maSessionLifecyclePurge   = "purge"

	maRunStatusRunning   = "running"
	maRunStatusCompleted = "completed"
	maRunStatusFailed    = "failed"

	maMatchStateMatch   = "match"
	maMatchStatePartial = "match_parziale"
	maMatchStateOutside = "fuori_criterio"

	// ATECO fit tiers (per-candidate relevance). neutral is never stored on a
	// candidate — it is the resolved fit of a code matching no candidate prefix.
	maFitCore     = "core"
	maFitWeak     = "weak"
	maFitExcluded = "excluded"
	maFitNeutral  = "neutral"

	// Veryshort rating preferiti: -1 escluso, 1..3 stelle, 0/assente = non valutato.
	maRatingExcluded = -1
	maRatingMax      = 3

	// Veryshort analisi approfondita (IT-full) lifecycle.
	maDeepStatusQueued  = "queued"
	maDeepStatusRunning = "running"
	maDeepStatusReady   = "ready"
	maDeepStatusFailed  = "failed"

	// maDeepMaxAttempts bounds how many worker ticks poll a running IT-full job
	// before it is marked failed (timeout guard for the async vendor request).
	// At the 5s tick interval this is ~5 minutes of polling; the attempt counter
	// is reset when a job is claimed (queued->running), so it is a pure poll budget.
	maDeepMaxAttempts = 60

	// maDeepLeaseSeconds is how long a worker owns a deep-analysis row after acquiring
	// its lease. Longer than the slowest single process() (the IT-full POST plus the
	// LLM brief), short enough that a crashed worker's rows are reclaimed promptly.
	maDeepLeaseSeconds = 60

	// ma_job async queue (estimate/execute/web validation run off the request path). Same state
	// machine as the deep worker; see deploy/migrations/049.
	maJobTypeEstimate      = "estimate"
	maJobTypeExecute       = "execute"
	maJobTypeWebValidation = "web_validation"

	maJobStatusQueued  = "queued"
	maJobStatusRunning = "running"
	// Rollout-safe active states for job types introduced after the original
	// ma_job worker. Older binaries only poll queued/running, so they ignore
	// pending/processing rows during progressive deploys.
	maJobStatusPending    = "pending"
	maJobStatusProcessing = "processing"
	maJobStatusReady      = "ready"
	maJobStatusFailed     = "failed"

	// maJobMaxAttempts bounds retries of a failing job before it is marked failed
	// (and the session moved to 'failed'). Estimate work is cheap+cached, so a
	// retry re-runs the whole job; the cap guards against a permanently broken
	// upstream rather than a poll budget.
	maJobMaxAttempts = 5

	// maJobLeaseSeconds is how long a worker owns a job row. The whole estimate runs
	// inside one process() call, so this must exceed the slowest single estimate
	// (parallelized probe fan-out); a crashed worker's row is reclaimed after it.
	maJobLeaseSeconds = 300

	// maEstimateProbeConcurrency caps concurrent dry-run probes against OpenAPI.it
	// during an estimate fan-out: polite to the upstream while collapsing the
	// previously-sequential wall-clock from ~90s to seconds.
	maEstimateProbeConcurrency = 6

	maEvidenceMatch   = "match"
	maEvidencePartial = "match_parziale"
	maEvidenceMissing = "criterio_mancante"
	maEvidenceOutside = "fuori_criterio"

	maModelScopeStrategy               = "ma_strategy"
	maModelScopeStrategyIntent         = "ma_strategy_intent"
	maModelScopeStrategyAteco          = "ma_strategy_ateco"
	maModelScopeStrategyAtecoHierarchy = "ma_strategy_ateco_hierarchy"
	maModelScopeSectorClassification   = "ma_sector_classification"
	maModelScopeDeepBrief              = "ma_deep_brief"
	maModelScopeWebSearchScorer        = "web_search_scorer"
	maModelScopeCandidateMatchAnalyst  = "candidate_match_analyst"

	maWebValidationPipelineVersion = "candidate-web-validation-v2-concept"
	maWebValidationFresh           = "fresh"
	maWebValidationStale           = "stale"
	maWebValidationExpired         = "expired"

	maEstimateSurfaceExact    = "exact"
	maEstimateSurfaceTooBroad = "too_broad"

	maATECOSuccessThreshold = 10
	maDefaultSearchLimit    = 100
	maVendorLimit           = 1000

	// maAtecoSubtreeProbeCap bounds how many dry-run probes (€0.01 each) the
	// subtree discovery may spend resolving which exact ATECO codes are
	// populated. Beyond this the selected sector is too broad to probe code by
	// code and the original selection is kept untouched.
	maAtecoSubtreeProbeCap = 60

	// maCostPerCompanyEUR is the OpenAPI.it advanced-enrichment price per
	// returned company; maCostPerDryRunEUR is the dry-run (count-only) price.
	maCostPerCompanyEUR = 0.10
	maCostPerDryRunEUR  = 0.01
	// maCostPerFullEUR is the IT-full (veryshort deep-dive) price per company;
	// the runtime value is overridable via the ma_parameter table (migration 038).
	maCostPerFullEUR = 0.30

	// Valuation defaults (Fase 4), overridable via ma_parameter (sme_haircut_pct,
	// ebitda_fallback_threshold). Haircut is a percent applied to sector multiples;
	// below the EBITDA-margin threshold (or EBITDA<=0) valuation falls back to EV/Sales.
	maSMEHaircutPctDefault     = 30.0
	maEBITDAFallbackPctDefault = 5.0

	// maDefaultBudgetEUR caps the projected enrichment spend of a single run
	// unless the analyst explicitly acknowledges a higher cost.
	maDefaultBudgetEUR = 50.0

	// maThesisFitHoldingHaircutPctDefault is the score haircut applied to a
	// holding-controlled company under the succession thesis (a clear corporate
	// majority owner is the antithesis of an exiting individual). Overridable via
	// ma_parameter (thesis_fit_holding_haircut_pct); mirrors sme_haircut_pct.
	// A haircut, not a knockout: the company stays visible, just demoted.
	maThesisFitHoldingHaircutPctDefault = 60.0

	// maSuccessionDefaultMinAge is the owner-age threshold used by the succession
	// signal/flag when the strategy does not specify SuccessionMinOwnerAge.
	maSuccessionDefaultMinAge = 60
)

type MACreateSessionRequest struct {
	Prompt   string `json:"prompt"`
	ModelID  string `json:"modelId,omitempty"`
	PromptID string `json:"promptId,omitempty"`
}

type MAEstimateSessionRequest struct {
	Strategy *MAStrategySpec `json:"strategy,omitempty"`
}

type MAExecuteSessionRequest struct {
	Strategy     *MAStrategySpec `json:"strategy,omitempty"`
	StrategyType string          `json:"strategyType,omitempty"`
	Limit        int             `json:"limit,omitempty"`
	// AcknowledgeCost lets the analyst proceed when the projected enrichment
	// spend exceeds the budget ceiling (the cost gate).
	AcknowledgeCost bool `json:"acknowledgeCost,omitempty"`
}

type MAExportRequest struct {
	Format string `json:"format,omitempty"`
}

type MATargetRatingRequest struct {
	CompanyKey string `json:"companyKey"`
	Rating     int    `json:"rating"`
}

type MAWebValidationUpsertRequest struct {
	PipelineVersion        string                          `json:"pipelineVersion,omitempty"`
	InputHash              string                          `json:"inputHash,omitempty"`
	KeywordSetHash         string                          `json:"keywordSetHash,omitempty"`
	Target                 MATarget                        `json:"target"`
	KeywordSet             CandidateMatchKeywordSet        `json:"keywordSet"`
	DomainResponse         DomainResolutionResponse        `json:"domainResponse"`
	SelectedDomain         *DomainResolutionCandidate      `json:"selectedDomain,omitempty"`
	EvidenceRuns           []CandidateMatchEvidenceRun     `json:"evidenceRuns"`
	Summary                CandidateMatchEvidenceSummary   `json:"summary"`
	CandidateMatchAnalysis *CandidateMatchAnalysisResponse `json:"candidateMatchAnalysis,omitempty"`
	CandidateMatchError    string                          `json:"candidateMatchError,omitempty"`
	FinalDecision          CandidateMatchFinalDecision     `json:"finalDecision"`
}

type MAWebValidationEnrichRequest struct {
	Limit              int   `json:"limit,omitempty"`
	Force              bool  `json:"force,omitempty"`
	IncludeIdentifiers bool  `json:"includeIdentifiers,omitempty"`
	AnalyzeWithLLM     *bool `json:"analyzeWithLLM,omitempty"`
	LLMOnAll           *bool `json:"llmOnAll,omitempty"`
	DomainCount        int   `json:"domainCount,omitempty"`
	KeywordCount       int   `json:"keywordCount,omitempty"`
	Rank               *bool `json:"rank,omitempty"`
	// Inline (dev-only): run the work in-process instead of enqueuing on the shared
	// ma_job queue, so a foreign worker on the shared DB can't claim it with stale code.
	Inline bool `json:"inline,omitempty"`
}

type MAParameter struct {
	Key            string     `json:"key"`
	Value          string     `json:"value"`
	ValueType      string     `json:"valueType"`
	Label          string     `json:"label"`
	Description    string     `json:"description,omitempty"`
	UpdatedByEmail string     `json:"updatedByEmail,omitempty"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
}

type MAParameterUpdateRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MALLMOptionsResponse struct {
	Models  []MALLMModelOption  `json:"models"`
	Prompts []MALLMPromptOption `json:"prompts"`
}

type MALLMModelOption struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`
	Name      string `json:"name"`
	Model     string `json:"model"`
	IsDefault bool   `json:"isDefault"`
}

type MALLMPromptOption struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

type MASessionSummary struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Prompt           string     `json:"prompt"`
	Status           string     `json:"status"`
	SelectedStrategy string     `json:"selectedStrategy,omitempty"`
	EstimatedCount   int        `json:"estimatedCount"`
	EstimatedCost    float64    `json:"estimatedCost"`
	ResultCount      int        `json:"resultCount"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	LastRunAt        *time.Time `json:"lastRunAt,omitempty"`
	ArchivedAt       *time.Time `json:"archivedAt,omitempty"`
	ArchivedByEmail  string     `json:"archivedByEmail,omitempty"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
	DeletedByEmail   string     `json:"deletedByEmail,omitempty"`
}

type MASessionDetail struct {
	Session   MASession          `json:"session"`
	Strategy  *MAStrategyVersion `json:"strategy,omitempty"`
	Estimates []MAEstimate       `json:"estimates"`
	Runs      []MAExecutionRun   `json:"runs"`
	Targets   []MATarget         `json:"targets"`
	// Cost summary (computed, not persisted). BudgetEUR is the active ceiling and
	// CostPerCompanyEUR the advanced-enrichment unit price, so the UI can show the
	// projected spend and the cost gate without duplicating the pricing constant.
	BudgetEUR         float64 `json:"budgetEur"`
	CostPerCompanyEUR float64 `json:"costPerCompanyEur"`
	CostFullEUR       float64 `json:"costFullEur"`
}

type MASession struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	Prompt            string     `json:"prompt"`
	Status            string     `json:"status"`
	SelectedStrategy  string     `json:"selectedStrategy,omitempty"`
	ActiveStrategyID  string     `json:"activeStrategyId,omitempty"`
	CreatedBySubject  string     `json:"createdBySubject,omitempty"`
	CreatedByEmail    string     `json:"createdByEmail,omitempty"`
	LastEstimatedAt   *time.Time `json:"lastEstimatedAt,omitempty"`
	LastExecutedAt    *time.Time `json:"lastExecutedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	ArchivedAt        *time.Time `json:"archivedAt,omitempty"`
	ArchivedBySubject string     `json:"archivedBySubject,omitempty"`
	ArchivedByEmail   string     `json:"archivedByEmail,omitempty"`
	DeletedAt         *time.Time `json:"deletedAt,omitempty"`
	DeletedBySubject  string     `json:"deletedBySubject,omitempty"`
	DeletedByEmail    string     `json:"deletedByEmail,omitempty"`
}

type MAStrategyVersion struct {
	ID             string         `json:"id"`
	SessionID      string         `json:"sessionId"`
	Version        int            `json:"version"`
	Strategy       MAStrategySpec `json:"strategy"`
	CreatedByEmail string         `json:"createdByEmail,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type MAStrategySpec struct {
	Title                  string               `json:"title,omitempty"`
	SectorDescription      string               `json:"sectorDescription"`
	TerritoryLabel         string               `json:"territoryLabel,omitempty"`
	Provinces              []string             `json:"provinces"`
	ActivityStatus         string               `json:"activityStatus"`
	TurnoverAround         *int                 `json:"turnoverAround,omitempty"`
	TurnoverMin            *int                 `json:"turnoverMin,omitempty"`
	TurnoverMax            *int                 `json:"turnoverMax,omitempty"`
	EmployeeMin            *int                 `json:"employeeMin,omitempty"`
	EmployeeMax            *int                 `json:"employeeMax,omitempty"`
	RevenuePerEmployeeMin  *int                 `json:"revenuePerEmployeeMin,omitempty"`
	MaxShareholders        *int                 `json:"maxShareholders,omitempty"`
	SearchLimit            int                  `json:"searchLimit"`
	AtecoCandidates        []MAAtecoCandidate   `json:"atecoCandidates"`
	Keywords               []string             `json:"keywords"`
	ScoringCriteria        []MAScoringCriterion `json:"scoringCriteria,omitempty"` // deprecated: free-form criteria, no longer generated
	Rationale              string               `json:"rationale"`
	MissingCriteria        []string             `json:"missingCriteria"`
	SelectedStrategy       string               `json:"selectedStrategy,omitempty"`
	ExpandedClassification string               `json:"expandedClassification,omitempty"`
	// Scoring v2: acquisition thesis (drives signal directions + family weights),
	// explicit legal-form constraint (server-side filter, removed from ranking),
	// and per-signal weight overrides selected from the scoring catalog.
	Thesis        string         `json:"thesis,omitempty"`
	LegalForms    []string       `json:"legalForms,omitempty"`
	SignalWeights map[string]int `json:"signalWeights,omitempty"`
	// MaxBudgetEUR overrides the per-run enrichment budget ceiling; nil uses
	// maDefaultBudgetEUR. Stored in the strategy JSONB blob (migration-free).
	MaxBudgetEUR *float64 `json:"maxBudgetEur,omitempty"`
	// SuccessionMinOwnerAge is the owner-age threshold (years) for the succession
	// thesis: it drives the succession_owner signal ramp and the ricambio flag.
	// Extracted by the strategy LLM; nil falls back to maSuccessionDefaultMinAge.
	SuccessionMinOwnerAge *int `json:"successionMinOwnerAge,omitempty"`
	// --- Transient (json:"-"): computed in-flight from the persisted fields above,
	// never stored. They survive only for the duration of one estimate/execute call.

	// SectorDivisions holds the distinct 2-digit ATECO divisions of the non-excluded
	// (core/weak) candidates. The curated candidate list stays intact for fit
	// resolution; this is derived once at canonicalize so the perimeter keeps a
	// division the analyst intended even if a leaf turns out unpopulated. Drives the
	// expanded net and the scoring gate.
	SectorDivisions []string `json:"-"`
	// AtecoQueryCandidates is the ATECO strategy's RETRIEVAL set: the populated
	// subtree of the core/weak candidates minus the excluded subtrees. Kept separate
	// from AtecoCandidates so the curated list (with fit) survives for scoring while
	// retrieval queries the exact populated leaves.
	AtecoQueryCandidates []MAAtecoCandidate `json:"-"`
	// ExpandedAtecoCandidates is the "expanded" strategy's retrieval set: the
	// populated subtree of SectorDivisions minus excluded. The expanded search
	// iterates these codes instead of dropping the ATECO filter entirely (which
	// retrieved the whole provincial economy). Empty for sector-less strategies.
	ExpandedAtecoCandidates []MAAtecoCandidate `json:"-"`
}

// MAIntent is the structured, trace-only intermediate contract for the M&A
// strategy v2 pipeline. Every field that can become a search, exclusion, or
// scoring constraint carries sourceText so validateIntentSpans can enforce that
// the constraint is grounded in the original analyst request.
type MAIntent struct {
	Title                 string                         `json:"title,omitempty"`
	Territory             MAIntentTerritory              `json:"territory,omitempty"`
	AtecoExplicit         []MAIntentAtecoConstraint      `json:"atecoExplicit,omitempty"`
	Sectors               MAIntentSectors                `json:"sectors,omitempty"`
	Turnover              *MAIntentNumericConstraint     `json:"turnover,omitempty"`
	Employees             *MAIntentNumericConstraint     `json:"employees,omitempty"`
	LegalForms            []MAIntentTextConstraint       `json:"legalForms,omitempty"`
	OwnerAge              *MAIntentNumericConstraint     `json:"ownerAge,omitempty"`
	Status                *MAIntentStatusConstraint      `json:"status,omitempty"`
	RevenuePerEmployeeMin *MAIntentValueConstraint       `json:"revenuePerEmployeeMin,omitempty"`
	MaxShareholders       *MAIntentValueConstraint       `json:"maxShareholders,omitempty"`
	Constraints           []MAIntentAdditionalConstraint `json:"constraints,omitempty"`
	Thesis                string                         `json:"thesis,omitempty"`
}

type MAIntentTerritory struct {
	IncludeRegions   []MAIntentTerritoryConstraint `json:"includeRegions,omitempty"`
	IncludeProvinces []MAIntentTerritoryConstraint `json:"includeProvinces,omitempty"`
	ExcludeProvinces []MAIntentTerritoryConstraint `json:"excludeProvinces,omitempty"`
}

type MAIntentTerritoryConstraint struct {
	Value       string `json:"value"`
	SourceText  string `json:"sourceText"`
	Disposition string `json:"disposition,omitempty"`
}

type MAIntentAtecoConstraint struct {
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
	Fit         string `json:"fit,omitempty"`
	SourceText  string `json:"sourceText"`
	Disposition string `json:"disposition,omitempty"`
}

type MAIntentSectors struct {
	Include []MAIntentTextConstraint `json:"include,omitempty"`
	Exclude []MAIntentTextConstraint `json:"exclude,omitempty"`
}

type MAIntentTextConstraint struct {
	Text        string `json:"text"`
	SourceText  string `json:"sourceText"`
	Disposition string `json:"disposition,omitempty"`
}

type MAIntentNumericConstraint struct {
	Mode        string `json:"mode,omitempty"`
	Min         *int   `json:"min,omitempty"`
	Max         *int   `json:"max,omitempty"`
	Around      *int   `json:"around,omitempty"`
	Unit        string `json:"unit,omitempty"`
	SourceText  string `json:"sourceText"`
	Disposition string `json:"disposition,omitempty"`
}

type MAIntentStatusConstraint struct {
	Value       string `json:"value"`
	SourceText  string `json:"sourceText"`
	Disposition string `json:"disposition,omitempty"`
}

type MAIntentValueConstraint struct {
	Value       *int   `json:"value,omitempty"`
	SourceText  string `json:"sourceText"`
	Disposition string `json:"disposition,omitempty"`
}

type MAIntentAdditionalConstraint struct {
	Kind        string `json:"kind,omitempty"`
	Disposition string `json:"disposition"`
	Text        string `json:"text"`
	SourceText  string `json:"sourceText,omitempty"`
}

type MAAtecoCandidate struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Rationale   string `json:"rationale"`
	// Fit is the curated relevance tier of this ATECO entry: "core" (bullseye),
	// "weak" (adjacent, kept but discounted) or "excluded" (removed from perimeter,
	// even inside an included group). Empty defaults to core. A target's fit is the
	// fit of the candidate that is its longest code prefix; codes matching no
	// candidate are "neutral". Drives ateco precision, the sector gate and retrieval.
	Fit        string `json:"fit,omitempty"`
	SearchCode string `json:"-"`
}

type MAScoringCriterion struct {
	ID          string              `json:"id"`
	Label       string              `json:"label"`
	Description string              `json:"description,omitempty"`
	Weight      int                 `json:"weight"`
	Evaluation  MAScoringEvaluation `json:"evaluation"`
	Source      string              `json:"source,omitempty"`
}

type MAScoringEvaluation struct {
	SourcePath string   `json:"sourcePath,omitempty"`
	Operator   string   `json:"operator,omitempty"`
	Value      any      `json:"value,omitempty"`
	Min        *float64 `json:"min,omitempty"`
	Max        *float64 `json:"max,omitempty"`
	Tolerance  *float64 `json:"tolerance,omitempty"`
	Match      string   `json:"match,omitempty"`
}

type MAEstimate struct {
	ID                string          `json:"id"`
	SessionID         string          `json:"sessionId"`
	StrategyVersionID string          `json:"strategyVersionId"`
	StrategyType      string          `json:"strategyType"`
	AtecoCode         string          `json:"atecoCode,omitempty"`
	AtecoDescription  string          `json:"atecoDescription,omitempty"`
	Province          string          `json:"province,omitempty"`
	EstimatedCount    int             `json:"estimatedCount"`
	EstimatedCost     float64         `json:"estimatedCost"`
	Selected          bool            `json:"selected"`
	SurfaceStatus     string          `json:"surfaceStatus"`
	ExecutionLimit    int             `json:"executionLimit"`
	ProbeCount        int             `json:"probeCount"`
	Params            json.RawMessage `json:"params,omitempty"`
	VendorResponse    json.RawMessage `json:"vendorResponse,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
}

type MAExecutionRun struct {
	ID                string     `json:"id"`
	SessionID         string     `json:"sessionId"`
	StrategyVersionID string     `json:"strategyVersionId"`
	StrategyType      string     `json:"strategyType"`
	Status            string     `json:"status"`
	EstimatedCount    int        `json:"estimatedCount"`
	ResultCount       int        `json:"resultCount"`
	ErrorCode         string     `json:"errorCode,omitempty"`
	StartedAt         time.Time  `json:"startedAt"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
}

type MATarget struct {
	ID               string               `json:"id"`
	SessionID        string               `json:"sessionId"`
	RunID            string               `json:"runId"`
	VendorID         string               `json:"vendorId,omitempty"`
	CompanyKey       string               `json:"companyKey,omitempty"`
	CompanyName      string               `json:"companyName"`
	VATCode          string               `json:"vatCode,omitempty"`
	TaxCode          string               `json:"taxCode,omitempty"`
	Province         string               `json:"province,omitempty"`
	Town             string               `json:"town,omitempty"`
	ActivityStatus   string               `json:"activityStatus,omitempty"`
	Turnover         *int                 `json:"turnover,omitempty"`
	TurnoverYear     *int                 `json:"turnoverYear,omitempty"`
	Employees        *int                 `json:"employees,omitempty"`
	AtecoCode        string               `json:"atecoCode,omitempty"`
	AtecoDescription string               `json:"atecoDescription,omitempty"`
	Score            int                  `json:"score"`
	MatchState       string               `json:"matchState"`
	Confidence       string               `json:"confidence,omitempty"`
	Rating           *int                 `json:"rating,omitempty"`
	Flags            []MATargetFlag       `json:"flags,omitempty"`
	Rationale        string               `json:"rationale"`
	MissingCriteria  []string             `json:"missingCriteria"`
	Evidence         []MATargetEvidence   `json:"evidence"`
	Adjustments      []MATargetAdjustment `json:"adjustments,omitempty"`
	Deep             *MADeepAnalysis      `json:"deep,omitempty"`
	WebValidation    *MAWebValidation     `json:"webValidation,omitempty"`
	VendorPayload    json.RawMessage      `json:"vendorPayload,omitempty"`
	CreatedAt        time.Time            `json:"createdAt"`
}

type MATargetEvidence struct {
	Criterion string  `json:"criterion"`
	Status    string  `json:"status"`
	Family    string  `json:"family,omitempty"`
	Label     string  `json:"label"`
	Value     string  `json:"value,omitempty"`
	Points    float64 `json:"points"`
	// Weight is the criterion's effective share of the re-normalized 100-point
	// budget for active rows (so Points <= Weight), and the nominal weight for
	// missing rows (Points is 0, shown as the foregone budget). See ma_scoring.go.
	Weight     float64 `json:"weight"`
	SourcePath string  `json:"sourcePath,omitempty"`
}

// MATargetFlag is a non-scoring annotation surfaced for manual triage.
// Severity is "neutral" (deal profile) or "warning" (data/risk caveats).
type MATargetFlag struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Severity string `json:"severity"`
}

type MAWebValidation struct {
	SessionID              string                          `json:"sessionId"`
	CompanyKey             string                          `json:"companyKey"`
	TargetID               string                          `json:"targetId,omitempty"`
	RunID                  string                          `json:"runId,omitempty"`
	PipelineVersion        string                          `json:"pipelineVersion"`
	InputHash              string                          `json:"inputHash,omitempty"`
	KeywordSetHash         string                          `json:"keywordSetHash,omitempty"`
	LLMModelID             string                          `json:"llmModelId,omitempty"`
	LLMPromptID            string                          `json:"llmPromptId,omitempty"`
	LLMModel               string                          `json:"llmModel,omitempty"`
	Freshness              string                          `json:"freshness"`
	StaleAfter             time.Time                       `json:"staleAfter"`
	ExpiresAt              time.Time                       `json:"expiresAt"`
	SelectedDomain         string                          `json:"selectedDomain,omitempty"`
	DomainConfidence       string                          `json:"domainConfidence,omitempty"`
	DomainScore            *int                            `json:"domainScore,omitempty"`
	WebScore               int                             `json:"webScore"`
	WebConfidence          string                          `json:"webConfidence,omitempty"`
	WebValidationState     string                          `json:"webValidationState"`
	FinalAction            string                          `json:"finalAction"`
	AnalystVerdict         string                          `json:"analystVerdict,omitempty"`
	AnalystAction          string                          `json:"analystAction,omitempty"`
	AnalystConfidence      string                          `json:"analystConfidence,omitempty"`
	Summary                CandidateMatchEvidenceSummary   `json:"summary"`
	KeywordSet             CandidateMatchKeywordSet        `json:"keywordSet"`
	SelectedDomainPayload  *DomainResolutionCandidate      `json:"selectedDomainPayload,omitempty"`
	DomainResponse         DomainResolutionResponse        `json:"domainResponse"`
	EvidenceRuns           []CandidateMatchEvidenceRun     `json:"evidenceRuns"`
	CandidateMatchAnalysis *CandidateMatchAnalysisResponse `json:"candidateMatchAnalysis,omitempty"`
	CandidateMatchError    string                          `json:"candidateMatchError,omitempty"`
	FinalDecision          CandidateMatchFinalDecision     `json:"finalDecision"`
	UpdatedByEmail         string                          `json:"updatedByEmail,omitempty"`
	UpdatedAt              time.Time                       `json:"updatedAt"`
}

// MATargetAdjustment is a multiplicative score factor applied OUTSIDE the
// additive signal blend (viability, thesis-fit). It is surfaced so the displayed
// score reconstructs from the breakdown: sum(evidence.points) x prod(factors).
// Only emitted when Factor < 1 (a no-op factor stays silent, like the confidence
// caveat). Factor is in (0,1]; Code links to the explaining flag (e.g. controllo_holding).
type MATargetAdjustment struct {
	Code   string  `json:"code"`
	Label  string  `json:"label"`
	Factor float64 `json:"factor"`
}

// MADeepAnalysis is the veryshort deep-dive artifact for one company (global,
// cached across sessions). Status drives the worker; Scorecard is filled in
// Fase 3, Valuation in Fase 4, Brief in Fase 5.
type MADeepAnalysis struct {
	CompanyKey string           `json:"companyKey"`
	Status     string           `json:"status"`
	Scorecard  *MADeepScorecard `json:"scorecard,omitempty"`
	Valuation  *MADeepValuation `json:"valuation,omitempty"`
	Brief      *MADeepBrief     `json:"brief,omitempty"`
	CostEUR    float64          `json:"costEur,omitempty"`
	ErrorCode  string           `json:"errorCode,omitempty"`
	UpdatedAt  *time.Time       `json:"updatedAt,omitempty"`
}

// MADeepScorecard is the deterministic financial reading of the IT-full payload:
// pre-computed KPIs read verbatim + RAG semaphores. Raw values (turnover, ebitda,
// netWorth, pfn) are carried for the valuation (Fase 4) and brief (Fase 5).
type MADeepScorecard struct {
	Metrics      []MADeepMetric `json:"metrics"`
	OverallRAG   string         `json:"overallRag"`
	Turnover     *float64       `json:"turnover,omitempty"`
	TurnoverYear *int           `json:"turnoverYear,omitempty"`
	Ebitda       *float64       `json:"ebitda,omitempty"`
	NetWorth     *float64       `json:"netWorth,omitempty"`
	PFN          *float64       `json:"pfn,omitempty"`
	AtecoCode    string         `json:"atecoCode,omitempty"`
}

type MADeepMetric struct {
	Group string   `json:"group"`
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Value *float64 `json:"value,omitempty"`
	Unit  string   `json:"unit"`
	RAG   string   `json:"rag"`
}

// MADeepValuation is populated in Fase 4 (Damodaran sector multiples).
type MADeepValuation struct {
	Method     string   `json:"method"`
	Multiple   float64  `json:"multiple"`
	HaircutPct float64  `json:"haircutPct"`
	EVLow      float64  `json:"evLow"`
	EVHigh     float64  `json:"evHigh"`
	EquityLow  *float64 `json:"equityLow,omitempty"`
	EquityHigh *float64 `json:"equityHigh,omitempty"`
	PFN        *float64 `json:"pfn,omitempty"`
	Sector     string   `json:"sector,omitempty"`
	NFirms     int      `json:"nFirms,omitempty"`
	Source     string   `json:"source,omitempty"`
	SourceDate string   `json:"sourceDate,omitempty"`
	Caveat     string   `json:"caveat,omitempty"`
}

// MADeepBrief is populated in Fase 5 (LLM narrative; numbers stay in scorecard/valuation).
type MADeepBrief struct {
	Verdict            string            `json:"verdict,omitempty"`
	RAG                string            `json:"rag,omitempty"`
	BusinessProfile    string            `json:"businessProfile,omitempty"`
	ThesisReading      string            `json:"thesisReading,omitempty"`
	Strengths          []string          `json:"strengths,omitempty"`
	RedFlags           []MADeepBriefFlag `json:"redFlags,omitempty"`
	ValuationRationale string            `json:"valuationRationale,omitempty"`
	DDQuestions        []string          `json:"ddQuestions,omitempty"`
	ThesisFit          string            `json:"thesisFit,omitempty"`
}

type MADeepBriefFlag struct {
	Severity   string `json:"severity"`
	Category   string `json:"category,omitempty"`
	Claim      string `json:"claim"`
	DDQuestion string `json:"ddQuestion,omitempty"`
}

// maDeepResult bundles what the worker persists when an IT-full job completes.
type maDeepResult struct {
	Payload   json.RawMessage
	Scorecard *MADeepScorecard
	Valuation *MADeepValuation
	Brief     *MADeepBrief
	ModelID   string
	PromptID  string
	CostEUR   float64
}

type MADeepDiveRequest struct {
	AcknowledgeCost bool `json:"acknowledgeCost,omitempty"`
}

// MACompanyDossier is the standalone P.IVA lookup response: our elaborations
// (scorecard/valuation/brief) plus the raw IT-full payload for the facts layer.
// Status drives the frontend: absent | cost_required | queued | running | ready | failed.
type MACompanyDossier struct {
	VATCode   string           `json:"vatCode"`
	Status    string           `json:"status"`
	Scorecard *MADeepScorecard `json:"scorecard,omitempty"`
	Valuation *MADeepValuation `json:"valuation,omitempty"`
	Brief     *MADeepBrief     `json:"brief,omitempty"`
	Raw       json.RawMessage  `json:"raw,omitempty"`
	CostEUR   float64          `json:"costEur,omitempty"`
	ErrorCode string           `json:"errorCode,omitempty"`
	UpdatedAt *time.Time       `json:"updatedAt,omitempty"`
}

type maStrategyDraftEnvelope struct {
	Strategy MAStrategySpec `json:"strategy"`
}

type maSessionCreate struct {
	Session  MASession
	Strategy MAStrategySpec
}

type maExecutionRunCreate struct {
	ID                string
	SessionID         string
	StrategyVersionID string
	StrategyType      string
	EstimatedCount    int
}
