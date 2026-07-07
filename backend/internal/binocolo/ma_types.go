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
	// maJobTypeGatedSearch is the gated-search pipeline: a single multi-stage job
	// (surface → address → UC2 gate → advanced+score on survivors) that reuses the
	// engines above but reorders them so only gate survivors pay the €0.10 Advanced.
	maJobTypeGatedSearch = "gated_search"
	// maJobTypeAssociateDomain re-gates ONE company held in manual_review (domain
	// unresolved) with an operator-supplied domain and, if it now survives, enriches +
	// re-scores it. Durable/queued (production remedy for the manual_review bucket, not
	// inline) so a crash resumes and the pre-leased row can't be stolen on the shared DB.
	maJobTypeAssociateDomain = "associate_domain"

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
	maModelScopeThesisReading          = "ma_thesis_reading"
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

	// Gated-search pipeline unit costs (migration 074), overridable via ma_parameter.
	// maCostPerAddressEUR is the OpenAPI.it IT-address (identity-only) price used by
	// the gate on the whole surface; the fastcrw scrape/search costs are per page /
	// per search of domain resolution + evidence. ($0.001 ≈ €0.001 at this scale.)
	maCostPerAddressEUR    = 0.01
	maCostPerScrapePageEUR = 0.001
	maCostPerSearchEUR     = 0.001

	// Gated-search cost-estimate assumptions (Step 1). Per company on the surface the
	// gate pays: one Address enrichment, up to maGateScrapePagesEst scraped/crawled
	// pages (the crawl cap is the conservative blended upper bound), and
	// maGateSearchesEst domain searches. survivor_rate_default is the expected
	// keep+forse fraction that then pays Advanced; the projected-cost band spans
	// maSurvivorRateLow..maSurvivorRateHigh.
	maGateScrapePagesEst  = maCrawlMaxPages
	maGateSearchesEst     = 2
	maSurvivorRateDefault = 0.35
	maSurvivorRateLow     = 0.25
	maSurvivorRateHigh    = 0.50

	// maGatedSurfaceCapDefault caps the surface a gated search may admit. With
	// "tutto automatico" spend, the surface cap is the one hard governor: the gate
	// pays ~€0.02/company across the WHOLE surface, so N is the cost driver. The gate
	// batch is no longer clamped in the gated flow (gatedSearchJobWork gates the whole
	// surface), so this can exceed the standalone gate's 100-cap: 1000 is a runaway
	// backstop (gate ~€20 at full surface), overridable via ma_parameter
	// (gated_surface_cap).
	maGatedSurfaceCapDefault = 1000

	// Enrichment levels of a ma_target row (migration 075). The execute path and
	// the gated enrich_score stage produce "advanced" (scored) rows; the gated
	// address stage produces "address" (identity-only, score/match_state NULL).
	// These are also the OpenAPI.it DataEnrichment values passed to IT-search.
	maEnrichmentAddress  = "address"
	maEnrichmentAdvanced = "advanced"

	// Gated-search verdict buckets. Survivors = keep+forse pay Advanced. scarta (rejected)
	// and manual_review (official domain unresolved → held for manual domain association)
	// stay identity-only and are NEVER auto-charged. sectorActionToBucket maps the shared
	// confirm/reject/… actions onto keep/scarta/forse; gatedTargetBucket layers the
	// manual_review carve-out on top so domain-unresolved companies are not auto-processed.
	maGatedBucketKeep         = "keep"
	maGatedBucketForse        = "forse"
	maGatedBucketReject       = "scarta"
	maGatedBucketManualReview = "manual_review"

	// Cross-session verified-domain registry methods (mig 082). auto_verified =
	// the target's P.IVA/CF was confirmed on-page during resolution; manual = the
	// operator associated the domain. The upsert never lets auto_verified
	// overwrite a manual entry.
	maDomainMethodAutoVerified = "auto_verified"
	maDomainMethodManual       = "manual"
	maDomainMethodNoWebsite    = "no_website"

	maWebValidationStateNoWebsite = "no_website_declared"
	maFinalActionNoWebsite        = "no_website_structured"

	// Domain-identity certainty of a web validation (mig 083). Recorded as a FACT
	// at validation time; the "asimmetria identitaria" policy (a reject may
	// suppress only from verified/vouched) is a read-time rule, gated on phase-(b)
	// measurement. Empty = legacy row or unresolved domain.
	maIdentityStateVerified = "verified" // target P.IVA/CF confirmed on-page
	maIdentityStateVouched  = "vouched"  // operator associated the domain
	maIdentityStateAssumed  = "assumed"  // name-match / brand-trust / score-only

	// Valuation defaults (Fase 4), overridable via ma_parameter (sme_haircut_pct,
	// ebitda_fallback_threshold). Haircut is a percent applied to sector multiples;
	// below the EBITDA-margin threshold (or EBITDA<=0) valuation falls back to EV/Sales.
	maSMEHaircutPctDefault     = 30.0
	maEBITDAFallbackPctDefault = 5.0

	// maVendorCEETolerancePctDefault è la soglia di allerta sugli scarti di
	// riconciliazione vendor-vs-CEE (override: ma_parameter vendor_cee_tolerance_pct).
	// Calibrata dall'inspect di Fase 0 su n=10: rumore massimo osservato 0,03%
	// (rounding dei ratio) — l'1% dà 30× di margine e cattura solo mismatch
	// definitori veri. Consumata dai quality flag di Fase 2.
	maVendorCEETolerancePctDefault = 1.0

	// Leve del bridge EV→equity e soglie dei flag di qualità (Fase 2, mig 092).
	// TFR al 100% = screening prudente (prassi deal 80-100). I flag A.5 si pesano
	// sull'EBITDA (2,5% dei ricavi può essere un terzo del margine), B.8 sui ricavi,
	// il perimetro standalone su attivo ed EBITDA.
	maTFRBridgePctDefault               = 100.0
	maA5EBITDAFlagPctDefault            = 20.0
	maB8RevenueFlagPctDefault           = 8.0
	maParticipationAssetsFlagPctDefault = 25.0
	maParticipationIncomeFlagPctDefault = 20.0

	// Haircut PMI graduato per taglia (Fase 3, mig 095): sostituisce il flat
	// sme_haircut_pct, che resta come fallback quando il fatturato non è noto.
	// Tier2 = 30 = status quo, così le mid-size non cambiano banda.
	maHaircutTier1PctDefault    = 35.0
	maHaircutTier2PctDefault    = 30.0
	maHaircutTier3PctDefault    = 20.0
	maHaircutTier1MaxEURDefault = 5_000_000.0
	maHaircutTier2MaxEURDefault = 20_000_000.0

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

	// Ancore delle rampe ASSOLUTE dei segnali economici (migrazione 081,
	// overridable via ma_parameter). I punteggi ai capi sono fissi in codice:
	// trend floor→0.1, crescita zero→0.5, top→1.0; produttività floor→0.1,
	// top→1.0, lineare in mezzo. Default provvisori (PMI ICT), da calibrare.
	maTrendCagrFloorDefault       = -0.10
	maTrendCagrTopDefault         = 0.15
	maProductivityFloorEURDefault = 50000.0
	maProductivityTopEURDefault   = 200000.0

	// maScoreVersion etichetta i punteggi prodotti da questa revisione dello
	// scoring (persistita in ma_target.score_version): 3 = catalogo v2 + bande
	// assolute + viability assente≠distressed. NULL in DB = versioni precedenti.
	maScoreVersion = 3

	// Eventi del log esiti (ma_target_outcome, migrazione 080).
	maOutcomeContattato = "contattato"
	maOutcomeBuonLead   = "buon_lead"
	maOutcomeNoGo       = "no_go"

	// Eventi di card aggiunti dall'evoluzione a log unico (migrazione 089,
	// INIZIATIVE-PRD.md §5): ma_target_outcome diventa il diario di
	// lavorazione, ancorato a initiative_id oltre che a session_id.
	maEventCardCreata   = "card_creata"
	maEventCardRimossa  = "card_rimossa"
	maEventCardRiaperta = "card_riaperta"
	maEventStato        = "stato"
	maEventNota         = "nota"
	maEventChiusura     = "chiusura"

	// Stati della card di lavorazione (ma_initiative_card, migrazione 088,
	// INIZIATIVE-PRD.md §4.4). "rimossa" non è mai una colonna del board.
	maCardStateDaContattare    = "da_contattare"
	maCardStateContattata      = "contattata"
	maCardStateInDialogo       = "in_dialogo"
	maCardStateApprofondimento = "approfondimento"
	maCardStateOfferta         = "offerta"
	maCardStateChiusa          = "chiusa"
	maCardStateRimossa         = "rimossa"

	// Esiti di chiusura della card (PRD §4.4): attributo della chiusura, non
	// colonne separate.
	maCardEsitoConclusa  = "conclusa"
	maCardEsitoNoGo      = "no_go"
	maCardEsitoNonIdonea = "non_idonea"
	maCardEsitoSfumata   = "sfumata"
	maCardEsitoRimandata = "rimandata"
)

// maCardActiveStates elenca gli stati non terminali di una card: usati per il
// filtro "attiva" delle collisioni cross-iniziativa (PRD §6.1) e dei badge
// nelle proiezioni (B5).
var maCardActiveStates = []string{
	maCardStateDaContattare,
	maCardStateContattata,
	maCardStateInDialogo,
	maCardStateApprofondimento,
	maCardStateOfferta,
}

func validMACardState(state string) bool {
	switch state {
	case maCardStateDaContattare, maCardStateContattata, maCardStateInDialogo, maCardStateApprofondimento, maCardStateOfferta, maCardStateChiusa, maCardStateRimossa:
		return true
	default:
		return false
	}
}

func validMACardEsito(esito string) bool {
	switch esito {
	case maCardEsitoConclusa, maCardEsitoNoGo, maCardEsitoNonIdonea, maCardEsitoSfumata, maCardEsitoRimandata:
		return true
	default:
		return false
	}
}

type MACreateSessionRequest struct {
	Prompt             string `json:"prompt"`
	ModelID            string `json:"modelId,omitempty"`
	PromptID           string `json:"promptId,omitempty"`
	GatedFlow          bool   `json:"gatedFlow,omitempty"`
	InitiativeID       string `json:"initiativeId,omitempty"`
	NewInitiativeTitle string `json:"newInitiativeTitle,omitempty"`
}

type MAEstimateSessionRequest struct {
	Strategy     *MAStrategySpec `json:"strategy,omitempty"`
	StrategyType string          `json:"strategyType,omitempty"`
}

type MAExecuteSessionRequest struct {
	Strategy     *MAStrategySpec `json:"strategy,omitempty"`
	StrategyType string          `json:"strategyType,omitempty"`
	Limit        int             `json:"limit,omitempty"`
	// Force re-runs the gated web-validation stage even when existing per-company
	// validations are still fresh. It does not force paid Advanced enrichment for
	// already-advanced survivors.
	Force bool `json:"force,omitempty"`
	// AcknowledgeCost lets the analyst proceed when the projected enrichment
	// spend exceeds the budget ceiling (the cost gate).
	AcknowledgeCost bool `json:"acknowledgeCost,omitempty"`
}

// MAGatedSearchTestRequest drives the developer test page: run the gated funnel inline
// on an existing session (which must already carry a fresh estimate). StrategyType/Limit
// override the persisted selection when set.
type MAGatedSearchTestRequest struct {
	SessionID    string `json:"sessionId"`
	StrategyType string `json:"strategyType,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

type MAExportRequest struct {
	Format string `json:"format,omitempty"`
}

// MATargetRatingRequest — oltre al voto, cattura la ground truth per la
// calibrazione futura (migrazione 080): il motivo dell'esclusione e lo snapshot
// di ciò che la UI mostrava al momento del giudizio (score/confidence arrivano
// dal client perché sono letteralmente ciò che l'analista stava guardando,
// anche se il backend ha ri-scorato nel frattempo).
type MATargetRatingRequest struct {
	CompanyKey         string `json:"companyKey"`
	Rating             int    `json:"rating"`
	Reason             string `json:"reason,omitempty"`
	ScoreAtRating      *int   `json:"scoreAtRating,omitempty"`
	ConfidenceAtRating string `json:"confidenceAtRating,omitempty"`
}

// MATargetOutcome è un evento del log append-only degli esiti reali a valle
// dello screening (migrazione 080): l'unica ground truth che permetterà di
// validare lo score. Agganciato a company_key come il rating.
type MATargetOutcome struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId,omitempty"`
	// InitiativeID ancora l'evento all'Iniziativa (migrazione 089): nuovo
	// ancoraggio primario per gli eventi di card; i tre eventi storici restano
	// ancorati alla sola sessione.
	InitiativeID string `json:"initiativeId,omitempty"`
	CompanyKey   string `json:"companyKey"`
	Event        string `json:"event"`
	Note         string `json:"note,omitempty"`
	// Payload porta i dettagli tipizzati dell'evento (from/to di stato, esito,
	// sessione/rating di provenienza): migrazione 089.
	Payload          json.RawMessage `json:"payload,omitempty"`
	CreatedBySubject string          `json:"-"`
	CreatedByEmail   string          `json:"createdByEmail,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
}

// MAInitiativeCard è la card di lavorazione (iniziativa, azienda) —
// migrazione 088, INIZIATIVE-PRD.md §4. Chiave primaria unica: la riapertura
// riusa la stessa card, mai una seconda (§4.3).
type MAInitiativeCard struct {
	InitiativeID       string     `json:"initiativeId"`
	CompanyKey         string     `json:"companyKey"`
	CompanyName        string     `json:"companyName"`
	VATCode            string     `json:"vatCode,omitempty"`
	TaxCode            string     `json:"taxCode,omitempty"`
	Province           string     `json:"province,omitempty"`
	State              string     `json:"state"`
	Esito              string     `json:"esito,omitempty"`
	CreatedFromSession string     `json:"createdFromSession,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	ClosedAt           *time.Time `json:"closedAt,omitempty"`
}

// MACompanyFact is one typed, revocable fact of the company registry (mig
// 090, INIZIATIVE-PRD.md §6): closed vocabulary, one active fact per
// (company_key, kind), history preserved via revocation instead of deletion.
type MACompanyFact struct {
	ID               string     `json:"id"`
	CompanyKey       string     `json:"companyKey"`
	VATCode          string     `json:"vatCode,omitempty"`
	TaxCode          string     `json:"taxCode,omitempty"`
	CompanyName      string     `json:"companyName,omitempty"`
	Kind             string     `json:"kind"`
	Note             string     `json:"note,omitempty"`
	CreatedBySubject string     `json:"-"`
	CreatedByEmail   string     `json:"createdByEmail,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	RevokedBySubject string     `json:"-"`
	RevokedByEmail   string     `json:"revokedByEmail,omitempty"`
	RevokeNote       string     `json:"revokeNote,omitempty"`
}

// MACompanyNote is an append-only free-text note of the company registry
// (mig 090, PRD §6): the expressiveness the closed fact vocabulary does not
// give. Registry content, not a log-of-events entry.
type MACompanyNote struct {
	ID               string    `json:"id"`
	CompanyKey       string    `json:"companyKey"`
	Body             string    `json:"body"`
	CreatedBySubject string    `json:"-"`
	CreatedByEmail   string    `json:"createdByEmail,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

// MACompanyRegistry is the aggregate returned by the registry read endpoint:
// facts include both active and revoked (the UI shows active, can reveal
// history), notes are the append-only timeline.
type MACompanyRegistry struct {
	Facts []MACompanyFact `json:"facts"`
	Notes []MACompanyNote `json:"notes"`
}

type MATargetOutcomeRequest struct {
	CompanyKey string `json:"companyKey"`
	Event      string `json:"event"`
	Note       string `json:"note,omitempty"`
}

// MACardMarker names one Iniziativa where the company has an ACTIVE card
// (PRD §6.1: collision marker, derived from the cards, no new data).
type MACardMarker struct {
	InitiativeID    string `json:"initiativeId"`
	InitiativeTitle string `json:"initiativeTitle"`
}

// MACardProvenance is one ≥1★ origin of a card: the sessione, the stella and
// the score fotografato al momento del giudizio (PRD §4.1/§4.2). A card can
// carry multiple provenances (ripescaggi da sessioni diverse della stessa
// iniziativa); nessuna aggregazione delle stelle.
type MACardProvenance struct {
	SessionID     string    `json:"sessionId"`
	SessionTitle  string    `json:"sessionTitle"`
	Rating        int       `json:"rating"`
	ScoreAtRating *int      `json:"scoreAtRating,omitempty"`
	RatedAt       time.Time `json:"ratedAt"`
}

// MAInitiativeCardView is one row of the board (B4): the card plus everything
// presentational the drawer/kanban need, computed in a handful of batch
// queries (never N+1).
type MAInitiativeCardView struct {
	MAInitiativeCard
	DossierStatus string             `json:"dossierStatus"`
	Collisions    []MACardMarker     `json:"collisions,omitempty"`
	RegistryFacts []string           `json:"registryFacts,omitempty"`
	Provenances   []MACardProvenance `json:"provenances,omitempty"`
	LastEvent     string             `json:"lastEvent,omitempty"`
}

// MAInitiativeBoard is the response of GET .../initiatives/{id}: the
// Iniziativa, its anchored sessions (chips) and its cards (board rows).
type MAInitiativeBoard struct {
	Initiative MAInitiative           `json:"initiative"`
	Sessions   []MASessionSummary     `json:"sessions"`
	Cards      []MAInitiativeCardView `json:"cards"`
}

// MACardStateRequest drives POST .../cards/{companyKey}/state (B4 passo 3):
// free transitions among the 5 active states (chiusa/rimossa go through
// their dedicated endpoints).
type MACardStateRequest struct {
	State string `json:"state"`
}

// MACardCloseRequest drives POST .../cards/{companyKey}/close (B4 passo 4):
// the esito is the attribute of the closure (PRD §4.4); RegisterFacts is the
// typed bridge to the company registry (§6), allowed only for esito
// no_go/rimandata and only for non_vende/in_trattativa_altrui.
type MACardCloseRequest struct {
	Esito         string   `json:"esito"`
	Note          string   `json:"note,omitempty"`
	RegisterFacts []string `json:"registerFacts,omitempty"`
}

// MACardCloseResponse reports the card plus which registerFacts actually got
// written (idempotent skip on already-active fact does not fail the close).
type MACardCloseResponse struct {
	Card            MAInitiativeCard `json:"card"`
	RegisteredFacts []string         `json:"registeredFacts,omitempty"`
	SkippedFacts    []string         `json:"skippedFacts,omitempty"`
}

// MACardRemoveRequest drives POST .../cards/{companyKey}/remove (B4 passo 5):
// removal is for triage error, never a verdict (PRD §4.3). CorrectRating
// reuses setTargetRating on the most recent ≥1★ provenance.
type MACardRemoveRequest struct {
	CorrectRating bool   `json:"correctRating,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// MACardRemoveResponse signals when the star correction was skipped (the
// provenance session is no longer operational) so the caller can toast it
// without failing the removal itself.
type MACardRemoveResponse struct {
	Card                    MAInitiativeCard `json:"card"`
	RatingCorrected         bool             `json:"ratingCorrected"`
	RatingCorrectionSkipped bool             `json:"ratingCorrectionSkipped,omitempty"`
}

// MACardNoteRequest drives POST .../cards/{companyKey}/note (B4 passo 7): the
// diario composer note (log event only, never the company registry — PRD
// §2).
type MACardNoteRequest struct {
	Body string `json:"body"`
}

// MACardDeepDiveResponse drives POST .../cards/{companyKey}/deep-dive (B6,
// PRD §7): il bottone a 3 stati sulla card legge solo il DossierStatus
// risultante, mai cifre di costo.
type MACardDeepDiveResponse struct {
	DossierStatus string `json:"dossierStatus"`
}

// MARescoreRequest — override della tesi da parte dell'analista: ri-scora i
// target advanced della sessione dai payload già persistiti (gratis, nessuna
// chiamata vendor) sotto la tesi indicata.
type MARescoreRequest struct {
	Thesis string `json:"thesis"`
}

// MAAssociateDomainRequest is the manual-review remedy input: the operator supplies an
// official domain for a company the gate held as domain-unresolved (manual_review), so it
// can be re-gated with that domain and, if it now survives, auto-enriched.
type MAAssociateDomainRequest struct {
	CompanyKey string `json:"companyKey"`
	Domain     string `json:"domain"`
}

type MACompanyKeyRequest struct {
	CompanyKey string `json:"companyKey"`
}

type MAWebValidationUpsertRequest struct {
	PipelineVersion        string                          `json:"pipelineVersion,omitempty"`
	InputHash              string                          `json:"inputHash,omitempty"`
	KeywordSetHash         string                          `json:"keywordSetHash,omitempty"`
	Target                 MATarget                        `json:"target"`
	KeywordSet             CandidateMatchKeywordSet        `json:"keywordSet"`
	DomainResponse         DomainResolutionResponse        `json:"domainResponse"`
	SelectedDomain         *DomainResolutionCandidate      `json:"selectedDomain,omitempty"`
	IdentityState          string                          `json:"identityState,omitempty"`
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
	// InitiativeID/InitiativeTitle are populated by a LEFT JOIN on
	// ma_initiative (mig 087) when the session is anchored to one. The chip
	// in the ricerche index (F5) reads InitiativeTitle directly.
	InitiativeID    string `json:"initiativeId,omitempty"`
	InitiativeTitle string `json:"initiativeTitle,omitempty"`
}

// MAInitiative is a light, non-CRM container for M&A workstream sessions
// (INIZIATIVE-PRD.md §3): title, optional description, author, archivable.
// No budget/KPI/deadline fields by design.
type MAInitiative struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	CreatedBySubject  string     `json:"createdBySubject,omitempty"`
	CreatedByEmail    string     `json:"createdByEmail,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	ArchivedAt        *time.Time `json:"archivedAt,omitempty"`
	ArchivedBySubject string     `json:"archivedBySubject,omitempty"`
	ArchivedByEmail   string     `json:"archivedByEmail,omitempty"`
	DeletedAt         *time.Time `json:"deletedAt,omitempty"`
	DeletedBySubject  string     `json:"deletedBySubject,omitempty"`
	DeletedByEmail    string     `json:"deletedByEmail,omitempty"`
	PurgedAt          *time.Time `json:"purgedAt,omitempty"`
	PurgedBySubject   string     `json:"purgedBySubject,omitempty"`
	PurgedByEmail     string     `json:"purgedByEmail,omitempty"`
}

// MAInitiativeSummary is the row shown in the /iniziative index (wireframe
// S1): the initiative plus counts/activity computed from anchored sessions
// and (from B2 onward) cards. Counts stays empty until B2 wires the
// per-state join; SessionCount/LastActivityAt are populated here in B1.
type MAInitiativeSummary struct {
	MAInitiative
	Counts            map[string]int `json:"counts"`
	LastActivityAt    *time.Time     `json:"lastActivityAt,omitempty"`
	LastActivityEvent string         `json:"lastActivityEvent,omitempty"`
	SessionCount      int            `json:"sessionCount"`
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
	// ScoringPlan è il registro "valutato / filtrato / ignorato" (computed, not
	// persisted): il confine esplicito di cosa il numero significa. Vedi
	// buildMAScoringPlan.
	ScoringPlan *MAScoringPlan `json:"scoringPlan,omitempty"`
}

// MAScoringPlan dichiara all'analista cosa lo score ha graduato, cosa è stato
// applicato come filtro duro a monte (quindi non ri-valutato), e cosa è stato
// lasciato cadere (vincoli free-form non supportati). La fiducia viene dal
// vedere il confine, non dal numero.
type MAScoringPlan struct {
	Thesis       string                `json:"thesis"`
	ScoreVersion int                   `json:"scoreVersion"`
	Evaluated    []MAScoringPlanSignal `json:"evaluated"`
	Filtered     []string              `json:"filtered"`
	Ignored      []string              `json:"ignored"`
}

// MAScoringPlanSignal è un segnale graduato dallo score con il suo peso
// nominale (su 100) sotto la tesi corrente.
type MAScoringPlanSignal struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Family string  `json:"family"`
	Weight float64 `json:"weight"`
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
	// InitiativeID anchors the session to an Iniziativa (mig 087, nullable).
	// InitiativeTitle is the joined display title when available, so create/detail
	// responses can render the anchor without a second frontend fetch.
	InitiativeID    string `json:"initiativeId,omitempty"`
	InitiativeTitle string `json:"initiativeTitle,omitempty"`
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
	SectorConcepts         []MAStrategyConcept  `json:"sectorConcepts,omitempty"`
	SectorRetrievalMode    string               `json:"sectorRetrievalMode,omitempty"`
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

type MAStrategyConcept struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Fit        string   `json:"fit"`
	Divisions  []string `json:"divisions,omitempty"`
	AtecoCodes []string `json:"atecoCodes,omitempty"`
}

type MAGatedProgressResponse struct {
	Stage   string                    `json:"stage"`
	Surface MAGatedProgressSurface    `json:"surface"`
	Gate    MAGatedProgressGate       `json:"gate"`
	Enrich  MAGatedProgressEnrich     `json:"enrich"`
	Run     MAGatedProgressRunSummary `json:"run"`
}

type MAGatedProgressSurface struct {
	Expected int `json:"expected"`
	Fetched  int `json:"fetched"`
}

type MAGatedProgressGate struct {
	Processed int                         `json:"processed"`
	Total     int                         `json:"total"`
	Buckets   MAGatedProgressBucketCounts `json:"buckets"`
}

type MAGatedProgressBucketCounts struct {
	Keep         int `json:"keep"`
	Forse        int `json:"forse"`
	Scarta       int `json:"scarta"`
	ManualReview int `json:"manualReview"`
}

type MAGatedProgressEnrich struct {
	Enriched  int `json:"enriched"`
	Survivors int `json:"survivors"`
}

type MAGatedProgressRunSummary struct {
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt"`
	ErrorCode   string     `json:"errorCode"`
}

type MAVerificationQueueResponse struct {
	Items []MAVerificationQueueItem `json:"items"`
}

type MAVerificationQueueItem struct {
	TargetID    string                    `json:"targetId"`
	CompanyKey  string                    `json:"companyKey"`
	CompanyName string                    `json:"companyName"`
	Province    string                    `json:"province"`
	Reason      MAVerificationQueueReason `json:"reason"`
	Remedies    []string                  `json:"remedies"`
}

type MAVerificationQueueReason struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type MAProvinceCatalogResponse struct {
	Items []MAProvinceCatalogItem `json:"items"`
}

type MAProvinceCatalogItem struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Region string `json:"region"`
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
	// Summary is a synthesis (not a citation, so no sourceText): the positive
	// business perimeter distilled by the intent prompt. Preferred over the
	// include fragments for the embedding query and the sector description;
	// empty with prompts that predate it.
	Summary string                   `json:"summary,omitempty"`
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
	ID               string `json:"id"`
	SessionID        string `json:"sessionId"`
	RunID            string `json:"runId"`
	VendorID         string `json:"vendorId,omitempty"`
	CompanyKey       string `json:"companyKey,omitempty"`
	CompanyName      string `json:"companyName"`
	VATCode          string `json:"vatCode,omitempty"`
	TaxCode          string `json:"taxCode,omitempty"`
	Province         string `json:"province,omitempty"`
	Town             string `json:"town,omitempty"`
	ActivityStatus   string `json:"activityStatus,omitempty"`
	Turnover         *int   `json:"turnover,omitempty"`
	TurnoverYear     *int   `json:"turnoverYear,omitempty"`
	Employees        *int   `json:"employees,omitempty"`
	AtecoCode        string `json:"atecoCode,omitempty"`
	AtecoDescription string `json:"atecoDescription,omitempty"`
	Score            int    `json:"score"`
	// ScoreVersion etichetta la revisione dello scoring che ha prodotto Score
	// (migrazione 081): nil = riga address (mai scorata) o punteggio legacy.
	// Punteggi con versione diversa non sono confrontabili.
	ScoreVersion *int   `json:"scoreVersion,omitempty"`
	MatchState   string `json:"matchState"`
	Confidence   string `json:"confidence,omitempty"`
	// Bucket è la destinazione di presentazione derivata A LETTURA dai campi
	// persistiti (mai salvata): principale / da_verificare / azionabile /
	// soppresso. Vedi maRouteTarget.
	Bucket          string               `json:"bucket,omitempty"`
	Rating          *int                 `json:"rating,omitempty"`
	Outcomes        []MATargetOutcome    `json:"outcomes,omitempty"`
	Flags           []MATargetFlag       `json:"flags,omitempty"`
	Rationale       string               `json:"rationale"`
	MissingCriteria []string             `json:"missingCriteria"`
	Evidence        []MATargetEvidence   `json:"evidence"`
	Adjustments     []MATargetAdjustment `json:"adjustments,omitempty"`
	Deep            *MADeepAnalysis      `json:"deep,omitempty"`
	WebValidation   *MAWebValidation     `json:"webValidation,omitempty"`
	VendorPayload   json.RawMessage      `json:"vendorPayload,omitempty"`
	EnrichmentLevel string               `json:"enrichmentLevel,omitempty"`
	CreatedAt       time.Time            `json:"createdAt"`
}

// MATargetRow è la proiezione leggera di un target per liste e derivazioni:
// niente vendor_payload, niente blob di validation, niente evidence.
type MATargetRow struct {
	ID              string          `json:"id"`
	RunID           string          `json:"runId"`
	CompanyKey      string          `json:"companyKey,omitempty"`
	CompanyName     string          `json:"companyName"`
	VATCode         string          `json:"vatCode,omitempty"`
	Province        string          `json:"province,omitempty"`
	Town            string          `json:"town,omitempty"`
	AtecoCode       string          `json:"atecoCode,omitempty"`
	Score           int             `json:"score"`
	ScoreVersion    *int            `json:"scoreVersion,omitempty"`
	MatchState      string          `json:"matchState"`
	Confidence      string          `json:"confidence,omitempty"`
	Bucket          string          `json:"bucket,omitempty"`
	Rating          *int            `json:"rating,omitempty"`
	Flags           []MATargetFlag  `json:"flags,omitempty"`
	EnrichmentLevel string          `json:"enrichmentLevel,omitempty"`
	WebValidation   *MATargetRowWeb `json:"webValidation,omitempty"`

	// RegistryFacts/InLavorazione sono decorazioni di sola presentazione
	// (PRD §6.1, R-D3-7): badge registro azienda + marker di card attive in
	// altre iniziative. Zero effetti su bucket/score/routing.
	RegistryFacts []string       `json:"registryFacts,omitempty"`
	InLavorazione []MACardMarker `json:"inLavorazione,omitempty"`

	SortTurnover         *int `json:"-"`
	HasOutsidePostFilter bool `json:"-"`
}

// MATargetRowWeb replica i soli percorsi JSON consumati dalle liste.
type MATargetRowWeb struct {
	WebValidationState string                   `json:"webValidationState"`
	FinalAction        string                   `json:"finalAction"`
	SelectedDomain     string                   `json:"selectedDomain,omitempty"`
	FinalDecision      MATargetRowFinalDecision `json:"finalDecision"`

	GroupSiteDomain     string `json:"-"`
	GroupSiteIdentifier string `json:"-"`
	CandidateCount      int    `json:"-"`
}

type MATargetRowFinalDecision struct {
	Reason string `json:"reason,omitempty"`
}

type MATargetListResponse struct {
	Items []MATargetRow `json:"items"`
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
	IdentityState          string                          `json:"identityState,omitempty"`
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
	// Reconciliation è il sanity check vendor-vs-CEE (Fase 1): assente quando il
	// payload non porta voci CEE.
	Reconciliation *MADeepReconciliation `json:"reconciliation,omitempty"`
	// QualityFlags (Fase 2): annotazioni deterministiche di confidenza sulla banda,
	// calcolate da buildMADeepQualityFlags con le soglie di ma_parameter. Mai nella
	// matematica della banda, mai nel RAG.
	QualityFlags []MADeepQualityFlag `json:"qualityFlags,omitempty"`
}

// MADeepReconciliation — scarti tra i numeri pre-calcolati dal vendor e le stesse
// grandezze rilette dalle voci CEE depositate. EBITDA e PFN sono scarti relativi in
// % (PFN con pavimento 1000€ al denominatore); ROE è in PUNTI percentuali. La PFN
// riporta anche la provenienza della rilettura (cee_detail | cee_total). Consumata
// dai quality flag di Fase 2 contro vendor_cee_tolerance_pct (default 1%).
type MADeepReconciliation struct {
	EBITDAPct     *float64 `json:"ebitdaPct,omitempty"`
	PFNPct        *float64 `json:"pfnPct,omitempty"`
	PFNProvenance string   `json:"pfnProvenance,omitempty"`
	ROEPointsDiff *float64 `json:"roePointsDiff,omitempty"`
}

type MADeepMetric struct {
	Group string   `json:"group"`
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Value *float64 `json:"value,omitempty"`
	Unit  string   `json:"unit"`
	RAG   string   `json:"rag"`
	// Tier (Fase 3, lente compratore): "contorno" = metrica sulla struttura del
	// capitale del VENDITORE (che il compratore sostituisce al closing) — resta
	// visibile ma non guida l'overall RAG. Vuoto = core.
	Tier string `json:"tier,omitempty"`
}

// MABMFamily — famiglia di business model di un'azienda (Fase 3, mig 093):
// suggerimento deterministico del motore + ratifica dell'analista. Alimenta le
// soglie RAG e la selezione della riga Damodaran (mai i fact presentation-only
// di ma_company_fact, che restano fuori da gate/scoring per PRD §6).
type MABMFamily struct {
	CompanyKey        string     `json:"companyKey"`
	SuggestedFamily   string     `json:"suggestedFamily,omitempty"`
	SuggestedSource   string     `json:"suggestedSource,omitempty"`
	SuggestedEvidence string     `json:"suggestedEvidence,omitempty"`
	Family            string     `json:"family,omitempty"`
	RatifiedByEmail   string     `json:"ratifiedByEmail,omitempty"`
	RatifiedAt        *time.Time `json:"ratifiedAt,omitempty"`
}

// Effective è la famiglia usata da soglie e multiplo: la ratifica vince; il solo
// suggerimento vale ma la valuation porta il caveat "non ratificata".
func (f *MABMFamily) Effective() (string, bool) {
	if f == nil {
		return "", false
	}
	if f.Family != "" {
		return f.Family, true
	}
	return f.SuggestedFamily, false
}

type MABMFamilyRequest struct {
	Family string `json:"family"` // vuoto = revoca della ratifica
}

// MADeepValuation is populated in Fase 4 (Damodaran sector multiples). Dal redesign
// (Fase 2) la banda è ASIMMETRICA: estremo alto su EBITDA reported, estremo basso su
// EBITDA prudenziale (al netto di A.4 capitalizzazioni e contributi) — si allarga in
// proporzione alla distorsione misurata, senza costanti arbitrarie. L'equity esce dal
// bridge esplicito (EV − PFN − TFR − fondo imposte), non più da EV − PFN secca.
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
	// Fase 2: trasparenza della banda asimmetrica e ponte EV→equity.
	PrudentialEbitda *float64      `json:"prudentialEbitda,omitempty"`
	LowMethod        string        `json:"lowMethod,omitempty"` // metodo dell'estremo basso, se diverso da Method
	Bridge           *MADeepBridge `json:"bridge,omitempty"`
}

// MADeepBridge è il ponte esplicito EV→equity (Fase 2): ogni riga è autoportante
// (le caveat viaggiano dentro il numero, anche nell'export). I finanziamenti soci
// sono GIÀ dentro la PFN: la loro riga è informativa/negoziale (al closing vengono
// spesso rinunciati o convertiti), non un'ulteriore deduzione.
type MADeepBridge struct {
	PFN              *MADeepBridgeRow `json:"pfn,omitempty"`
	TFR              *MADeepBridgeRow `json:"tfr,omitempty"`
	TaxFund          *MADeepBridgeRow `json:"taxFund,omitempty"`
	ShareholderLoans *MADeepBridgeRow `json:"shareholderLoans,omitempty"`
	EquityLow        *float64         `json:"equityLow,omitempty"`
	EquityHigh       *float64         `json:"equityHigh,omitempty"`
}

type MADeepBridgeRow struct {
	Value      float64 `json:"value"`
	Provenance string  `json:"provenance,omitempty"` // cee_detail | cee_total | vendor_ratio
	Note       string  `json:"note,omitempty"`
}

// MADeepQualityFlag è un segnale deterministico di qualità del dato/margine (Fase 2):
// annota la confidenza della banda SENZA entrare nella sua matematica né nel RAG.
// Ogni flag porta l'evidenza numerica precomposta e la domanda DD standard (il seme
// dell'information request list, Fase 6).
type MADeepQualityFlag struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"` // warning | info
	Label      string `json:"label"`
	Evidence   string `json:"evidence"`
	DDQuestion string `json:"ddQuestion,omitempty"`
}

// MADeepBrief is the LLM narrative of the NEUTRAL dossier (numbers stay in
// scorecard/valuation). FinancialReading è il nome corretto della lettura
// finanziaria (prompt v3); ThesisReading è la chiave legacy delle righe cached
// pre-v3 (parseMADeepBrief la riversa in FinancialReading) — la lettura di tesi
// VERA è context-scoped sulla card (MACardThesisReading).
type MADeepBrief struct {
	Verdict            string            `json:"verdict,omitempty"`
	RAG                string            `json:"rag,omitempty"`
	BusinessProfile    string            `json:"businessProfile,omitempty"`
	FinancialReading   string            `json:"financialReading,omitempty"`
	ThesisReading      string            `json:"thesisReading,omitempty"` // legacy pre-v3
	Strengths          []string          `json:"strengths,omitempty"`
	RedFlags           []MADeepBriefFlag `json:"redFlags,omitempty"`
	ValuationRationale string            `json:"valuationRationale,omitempty"`
	DDQuestions        []string          `json:"ddQuestions,omitempty"`
}

// MAThesisReading è la lettura di tesi context-scoped (Fase 5): il "memo" che
// applica la tesi della sessione di provenienza al dossier neutro. Fit che cita
// i fatti, flag ri-pesate (mai nuove), DD di tesi additive, sinergie come
// ipotesi, postura valutativa senza numeri nuovi.
type MAThesisReading struct {
	FitLevel          string   `json:"fitLevel,omitempty"` // alto | medio | basso | non_valutabile
	Fit               string   `json:"fit,omitempty"`
	BlockingFlags     []string `json:"blockingFlags,omitempty"`
	TolerableFlags    []string `json:"tolerableFlags,omitempty"`
	ThesisDDQuestions []string `json:"thesisDdQuestions,omitempty"`
	SynergyHypotheses []string `json:"synergyHypotheses,omitempty"`
	ValuationStance   string   `json:"valuationStance,omitempty"`
	NotAddressed      []string `json:"notAddressed,omitempty"`
}

// MACardThesisReading è il record persistito per (iniziativa, azienda) con lo
// snapshot della tesi usata (per la staleness) e la data dell'evidenza web.
type MACardThesisReading struct {
	InitiativeID     string           `json:"initiativeId"`
	CompanyKey       string           `json:"companyKey"`
	SessionID        string           `json:"sessionId,omitempty"`
	ThesisSnapshot   string           `json:"thesisSnapshot,omitempty"`
	Reading          *MAThesisReading `json:"reading,omitempty"`
	WebEvidenceDate  *time.Time       `json:"webEvidenceDate,omitempty"`
	GeneratedByEmail string           `json:"generatedByEmail,omitempty"`
	UpdatedAt        *time.Time       `json:"updatedAt,omitempty"`
	// StaleThesis è calcolato in lettura: lo snapshot non coincide più con la
	// tesi corrente della sessione di provenienza (rigenerazione esplicita).
	StaleThesis bool `json:"staleThesis,omitempty"`
}

// MACardIRLItem è una voce della Information Request List della card (Fase 6):
// domanda DD con provenienza (source/sourceRef, chiave del re-seed additivo) e
// tracking leggero dello stato. Sopravvive all'archiviazione della card.
type MACardIRLItem struct {
	ID             string    `json:"id"`
	InitiativeID   string    `json:"initiativeId"`
	CompanyKey     string    `json:"companyKey"`
	Category       string    `json:"category"`
	Question       string    `json:"question"`
	Source         string    `json:"source"` // flag | brief | thesis | template | analyst
	SourceRef      string    `json:"sourceRef,omitempty"`
	Status         string    `json:"status"` // aperta | chiesta | risposta | na
	Position       int       `json:"position"`
	CreatedByEmail string    `json:"createdByEmail,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// MAIRLTemplate è una voce standard per famiglia di business model: la
// componente di completezza del seed (le fonti generate sono le anomalie).
type MAIRLTemplate struct {
	ID       string
	Family   string
	Category string
	Question string
	Position int
}

// MAIRLSeedReport riassume un seed: proposte assemblate dalle fonti vs voci
// davvero inserite (il re-seed additivo salta i source_ref già presenti).
type MAIRLSeedReport struct {
	Proposed int            `json:"proposed"`
	Inserted int            `json:"inserted"`
	BySource map[string]int `json:"bySource,omitempty"`
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

// MADeepInspectReport — read-only aggregates over the cached deep payloads (Fase 0,
// DEEP-DIVE-IMPLEMENTATION-PLAN.md): answers the empirical unknowns of the redesign
// (code-family mix, debt granularity, delta/L2Y coverage, provisions treatment,
// vendor-vs-CEE reconciliation) without any vendor call and without persisting.
type MADeepInspectReport struct {
	StatusCounts map[string]int `json:"statusCounts"`
	// BriefFormats/BriefStaleKeys classify the cached briefs by shape
	// (financialReading | legacy | none): rollout di prompt verificabile
	// dall'inspect senza rigenerare nulla.
	BriefFormats     map[string]int              `json:"briefFormats"`
	BriefStaleKeys   []string                    `json:"briefStaleKeys"`
	PayloadsAnalyzed int                         `json:"payloadsAnalyzed"`
	CodeFamilies     MADeepInspectFamilies       `json:"codeFamilies"`
	Granularity      MADeepInspectGranularity    `json:"granularity"`
	Coverage         map[string]int              `json:"coverage"`
	ProvisionsB12B13 int                         `json:"provisionsB12B13NonZero"`
	Reconciliation   MADeepInspectReconciliation `json:"reconciliation"`
	Vintage          MADeepInspectVintage        `json:"vintage"`
}

type MADeepInspectFamilies struct {
	IICOnly int `json:"iicOnly"`
	// WithIPL counts payloads carrying IPL codes OTHER than the two legend typos
	// (IPL231/IPL232 are canonical: the vendor's legend misprints them in the IIC
	// column and the API emits them verbatim in IIC-division payloads).
	WithIPL          int `json:"withIpl"`
	KnownLegendTypos int `json:"knownLegendTypos"`
}

type MADeepInspectGranularity struct {
	DebtDetail int `json:"debtDetail"` // per-voce split entro/oltre presente
	TotalsOnly int `json:"totalsOnly"` // solo totali per voce (IIC329-343)
	NoDebts    int `json:"noDebts"`
}

type MADeepInspectReconciliation struct {
	EBITDA MADeepInspectDeviation `json:"ebitda"`
	PFN    MADeepInspectDeviation `json:"pfn"`
}

type MADeepInspectDeviation struct {
	Computable int                     `json:"computable"`
	MaxAbsPct  float64                 `json:"maxAbsPct"`
	P50AbsPct  float64                 `json:"p50AbsPct"`
	Over1Pct   int                     `json:"over1pct"`
	Worst      []MADeepInspectOffender `json:"worst,omitempty"`
}

type MADeepInspectOffender struct {
	CompanyKey   string  `json:"companyKey"`
	VendorValue  float64 `json:"vendorValue"`
	CEEValue     float64 `json:"ceeValue"`
	DeviationPct float64 `json:"deviationPct"`
}

type MADeepInspectVintage struct {
	Rows      int    `json:"rows"`
	Companies int    `json:"companies"`
	Error     string `json:"error,omitempty"` // es. migrazione 091 non ancora applicata
}

// MACompanyDossier is the standalone P.IVA lookup response: our elaborations
// (scorecard/valuation/brief) plus the raw IT-full payload for the facts layer.
// Status drives the frontend: absent | cost_required | queued | running | ready | failed.
type MACompanyDossier struct {
	VATCode string `json:"vatCode"`
	// CompanyKey è la chiave del record deep (il funnel chiave per vendor id, il
	// lookup standalone per VAT): è la chiave giusta per famiglia BM e ratifica.
	CompanyKey string           `json:"companyKey,omitempty"`
	Status     string           `json:"status"`
	Scorecard  *MADeepScorecard `json:"scorecard,omitempty"`
	Valuation  *MADeepValuation `json:"valuation,omitempty"`
	Brief      *MADeepBrief     `json:"brief,omitempty"`
	BMFamily   *MABMFamily      `json:"bmFamily,omitempty"`
	Raw        json.RawMessage  `json:"raw,omitempty"`
	CostEUR    float64          `json:"costEur,omitempty"`
	ErrorCode  string           `json:"errorCode,omitempty"`
	UpdatedAt  *time.Time       `json:"updatedAt,omitempty"`
}

type maStrategyDraftEnvelope struct {
	Strategy MAStrategySpec `json:"strategy"`
}

type maSessionCreate struct {
	Session      MASession
	Strategy     MAStrategySpec
	InitiativeID string
}

type maExecutionRunCreate struct {
	ID                string
	SessionID         string
	StrategyVersionID string
	StrategyType      string
	EstimatedCount    int
}
