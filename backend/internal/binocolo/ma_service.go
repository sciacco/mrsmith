package binocolo

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/brave"
	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openapiit"
	"github.com/sciacco/mrsmith/internal/platform/scrape"
	"github.com/xuri/excelize/v2"
)

var (
	errMAStoreUnavailable      = errors.New("ma store unavailable")
	errMAOpenAPIITUnavailable  = errors.New("openapiit unavailable")
	errMAOpenRouterUnavailable = errors.New("openrouter unavailable")
	errMABraveUnavailable      = errors.New("brave unavailable")
	errMALLMConfigUnavailable  = errors.New("ma llm config unavailable")
	errMAEstimateTooLarge      = errors.New("estimate too large")
	errMAEstimateOverBudget    = errors.New("estimate over budget")
	errMAVisibilityInvalid     = errors.New("ma session visibility invalid")
	errMASessionArchived       = errors.New("ma session archived")
	errMASessionDeleted        = errors.New("ma session deleted")
	// errMAEstimateSuperseded is returned by ReplaceMAEstimates when the active
	// strategy version changed mid-estimate (the user re-submitted). The estimate
	// worker loops on it to re-run against the now-active version, so the latest
	// strategy is always the one that ends up estimated.
	errMAEstimateSuperseded = errors.New("ma estimate superseded by newer strategy version")
)

const (
	maAtecoToolName          = "search_ateco_2025"
	maAtecoChildrenToolName  = "list_ateco_children"
	maProvinceRegionToolName = "list_italian_provinces_regions"
	maCompanySurfaceToolName = "probe_company_search_surface"
	maMaxToolRounds          = 4
)

type maSurfaceProbeResult struct {
	Status         string
	EstimatedCount int
	EstimatedCost  float64
	ProbeCount     int
	Params         json.RawMessage
	VendorResponse json.RawMessage
}

type maSurfaceProbeAudit struct {
	Skip     int             `json:"skip"`
	Count    int             `json:"count"`
	Cost     float64         `json:"cost"`
	Params   json.RawMessage `json:"params"`
	Response json.RawMessage `json:"response"`
}

// maAIClient is the chat seam — the per-call OpenAI-compatible client. Kept as
// an interface so the agentic loop stays testable.
type maAIClient interface {
	Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error)
}

// maApp is this module's app namespace in the centralized llm registry.
const maApp = "binocolo"

// maLLMProvider is binocolo's view of the shared llm.Service: model/prompt
// resolution (scoped to app="binocolo"), per-call client construction, audit,
// and option lists. *llm.Service is adapted to it by maLLMAdapter.
type maLLMProvider interface {
	ResolveModel(ctx context.Context, scope, modelID string) (llm.Model, error)
	ResolvePrompt(ctx context.Context, scope, promptID string) (llm.Prompt, error)
	ClientForModel(ctx context.Context, m llm.Model) (maAIClient, error)
	RecordAudit(ctx context.Context, audit llm.CallAudit) error
	ListModels(ctx context.Context) ([]llm.Model, error)
	ListPrompts(ctx context.Context) ([]llm.Prompt, error)
	// Embedding capability (use case 1 ATECO retrieval). ResolveEmbeddingModel is
	// row-driven (by id carried on the stored KB vectors) so query and documents
	// share one model; Embed returns one vector per input plus token usage.
	ResolveEmbeddingModel(ctx context.Context, id string) (llm.EmbeddingModel, error)
	Embed(ctx context.Context, m llm.EmbeddingModel, inputs []string) ([][]float32, llm.Usage, error)
	// Reranking capability (use case 2 sector classification). ResolveRerankModel
	// resolves by (app, scope); Rerank returns one yes-probability per document.
	ResolveRerankModel(ctx context.Context, scope, modelID string) (llm.RerankModel, error)
	Rerank(ctx context.Context, m llm.RerankModel, instruction, query string, documents []string) ([]float64, llm.Usage, error)
}

type maLLMAdapter struct{ svc *llm.Service }

func newMALLMProvider(svc *llm.Service) maLLMProvider {
	if svc == nil {
		return nil
	}
	return maLLMAdapter{svc: svc}
}

func (a maLLMAdapter) ResolveModel(ctx context.Context, scope, modelID string) (llm.Model, error) {
	return a.svc.ResolveModel(ctx, maApp, scope, modelID)
}

func (a maLLMAdapter) ResolvePrompt(ctx context.Context, scope, promptID string) (llm.Prompt, error) {
	return a.svc.ResolvePrompt(ctx, maApp, scope, promptID)
}

func (a maLLMAdapter) ClientForModel(ctx context.Context, m llm.Model) (maAIClient, error) {
	client, _, err := a.svc.ClientForModel(ctx, m)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (a maLLMAdapter) RecordAudit(ctx context.Context, audit llm.CallAudit) error {
	return a.svc.RecordAudit(ctx, audit)
}

func (a maLLMAdapter) ListModels(ctx context.Context) ([]llm.Model, error) {
	return a.svc.ListModels(ctx, maApp)
}

func (a maLLMAdapter) ListPrompts(ctx context.Context) ([]llm.Prompt, error) {
	return a.svc.ListPrompts(ctx, maApp)
}

func (a maLLMAdapter) ResolveEmbeddingModel(ctx context.Context, id string) (llm.EmbeddingModel, error) {
	return a.svc.ResolveEmbeddingModel(ctx, id)
}

func (a maLLMAdapter) Embed(ctx context.Context, m llm.EmbeddingModel, inputs []string) ([][]float32, llm.Usage, error) {
	return a.svc.Embed(ctx, m, inputs)
}

func (a maLLMAdapter) ResolveRerankModel(ctx context.Context, scope, modelID string) (llm.RerankModel, error) {
	return a.svc.ResolveRerankModel(ctx, maApp, scope, modelID)
}

func (a maLLMAdapter) Rerank(ctx context.Context, m llm.RerankModel, instruction, query string, documents []string) ([]float64, llm.Usage, error) {
	return a.svc.Rerank(ctx, m, instruction, query, documents)
}

type maService struct {
	store         maWorkspaceStore
	searchCache   companySearchCacheStore
	provinceCache provinceCacheStore
	ateco         atecoStore
	// kb backs use-case-1 ATECO retrieval (curated concept vectors). Soft
	// dependency, set post-construction like brave; nil falls back to the LLM
	// hierarchy resolver.
	kb        kbStore
	openapiit *openapiit.Client
	brave     interface {
		LLMContext(context.Context, brave.LLMContextParams) (brave.LLMContextResult, error)
	}
	// scrape backs UC2 domain entity-verification + page-content evidence. Soft
	// dependency, set post-construction like brave; nil falls back to the
	// score-only domain pick and Brave-snippet evidence (today's behavior).
	scrape interface {
		Scrape(context.Context, string) (scrape.Result, error)
		ScrapeFull(context.Context, string) (scrape.Result, error)
		Search(context.Context, string, int) ([]scrape.SearchResult, error)
		Crawl(context.Context, string, int, int) ([]scrape.CrawlPage, error)
		Map(context.Context, string, int) ([]string, error)
	}
	llmp maLLMProvider
	now  func() time.Time
	// owner is this instance's stable identity (config InstanceOwner), stamped on
	// every enqueued ma_job so the job is pre-leased to this instance and foreign
	// workers on the shared DB can't steal it. Set post-construction like brave.
	owner string
	// compareSnippetCache memoizes re-gathered neutral web snippets per domain for the
	// model-comparison harness (sector-eval-models with includeSnippets). Test-support
	// only; persists for the process lifetime so a multi-request eval pays Brave once.
	compareSnippetCache sync.Map
}

func newMAService(store maWorkspaceStore, searchCache companySearchCacheStore, provinceCache provinceCacheStore, ateco atecoStore, openapiitClient *openapiit.Client, llmp maLLMProvider) *maService {
	return &maService{
		store:         store,
		searchCache:   searchCache,
		provinceCache: provinceCache,
		ateco:         ateco,
		openapiit:     openapiitClient,
		llmp:          llmp,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

func (s *maService) listSessions(ctx context.Context, visibility string) ([]MASessionSummary, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	normalized, err := normalizeMASessionVisibility(visibility)
	if err != nil {
		return nil, err
	}
	return s.store.ListMASessions(ctx, normalized)
}

func (s *maService) getSession(ctx context.Context, id string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	detail, err := s.store.GetMASession(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	if detail.Session.DeletedAt != nil {
		return MASessionDetail{}, errMASessionDeleted
	}
	// Vista di lettura (mai persistita): registro valutato/filtrato/ignorato e
	// destinazione di presentazione per ogni target (routing scarti).
	thesis := ""
	if detail.Strategy != nil {
		thesis = detail.Strategy.Strategy.Thesis
		plan := buildMAScoringPlan(detail.Strategy.Strategy)
		detail.ScoringPlan = &plan
	}
	for i := range detail.Targets {
		detail.Targets[i].Bucket = maRouteTarget(detail.Targets[i], thesis)
	}
	return decorateMACost(detail, s.loadPricing(ctx)), nil
}

func (s *maService) getSessionLean(ctx context.Context, id string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	detail, err := s.store.GetMASessionLean(ctx, id)
	if err != nil {
		return MASessionDetail{}, err
	}
	if detail.Session.DeletedAt != nil {
		return MASessionDetail{}, errMASessionDeleted
	}
	if detail.Strategy != nil {
		plan := buildMAScoringPlan(detail.Strategy.Strategy)
		detail.ScoringPlan = &plan
	}
	return decorateMACost(detail, s.loadPricing(ctx)), nil
}

func (s *maService) getSessionShape(ctx context.Context, id string, lean bool) (MASessionDetail, error) {
	if lean {
		return s.getSessionLean(ctx, id)
	}
	return s.getSession(ctx, id)
}

func (s *maService) sessionTargetRows(ctx context.Context, sessionID string) ([]MATargetRow, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	detail, err := s.store.GetMASessionLean(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if detail.Session.DeletedAt != nil {
		return nil, errMASessionDeleted
	}
	thesis := ""
	if detail.Strategy != nil {
		thesis = detail.Strategy.Strategy.Thesis
	}
	rows, err := s.store.ListMATargetRows(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	for index := range rows {
		rows[index].Bucket = maRouteTarget(rowAsTarget(rows[index]), thesis)
	}
	if err := s.decorateTargetRowsWithRegistryAndCards(ctx, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// decorateTargetRowsWithRegistryAndCards arricchisce le righe con i badge del
// registro azienda e il marker di card attive in altre iniziative (PRD §6.1,
// R-D3-7). Decorazione di sola presentazione: NON tocca bucket/score/routing.
func (s *maService) decorateTargetRowsWithRegistryAndCards(ctx context.Context, rows []MATargetRow) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	companyKeys := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		key := normalizeMACompanyKey(row.CompanyKey)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		companyKeys = append(companyKeys, key)
	}
	if len(companyKeys) == 0 {
		return nil
	}
	registryFacts, err := s.store.ListMACompanyFactsActive(ctx, companyKeys)
	if err != nil {
		return err
	}
	activeCards, err := s.store.ListMAActiveCardsByCompany(ctx, companyKeys)
	if err != nil {
		return err
	}
	titles := map[string]string{}
	for index := range rows {
		key := normalizeMACompanyKey(rows[index].CompanyKey)
		if key == "" {
			continue
		}
		rows[index].RegistryFacts = registryFacts[key]
		for _, card := range activeCards[key] {
			title, ok := titles[card.InitiativeID]
			if !ok {
				if initiative, err := s.store.GetMAInitiative(ctx, card.InitiativeID); err == nil {
					title = initiative.Title
				}
				titles[card.InitiativeID] = title
			}
			rows[index].InLavorazione = append(rows[index].InLavorazione, MACardMarker{
				InitiativeID:    card.InitiativeID,
				InitiativeTitle: title,
			})
		}
	}
	return nil
}

func (s *maService) sessionTargetDetail(ctx context.Context, sessionID, targetID string) (MATarget, error) {
	if s.store == nil {
		return MATarget{}, errMAStoreUnavailable
	}
	detail, err := s.store.GetMASessionLean(ctx, sessionID)
	if err != nil {
		return MATarget{}, err
	}
	if detail.Session.DeletedAt != nil {
		return MATarget{}, errMASessionDeleted
	}
	thesis := ""
	if detail.Strategy != nil {
		thesis = detail.Strategy.Strategy.Thesis
	}
	target, err := s.store.GetMATargetByID(ctx, sessionID, targetID)
	if err != nil {
		return MATarget{}, err
	}
	target.Bucket = maRouteTarget(target, thesis)
	return target, nil
}

func (s *maService) archiveSession(ctx context.Context, id, subject, email string) error {
	return s.updateSessionLifecycle(ctx, id, maSessionLifecycleArchive, subject, email)
}

func (s *maService) restoreSession(ctx context.Context, id, subject, email string) error {
	return s.updateSessionLifecycle(ctx, id, maSessionLifecycleRestore, subject, email)
}

func (s *maService) softDeleteSession(ctx context.Context, id, subject, email string) error {
	return s.updateSessionLifecycle(ctx, id, maSessionLifecycleDelete, subject, email)
}

// purgeSession permanently hides a trashed session from every UI view. No data is
// deleted; the session must already be in the trash (deleted_at set).
func (s *maService) purgeSession(ctx context.Context, id, subject, email string) error {
	return s.updateSessionLifecycle(ctx, id, maSessionLifecyclePurge, subject, email)
}

func (s *maService) updateSessionLifecycle(ctx context.Context, id, action, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	updated, err := s.store.UpdateMASessionLifecycle(ctx, id, action, subject, email)
	if err != nil {
		return err
	}
	if !updated {
		return sql.ErrNoRows
	}
	return nil
}

func normalizeMASessionVisibility(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", maSessionVisibilityActive:
		return maSessionVisibilityActive, nil
	case maSessionVisibilityArchived:
		return maSessionVisibilityArchived, nil
	case maSessionVisibilityDeleted:
		return maSessionVisibilityDeleted, nil
	default:
		return "", errMAVisibilityInvalid
	}
}

func ensureMASessionOperational(session MASession) error {
	if session.DeletedAt != nil {
		return errMASessionDeleted
	}
	if session.ArchivedAt != nil {
		return errMASessionArchived
	}
	return nil
}

// errMAInitiativeArchived mirrors errMASessionArchived: an archived Iniziativa
// can still be viewed (index "Archivio") but refuses new anchors.
var errMAInitiativeArchived = errors.New("ma initiative archived")

func ensureMAInitiativeOperational(initiative MAInitiative) error {
	if initiative.ArchivedAt != nil {
		return errMAInitiativeArchived
	}
	return nil
}

// createInitiative validates and persists a new Iniziativa (PRD §3.1): title
// obbligatorio, description opzionale. Non-CRM by design — no budget/KPI/
// deadline fields.
func (s *maService) createInitiative(ctx context.Context, title, description, subject, email string) (MAInitiative, error) {
	if s.store == nil {
		return MAInitiative{}, errMAStoreUnavailable
	}
	title = cleanText(title, 120)
	if title == "" {
		return MAInitiative{}, fmt.Errorf("%w: title", errMAStrategyInvalid)
	}
	description = cleanText(description, 500)
	initiative := MAInitiative{
		ID:               uuid.NewString(),
		Title:            title,
		Description:      description,
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	}
	return s.store.CreateMAInitiative(ctx, initiative)
}

// listInitiatives returns the /iniziative index rows, active or archived.
func (s *maService) listInitiatives(ctx context.Context, includeArchived bool) ([]MAInitiativeSummary, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	return s.store.ListMAInitiatives(ctx, includeArchived)
}

func (s *maService) archiveInitiative(ctx context.Context, id, subject, email string) error {
	return s.updateInitiativeLifecycle(ctx, id, maSessionLifecycleArchive, subject, email)
}

func (s *maService) restoreInitiative(ctx context.Context, id, subject, email string) error {
	return s.updateInitiativeLifecycle(ctx, id, maSessionLifecycleRestore, subject, email)
}

func (s *maService) updateInitiativeLifecycle(ctx context.Context, id, action, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	updated, err := s.store.UpdateMAInitiativeLifecycle(ctx, id, action, subject, email)
	if err != nil {
		return err
	}
	if !updated {
		return sql.ErrNoRows
	}
	return nil
}

// setSessionInitiative anchors or unanchors (initiativeID == "") a session to
// an Iniziativa (PRD §3.2). The session must be operational; a non-empty
// target initiative must exist and not be archived. Card backfill for
// already-rated companies is wired in B2 (ensureInitiativeCard) — B1 only
// writes the anchor column.
func (s *maService) setSessionInitiative(ctx context.Context, sessionID, initiativeID, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	session, err := s.store.GetMASessionState(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := ensureMASessionOperational(session); err != nil {
		return err
	}
	initiativeID = strings.TrimSpace(initiativeID)
	if initiativeID != "" {
		initiative, err := s.store.GetMAInitiative(ctx, initiativeID)
		if err != nil {
			return err
		}
		if err := ensureMAInitiativeOperational(initiative); err != nil {
			return err
		}
	}
	if err := s.store.SetMASessionInitiative(ctx, sessionID, initiativeID); err != nil {
		return err
	}
	// Backfill (PRD §3.2): agganciare una sessione che ha già >=1★ crea le
	// card mancanti, stessa regola d'ingresso di §4.1 applicata
	// retroattivamente. Sgancio (initiativeID == "") non tocca le card
	// esistenti: sono già autonome (§4.2).
	if initiativeID != "" {
		ratings, err := s.store.ListMARatings(ctx, sessionID)
		if err != nil {
			return err
		}
		for companyKey, rating := range ratings {
			if rating < 1 {
				continue
			}
			if err := s.ensureInitiativeCard(ctx, initiativeID, sessionID, companyKey, rating, subject, email); err != nil {
				return err
			}
		}
	}
	return nil
}

// maCompanyFactKinds is the closed vocabulary of the company registry (PRD
// §6, mig 090): warning kinds (non_vende, in_trattativa_altrui, da_evitare)
// and informational kinds (gia_cliente, partner).
var maCompanyFactKinds = map[string]bool{
	"non_vende":            true,
	"in_trattativa_altrui": true,
	"da_evitare":           true,
	"gia_cliente":          true,
	"partner":              true,
}

// errMACompanyFactKindInvalid signals a kind outside the closed vocabulary.
var errMACompanyFactKindInvalid = fmt.Errorf("%w: kind", errMAStrategyInvalid)

// addCompanyFact records a new typed fact in the company registry (PRD §6).
// vatCode/taxCode/companyName are an optional snapshot supplied by the
// caller (mirrors mig 082's ma_company_domain pattern) — empty when unknown.
func (s *maService) addCompanyFact(ctx context.Context, companyKey, kind, note, vatCode, taxCode, companyName, subject, email string) (MACompanyFact, error) {
	if s.store == nil {
		return MACompanyFact{}, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return MACompanyFact{}, fmt.Errorf("%w: companyKey", errMAStrategyInvalid)
	}
	if !maCompanyFactKinds[kind] {
		return MACompanyFact{}, errMACompanyFactKindInvalid
	}
	fact := MACompanyFact{
		ID:               uuid.NewString(),
		CompanyKey:       companyKey,
		VATCode:          strings.TrimSpace(vatCode),
		TaxCode:          strings.TrimSpace(taxCode),
		CompanyName:      strings.TrimSpace(companyName),
		Kind:             kind,
		Note:             cleanText(note, 500),
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	}
	return s.store.InsertMACompanyFact(ctx, fact)
}

// revokeCompanyFact revokes an active fact (PRD §6: history preserved, never
// deleted). Returns sql.ErrNoRows when no active fact with that id exists.
func (s *maService) revokeCompanyFact(ctx context.Context, id, note, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	revoked, err := s.store.RevokeMACompanyFact(ctx, id, subject, email, cleanText(note, 500))
	if err != nil {
		return err
	}
	if !revoked {
		return sql.ErrNoRows
	}
	return nil
}

// addCompanyNote appends a free-text note to the company registry (PRD §6):
// registry content, never a log-of-events entry.
func (s *maService) addCompanyNote(ctx context.Context, companyKey, body, subject, email string) (MACompanyNote, error) {
	if s.store == nil {
		return MACompanyNote{}, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return MACompanyNote{}, fmt.Errorf("%w: companyKey", errMAStrategyInvalid)
	}
	body = cleanText(body, 1000)
	if body == "" {
		return MACompanyNote{}, fmt.Errorf("%w: body", errMAStrategyInvalid)
	}
	note := MACompanyNote{
		ID:               uuid.NewString(),
		CompanyKey:       companyKey,
		Body:             body,
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	}
	return s.store.InsertMACompanyNote(ctx, note)
}

// getCompanyRegistry loads the full registry (facts + notes) for the dossier
// §6 section.
func (s *maService) getCompanyRegistry(ctx context.Context, companyKey string) (MACompanyRegistry, error) {
	if s.store == nil {
		return MACompanyRegistry{}, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return MACompanyRegistry{}, fmt.Errorf("%w: companyKey", errMAStrategyInvalid)
	}
	return s.store.GetMACompanyRegistry(ctx, companyKey)
}

// maPricing holds the business pricing/budget levers, sourced from the
// ma_parameter table (migration 038) with the compiled constants as fallback.
type maPricing struct {
	CostAdvanced               float64
	CostFull                   float64
	CostDryRun                 float64
	BudgetDefault              float64
	SMEHaircutPct              float64
	EBITDAFallbackPct          float64
	VendorCEETolerancePct      float64
	TFRBridgePct               float64
	A5EBITDAFlagPct            float64
	B8RevenueFlagPct           float64
	ParticipationAssetsFlagPct float64
	ParticipationIncomeFlagPct float64
	ThesisFitHoldingHaircutPct float64
	// Gated-search pipeline levers (migration 074): CostAddress = per-company gate
	// enrichment, CostScrapePage/CostSearch = fastcrw unit costs, SurvivorRate =
	// expected keep+forse fraction (0-1) used by the estimate's Advanced projection.
	CostAddress    float64
	CostScrapePage float64
	CostSearch     float64
	SurvivorRate   float64
	// SurfaceCap is the max surface a gated search may admit (the one hard cost
	// governor under automatic spend); overridable via ma_parameter.
	SurfaceCap int
	// Ancore delle rampe assolute dei segnali economici (migrazione 081).
	// TrendCagrDeclineFloorPct è il MODULO del declino (10 = -10%/anno) perché
	// paramFloat rifiuta i negativi.
	TrendCagrDeclineFloorPct float64
	TrendCagrTopPct          float64
	ProductivityFloorEUR     float64
	ProductivityTopEUR       float64
}

// maScoringParamsFromPricing converte le leve ma_parameter nei parametri dello
// scoring puro (unico punto di conversione: i 3 callsite di scoreMATargetsV2
// devono restare coerenti tra loro).
func maScoringParamsFromPricing(pricing maPricing) maScoringParams {
	return maScoringParams{
		ThesisFitHoldingFactor: 1 - pricing.ThesisFitHoldingHaircutPct/100,
		TrendCagrFloor:         -pricing.TrendCagrDeclineFloorPct / 100,
		TrendCagrTop:           pricing.TrendCagrTopPct / 100,
		ProductivityFloorEUR:   pricing.ProductivityFloorEUR,
		ProductivityTopEUR:     pricing.ProductivityTopEUR,
	}
}

// loadPricing reads the configurable pricing levers; missing/unreadable values
// fall back to the compiled defaults so the feature degrades gracefully.
func (s *maService) loadPricing(ctx context.Context) maPricing {
	if s.store == nil {
		return maPricingFromParameters(nil)
	}
	params, err := s.store.ListMAParameters(ctx)
	if err != nil {
		return maPricingFromParameters(nil)
	}
	return maPricingFromParameters(params)
}

func maPricingFromParameters(params []MAParameter) maPricing {
	pricing := maPricing{
		CostAdvanced:               maCostPerCompanyEUR,
		CostFull:                   maCostPerFullEUR,
		CostDryRun:                 maCostPerDryRunEUR,
		BudgetDefault:              maDefaultBudgetEUR,
		SMEHaircutPct:              maSMEHaircutPctDefault,
		EBITDAFallbackPct:          maEBITDAFallbackPctDefault,
		VendorCEETolerancePct:      maVendorCEETolerancePctDefault,
		TFRBridgePct:               maTFRBridgePctDefault,
		A5EBITDAFlagPct:            maA5EBITDAFlagPctDefault,
		B8RevenueFlagPct:           maB8RevenueFlagPctDefault,
		ParticipationAssetsFlagPct: maParticipationAssetsFlagPctDefault,
		ParticipationIncomeFlagPct: maParticipationIncomeFlagPctDefault,
		ThesisFitHoldingHaircutPct: maThesisFitHoldingHaircutPctDefault,
		CostAddress:                maCostPerAddressEUR,
		CostScrapePage:             maCostPerScrapePageEUR,
		CostSearch:                 maCostPerSearchEUR,
		SurvivorRate:               maSurvivorRateDefault,
		SurfaceCap:                 maGatedSurfaceCapDefault,
		TrendCagrDeclineFloorPct:   -maTrendCagrFloorDefault * 100,
		TrendCagrTopPct:            maTrendCagrTopDefault * 100,
		ProductivityFloorEUR:       maProductivityFloorEURDefault,
		ProductivityTopEUR:         maProductivityTopEURDefault,
	}
	values := make(map[string]string, len(params))
	for _, param := range params {
		values[param.Key] = param.Value
	}
	if v, ok := paramFloat(values, "cost_advanced_eur"); ok {
		pricing.CostAdvanced = v
	}
	if v, ok := paramFloat(values, "cost_full_eur"); ok {
		pricing.CostFull = v
	}
	if v, ok := paramFloat(values, "cost_dryrun_eur"); ok {
		pricing.CostDryRun = v
	}
	if v, ok := paramFloat(values, "budget_default_eur"); ok {
		pricing.BudgetDefault = v
	}
	if v, ok := paramFloat(values, "sme_haircut_pct"); ok {
		pricing.SMEHaircutPct = v
	}
	if v, ok := paramFloat(values, "ebitda_fallback_threshold"); ok {
		pricing.EBITDAFallbackPct = v
	}
	if v, ok := paramFloat(values, "vendor_cee_tolerance_pct"); ok {
		pricing.VendorCEETolerancePct = v
	}
	if v, ok := paramFloat(values, "tfr_bridge_pct"); ok && v <= 100 {
		pricing.TFRBridgePct = v
	}
	if v, ok := paramFloat(values, "a5_ebitda_flag_pct"); ok {
		pricing.A5EBITDAFlagPct = v
	}
	if v, ok := paramFloat(values, "b8_revenue_flag_pct"); ok {
		pricing.B8RevenueFlagPct = v
	}
	if v, ok := paramFloat(values, "participation_assets_flag_pct"); ok {
		pricing.ParticipationAssetsFlagPct = v
	}
	if v, ok := paramFloat(values, "participation_income_flag_pct"); ok {
		pricing.ParticipationIncomeFlagPct = v
	}
	if v, ok := paramFloat(values, "thesis_fit_holding_haircut_pct"); ok && v < 100 {
		pricing.ThesisFitHoldingHaircutPct = v
	}
	if v, ok := paramFloat(values, "cost_address_eur"); ok {
		pricing.CostAddress = v
	}
	if v, ok := paramFloat(values, "cost_scrape_page_eur"); ok {
		pricing.CostScrapePage = v
	}
	if v, ok := paramFloat(values, "cost_search_eur"); ok {
		pricing.CostSearch = v
	}
	if v, ok := paramFloat(values, "survivor_rate_default"); ok && v > 0 && v <= 1 {
		pricing.SurvivorRate = v
	}
	if v, ok := paramInt(values, "gated_surface_cap"); ok && v > 0 {
		pricing.SurfaceCap = v
	}
	if v, ok := paramFloat(values, "trend_cagr_decline_floor_pct"); ok && v > 0 {
		pricing.TrendCagrDeclineFloorPct = v
	}
	if v, ok := paramFloat(values, "trend_cagr_top_pct"); ok && v > 0 {
		pricing.TrendCagrTopPct = v
	}
	if v, ok := paramFloat(values, "productivity_floor_eur"); ok && v > 0 {
		pricing.ProductivityFloorEUR = v
	}
	if v, ok := paramFloat(values, "productivity_top_eur"); ok && v > 0 {
		pricing.ProductivityTopEUR = v
	}
	if pricing.ProductivityTopEUR <= pricing.ProductivityFloorEUR {
		pricing.ProductivityFloorEUR = maProductivityFloorEURDefault
		pricing.ProductivityTopEUR = maProductivityTopEURDefault
	}
	return pricing
}

func paramFloat(values map[string]string, key string) (float64, bool) {
	raw, ok := values[key]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func paramInt(values map[string]string, key string) (int, bool) {
	raw, ok := values[key]
	if !ok {
		return 0, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return v, true
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// maGatedCostProjection is the up-front cost estimate for a gated-search submission
// over surfaceCount companies. With "tutto automatico" spend control this projection
// (shown before launch) plus the surface cap are the only cost guardrails. The gate
// cost is CERTAIN — every company on the surface pays Address + scrape + search; the
// Advanced cost is EXPECTED — only the keep+forse survivors pay it — so it is a band
// around survivor_rate_default. Wired into the gated-search estimate + cap in Step 3.
type maGatedCostProjection struct {
	SurfaceCount        int     `json:"surfaceCount"`
	GatePerCompanyEUR   float64 `json:"gatePerCompanyEur"`
	GateCostEUR         float64 `json:"gateCostEur"`         // certain: N × per-company gate
	SurvivorRate        float64 `json:"survivorRate"`        // mid (survivor_rate_default)
	ExpectedAdvancedEUR float64 `json:"expectedAdvancedEur"` // N × survivorRate × advanced
	TotalEUR            float64 `json:"totalEur"`            // gate + expected advanced (mid)
	TotalLowEUR         float64 `json:"totalLowEur"`         // gate + N × survivorLow × advanced
	TotalHighEUR        float64 `json:"totalHighEur"`        // gate + N × survivorHigh × advanced
}

// projectGatedSearchCost applies the two-part gated-search cost formula. Pure
// function of the surface count and pricing so it is unit-tested in isolation.
func projectGatedSearchCost(surfaceCount int, pricing maPricing) maGatedCostProjection {
	if surfaceCount < 0 {
		surfaceCount = 0
	}
	n := float64(surfaceCount)
	gatePerCompany := pricing.CostAddress +
		float64(maGateScrapePagesEst)*pricing.CostScrapePage +
		float64(maGateSearchesEst)*pricing.CostSearch
	gateCost := n * gatePerCompany
	advanced := func(rate float64) float64 { return n * rate * pricing.CostAdvanced }
	return maGatedCostProjection{
		SurfaceCount:        surfaceCount,
		GatePerCompanyEUR:   round2(gatePerCompany),
		GateCostEUR:         round2(gateCost),
		SurvivorRate:        pricing.SurvivorRate,
		ExpectedAdvancedEUR: round2(advanced(pricing.SurvivorRate)),
		TotalEUR:            round2(gateCost + advanced(pricing.SurvivorRate)),
		TotalLowEUR:         round2(gateCost + advanced(maSurvivorRateLow)),
		TotalHighEUR:        round2(gateCost + advanced(maSurvivorRateHigh)),
	}
}

// decorateMACost attaches the active enrichment budget and unit price so the UI
// can show projected spend and the cost gate without duplicating the pricing.
func decorateMACost(detail MASessionDetail, pricing maPricing) MASessionDetail {
	detail.CostPerCompanyEUR = pricing.CostAdvanced
	detail.CostFullEUR = pricing.CostFull
	detail.BudgetEUR = pricing.BudgetDefault
	if detail.Strategy != nil {
		detail.BudgetEUR = maStrategyBudget(detail.Strategy.Strategy, pricing.BudgetDefault)
	}
	return detail
}

func (s *maService) listLLMOptions(ctx context.Context) (MALLMOptionsResponse, error) {
	if s.llmp == nil {
		return MALLMOptionsResponse{}, errMAOpenRouterUnavailable
	}
	models, err := s.llmp.ListModels(ctx)
	if err != nil {
		return MALLMOptionsResponse{}, err
	}
	prompts, err := s.llmp.ListPrompts(ctx)
	if err != nil {
		return MALLMOptionsResponse{}, err
	}
	out := MALLMOptionsResponse{
		Models:  make([]MALLMModelOption, 0, len(models)),
		Prompts: make([]MALLMPromptOption, 0, len(prompts)),
	}
	for _, m := range models {
		out.Models = append(out.Models, MALLMModelOption{ID: m.ID, Scope: m.Scope, Name: m.Name, Model: m.Model, IsDefault: m.IsDefault})
	}
	for _, p := range prompts {
		out.Prompts = append(out.Prompts, MALLMPromptOption{ID: p.ID, Scope: p.Scope, Name: p.Name, IsDefault: p.IsDefault})
	}
	return out, nil
}

func (s *maService) createSession(ctx context.Context, req MACreateSessionRequest, subject, email string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	prompt := cleanText(req.Prompt, 4000)
	if prompt == "" {
		return MASessionDetail{}, fmt.Errorf("%w: prompt", errMAStrategyInvalid)
	}
	params := []MAParameter(nil)
	if listed, err := s.store.ListMAParameters(ctx); err == nil {
		params = listed
	}
	pipeline := maStrategyPipelineFromParameters(params)
	pricing := maPricingFromParameters(params)
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_strategy_pipeline_selected",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"pipeline":      pipeline,
			"parameter_key": maStrategyPipelineParameter,
		}),
	}); err != nil {
		return MASessionDetail{}, err
	}

	var strategy MAStrategySpec
	var audits []llm.CallAudit
	if pipeline == maStrategyPipelineMonolith {
		strategyAudit := llm.CallAudit{}
		var err error
		strategy, strategyAudit, err = s.draftStrategy(ctx, prompt, req.ModelID, req.PromptID, subject, email)
		if err != nil {
			return MASessionDetail{}, err
		}
		audits = append(audits, strategyAudit)
	} else {
		intent, intentAudit, err := s.extractIntent(ctx, prompt, subject, email)
		if err != nil {
			return MASessionDetail{}, err
		}
		audits = append(audits, intentAudit)

		resolveAudits := []llm.CallAudit{}
		strategy, resolveAudits, err = s.resolveIntent(ctx, prompt, intent, subject, email)
		audits = append(audits, resolveAudits...)
		if err != nil {
			return MASessionDetail{}, err
		}
	}
	if req.GatedFlow {
		limit := pricing.SurfaceCap
		if limit <= 0 {
			limit = maVendorLimit
		}
		strategy.SearchLimit = normalizeMASearchLimit(limit)
	}
	title := strategy.Title
	if title == "" {
		title = titleFromPrompt(prompt)
	}
	detail, err := s.store.CreateMASession(ctx, maSessionCreate{
		Session: MASession{
			ID:               uuid.NewString(),
			Title:            title,
			Prompt:           prompt,
			CreatedBySubject: subject,
			CreatedByEmail:   email,
		},
		Strategy: strategy,
	})
	if err != nil {
		return MASessionDetail{}, err
	}
	if detail.Strategy != nil {
		if err := s.linkTrace(ctx, maTraceLink{SessionID: detail.Session.ID, StrategyVersionID: detail.Strategy.ID}); err != nil {
			return MASessionDetail{}, err
		}
	} else if err := s.linkTrace(ctx, maTraceLink{SessionID: detail.Session.ID}); err != nil {
		return MASessionDetail{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_session_created",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"session_id":          detail.Session.ID,
			"strategy_version_id": nullableTraceString(detail.Session.ActiveStrategyID),
			"title":               detail.Session.Title,
		}),
	}); err != nil {
		return MASessionDetail{}, err
	}
	strategyVersionID := ""
	if detail.Strategy != nil {
		strategyVersionID = detail.Strategy.ID
	}
	if err := s.recordMAStrategyAudits(ctx, audits, detail.Session.ID, strategyVersionID); err != nil {
		return MASessionDetail{}, err
	}
	return decorateMACost(detail, pricing), nil
}

const (
	maStrategyPipelineParameter = "ma_strategy_pipeline"
	maStrategyPipelineV2        = "v2"
	maStrategyPipelineMonolith  = "monolith"
)

func (s *maService) loadMAStrategyPipeline(ctx context.Context) string {
	if s.store == nil {
		return maStrategyPipelineFromParameters(nil)
	}
	params, err := s.store.ListMAParameters(ctx)
	if err != nil {
		return maStrategyPipelineFromParameters(nil)
	}
	return maStrategyPipelineFromParameters(params)
}

func maStrategyPipelineFromParameters(params []MAParameter) string {
	for _, param := range params {
		if param.Key != maStrategyPipelineParameter {
			continue
		}
		switch param.Value {
		case maStrategyPipelineMonolith:
			return maStrategyPipelineMonolith
		case maStrategyPipelineV2:
			return maStrategyPipelineV2
		default:
			return maStrategyPipelineV2
		}
	}
	return maStrategyPipelineV2
}

func (s *maService) recordMAStrategyAudits(ctx context.Context, audits []llm.CallAudit, sessionID, strategyVersionID string) error {
	if len(audits) == 0 {
		return nil
	}
	if s.llmp == nil {
		return errMAOpenRouterUnavailable
	}
	auditCtx := map[string]string{
		"session_id":          sessionID,
		"strategy_version_id": strategyVersionID,
	}
	raw, err := json.Marshal(auditCtx)
	if err != nil {
		return err
	}
	for _, audit := range audits {
		audit.Context = raw
		if err := s.llmp.RecordAudit(ctx, audit); err != nil {
			return err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_model_audit_recorded",
			Status:    maTraceEventSucceeded,
			Metadata: maTraceJSON(map[string]any{
				"scope":               audit.Scope,
				"model_id":            audit.ModelID,
				"prompt_id":           audit.PromptID,
				"session_id":          sessionID,
				"strategy_version_id": strategyVersionID,
			}),
		}); err != nil {
			return err
		}
	}
	return nil
}

type maEstimateJobPayload struct {
	StrategyType string `json:"strategy_type,omitempty"`
}

// enqueueEstimate is the synchronous half of the estimate flow: validate the
// request, materialize the strategy version, and queue a background job. The
// expensive surface probing runs in the estimate worker (runEstimateJob), so the
// HTTP request returns immediately — it cannot time out while the backend works,
// and a client disconnect can no longer cancel the work mid-flight.
func (s *maService) enqueueEstimate(ctx context.Context, sessionID string, req MAEstimateSessionRequest, subject, email string, lean bool) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MASessionDetail{}, errMAOpenAPIITUnavailable
	}
	var detail MASessionDetail
	var err error
	if lean {
		detail, err = s.store.GetMASessionLean(ctx, sessionID)
	} else {
		detail, err = s.store.GetMASession(ctx, sessionID)
	}
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion := detail.Strategy
	if req.Strategy != nil {
		strategy, err := validateMAStrategy(*req.Strategy)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, nil, false)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategyVersion, err = s.store.AddMAStrategyVersion(ctx, sessionID, strategy, email)
		if err != nil {
			return MASessionDetail{}, err
		}
	}
	if strategyVersion == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}
	strategyType := normalizeMAStrategyType(req.StrategyType)
	payload, err := json.Marshal(maEstimateJobPayload{StrategyType: strategyType})
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("marshal ma estimate payload: %w", err)
	}
	// One in-flight estimate per session (ma_job_inflight_idx). A re-submit while a
	// job is queued/running is a no-op here: that job estimates whatever the active
	// version is at run time and loops if it changes, so the latest strategy is
	// always covered without launching duplicate work.
	if _, _, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType:           maJobTypeEstimate,
		SessionID:         sessionID,
		StrategyVersionID: strategyVersion.ID,
		Subject:           subject,
		Email:             email,
		Payload:           payload,
		Owner:             s.owner,
	}); err != nil {
		return MASessionDetail{}, err
	}
	// Reflect the in-flight state so the UI polls. Gated on active match: if a newer
	// version was activated between load and here, that newer enqueue owns the state.
	if err := s.store.SetMASessionEstimateStatus(ctx, sessionID, strategyVersion.ID, maSessionStatusEstimating); err != nil {
		return MASessionDetail{}, err
	}
	return s.getSessionShape(ctx, sessionID, lean)
}

// runEstimateJob executes a queued estimate job off the request path. It owns the
// operation trace for the run (the request handler no longer can, having already
// returned) and returns the trace id for correlation. estimateJobWork retries
// internally against the active strategy version, so this surfaces only a real
// (non-supersession) error to the worker for job-row bookkeeping.
func (s *maService) runEstimateJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_session_estimate",
		SessionID:        job.SessionID,
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	if workErr := s.estimateJobWork(ctx, job); workErr != nil {
		_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusFailed, ErrorMessage: workErr.Error()})
		return trace.id, workErr
	}
	_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusSucceeded, HTTPStatus: http.StatusOK})
	return trace.id, nil
}

// estimateJobWork probes the search surface for the session's active strategy and
// persists the estimates. It always targets the *current* active version: if the
// user activated a newer version mid-run, ReplaceMAEstimates reports it superseded
// and the loop re-runs for the now-active version. A small cap guards against a
// pathological flip-flop.
func (s *maService) estimateJobWork(ctx context.Context, job maJob) error {
	const maxSupersedeLoops = 3
	var payload maEstimateJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma estimate payload: %w", err)
		}
	}
	requestedStrategyType := normalizeMAStrategyType(payload.StrategyType)
	for attempt := 0; attempt < maxSupersedeLoops; attempt++ {
		detail, err := s.store.GetMASession(ctx, job.SessionID)
		if err != nil {
			return err
		}
		if err := ensureMASessionOperational(detail.Session); err != nil {
			return err
		}
		version := detail.Strategy
		if version == nil {
			return fmt.Errorf("%w: strategy", errMAStrategyInvalid)
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_estimate_started",
			Status:    maTraceEventStarted,
			Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "strategy_version_id": version.ID, "attempt": attempt}),
		}); err != nil {
			return err
		}
		if err := s.linkTrace(ctx, maTraceLink{SessionID: job.SessionID, StrategyVersionID: version.ID}); err != nil {
			return err
		}
		strategy := version.Strategy
		strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, nil, false)
		if err != nil {
			return err
		}
		strategy, err = s.expandStrategyAteco(ctx, strategy, job.CreatedBySubject, job.CreatedByEmail)
		if err != nil {
			return err
		}
		strategy, err = s.expandStrategyExpansion(ctx, strategy, job.CreatedBySubject, job.CreatedByEmail)
		if err != nil {
			return err
		}
		estimates, selected, err := s.runEstimates(ctx, job.SessionID, version.ID, strategy, requestedStrategyType, job.CreatedBySubject, job.CreatedByEmail)
		if err != nil {
			return err
		}
		err = s.store.ReplaceMAEstimates(ctx, job.SessionID, version.ID, selected, estimates)
		if errors.Is(err, errMAEstimateSuperseded) {
			continue // active version changed during the run; re-estimate the new one
		}
		if err != nil {
			return err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_estimates_saved",
			Status:    maTraceEventSucceeded,
			Metadata: maTraceJSON(map[string]any{
				"session_id":          job.SessionID,
				"strategy_version_id": version.ID,
				"selected_strategy":   selected,
				"estimate_count":      len(estimates),
			}),
		}); err != nil {
			return err
		}
		return nil
	}
	return errMAEstimateSuperseded
}

// maExecuteJobPayload is the execute job's self-contained args: the analyst's
// confirmed choices, validated synchronously at enqueue. The strategy version is
// pinned via the job row (StrategyVersionID), so the worker executes exactly what
// was confirmed.
type maExecuteJobPayload struct {
	StrategyType   string `json:"strategy_type"`
	Limit          int    `json:"limit"`
	EstimatedCount int    `json:"estimated_count"`
}

// enqueueExecute is the synchronous half of the execute flow. It runs the fast
// validation (estimate freshness, surface ceiling, budget gate) against the
// already-persisted estimates so the analyst gets an immediate 4xx, then queues a
// background job. The heavy network work — subtree expansion and the paid company
// fetch — runs in the worker (runExecuteJob), so the request cannot time out and a
// client disconnect cannot cancel the run.
func (s *maService) enqueueExecute(ctx context.Context, sessionID string, req MAExecuteSessionRequest, subject, email string, lean bool) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MASessionDetail{}, errMAOpenAPIITUnavailable
	}
	var detail MASessionDetail
	var err error
	if lean {
		detail, err = s.store.GetMASessionLean(ctx, sessionID)
	} else {
		detail, err = s.store.GetMASession(ctx, sessionID)
	}
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	strategyVersion := detail.Strategy
	if req.Strategy != nil {
		strategy, err := validateMAStrategy(*req.Strategy)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, nil, false)
		if err != nil {
			return MASessionDetail{}, err
		}
		strategyVersion, err = s.store.AddMAStrategyVersion(ctx, sessionID, strategy, email)
		if err != nil {
			return MASessionDetail{}, err
		}
		if lean {
			detail, err = s.store.GetMASessionLean(ctx, sessionID)
		} else {
			detail, err = s.store.GetMASession(ctx, sessionID)
		}
		if err != nil {
			return MASessionDetail{}, err
		}
	}
	if strategyVersion == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}

	// Validation uses the persisted estimates and the strategy's base fields only —
	// no subtree expansion needed here, so it stays fast and synchronous.
	strategyType := normalizeMAStrategyType(req.StrategyType)
	if strategyType == "" {
		strategyType = detail.Session.SelectedStrategy
	}
	if strategyType == "" {
		strategyType = strategyVersion.Strategy.SelectedStrategy
	}
	if strategyType == "" {
		strategyType = chooseSelectedStrategyFromEstimates(detail.Estimates, estimateTotal(detail.Estimates, maStrategyTypeATECO) > 0)
	}
	estimatedCount := estimateTotal(detail.Estimates, strategyType)
	if len(detail.Estimates) == 0 || estimatedCount == 0 {
		return MASessionDetail{}, fmt.Errorf("%w: estimate required", errMAStrategyInvalid)
	}
	if estimatesTooBroad(detail.Estimates, strategyType) {
		return MASessionDetail{}, errMAEstimateTooLarge
	}
	limit := normalizeMASearchLimit(req.Limit)
	if limit != strategyVersion.Strategy.SearchLimit {
		return MASessionDetail{}, fmt.Errorf("%w: stale estimate", errMAStrategyInvalid)
	}
	if !estimatesMatchSearchLimit(detail.Estimates, strategyType, limit) {
		return MASessionDetail{}, fmt.Errorf("%w: stale estimate", errMAStrategyInvalid)
	}
	pricing := s.loadPricing(ctx)
	budget := maStrategyBudget(strategyVersion.Strategy, pricing.BudgetDefault)
	projectedCost := maProjectedSpend(estimatedCount, limit, pricing.CostAdvanced)
	if projectedCost > budget && !req.AcknowledgeCost {
		return MASessionDetail{}, errMAEstimateOverBudget
	}

	payload, err := json.Marshal(maExecuteJobPayload{StrategyType: strategyType, Limit: limit, EstimatedCount: estimatedCount})
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("marshal ma execute payload: %w", err)
	}
	// One in-flight execute per session (ma_job_inflight_idx); a duplicate submit
	// while a run is queued/running is a no-op (no second run created). The worker
	// creates the execution run when it picks the job up, so there is never a
	// dangling run without a job.
	_, created, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType:           maJobTypeExecute,
		SessionID:         sessionID,
		StrategyVersionID: strategyVersion.ID,
		Subject:           subject,
		Email:             email,
		Payload:           payload,
		Owner:             s.owner,
	})
	if err != nil {
		return MASessionDetail{}, err
	}
	if created {
		if err := s.store.MarkMASessionExecuting(ctx, sessionID); err != nil {
			return MASessionDetail{}, err
		}
	}
	return s.getSessionShape(ctx, sessionID, lean)
}

// runExecuteJob executes a queued execute job off the request path. It owns the
// operation trace for the run and returns the trace id for correlation.
func (s *maService) runExecuteJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_session_execute",
		SessionID:        job.SessionID,
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	if workErr := s.executeJobWork(ctx, job); workErr != nil {
		_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusFailed, ErrorMessage: workErr.Error()})
		return trace.id, workErr
	}
	_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusSucceeded, HTTPStatus: http.StatusOK})
	return trace.id, nil
}

// executeJobWork creates the execution run, expands the pinned strategy, runs the
// paid company fetch, scores and persists targets. It returns an error ONLY for
// pre-run infra failures (which the worker retries); once a run exists, an
// execution failure is recorded as a failed run (which moves the session to
// 'failed') and returns nil — a paid execution is never silently re-run.
func (s *maService) executeJobWork(ctx context.Context, job maJob) error {
	var payload maExecuteJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma execute payload: %w", err)
		}
	}
	detail, err := s.store.GetMASession(ctx, job.SessionID)
	if err != nil {
		return err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return err
	}
	version, err := s.store.GetMAStrategyVersion(ctx, job.SessionID, job.StrategyVersionID)
	if err != nil {
		return err
	}
	strategyType := normalizeMAStrategyType(payload.StrategyType)
	limit := normalizeMASearchLimit(payload.Limit)

	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_execute_started",
		Status:    maTraceEventStarted,
		Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "strategy_version_id": version.ID, "strategy_type": strategyType, "limit": limit}),
	}); err != nil {
		return err
	}

	// Charge guard: if a prior attempt already created a run (a worker crashed
	// mid-fetch and this is a reclaim), do NOT create a second run and re-charge the
	// paid company fetch. Abandon the orphan; the analyst re-executes deliberately.
	// Fresh runs and pre-run-failure retries pass this — no run exists yet.
	inflight, err := s.store.HasRunningMAExecution(ctx, job.SessionID)
	if err != nil {
		return err
	}
	if inflight {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_execution_abandoned",
			Status:    maTraceEventFailed,
			Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "reason": "execution already in flight (worker likely crashed mid-run)"}),
		})
		return s.store.AbandonMASessionExecution(ctx, job.SessionID, "execute_interrupted")
	}

	run, err := s.store.CreateMAExecutionRun(ctx, maExecutionRunCreate{
		SessionID:         job.SessionID,
		StrategyVersionID: version.ID,
		StrategyType:      strategyType,
		EstimatedCount:    payload.EstimatedCount,
	})
	if err != nil {
		return err
	}
	// Best-effort from here: once the run exists, never return an error that would
	// make the worker retry (and re-charge). Failures below are recorded on the run.
	_ = s.linkTrace(ctx, maTraceLink{SessionID: job.SessionID, StrategyVersionID: version.ID, ExecutionRunID: run.ID})

	strategy := version.Strategy
	strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, nil, false)
	if err != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, "canonicalize_error")
		return nil
	}
	strategy, err = s.expandStrategyAteco(ctx, strategy, job.CreatedBySubject, job.CreatedByEmail)
	if err != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, maErrorCode(err))
		return nil
	}
	strategy, err = s.expandStrategyExpansion(ctx, strategy, job.CreatedBySubject, job.CreatedByEmail)
	if err != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, maErrorCode(err))
		return nil
	}

	targets, execErr := s.runExecution(ctx, strategy, strategyType, limit, job.CreatedBySubject, job.CreatedByEmail, maEnrichmentAdvanced)
	if execErr != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, maErrorCode(execErr))
		return nil
	}
	for index := range targets {
		targets[index].SessionID = job.SessionID
		targets[index].RunID = run.ID
	}
	pricing := s.loadPricing(ctx)
	scoringParams := maScoringParamsFromPricing(pricing)
	targets = scoreMATargetsV2(targets, strategy, scoringParams, s.now())
	missingFinancials := 0
	for _, target := range targets {
		for _, flag := range target.Flags {
			if flag.Code == "bilancio_assente" {
				missingFinancials++
				break
			}
		}
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_scoring_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "run_id": run.ID, "result_count": len(targets), "missing_financials": missingFinancials, "thesis": normalizeMAThesis(strategy.Thesis)}),
	})
	if err := s.store.ReplaceMATargets(ctx, job.SessionID, run.ID, targets); err != nil {
		_ = s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusFailed, 0, "store_error")
		return nil
	}
	if err := s.store.CompleteMAExecutionRun(ctx, run.ID, maRunStatusCompleted, len(targets), ""); err != nil {
		return err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_execution_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": job.SessionID, "run_id": run.ID, "result_count": len(targets)}),
	})
	return nil
}

func (s *maService) exportSession(ctx context.Context, sessionID string, format string, email string) ([]byte, string, string, error) {
	if s.store == nil {
		return nil, "", "", errMAStoreUnavailable
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_export_started",
		Status:    maTraceEventStarted,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "format": format}),
	}); err != nil {
		return nil, "", "", err
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return nil, "", "", err
	}
	if detail.Session.DeletedAt != nil {
		return nil, "", "", errMASessionDeleted
	}
	if err := s.linkTrace(ctx, maTraceLink{SessionID: sessionID, StrategyVersionID: detail.Session.ActiveStrategyID}); err != nil {
		return nil, "", "", err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "xlsx"
	}
	if format != "xlsx" {
		return nil, "", "", fmt.Errorf("%w: export format", errMAStrategyInvalid)
	}
	rows := maExportRows(detail.Targets)
	content, err := buildMAXLSX(rows)
	if err != nil {
		return nil, "", "", err
	}
	if err := s.store.RecordMAExport(ctx, sessionID, "xlsx", len(detail.Targets), email); err != nil {
		return nil, "", "", err
	}
	filename := "target-ma-" + safeFilenamePart(detail.Session.Title) + ".xlsx"
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_export_recorded",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "format": "xlsx", "filename": filename, "row_count": len(detail.Targets)}),
	}); err != nil {
		return nil, "", "", err
	}
	return content, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

// setTargetRating salva il voto preferiti dell'analista su un'azienda della
// sessione (memoria della preferenza + selezione per il deep-dive). Il voto è
// agganciato a company_key, quindi sopravvive al re-execute della sessione.
// Con il voto persiste la ground truth (migrazione 080): motivo dell'esclusione
// e snapshot di ciò che la UI mostrava al momento del giudizio.
func (s *maService) setTargetRating(ctx context.Context, sessionID string, input MATargetRatingRequest, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	input.CompanyKey = normalizeMACompanyKey(input.CompanyKey)
	if input.CompanyKey == "" {
		return fmt.Errorf("%w: company key", errMAStrategyInvalid)
	}
	if !validMARating(input.Rating) {
		return fmt.Errorf("%w: rating", errMAStrategyInvalid)
	}
	input.Reason = cleanText(input.Reason, 300)
	switch input.ConfidenceAtRating {
	case "", "alta", "media", "bassa":
	default:
		input.ConfidenceAtRating = ""
	}
	if input.ScoreAtRating != nil && (*input.ScoreAtRating < 0 || *input.ScoreAtRating > 100) {
		input.ScoreAtRating = nil
	}
	session, err := s.store.GetMASessionState(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := ensureMASessionOperational(session); err != nil {
		return err
	}
	if err := s.store.UpsertMATargetRating(ctx, sessionID, input, subject, email); err != nil {
		return err
	}
	// La prima >=1 stella su una sessione agganciata crea (o riapre) la card di
	// lavorazione (PRD §4.1, §4.3). Errore dell'hook: propagato — il rating non
	// deve riuscire con la card rotta.
	if input.Rating >= 1 && session.InitiativeID != "" {
		if err := s.ensureInitiativeCard(ctx, session.InitiativeID, sessionID, input.CompanyKey, input.Rating, subject, email); err != nil {
			return err
		}
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_target_rated",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "company_key": input.CompanyKey, "rating": input.Rating, "reason": input.Reason}),
	})
	return nil
}

// ensureInitiativeCard applies the PRD §4.1/§4.3 entry rule: create the card
// (state approfondimento) if it does not exist yet, reopen it if it exists
// closed/removed (evento card_riaperta), or do nothing if it exists active
// (autonomy, §4.2 — the star no longer governs an active card). Used both by
// the setTargetRating hook and by the retro-anchor backfill.
func (s *maService) ensureInitiativeCard(ctx context.Context, initiativeID, sessionID, companyKey string, rating int, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	existing, err := s.store.GetMAInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return err
	}
	if existing != nil && existing.State != maCardStateChiusa && existing.State != maCardStateRimossa {
		return nil
	}

	card := MAInitiativeCard{
		InitiativeID:       initiativeID,
		CompanyKey:         companyKey,
		State:              maCardStateApprofondimento,
		CreatedFromSession: sessionID,
	}
	if existing != nil {
		// Riapertura: preserva lo snapshot già registrato, resetta solo stato/esito.
		card.CompanyName = existing.CompanyName
		card.VATCode = existing.VATCode
		card.TaxCode = existing.TaxCode
		card.Province = existing.Province
		card.CreatedFromSession = existing.CreatedFromSession
	} else if rows, err := s.store.ListMATargetRows(ctx, sessionID); err == nil {
		for _, row := range rows {
			if row.CompanyKey == companyKey {
				card.CompanyName = row.CompanyName
				card.VATCode = row.VATCode
				card.Province = row.Province
				break
			}
		}
	}

	if err := s.store.UpsertMAInitiativeCard(ctx, card); err != nil {
		return err
	}

	event := maEventCardCreata
	if existing != nil {
		event = maEventCardRiaperta
	}
	if err := s.store.InsertMATargetOutcome(ctx, MATargetOutcome{
		SessionID:        sessionID,
		InitiativeID:     initiativeID,
		CompanyKey:       companyKey,
		Event:            event,
		Payload:          maTraceJSON(map[string]any{"sessionId": sessionID, "rating": rating}),
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	}); err != nil {
		return err
	}
	return nil
}

// errMACardNotFound signals a missing card at the (initiative, companyKey)
// key: mapped to 404 like sql.ErrNoRows.
var errMACardNotFound = sql.ErrNoRows

// errMACardDossierNotFound signals that a card exists but no target could be
// resolved for it (sessioni sganciate dopo la creazione della card): the
// dossier page shows an empty state instead of the 404 generico di sessione.
var errMACardDossierNotFound = errors.New("ma card dossier target not found")

// MACardDossier is the response of getCardDossier: il target completo
// (vendor payload + deep + web validation) risolto per (iniziativa,
// companyKey), più il riferimento alla sessione di provenienza.
type MACardDossier struct {
	Target       MATarget         `json:"target"`
	SessionID    string           `json:"sessionId"`
	SessionTitle string           `json:"sessionTitle"`
	Card         MAInitiativeCard `json:"card"`
}

// getCardDossier risolve il target più recente per (iniziativa, companyKey)
// fra le sessioni agganciate e lo idrata completamente (PRD §7): la pagina
// dossier-card mostra le tab del vecchio modale TargetPage a partire da
// questo target. L'iniziativa archiviata può comunque leggere; la card deve
// esistere (stesso guard delle altre letture/scritture card).
func (s *maService) getCardDossier(ctx context.Context, initiativeID, companyKey string) (MACardDossier, error) {
	if s.store == nil {
		return MACardDossier{}, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if _, err := s.store.GetMAInitiative(ctx, initiativeID); err != nil {
		return MACardDossier{}, err
	}
	card, err := s.store.GetMAInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return MACardDossier{}, err
	}
	if card == nil {
		return MACardDossier{}, errMACardNotFound
	}
	sessionID, targetID, err := s.store.FindMALatestTargetForCard(ctx, initiativeID, companyKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MACardDossier{}, errMACardDossierNotFound
		}
		return MACardDossier{}, err
	}
	target, err := s.store.GetMATargetByID(ctx, sessionID, targetID)
	if err != nil {
		return MACardDossier{}, err
	}
	session, err := s.store.GetMASessionState(ctx, sessionID)
	if err != nil {
		return MACardDossier{}, err
	}
	return MACardDossier{Target: target, SessionID: sessionID, SessionTitle: session.Title, Card: *card}, nil
}

// getInitiativeBoard assembles the board (B4 passo 1): l'Iniziativa, le
// sessioni agganciate (chips) e le card decorate con stato dossier,
// collisioni, badge registro e provenienze — tutto in query batch, mai N+1.
func (s *maService) getInitiativeBoard(ctx context.Context, initiativeID string) (MAInitiativeBoard, error) {
	if s.store == nil {
		return MAInitiativeBoard{}, errMAStoreUnavailable
	}
	initiative, err := s.store.GetMAInitiative(ctx, initiativeID)
	if err != nil {
		return MAInitiativeBoard{}, err
	}
	sessions, err := s.store.ListMASessionsByInitiative(ctx, initiativeID)
	if err != nil {
		return MAInitiativeBoard{}, err
	}
	cards, err := s.store.ListMAInitiativeCards(ctx, initiativeID)
	if err != nil {
		return MAInitiativeBoard{}, err
	}
	companyKeys := make([]string, 0, len(cards))
	for _, card := range cards {
		companyKeys = append(companyKeys, card.CompanyKey)
	}
	deep, err := s.store.ListMADeepAnalysis(ctx, companyKeys)
	if err != nil {
		return MAInitiativeBoard{}, err
	}
	activeCards, err := s.store.ListMAActiveCardsByCompany(ctx, companyKeys)
	if err != nil {
		return MAInitiativeBoard{}, err
	}
	registryFacts, err := s.store.ListMACompanyFactsActive(ctx, companyKeys)
	if err != nil {
		return MAInitiativeBoard{}, err
	}
	provenances, err := s.store.ListMACardProvenances(ctx, initiativeID, companyKeys)
	if err != nil {
		return MAInitiativeBoard{}, err
	}

	views := make([]MAInitiativeCardView, 0, len(cards))
	for _, card := range cards {
		view := MAInitiativeCardView{
			MAInitiativeCard: card,
			DossierStatus:    maCardDossierStatus(deep[card.CompanyKey]),
			RegistryFacts:    registryFacts[card.CompanyKey],
			Provenances:      provenances[card.CompanyKey],
		}
		for _, other := range activeCards[card.CompanyKey] {
			if other.InitiativeID == initiativeID {
				continue
			}
			view.Collisions = append(view.Collisions, MACardMarker{InitiativeID: other.InitiativeID})
		}
		views = append(views, view)
	}
	// Titoli delle collisioni: un'unica passata sulle iniziative referenziate
	// (di solito poche), evitando N chiamate a GetMAInitiative.
	if titles, err := s.collisionInitiativeTitles(ctx, views); err == nil {
		for i := range views {
			for j := range views[i].Collisions {
				views[i].Collisions[j].InitiativeTitle = titles[views[i].Collisions[j].InitiativeID]
			}
		}
	}

	return MAInitiativeBoard{Initiative: initiative, Sessions: sessions, Cards: views}, nil
}

// collisionInitiativeTitles risolve i titoli delle iniziative citate come
// collisione, deduplicando le chiamate a GetMAInitiative.
func (s *maService) collisionInitiativeTitles(ctx context.Context, views []MAInitiativeCardView) (map[string]string, error) {
	titles := map[string]string{}
	for _, view := range views {
		for _, collision := range view.Collisions {
			if _, ok := titles[collision.InitiativeID]; ok {
				continue
			}
			other, err := s.store.GetMAInitiative(ctx, collision.InitiativeID)
			if err != nil {
				continue
			}
			titles[collision.InitiativeID] = other.Title
		}
	}
	return titles, nil
}

// maCardDossierStatus proietta lo stato del deep-dive (PRD §7) sul ciclo di
// vita del bottone: assente/failed → none, queued/running → working, ready →
// ready.
func maCardDossierStatus(deep MADeepAnalysis) string {
	switch deep.Status {
	case maDeepStatusQueued, maDeepStatusRunning:
		return "working"
	case maDeepStatusReady:
		return "ready"
	default:
		return "none"
	}
}

// getInitiativeCardEvents carica il diario di una card (B4 passo 2): eventi
// ancorati all'iniziativa OR alle sessioni agganciate, così gli esiti storici
// di D2 (contattato/buon_lead/no_go) restano visibili.
func (s *maService) getInitiativeCardEvents(ctx context.Context, initiativeID, companyKey string) ([]MATargetOutcome, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	if companyKey == "" {
		return nil, fmt.Errorf("%w: companyKey", errMAStrategyInvalid)
	}
	if _, err := s.store.GetMAInitiative(ctx, initiativeID); err != nil {
		return nil, err
	}
	sessions, err := s.store.ListMASessionsByInitiative(ctx, initiativeID)
	if err != nil {
		return nil, err
	}
	sessionIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ID)
	}
	return s.store.ListMAInitiativeCardEvents(ctx, initiativeID, sessionIDs, companyKey)
}

// requireOperationalInitiativeCard is the common guard for every card write
// (B4 passo 8): l'iniziativa deve esistere e non essere archiviata, la card
// deve esistere.
func (s *maService) requireOperationalInitiativeCard(ctx context.Context, initiativeID, companyKey string) (MAInitiativeCard, error) {
	if s.store == nil {
		return MAInitiativeCard{}, errMAStoreUnavailable
	}
	initiative, err := s.store.GetMAInitiative(ctx, initiativeID)
	if err != nil {
		return MAInitiativeCard{}, err
	}
	if err := ensureMAInitiativeOperational(initiative); err != nil {
		return MAInitiativeCard{}, err
	}
	card, err := s.store.GetMAInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return MAInitiativeCard{}, err
	}
	if card == nil {
		return MAInitiativeCard{}, errMACardNotFound
	}
	return *card, nil
}

// setCardState applica una transizione libera fra i 5 stati attivi (PRD
// §4.4: nessun vincolo di sequenza). Chiusura e rimozione hanno endpoint
// dedicati perché portano side-effect (esito, correzione stella).
func (s *maService) setCardState(ctx context.Context, initiativeID, companyKey, state string, subject, email string) (MAInitiativeCard, error) {
	companyKey = normalizeMACompanyKey(companyKey)
	if !validMACardState(state) || state == maCardStateChiusa || state == maCardStateRimossa {
		return MAInitiativeCard{}, fmt.Errorf("%w: state", errMAStrategyInvalid)
	}
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return MAInitiativeCard{}, err
	}
	from := card.State
	card.State = state
	if err := s.store.UpsertMAInitiativeCard(ctx, card); err != nil {
		return MAInitiativeCard{}, err
	}
	if err := s.store.InsertMATargetOutcome(ctx, MATargetOutcome{
		InitiativeID:     initiativeID,
		CompanyKey:       companyKey,
		Event:            maEventStato,
		Payload:          maTraceJSON(map[string]any{"from": from, "to": state}),
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	}); err != nil {
		return MAInitiativeCard{}, err
	}
	return card, nil
}

// maCardCloseRegisterableKinds is the closed set of fatti tipizzati che la
// chiusura può proporre (PRD §4.4/§6): mai per non_idonea, mai un fatto fuori
// da questi due.
var maCardCloseRegisterableKinds = map[string]bool{
	"non_vende":            true,
	"in_trattativa_altrui": true,
}

// closeCard chiude la card con un esito (PRD §4.4: attributo della chiusura,
// non colonne separate) e opzionalmente propone il ponte tipizzato verso il
// registro azienda per no_go/rimandata (§6). Un fatto già attivo non fa
// fallire la chiusura (idempotenza, B4 passo 4).
func (s *maService) closeCard(ctx context.Context, initiativeID, companyKey string, input MACardCloseRequest, subject, email string) (MACardCloseResponse, error) {
	companyKey = normalizeMACompanyKey(companyKey)
	if !validMACardEsito(input.Esito) {
		return MACardCloseResponse{}, fmt.Errorf("%w: esito", errMAStrategyInvalid)
	}
	note := cleanText(input.Note, 500)
	var registerKinds []string
	if len(input.RegisterFacts) > 0 {
		if input.Esito != maCardEsitoNoGo && input.Esito != maCardEsitoRimandata {
			return MACardCloseResponse{}, fmt.Errorf("%w: registerFacts non ammesso per questo esito", errMAStrategyInvalid)
		}
		for _, kind := range input.RegisterFacts {
			if !maCardCloseRegisterableKinds[kind] {
				return MACardCloseResponse{}, fmt.Errorf("%w: registerFacts kind", errMAStrategyInvalid)
			}
			registerKinds = append(registerKinds, kind)
		}
	}
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return MACardCloseResponse{}, err
	}
	card.State = maCardStateChiusa
	card.Esito = input.Esito
	now := time.Now()
	card.ClosedAt = &now
	if err := s.store.UpsertMAInitiativeCard(ctx, card); err != nil {
		return MACardCloseResponse{}, err
	}
	if err := s.store.InsertMATargetOutcome(ctx, MATargetOutcome{
		InitiativeID:     initiativeID,
		CompanyKey:       companyKey,
		Event:            maEventChiusura,
		Note:             note,
		Payload:          maTraceJSON(map[string]any{"esito": input.Esito}),
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	}); err != nil {
		return MACardCloseResponse{}, err
	}

	response := MACardCloseResponse{Card: card}
	for _, kind := range registerKinds {
		_, err := s.store.InsertMACompanyFact(ctx, MACompanyFact{
			ID:               uuid.NewString(),
			CompanyKey:       companyKey,
			VATCode:          card.VATCode,
			TaxCode:          card.TaxCode,
			CompanyName:      card.CompanyName,
			Kind:             kind,
			Note:             note,
			CreatedBySubject: subject,
			CreatedByEmail:   email,
		})
		if err != nil {
			if errors.Is(err, errMACompanyFactActive) {
				response.SkippedFacts = append(response.SkippedFacts, kind)
				continue
			}
			return MACardCloseResponse{}, err
		}
		response.RegisteredFacts = append(response.RegisteredFacts, kind)
	}
	return response, nil
}

// removeCard è l'uscita per errore di triage (PRD §4.3): mai un verdetto.
// CorrectRating riusa setTargetRating sulla provenienza ≥1★ più recente; se
// quella sessione non è più operativa, la correzione viene saltata e
// segnalata nella risposta, la rimozione non fallisce.
func (s *maService) removeCard(ctx context.Context, initiativeID, companyKey string, input MACardRemoveRequest, subject, email string) (MACardRemoveResponse, error) {
	companyKey = normalizeMACompanyKey(companyKey)
	reason := cleanText(input.Reason, 300)
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return MACardRemoveResponse{}, err
	}
	card.State = maCardStateRimossa
	if err := s.store.UpsertMAInitiativeCard(ctx, card); err != nil {
		return MACardRemoveResponse{}, err
	}
	if err := s.store.InsertMATargetOutcome(ctx, MATargetOutcome{
		InitiativeID:     initiativeID,
		CompanyKey:       companyKey,
		Event:            maEventCardRimossa,
		Note:             reason,
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	}); err != nil {
		return MACardRemoveResponse{}, err
	}

	response := MACardRemoveResponse{Card: card}
	if input.CorrectRating {
		provenances, err := s.store.ListMACardProvenances(ctx, initiativeID, []string{companyKey})
		if err != nil {
			return MACardRemoveResponse{}, err
		}
		rows := provenances[companyKey]
		if len(rows) == 0 {
			response.RatingCorrectionSkipped = true
		} else {
			latest := rows[0]
			session, err := s.store.GetMASessionState(ctx, latest.SessionID)
			if err != nil {
				return MACardRemoveResponse{}, err
			}
			if err := ensureMASessionOperational(session); err != nil {
				response.RatingCorrectionSkipped = true
			} else {
				if err := s.setTargetRating(ctx, latest.SessionID, MATargetRatingRequest{
					CompanyKey: companyKey,
					Rating:     -1,
					Reason:     reason,
				}, subject, email); err != nil {
					return MACardRemoveResponse{}, err
				}
				response.RatingCorrected = true
			}
		}
	}
	return response, nil
}

// reopenCard riporta a "da contattare" una card chiusa o rimossa (B4 passo
// 6): riapertura manuale, diario continuo (PRD §4.3).
func (s *maService) reopenCard(ctx context.Context, initiativeID, companyKey, subject, email string) (MAInitiativeCard, error) {
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return MAInitiativeCard{}, err
	}
	if card.State != maCardStateChiusa && card.State != maCardStateRimossa {
		return MAInitiativeCard{}, fmt.Errorf("%w: card non chiusa né rimossa", errMAStrategyInvalid)
	}
	card.State = maCardStateApprofondimento
	card.Esito = ""
	card.ClosedAt = nil
	if err := s.store.UpsertMAInitiativeCard(ctx, card); err != nil {
		return MAInitiativeCard{}, err
	}
	if err := s.store.InsertMATargetOutcome(ctx, MATargetOutcome{
		InitiativeID:     initiativeID,
		CompanyKey:       companyKey,
		Event:            maEventCardRiaperta,
		Payload:          maTraceJSON(map[string]any{"manual": true}),
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	}); err != nil {
		return MAInitiativeCard{}, err
	}
	return card, nil
}

// addCardNote appende la nota di diario del composer S4 (B4 passo 7): solo
// il log eventi, mai il registro azienda (PRD §2: generi diversi).
func (s *maService) addCardNote(ctx context.Context, initiativeID, companyKey, body, subject, email string) error {
	companyKey = normalizeMACompanyKey(companyKey)
	body = cleanText(body, 1000)
	if body == "" {
		return fmt.Errorf("%w: body", errMAStrategyInvalid)
	}
	if _, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey); err != nil {
		return err
	}
	return s.store.InsertMATargetOutcome(ctx, MATargetOutcome{
		InitiativeID:     initiativeID,
		CompanyKey:       companyKey,
		Event:            maEventNota,
		Note:             body,
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	})
}

// addTargetOutcome appende un esito reale (contattato / buon lead / no go) al
// log append-only: la ground truth che renderà validabile lo score. Agganciato
// a company_key come il rating.
func (s *maService) addTargetOutcome(ctx context.Context, sessionID string, input MATargetOutcomeRequest, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	companyKey := normalizeMACompanyKey(input.CompanyKey)
	if companyKey == "" {
		return fmt.Errorf("%w: company key", errMAStrategyInvalid)
	}
	event := strings.ToLower(strings.TrimSpace(input.Event))
	switch event {
	case maOutcomeContattato, maOutcomeBuonLead, maOutcomeNoGo:
	default:
		return fmt.Errorf("%w: outcome event", errMAStrategyInvalid)
	}
	session, err := s.store.GetMASessionState(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := ensureMASessionOperational(session); err != nil {
		return err
	}
	if err := s.store.InsertMATargetOutcome(ctx, MATargetOutcome{
		SessionID:        sessionID,
		CompanyKey:       companyKey,
		Event:            event,
		Note:             cleanText(input.Note, 500),
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	}); err != nil {
		return err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_target_outcome",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "company_key": companyKey, "event": event}),
	})
	return nil
}

// rescoreSession è l'override di tesi dell'analista: la tesi muove ~metà del
// budget pesi e gata interi segnali, e l'inferenza LLM può sbagliarla — deve
// essere correggibile a un click. Ri-scora i target advanced della sessione dai
// payload GIÀ persistiti (nessuna chiamata vendor, nessun costo, sincrono: non
// passa dalla coda condivisa), registrando la tesi come nuova versione di
// strategia per l'audit trail. Le righe identity-only restano intatte; rating,
// web-validation e deep sopravvivono perché agganciati a company_key.
func (s *maService) rescoreSession(ctx context.Context, sessionID, thesisRaw, subject, email string, lean bool) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	thesis := normalizeMAThesis(thesisRaw)
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	if detail.Session.Status == maSessionStatusRunning || detail.Session.Status == maSessionStatusEstimating {
		return MASessionDetail{}, fmt.Errorf("%w: session busy", errMAStrategyInvalid)
	}
	if detail.Strategy == nil {
		return MASessionDetail{}, fmt.Errorf("%w: no strategy to rescore", errMAStrategyInvalid)
	}

	strategy := detail.Strategy.Strategy
	strategy.Thesis = thesis
	version, err := s.store.AddMARescoreStrategyVersion(ctx, sessionID, strategy, email)
	if err != nil {
		return MASessionDetail{}, err
	}
	// SectorDivisions è transiente (json:"-"): va ricomputato come nei percorsi
	// estimate/execute, altrimenti il gate settore non gaterebbe nulla.
	strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, nil, false)
	if err != nil {
		return MASessionDetail{}, err
	}

	var toScore, carried []MATarget
	runID := ""
	for _, target := range detail.Targets {
		if target.EnrichmentLevel == maEnrichmentAdvanced && len(target.VendorPayload) > 0 {
			toScore = append(toScore, target)
			if runID == "" {
				runID = target.RunID
			}
		} else {
			carried = append(carried, target)
		}
	}
	if len(toScore) == 0 {
		return MASessionDetail{}, fmt.Errorf("%w: no scored targets to rescore", errMAStrategyInvalid)
	}

	scored := scoreMATargetsV2(toScore, strategy, maScoringParamsFromPricing(s.loadPricing(ctx)), s.now())
	merged := make([]MATarget, 0, len(scored)+len(carried))
	merged = append(merged, scored...)
	merged = append(merged, carried...)
	if err := s.store.ReplaceMATargets(ctx, sessionID, runID, merged); err != nil {
		return MASessionDetail{}, err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_session_rescored",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "thesis": thesis, "strategy_version_id": version.ID, "rescored": len(scored), "requested_by": subject}),
	})
	return s.getSessionShape(ctx, sessionID, lean)
}

func (s *maService) upsertTargetWebValidation(ctx context.Context, sessionID string, body MAWebValidationUpsertRequest, subject, email string) (MAWebValidation, error) {
	if s.store == nil {
		return MAWebValidation{}, errMAStoreUnavailable
	}
	if strings.TrimSpace(body.Target.SessionID) != "" && body.Target.SessionID != sessionID {
		return MAWebValidation{}, fmt.Errorf("%w: session mismatch", errMAStrategyInvalid)
	}
	session, err := s.store.GetMASessionState(ctx, sessionID)
	if err != nil {
		return MAWebValidation{}, err
	}
	if err := ensureMASessionOperational(session); err != nil {
		return MAWebValidation{}, err
	}

	companyKey := normalizeMACompanyKey(body.Target.CompanyKey)
	if companyKey == "" {
		companyKey = normalizeMACompanyKey(maTargetDedupeKey(body.Target))
	}
	if companyKey == "" {
		return MAWebValidation{}, fmt.Errorf("%w: company key", errMAStrategyInvalid)
	}

	if body.FinalDecision.FinalAction == "" || body.FinalDecision.WebValidationState == "" {
		body.FinalDecision = defaultManualMAFinalDecision(body)
	}
	if !validMAFinalAction(body.FinalDecision.FinalAction) {
		return MAWebValidation{}, fmt.Errorf("%w: final action", errMAStrategyInvalid)
	}
	if !validMAWebValidationState(body.FinalDecision.WebValidationState) {
		return MAWebValidation{}, fmt.Errorf("%w: web validation state", errMAStrategyInvalid)
	}
	pipelineVersion := strings.TrimSpace(body.PipelineVersion)
	if pipelineVersion == "" {
		pipelineVersion = maWebValidationPipelineVersion
	}
	keywordSetHash := strings.TrimSpace(body.KeywordSetHash)
	if keywordSetHash == "" {
		keywordSetHash = maWebValidationHash(maWebValidationKeywordSetFingerprint(body.KeywordSet))
	}
	inputHash := strings.TrimSpace(body.InputHash)
	if inputHash == "" {
		inputHash = maWebValidationHash(map[string]any{
			"pipelineVersion": pipelineVersion,
			"target":          maWebValidationTargetFingerprint(body.Target),
			"keywordSet":      maWebValidationKeywordSetFingerprint(body.KeywordSet),
		})
	}
	staleAfter, expiresAt := maWebValidationFreshnessBounds(time.Now().UTC(), body.FinalDecision.FinalAction, body.CandidateMatchError)
	// identity_state is a producer-stamped fact (mig 083): tolerate legacy/unknown
	// callers by recording nothing rather than failing the validation.
	identityState := strings.TrimSpace(body.IdentityState)
	switch identityState {
	case "", maIdentityStateVerified, maIdentityStateVouched, maIdentityStateAssumed:
	default:
		identityState = ""
	}

	summaryRaw, err := json.Marshal(body.Summary)
	if err != nil {
		return MAWebValidation{}, fmt.Errorf("marshal web validation summary: %w", err)
	}
	keywordSetRaw, err := json.Marshal(body.KeywordSet)
	if err != nil {
		return MAWebValidation{}, fmt.Errorf("marshal web validation keyword set: %w", err)
	}
	selectedDomainRaw := json.RawMessage(`{}`)
	selectedDomain := ""
	domainConfidence := ""
	var domainScore *int
	if body.SelectedDomain != nil {
		raw, err := json.Marshal(body.SelectedDomain)
		if err != nil {
			return MAWebValidation{}, fmt.Errorf("marshal web validation selected domain: %w", err)
		}
		selectedDomainRaw = raw
		selectedDomain = body.SelectedDomain.Domain
		domainConfidence = body.SelectedDomain.Confidence
		domainScore = &body.SelectedDomain.Score
	}
	domainResponseRaw, err := json.Marshal(body.DomainResponse)
	if err != nil {
		return MAWebValidation{}, fmt.Errorf("marshal web validation domain response: %w", err)
	}
	evidenceRunsRaw, err := json.Marshal(body.EvidenceRuns)
	if err != nil {
		return MAWebValidation{}, fmt.Errorf("marshal web validation evidence runs: %w", err)
	}
	analysisRaw := json.RawMessage(`{}`)
	if body.CandidateMatchAnalysis != nil {
		raw, err := json.Marshal(body.CandidateMatchAnalysis)
		if err != nil {
			return MAWebValidation{}, fmt.Errorf("marshal web validation analyst: %w", err)
		}
		analysisRaw = raw
	}
	finalDecisionRaw, err := json.Marshal(body.FinalDecision)
	if err != nil {
		return MAWebValidation{}, fmt.Errorf("marshal web validation final decision: %w", err)
	}

	analystVerdict := body.FinalDecision.AnalystVerdict
	analystAction := body.FinalDecision.AnalystAction
	analystConfidence := ""
	llmModelID := ""
	llmPromptID := ""
	llmModel := ""
	if body.CandidateMatchAnalysis != nil {
		if analystVerdict == "" {
			analystVerdict = body.CandidateMatchAnalysis.Verdict
		}
		if analystAction == "" {
			analystAction = body.CandidateMatchAnalysis.RecommendedAction
		}
		analystConfidence = body.CandidateMatchAnalysis.Confidence
		llmModelID = body.CandidateMatchAnalysis.ModelID
		llmPromptID = body.CandidateMatchAnalysis.PromptID
		llmModel = body.CandidateMatchAnalysis.Model
	}
	validation, err := s.store.UpsertMAWebValidation(ctx, maWebValidationUpsert{
		SessionID:              sessionID,
		CompanyKey:             companyKey,
		TargetID:               body.Target.ID,
		RunID:                  body.Target.RunID,
		PipelineVersion:        pipelineVersion,
		InputHash:              inputHash,
		KeywordSetHash:         keywordSetHash,
		LLMModelID:             llmModelID,
		LLMPromptID:            llmPromptID,
		LLMModel:               llmModel,
		StaleAfter:             staleAfter,
		ExpiresAt:              expiresAt,
		SelectedDomain:         selectedDomain,
		DomainConfidence:       domainConfidence,
		DomainScore:            domainScore,
		IdentityState:          identityState,
		WebScore:               body.FinalDecision.WebScore,
		WebConfidence:          body.FinalDecision.Confidence,
		WebValidationState:     body.FinalDecision.WebValidationState,
		FinalAction:            body.FinalDecision.FinalAction,
		AnalystVerdict:         analystVerdict,
		AnalystAction:          analystAction,
		AnalystConfidence:      analystConfidence,
		Summary:                summaryRaw,
		KeywordSet:             keywordSetRaw,
		SelectedDomainPayload:  selectedDomainRaw,
		DomainResponse:         domainResponseRaw,
		EvidenceRuns:           evidenceRunsRaw,
		CandidateMatchAnalysis: analysisRaw,
		CandidateMatchError:    body.CandidateMatchError,
		FinalDecision:          finalDecisionRaw,
		Subject:                subject,
		Email:                  email,
	})
	if err != nil {
		return MAWebValidation{}, err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_target_web_validation_upserted",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"session_id":    sessionID,
			"company_key":   companyKey,
			"final_action":  body.FinalDecision.FinalAction,
			"fresh_until":   staleAfter,
			"expires_at":    expiresAt,
			"input_hash":    inputHash,
			"web_score":     body.FinalDecision.WebScore,
			"selected_site": selectedDomain,
		}),
	})
	return validation, nil
}

// defaultManualMAFinalDecision fills a FinalDecision for a manual web-validation
// upsert (analyst-submitted from the UI) that omits one. It is a plain default — no
// magic thresholds (those were demolished with the concept rework): unresolved
// domain needs a domain review, otherwise it lands as a business-validation review.
func defaultManualMAFinalDecision(body MAWebValidationUpsertRequest) CandidateMatchFinalDecision {
	decision := CandidateMatchFinalDecision{
		InitialMatchState:  body.Target.MatchState,
		DeterministicScore: body.Target.Score,
		WebScore:           body.Summary.Score,
		Confidence:         "bassa",
	}
	if body.SelectedDomain == nil || strings.TrimSpace(body.SelectedDomain.Domain) == "" {
		decision.WebValidationState = "domain_unresolved"
		decision.FinalAction = "needs_domain_review"
		decision.Reason = "Nessun dominio ufficiale credibile risolto."
	} else {
		decision.WebValidationState = "unclear"
		decision.FinalAction = "needs_business_validation"
		decision.Reason = "Validazione manuale richiesta."
	}
	if body.CandidateMatchAnalysis != nil {
		decision.AnalystVerdict = body.CandidateMatchAnalysis.Verdict
		decision.AnalystAction = body.CandidateMatchAnalysis.RecommendedAction
	}
	return decision
}

func validMAFinalAction(action string) bool {
	switch action {
	case "confirm", "deprioritize", "reject", "needs_domain_review", "needs_business_validation":
		return true
	default:
		return false
	}
}

func validMAWebValidationState(state string) bool {
	switch state {
	case "confirmed", "deprioritized", "domain_unresolved", "analysis_unavailable", "rejected", "unclear":
		return true
	default:
		return false
	}
}

func maWebValidationFreshnessBounds(now time.Time, finalAction, analysisError string) (time.Time, time.Time) {
	staleAfter := now.Add(90 * 24 * time.Hour)
	if finalAction == "needs_domain_review" || finalAction == "needs_business_validation" || strings.TrimSpace(analysisError) != "" {
		staleAfter = now.Add(30 * 24 * time.Hour)
	}
	expiresAt := now.Add(180 * 24 * time.Hour)
	if staleAfter.After(expiresAt) {
		staleAfter = expiresAt
	}
	return staleAfter, expiresAt
}

func maWebValidationFreshness(now, staleAfter, expiresAt time.Time) string {
	if !expiresAt.IsZero() && now.After(expiresAt) {
		return maWebValidationExpired
	}
	if !staleAfter.IsZero() && now.After(staleAfter) {
		return maWebValidationStale
	}
	return maWebValidationFresh
}

func maWebValidationHash(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	hash := fnv.New32a()
	_, _ = hash.Write(raw)
	return fmt.Sprintf("fnv1a:%08x", hash.Sum32())
}

func maWebValidationTargetFingerprint(target MATarget) map[string]any {
	var staffCost *int
	if value := candidateMatchLatestStaffCost(target); value != nil {
		copyValue := *value
		staffCost = &copyValue
	}
	return map[string]any{
		"activityStatus":   target.ActivityStatus,
		"adjustments":      target.Adjustments,
		"atecoCode":        target.AtecoCode,
		"atecoDescription": target.AtecoDescription,
		"companyKey":       target.CompanyKey,
		"companyName":      target.CompanyName,
		"confidence":       target.Confidence,
		"employees":        target.Employees,
		"evidence":         target.Evidence,
		"flags":            target.Flags,
		"latestStaffCost":  staffCost,
		"matchState":       target.MatchState,
		"missingCriteria":  target.MissingCriteria,
		"province":         target.Province,
		"rating":           target.Rating,
		"score":            target.Score,
		"taxCode":          target.TaxCode,
		"town":             target.Town,
		"turnover":         target.Turnover,
		"turnoverYear":     target.TurnoverYear,
		"vatCode":          target.VATCode,
	}
}

func maWebValidationKeywordSetFingerprint(keywordSet CandidateMatchKeywordSet) map[string]any {
	return map[string]any{
		"adjacentTerms": keywordSet.AdjacentTerms,
		"coreTerms":     keywordSet.CoreTerms,
		"intentLabel":   keywordSet.IntentLabel,
		"negativeTerms": keywordSet.NegativeTerms,
		"sources":       keywordSet.Sources,
	}
}

func validMARating(rating int) bool {
	return rating == 0 || rating == maRatingExcluded || (rating >= 1 && rating <= maRatingMax)
}

func normalizeMACompanyKey(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func (s *maService) listParameters(ctx context.Context) ([]MAParameter, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	return s.store.ListMAParameters(ctx)
}

// updateParameter validates and persists one configurable business parameter.
// Only seeded keys are editable (the UPDATE matches an existing row). Numeric
// parameters must be non-negative; ma_strategy_pipeline is an enum kill-switch.
// Each change is audited via a trace event.
func (s *maService) updateParameter(ctx context.Context, key, value, subject, email string) error {
	if s.store == nil {
		return errMAStoreUnavailable
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: parameter key", errMAStrategyInvalid)
	}
	value = strings.TrimSpace(value)
	switch key {
	case maStrategyPipelineParameter:
		if value != maStrategyPipelineV2 && value != maStrategyPipelineMonolith {
			return fmt.Errorf("%w: parameter value ma_strategy_pipeline must be v2 or monolith", errMAStrategyInvalid)
		}
	case maAtecoRetrievalMethodParameter:
		if value != maAtecoRetrievalMethodEmbedding && value != maAtecoRetrievalMethodLLM {
			return fmt.Errorf("%w: parameter value ateco_retrieval_method must be embedding or llm", errMAStrategyInvalid)
		}
	default:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return fmt.Errorf("%w: parameter value", errMAStrategyInvalid)
		}
	}
	if err := s.store.UpdateMAParameter(ctx, key, value, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown parameter", errMAStrategyInvalid)
		}
		return err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_parameter_updated",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"key": key, "value": value, "by": email, "subject": subject}),
	})
	return nil
}

// deepDive enqueues the rated (>=1 star) companies of a session for IT-full deep
// analysis. Companies already analysed (status=ready, cached globally) are skipped
// and not charged. The projected incremental spend (chargeable x cost_full) gates
// the batch unless the analyst acknowledges going over budget. The async worker
// picks up the queued rows; the returned detail reflects the new statuses.
// regenerateMADeepBriefs re-runs the LLM brief for every cached analysis from its stored
// payload + valuation (scorecard rebuilt deterministically). No IT-full call — used to
// roll out a new brief prompt to already-analyzed companies. Per-row LLM failures are
// skipped (best-effort), so one bad row never aborts the batch.
func (s *maService) regenerateMADeepBriefs(ctx context.Context) (int, error) {
	if s.store == nil {
		return 0, errMAStoreUnavailable
	}
	if s.llmp == nil {
		return 0, errMAOpenRouterUnavailable
	}
	model, err := s.llmp.ResolveModel(ctx, maModelScopeDeepBrief, "")
	if err != nil {
		return 0, err
	}
	prompt, err := s.llmp.ResolvePrompt(ctx, maModelScopeDeepBrief, "")
	if err != nil {
		return 0, err
	}
	rows, err := s.store.ListMADeepReadyForBrief(ctx)
	if err != nil {
		return 0, err
	}
	regenerated := 0
	for _, row := range rows {
		scorecard := buildMADeepScorecard(row.Payload)
		if scorecard == nil {
			continue
		}
		brief, err := buildMADeepBriefLLM(ctx, s.llmp, model, prompt, row.Payload, scorecard, row.Valuation)
		if err != nil {
			logging.FromContext(ctx).Warn("binocolo brief regenerate failed", "component", "binocolo", "operation", "ma_deep_regenerate_briefs", "company_key", row.CompanyKey, "error", err)
			continue
		}
		if err := s.store.UpdateMADeepBrief(ctx, row.CompanyKey, brief, model.ID, prompt.ID); err != nil {
			return regenerated, err
		}
		regenerated++
	}
	return regenerated, nil
}

// recomputeMADeepScorecards rebuilds the deterministic scorecard (+ quality flags,
// Fase 2) for every cached deep analysis straight from its stored IT-full payload —
// no vendor call, no LLM, no charge. It rolls out an engine calibration to
// already-analyzed companies (the funnel and the dossier share this global cache).
// With withValuation the valuation is rebuilt too (bridge + banda asimmetrica: dal
// redesign Fase 2 la semantica della valuation cambia e il refresh è voluto). The
// brief is regenerated separately (it needs the LLM).
func (s *maService) recomputeMADeepScorecards(ctx context.Context, withValuation bool) (int, error) {
	if s.store == nil {
		return 0, errMAStoreUnavailable
	}
	rows, err := s.store.ListMADeepReadyPayloads(ctx)
	if err != nil {
		return 0, err
	}
	pricing := s.loadPricing(ctx)
	recomputed := 0
	for _, row := range rows {
		scorecard := buildMADeepScorecard(row.Payload)
		if scorecard == nil {
			continue
		}
		reading := maCEEReadingFromPayload(row.Payload)
		scorecard.QualityFlags = buildMADeepQualityFlags(scorecard, reading, pricing)
		if err := s.store.UpdateMADeepScorecard(ctx, row.CompanyKey, scorecard); err != nil {
			return recomputed, err
		}
		if withValuation {
			multiple, err := s.store.ResolveSectorMultiple(ctx, scorecard.AtecoCode)
			if err != nil {
				logging.FromContext(ctx).Warn("binocolo recompute sector multiple failed", "component", "binocolo", "company_key", row.CompanyKey, "error", err)
			} else if valuation := buildMADeepValuation(scorecard, reading, multiple, pricing); valuation != nil {
				if err := s.store.UpdateMADeepValuation(ctx, row.CompanyKey, valuation); err != nil {
					return recomputed, err
				}
			}
		}
		recomputed++
	}
	return recomputed, nil
}

// inspectMADeep computes the Fase 0 read-only diagnostics over the cached deep
// payloads (DEEP-DIVE-DECISIONS.md §1/§8): code-family mix, debt granularity,
// coverage of the still-unused vendor deltas and L2Y absolutes, provisions usage,
// and the vendor-vs-CEE reconciliation distribution that will calibrate
// vendor_cee_tolerance_pct. No vendor call, nothing persisted.
func (s *maService) inspectMADeep(ctx context.Context) (MADeepInspectReport, error) {
	if s.store == nil {
		return MADeepInspectReport{}, errMAStoreUnavailable
	}
	report := MADeepInspectReport{Coverage: map[string]int{}}
	counts, err := s.store.CountMADeepByStatus(ctx)
	if err != nil {
		return MADeepInspectReport{}, err
	}
	report.StatusCounts = counts
	// Vintage stats are best-effort: before migration 091 the table does not exist
	// and the whole diagnostic must still work.
	if vintageRows, vintageCompanies, err := s.store.CountMADeepVintage(ctx); err != nil {
		report.Vintage.Error = err.Error()
	} else {
		report.Vintage.Rows = vintageRows
		report.Vintage.Companies = vintageCompanies
	}
	rows, err := s.store.ListMADeepReadyPayloads(ctx)
	if err != nil {
		return MADeepInspectReport{}, err
	}
	var ebitdaDevs, pfnDevs []maInspectDeviation
	for _, row := range rows {
		object, err := decodeVendorObject(row.Payload)
		if err != nil || object == nil {
			continue
		}
		root := deepFullRoot(object)
		rawCodes := maInspectCodeValues(root)
		ceeCodes := maCEECodesFromRoot(root)
		reading := maCEEReadingFromRoot(root)
		report.PayloadsAnalyzed++

		// Famiglie di codici (sui codici GREZZI). IPL231/IPL232/IICC351 sono refusi
		// della legend del vendor emessi verbatim dall'API: non contano come
		// divisione PL.
		hasIIC, hasOtherIPL, hasTypo := false, false, false
		for code := range rawCodes {
			switch {
			case code == "IPL231" || code == "IPL232" || code == "IICC351":
				hasTypo = true
			case strings.HasPrefix(code, "IPL"):
				hasOtherIPL = true
			case strings.HasPrefix(code, "IIC"):
				hasIIC = true
			}
		}
		if hasOtherIPL {
			report.CodeFamilies.WithIPL++
		} else if hasIIC {
			report.CodeFamilies.IICOnly++
		}
		if hasTypo {
			report.CodeFamilies.KnownLegendTypos++
		}

		// Granularità debiti: dettaglio con split entro/oltre vs soli totali per voce.
		splitPresent, totalPresent := false, false
		for _, num := range maInspectDebtSplitCodes {
			if _, ok := ceeCodes[num]; ok {
				splitPresent = true
				break
			}
		}
		for _, num := range maInspectDebtTotalCodes {
			if _, ok := ceeCodes[num]; ok {
				totalPresent = true
				break
			}
		}
		switch {
		case splitPresent:
			report.Granularity.DebtDetail++
		case totalPresent:
			report.Granularity.TotalsOnly++
		default:
			report.Granularity.NoDebts++
		}

		// Copertura dei campi vendor oggi inutilizzati (delta YoY + assoluti L2Y).
		for label, path := range maInspectCoveragePaths {
			if _, ok := maInspectNumber(root, path); ok {
				report.Coverage[label]++
			}
		}

		// Accantonamenti B.12/B.13: dove sono valorizzati, discriminano la
		// definizione EBITDA del vendor (aperta: zero su tutta la cache a n=10).
		if ceeCodes["146"] != 0 || ceeCodes["147"] != 0 {
			report.ProvisionsB12B13++
		}

		// Riconciliazioni vendor-vs-CEE: stessa lettura del motore (ma_deep_cee.go,
		// fonte unica), qui in forma distribuzionale. La PFN entra SOLO quando la
		// rilettura è davvero CEE (il fallback vendor_ratio coinciderebbe sempre).
		if reading != nil {
			vendorEBITDA, okVendorEBITDA := maInspectNumber(root, "operatingResults.ebitda")
			if okVendorEBITDA && vendorEBITDA != 0 && reading.EBITDACEE != nil {
				pct := (*reading.EBITDACEE - vendorEBITDA) / math.Abs(vendorEBITDA) * 100
				ebitdaDevs = append(ebitdaDevs, maInspectDeviation{companyKey: row.CompanyKey, vendor: vendorEBITDA, cee: *reading.EBITDACEE, pct: pct})
			}
			if reading.PFN != nil && reading.PFN.Provenance != maCEEProvVendorRatio && okVendorEBITDA {
				if ratio, ok := maInspectNumber(root, "leverageRatios.pfnEbitda"); ok {
					vendorPFN := ratio * vendorEBITDA
					denom := math.Max(math.Abs(vendorPFN), 1000)
					pct := (reading.PFN.Value - vendorPFN) / denom * 100
					pfnDevs = append(pfnDevs, maInspectDeviation{companyKey: row.CompanyKey, vendor: vendorPFN, cee: reading.PFN.Value, pct: pct})
				}
			}
		}
	}
	report.Reconciliation.EBITDA = maInspectSummarize(ebitdaDevs)
	report.Reconciliation.PFN = maInspectSummarize(pfnDevs)
	return report, nil
}

// Voci D con split entro/oltre (codici base+gemello) e totali per voce: la presenza
// dell'uno o dell'altro classifica la granularità del deposito.
var maInspectDebtSplitCodes = []string{
	"090", "091", "092", "093", "184", "185", "094", "095", "096", "097",
	"098", "099", "100", "101", "102", "103", "104", "105", "106", "107",
	"108", "109", "220", "221", "110", "111", "112", "113", "114", "115",
}

var maInspectDebtTotalCodes = []string{
	"329", "330", "331", "332", "333", "334", "335", "336", "337", "338",
	"339", "340", "341", "342", "343",
}

var maInspectCoveragePaths = map[string]string{
	"grossFinancialDebt": "development.grossFinancialDebt",
	"totalAssets":        "development.totalAssets",
	"addedValue":         "development.addedValue",
	"employeeTrend":      "employees.employeeTrend",
	"ebitdaL2Y":          "operatingResults.ebitdaL2Y",
	"ebitL2Y":            "operatingResults.ebitL2Y",
	"cashFlowL2Y":        "operatingResults.cashFlowL2Y",
}

type maInspectDeviation struct {
	companyKey string
	vendor     float64
	cee        float64
	pct        float64
}

// maInspectCodeValues flattens every {code,value} array of the payload (debts,
// credits, productionCosts, ...) into a single code→value map.
func maInspectCodeValues(root map[string]any) map[string]float64 {
	out := map[string]float64{}
	for _, value := range root {
		list, ok := value.([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			code, _ := entry["code"].(string)
			if code == "" {
				continue
			}
			if n, ok := vendorNumber(entry["value"]); ok {
				out[code] = n
			}
		}
	}
	return out
}

func maInspectNumber(root map[string]any, path string) (float64, bool) {
	value, ok := vendorPath(root, path)
	if !ok {
		return 0, false
	}
	return vendorNumber(value)
}

// maInspectSummarize condenses the deviations: max/median of |pct|, count over 1%,
// and the 10 worst offenders (the rows to eyeball before fixing the tolerance).
func maInspectSummarize(devs []maInspectDeviation) MADeepInspectDeviation {
	out := MADeepInspectDeviation{Computable: len(devs)}
	if len(devs) == 0 {
		return out
	}
	sort.Slice(devs, func(i, j int) bool { return math.Abs(devs[i].pct) > math.Abs(devs[j].pct) })
	round2 := func(v float64) float64 { return math.Round(v*100) / 100 }
	out.MaxAbsPct = round2(math.Abs(devs[0].pct))
	out.P50AbsPct = round2(math.Abs(devs[len(devs)/2].pct))
	for _, dev := range devs {
		if math.Abs(dev.pct) > 1 {
			out.Over1Pct++
		}
	}
	limit := 10
	if len(devs) < limit {
		limit = len(devs)
	}
	for _, dev := range devs[:limit] {
		out.Worst = append(out.Worst, MADeepInspectOffender{
			CompanyKey:   dev.companyKey,
			VendorValue:  math.Round(dev.vendor),
			CEEValue:     math.Round(dev.cee),
			DeviationPct: round2(dev.pct),
		})
	}
	return out
}

// companyDossier is the standalone P.IVA lookup (POST). Cache-first by vat_code: a
// ready or in-flight analysis is served as-is (no charge, no enqueue). A miss or a
// previously failed row needs an explicit cost acknowledgement before a fresh IT-full
// call (~€0.30) is queued; without it the caller gets status "cost_required".
func (s *maService) companyDossier(ctx context.Context, vat string, ack bool, email string) (MACompanyDossier, error) {
	if s.store == nil {
		return MACompanyDossier{}, errMAStoreUnavailable
	}
	rec, err := s.store.GetMADeepByVAT(ctx, vat)
	if err != nil {
		return MACompanyDossier{}, err
	}
	if rec != nil && (rec.Status == maDeepStatusReady || rec.Status == maDeepStatusQueued || rec.Status == maDeepStatusRunning) {
		return mapMACompanyDossier(vat, rec), nil
	}
	if !ack {
		dossier := mapMACompanyDossier(vat, rec)
		dossier.Status = "cost_required"
		dossier.CostEUR = s.loadPricing(ctx).CostFull
		return dossier, nil
	}
	if err := s.store.EnqueueMADeepAnalysis(ctx, vat, vat, "", email); err != nil {
		return MACompanyDossier{}, err
	}
	return MACompanyDossier{VATCode: vat, Status: maDeepStatusQueued}, nil
}

// getCompanyDossier returns the current cached state for a P.IVA without ever
// triggering a paid lookup; the frontend polls it until ready/failed.
func (s *maService) getCompanyDossier(ctx context.Context, vat string) (MACompanyDossier, error) {
	if s.store == nil {
		return MACompanyDossier{}, errMAStoreUnavailable
	}
	rec, err := s.store.GetMADeepByVAT(ctx, vat)
	if err != nil {
		return MACompanyDossier{}, err
	}
	return mapMACompanyDossier(vat, rec), nil
}

func mapMACompanyDossier(vat string, rec *maDeepVATRecord) MACompanyDossier {
	if rec == nil {
		return MACompanyDossier{VATCode: vat, Status: "absent"}
	}
	dossier := MACompanyDossier{
		VATCode:   vat,
		Status:    rec.Status,
		Scorecard: rec.Scorecard,
		Valuation: rec.Valuation,
		Brief:     rec.Brief,
		Raw:       rec.Payload,
		CostEUR:   rec.CostEUR,
		ErrorCode: rec.ErrorCode,
	}
	if !rec.UpdatedAt.IsZero() {
		ts := rec.UpdatedAt
		dossier.UpdatedAt = &ts
	}
	return dossier
}

func (s *maService) deepDive(ctx context.Context, sessionID string, ack bool, email string, lean bool) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MASessionDetail{}, errMAOpenAPIITUnavailable
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	type candidate struct{ key, vat, tax string }
	candidates := make([]candidate, 0)
	keys := make([]string, 0)
	for _, target := range detail.Targets {
		if target.Rating != nil && *target.Rating >= 1 && target.CompanyKey != "" {
			candidates = append(candidates, candidate{key: target.CompanyKey, vat: target.VATCode, tax: target.TaxCode})
			keys = append(keys, target.CompanyKey)
		}
	}
	if len(candidates) == 0 {
		return MASessionDetail{}, fmt.Errorf("%w: nessun preferito da approfondire", errMAStrategyInvalid)
	}
	existing, err := s.store.ListMADeepAnalysis(ctx, keys)
	if err != nil {
		return MASessionDetail{}, err
	}
	pricing := s.loadPricing(ctx)
	budget := pricing.BudgetDefault
	if detail.Strategy != nil {
		budget = maStrategyBudget(detail.Strategy.Strategy, pricing.BudgetDefault)
	}
	chargeable := 0
	for _, c := range candidates {
		if record, ok := existing[c.key]; ok && record.Status != maDeepStatusFailed {
			// ready/queued/running are not (re)charged nor (re)enqueued.
			continue
		}
		chargeable++
	}
	projected := float64(chargeable) * pricing.CostFull
	if projected > budget && !ack {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_deep_dive_over_budget",
			Status:    maTraceEventInfo,
			Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "chargeable": chargeable, "projected_cost": projected, "budget": budget}),
		})
		return MASessionDetail{}, errMAEstimateOverBudget
	}
	enqueued := 0
	for _, c := range candidates {
		if record, ok := existing[c.key]; ok && record.Status != maDeepStatusFailed {
			// ready/queued/running are not (re)charged nor (re)enqueued.
			continue
		}
		if err := s.store.EnqueueMADeepAnalysis(ctx, c.key, c.vat, c.tax, email); err != nil {
			return MASessionDetail{}, err
		}
		enqueued++
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_deep_dive_started",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"session_id": sessionID, "candidates": len(candidates), "enqueued": enqueued, "projected_cost": projected}),
	})
	return s.getSessionShape(ctx, sessionID, lean)
}

// deepDiveCard avvia l'analisi completa per-azienda della card (B6, PRD §7):
// variante per-azienda del deep-dive di sessione (deepDive sopra), stesso
// gate di spesa e stesso worker/cache, con count=1. No-op se già ready o già
// in coda/running (il bottone in UI resta coerente senza doppia spesa).
func (s *maService) deepDiveCard(ctx context.Context, initiativeID, companyKey string, ack bool, email string) (MACardDeepDiveResponse, error) {
	if s.store == nil {
		return MACardDeepDiveResponse{}, errMAStoreUnavailable
	}
	if s.openapiit == nil {
		return MACardDeepDiveResponse{}, errMAOpenAPIITUnavailable
	}
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return MACardDeepDiveResponse{}, err
	}
	existing, err := s.store.ListMADeepAnalysis(ctx, []string{card.CompanyKey})
	if err != nil {
		return MACardDeepDiveResponse{}, err
	}
	if record, ok := existing[card.CompanyKey]; ok {
		switch record.Status {
		case maDeepStatusReady, maDeepStatusQueued, maDeepStatusRunning:
			// Già pronto o già in coda: no-op, si riflette lo stato esistente.
			return MACardDeepDiveResponse{DossierStatus: maCardDossierStatus(record)}, nil
		}
	}
	pricing := s.loadPricing(ctx)
	projected := pricing.CostFull
	if projected > pricing.BudgetDefault && !ack {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_card_deep_dive_over_budget",
			Status:    maTraceEventInfo,
			Metadata:  maTraceJSON(map[string]any{"initiative_id": initiativeID, "company_key": card.CompanyKey, "projected_cost": projected, "budget": pricing.BudgetDefault}),
		})
		return MACardDeepDiveResponse{}, errMAEstimateOverBudget
	}
	if err := s.store.EnqueueMADeepAnalysis(ctx, card.CompanyKey, card.VATCode, card.TaxCode, email); err != nil {
		return MACardDeepDiveResponse{}, err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_card_deep_dive_started",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"initiative_id": initiativeID, "company_key": card.CompanyKey, "projected_cost": projected}),
	})
	return MACardDeepDiveResponse{DossierStatus: "working"}, nil
}

func (s *maService) extractIntent(ctx context.Context, prompt, subject, email string) (MAIntent, llm.CallAudit, error) {
	if s.llmp == nil {
		return MAIntent{}, llm.CallAudit{}, errMAOpenRouterUnavailable
	}
	modelConfig, err := s.llmp.ResolveModel(ctx, maModelScopeStrategyIntent, "")
	if err != nil {
		return MAIntent{}, llm.CallAudit{}, llmConfigError(err)
	}
	promptConfig, err := s.llmp.ResolvePrompt(ctx, maModelScopeStrategyIntent, "")
	if err != nil {
		return MAIntent{}, llm.CallAudit{}, llmConfigError(err)
	}
	client, err := s.llmp.ClientForModel(ctx, modelConfig)
	if err != nil {
		return MAIntent{}, llm.CallAudit{}, llmConfigError(err)
	}
	reqParams := modelConfig.RawParams()
	if _, ok := reqParams["max_tokens"]; !ok {
		reqParams["max_tokens"] = 2400
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_intent_extract_config",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"model_id":     modelConfig.ID,
			"model_scope":  modelConfig.Scope,
			"model":        modelConfig.Model,
			"prompt_id":    promptConfig.ID,
			"prompt_scope": promptConfig.Scope,
			"prompt_name":  promptConfig.Name,
			"prompt":       promptConfig.Prompt,
		}),
	}); err != nil {
		return MAIntent{}, llm.CallAudit{}, err
	}
	req := llm.ChatRequest{
		Model:          modelConfig.Model,
		Params:         reqParams,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
		Messages: []llm.Message{
			{Role: "system", Content: promptConfig.Prompt},
			{Role: "user", Content: prompt},
		},
		Tools: nil,
	}
	requestBody, err := llm.BuildRequestBody(req)
	if err != nil {
		return MAIntent{}, llm.CallAudit{}, err
	}
	requestRaw, _ := json.Marshal(requestBody)

	start := time.Now()
	response, err := client.Chat(ctx, req)
	duration := maTraceDuration(start)
	usageRaw, _ := json.Marshal(response.Usage)
	audit := llm.CallAudit{
		App:          maApp,
		Scope:        maModelScopeStrategyIntent,
		ProviderID:   modelConfig.ProviderID,
		ModelID:      modelConfig.ID,
		PromptID:     promptConfig.ID,
		Model:        modelConfig.Model,
		Request:      requestRaw,
		Usage:        usageRaw,
		ActorSubject: subject,
		ActorEmail:   email,
		DurationMS:   duration,
	}
	if err != nil {
		audit.Status = "failed"
		audit.ErrorMessage = err.Error()
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "ma_intent_extract_chat",
			ExternalSystem: "openrouter",
			Status:         maTraceEventFailed,
			DurationMS:     duration,
			Request:        requestRaw,
			Error:          err.Error(),
		})
		return MAIntent{}, audit, err
	}
	responseRaw := json.RawMessage([]byte(strings.TrimSpace(response.Content)))
	audit.Response = responseRaw
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType:      "ma_intent_extract_chat",
		ExternalSystem: "openrouter",
		Status:         maTraceEventSucceeded,
		DurationMS:     duration,
		Request:        requestRaw,
		Response:       maTraceJSON(response),
		Metadata:       maTraceJSON(map[string]any{"response_id": response.ID, "response_model": response.Model}),
	}); err != nil {
		return MAIntent{}, audit, err
	}

	intent, err := decodeMAIntentResponse(responseRaw, prompt)
	if err != nil {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_intent_extract_decode",
			Status:    maTraceEventFailed,
			Response:  maTraceJSON(responseRaw),
			Error:     err.Error(),
		})
		return MAIntent{}, audit, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_intent_extract_decode",
		Status:    maTraceEventSucceeded,
		Response:  maTraceJSON(intent),
	}); err != nil {
		return MAIntent{}, audit, err
	}
	return intent, audit, nil
}

func decodeMAIntentResponse(responseRaw json.RawMessage, promptText string) (MAIntent, error) {
	if len(strings.TrimSpace(string(responseRaw))) == 0 {
		return MAIntent{}, fmt.Errorf("%w: empty intent response", errMAStrategyInvalid)
	}
	var envelope struct {
		Intent *MAIntent `json:"intent"`
	}
	if err := json.Unmarshal(responseRaw, &envelope); err == nil && envelope.Intent != nil {
		return validateIntentSpans(*envelope.Intent, promptText), nil
	}
	var intent MAIntent
	if err := json.Unmarshal(responseRaw, &intent); err != nil {
		return MAIntent{}, fmt.Errorf("decode ma intent: %w", err)
	}
	return validateIntentSpans(intent, promptText), nil
}

type maAtecoRerankSector struct {
	Text       string `json:"text"`
	SourceText string `json:"sourceText,omitempty"`
}

type maAtecoHierarchyCode struct {
	Code         string `json:"code"`
	SearchCode   string `json:"searchCode,omitempty"`
	Title        string `json:"title"`
	Hierarchy    int    `json:"hierarchy,omitempty"`
	ChildCount   int    `json:"childCount,omitempty"`
	SubtreeCount int    `json:"subtreeCount,omitempty"`
}

type maAtecoHierarchyInput struct {
	Prompt         string                 `json:"prompt"`
	Intent         MAIntent               `json:"intent"`
	IncludeSectors []maAtecoRerankSector  `json:"includeSectors"`
	ExcludeSectors []maAtecoRerankSector  `json:"excludeSectors"`
	Divisions      []maAtecoHierarchyCode `json:"divisions"`
}

type maAtecoRerankOutput struct {
	Selected         []maAtecoRerankSelected         `json:"selected"`
	ExcludedPrefixes []maAtecoRerankExcludedPrefix   `json:"excludedPrefixes"`
	MissingCriteria  []maAtecoRerankMissingCriterion `json:"missingCriteria"`
}

type maAtecoRerankSelected struct {
	Code   string `json:"code"`
	Fit    string `json:"fit"`
	Reason string `json:"reason"`
}

type maAtecoRerankExcludedPrefix struct {
	Prefix string `json:"prefix"`
	Code   string `json:"code,omitempty"`
	Reason string `json:"reason"`
}

type maAtecoRerankMissingCriterion struct {
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

func (s *maService) resolveIntent(ctx context.Context, promptText string, intent MAIntent, subject, email string) (strategy MAStrategySpec, audits []llm.CallAudit, err error) {
	start := time.Now()
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_intent_resolve",
		Status:    maTraceEventStarted,
		Metadata: maTraceJSON(map[string]any{
			"has_territory":  hasMAIntentTerritory(intent.Territory),
			"ateco_explicit": len(intent.AtecoExplicit),
			"sector_include": len(intent.Sectors.Include),
			"sector_exclude": len(intent.Sectors.Exclude),
			"legal_forms":    len(intent.LegalForms),
		}),
	}); err != nil {
		return MAStrategySpec{}, nil, err
	}
	defer func() {
		if err != nil {
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:  "ma_intent_resolve",
				Status:     maTraceEventFailed,
				DurationMS: maTraceDuration(start),
				Error:      err.Error(),
			})
		}
	}()

	intent = validateIntentSpans(intent, promptText)
	missing := []string{}

	allowedProvinces := map[string]openapiit.Province{}
	provinces, territoryLabel, territoryMissing, err := s.resolveMAIntentTerritory(ctx, intent.Territory, allowedProvinces)
	if err != nil {
		return MAStrategySpec{}, nil, err
	}
	missing = appendMAMissingCriteria(missing, territoryMissing...)

	allowedAteco := map[string]AtecoCode{}
	atecoCandidates, sectorConcepts, sectorRetrievalMode, atecoMissing, atecoAudits, err := s.resolveMAIntentAteco(ctx, promptText, intent, allowedAteco, subject, email)
	audits = append(audits, atecoAudits...)
	if err != nil {
		return MAStrategySpec{}, audits, err
	}
	missing = appendMAMissingCriteria(missing, atecoMissing...)

	legalForms, legalFormMissing, err := s.resolveMAIntentLegalForms(ctx, intent.LegalForms)
	if err != nil {
		return MAStrategySpec{}, audits, err
	}
	missing = appendMAMissingCriteria(missing, legalFormMissing...)
	missing = appendMAMissingCriteria(missing, maIntentUnsupportedCriteria(intent)...)

	activityStatus, statusMissing := maIntentActivityStatus(intent.Status)
	missing = appendMAMissingCriteria(missing, statusMissing...)

	strategy = MAStrategySpec{
		Title:                  maIntentTitle(promptText, intent),
		SectorDescription:      maIntentSectorDescription(intent, atecoCandidates),
		TerritoryLabel:         territoryLabel,
		Provinces:              provinces,
		ActivityStatus:         activityStatus,
		SearchLimit:            maDefaultSearchLimit,
		AtecoCandidates:        atecoCandidates,
		SectorConcepts:         sectorConcepts,
		SectorRetrievalMode:    sectorRetrievalMode,
		Keywords:               maIntentKeywords(intent, atecoCandidates),
		Rationale:              maIntentRationale(intent, provinces, atecoCandidates, legalForms),
		MissingCriteria:        missing,
		Thesis:                 normalizeMAThesis(intent.Thesis),
		LegalForms:             legalForms,
		RevenuePerEmployeeMin:  maIntentValueConstraintValue(intent.RevenuePerEmployeeMin),
		MaxShareholders:        maIntentValueConstraintValue(intent.MaxShareholders),
		SuccessionMinOwnerAge:  maIntentSuccessionMinOwnerAge(intent.OwnerAge),
		SignalWeights:          nil,
		ScoringCriteria:        nil,
		SelectedStrategy:       "",
		ExpandedClassification: "",
	}
	applyMAIntentNumericRange(intent.Turnover, &strategy.TurnoverAround, &strategy.TurnoverMin, &strategy.TurnoverMax)
	applyMAIntentNumericRange(intent.Employees, nil, &strategy.EmployeeMin, &strategy.EmployeeMax)

	strategy, err = validateMAStrategy(strategy)
	if err != nil {
		return MAStrategySpec{}, audits, err
	}
	strategy, err = s.canonicalizeMAStrategyProvinces(strategy, allowedProvinces, true)
	if err != nil {
		return MAStrategySpec{}, audits, err
	}
	strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, allowedAteco, true)
	if err != nil {
		return MAStrategySpec{}, audits, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType:  "ma_intent_resolve",
		Status:     maTraceEventSucceeded,
		DurationMS: maTraceDuration(start),
		Response:   maTraceJSON(strategy),
		Metadata: maTraceJSON(map[string]any{
			"province_count":         len(strategy.Provinces),
			"allowed_province_count": len(allowedProvinces),
			"ateco_candidate_count":  len(strategy.AtecoCandidates),
			"allowed_ateco_count":    len(allowedAteco),
			"legal_form_count":       len(strategy.LegalForms),
			"missing_count":          len(strategy.MissingCriteria),
			"ateco_audit_count":      len(audits),
		}),
	}); err != nil {
		return MAStrategySpec{}, audits, err
	}
	return strategy, audits, nil
}

func maIntentTitle(promptText string, intent MAIntent) string {
	if title := cleanText(intent.Title, 120); title != "" {
		return title
	}
	return titleFromPrompt(promptText)
}

func maIntentActivityStatus(status *MAIntentStatusConstraint) (string, []string) {
	if status == nil {
		return "ATTIVA", nil
	}
	normalized, ok := normalizeCompanyActivityStatus(status.Value)
	if !ok || normalized == "" {
		return "ATTIVA", []string{"Stato attivita non mappabile: " + maIntentSourceLabel(status.Value, status.SourceText)}
	}
	return normalized, nil
}

func applyMAIntentNumericRange(item *MAIntentNumericConstraint, aroundField, minField, maxField **int) {
	if item == nil {
		return
	}
	if aroundField != nil && item.Around != nil && *item.Around >= 0 {
		value := *item.Around
		*aroundField = &value
	}
	if minField != nil && item.Min != nil && *item.Min >= 0 {
		value := *item.Min
		*minField = &value
	}
	if maxField != nil && item.Max != nil && *item.Max >= 0 {
		value := *item.Max
		*maxField = &value
	}
}

func maIntentSuccessionMinOwnerAge(item *MAIntentNumericConstraint) *int {
	if item == nil {
		return nil
	}
	for _, value := range []*int{item.Min, item.Around} {
		if value != nil && *value >= 0 {
			out := *value
			return &out
		}
	}
	return nil
}

func maIntentValueConstraintValue(item *MAIntentValueConstraint) *int {
	if item == nil || item.Value == nil {
		return nil
	}
	value := *item.Value
	return &value
}

func maIntentSectorDescription(intent MAIntent, candidates []MAAtecoCandidate) string {
	if summary := cleanText(positiveSectorText(intent.Sectors.Summary), 300); summary != "" {
		return summary
	}
	parts := []string{}
	for _, sector := range intent.Sectors.Include {
		if text := cleanText(positiveSectorText(sector.Text), 140); text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		for _, candidate := range candidates {
			if normalizeMAFit(candidate.Fit) == maFitExcluded {
				continue
			}
			if desc := cleanText(candidate.Description, 140); desc != "" {
				parts = append(parts, desc)
			}
			if len(parts) >= 4 {
				break
			}
		}
	}
	if len(parts) == 0 {
		return "Settore non specificato"
	}
	return cleanText(strings.Join(cleanStringList(parts, 6, 140), "; "), 300)
}

func maIntentKeywords(intent MAIntent, candidates []MAAtecoCandidate) []string {
	values := []string{}
	for _, sector := range intent.Sectors.Include {
		text := positiveSectorText(sector.Text)
		if cleaned := cleanText(text, 80); cleaned != "" {
			values = append(values, cleaned)
		}
		values = append(values, atecoSearchTokens(text)...)
	}
	if len(values) == 0 {
		for _, candidate := range candidates {
			if normalizeMAFit(candidate.Fit) == maFitExcluded {
				continue
			}
			values = append(values, atecoSearchTokens(candidate.Description)...)
		}
	}
	return cleanStringList(values, 12, 80)
}

func maIntentRationale(intent MAIntent, provinces []string, candidates []MAAtecoCandidate, legalForms []string) string {
	parts := []string{"Strategia assemblata da intent strutturato con resolver deterministici."}
	if len(intent.AtecoExplicit) > 0 {
		parts = append(parts, "ATECO validati da codici espliciti.")
	} else if len(intent.Sectors.Include)+len(intent.Sectors.Exclude) > 0 {
		parts = append(parts, "ATECO selezionati con resolver gerarchico vincolato al catalogo.")
	}
	if len(provinces) > 0 {
		parts = append(parts, fmt.Sprintf("Territorio risolto su %d province.", len(provinces)))
	}
	if len(candidates) > 0 {
		parts = append(parts, fmt.Sprintf("Perimetro ATECO con %d candidati.", len(candidates)))
	}
	if len(legalForms) > 0 {
		parts = append(parts, fmt.Sprintf("Forme giuridiche mappate: %s.", strings.Join(legalForms, ", ")))
	}
	return cleanText(strings.Join(parts, " "), 600)
}

func maIntentUnsupportedCriteria(intent MAIntent) []string {
	out := []string{}
	if intent.OwnerAge != nil && intent.OwnerAge.Max != nil && intent.OwnerAge.Min == nil && intent.OwnerAge.Around == nil {
		out = append(out, "Eta proprietario massima non applicabile al filtro successione: "+maIntentSourceLabel(strconv.Itoa(*intent.OwnerAge.Max), intent.OwnerAge.SourceText))
	}
	for _, item := range intent.Constraints {
		label := maIntentSourceLabel(item.Text, item.SourceText)
		if label == "" {
			label = item.Kind
		}
		disposition := cleanText(item.Disposition, 80)
		if disposition == "" {
			disposition = "unsupported"
		}
		out = append(out, fmt.Sprintf("Vincolo %s non applicato: %s", disposition, label))
	}
	return out
}

func maIntentSourceLabel(value, sourceText string) string {
	if source := cleanText(sourceText, 120); source != "" {
		return source
	}
	return cleanText(value, 120)
}

func appendMAMissingCriteria(existing []string, values ...string) []string {
	out := append([]string{}, existing...)
	seen := map[string]struct{}{}
	for _, value := range out {
		seen[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for _, raw := range values {
		value := cleanText(raw, 120)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

type maProvinceLookup struct {
	byCode      map[string]openapiit.Province
	byName      map[string][]openapiit.Province
	byCompact   map[string][]openapiit.Province
	regionByKey map[string]*maRegionProvinceGroup
}

type maRegionProvinceGroup struct {
	Name  string
	Codes []string
}

func hasMAIntentTerritory(territory MAIntentTerritory) bool {
	return len(territory.IncludeRegions)+len(territory.IncludeProvinces)+len(territory.ExcludeProvinces) > 0
}

func (s *maService) resolveMAIntentTerritory(ctx context.Context, territory MAIntentTerritory, allowed map[string]openapiit.Province) ([]string, string, []string, error) {
	if !hasMAIntentTerritory(territory) {
		return nil, "Italia", nil, nil
	}
	envelope, _, err := listProvincesWithCache(ctx, s.provinceCache, s.openapiit, s.now)
	if err != nil {
		return nil, "", nil, err
	}
	lookup := newMAProvinceLookup(envelope.Data)
	included := map[string]struct{}{}
	regionLabels := []string{}
	provinceLabels := []string{}
	excludedLabels := []string{}
	missing := []string{}

	for _, item := range territory.IncludeRegions {
		group, ok := lookup.resolveRegion(item.Value)
		if !ok {
			missing = append(missing, "Regione non risolvibile: "+maIntentSourceLabel(item.Value, item.SourceText))
			continue
		}
		for _, code := range group.Codes {
			included[code] = struct{}{}
		}
		regionLabels = append(regionLabels, group.Name)
	}
	for _, item := range territory.IncludeProvinces {
		province, ok := lookup.resolveProvince(item.Value)
		if !ok {
			missing = append(missing, "Provincia non risolvibile: "+maIntentSourceLabel(item.Value, item.SourceText))
			continue
		}
		included[province.Sigla] = struct{}{}
		provinceLabels = append(provinceLabels, province.Sigla)
	}
	for _, item := range territory.ExcludeProvinces {
		province, ok := lookup.resolveProvince(item.Value)
		if !ok {
			missing = append(missing, "Provincia esclusa non risolvibile: "+maIntentSourceLabel(item.Value, item.SourceText))
			continue
		}
		if _, exists := included[province.Sigla]; !exists {
			missing = append(missing, "Provincia esclusa fuori dal perimetro incluso: "+maIntentSourceLabel(item.Value, item.SourceText))
			continue
		}
		delete(included, province.Sigla)
		excludedLabels = append(excludedLabels, province.Sigla)
	}

	provinces := make([]string, 0, len(included))
	for code := range included {
		if province, ok := lookup.byCode[code]; ok {
			rememberAllowedProvince(allowed, province)
		}
		provinces = append(provinces, code)
	}
	sort.Strings(provinces)
	return provinces, maIntentTerritoryLabel(regionLabels, provinceLabels, excludedLabels, provinces), missing, nil
}

func newMAProvinceLookup(items []openapiit.Province) maProvinceLookup {
	lookup := maProvinceLookup{
		byCode:      map[string]openapiit.Province{},
		byName:      map[string][]openapiit.Province{},
		byCompact:   map[string][]openapiit.Province{},
		regionByKey: map[string]*maRegionProvinceGroup{},
	}
	regionsByPrimary := map[string]*maRegionProvinceGroup{}
	for _, item := range items {
		code, ok := normalizeProvince(item.Sigla)
		if !ok || code == "" {
			continue
		}
		item.Sigla = code
		item.Provincia = cleanText(item.Provincia, 120)
		item.Regione = cleanText(item.Regione, 120)
		if item.Provincia == "" || item.Regione == "" {
			continue
		}
		lookup.byCode[code] = item
		if key := normalizeMAFoldKey(item.Provincia); key != "" {
			lookup.byName[key] = append(lookup.byName[key], item)
		}
		if key := normalizeMACompactKey(item.Provincia); key != "" {
			lookup.byCompact[key] = append(lookup.byCompact[key], item)
		}

		primary := normalizeMAFoldKey(item.Regione)
		group := regionsByPrimary[primary]
		if group == nil {
			group = &maRegionProvinceGroup{Name: item.Regione}
			regionsByPrimary[primary] = group
			for _, key := range maRegionLookupKeys(item.Regione) {
				lookup.regionByKey[key] = group
			}
		}
		group.Codes = append(group.Codes, code)
	}
	for _, group := range regionsByPrimary {
		sort.Strings(group.Codes)
	}
	return lookup
}

func (lookup maProvinceLookup) resolveRegion(value string) (*maRegionProvinceGroup, bool) {
	for _, key := range maRegionLookupKeys(value) {
		if group := lookup.regionByKey[key]; group != nil {
			return group, true
		}
	}
	return nil, false
}

func (lookup maProvinceLookup) resolveProvince(value string) (openapiit.Province, bool) {
	if code, ok := normalizeProvince(value); ok && code != "" {
		item, exists := lookup.byCode[code]
		return item, exists
	}
	if key := normalizeMAFoldKey(value); key != "" {
		if match, ok := singleMAProvinceMatch(lookup.byName[key]); ok {
			return match, true
		}
	}
	if key := normalizeMACompactKey(value); key != "" {
		if match, ok := singleMAProvinceMatch(lookup.byCompact[key]); ok {
			return match, true
		}
	}
	key := normalizeMAFoldKey(value)
	if len([]rune(key)) < 4 {
		return openapiit.Province{}, false
	}
	matches := []openapiit.Province{}
	for nameKey, items := range lookup.byName {
		if strings.Contains(nameKey, key) || strings.Contains(key, nameKey) {
			matches = append(matches, items...)
		}
	}
	if match, ok := singleMAProvinceMatch(matches); ok {
		return match, true
	}
	return lookup.resolveProvinceFuzzy(value)
}

func (lookup maProvinceLookup) resolveProvinceFuzzy(value string) (openapiit.Province, bool) {
	key := normalizeMACompactKey(value)
	keyLen := len([]rune(key))
	if keyLen < 5 {
		return openapiit.Province{}, false
	}
	limit := maProvinceFuzzyDistanceLimit(keyLen)
	bestDistance := limit + 1
	matches := []openapiit.Province{}
	for nameKey, items := range lookup.byCompact {
		distance := maBoundedEditDistance(key, nameKey, limit)
		if distance > limit {
			continue
		}
		if distance < bestDistance {
			bestDistance = distance
			matches = matches[:0]
		}
		if distance == bestDistance {
			matches = append(matches, items...)
		}
	}
	if bestDistance > limit {
		return openapiit.Province{}, false
	}
	return singleMAProvinceMatch(matches)
}

func maProvinceFuzzyDistanceLimit(keyLen int) int {
	if keyLen >= 10 {
		return 2
	}
	return 1
}

func maBoundedEditDistance(a, b string, limit int) int {
	left := []rune(a)
	right := []rune(b)
	if len(left) == 0 {
		return len(right)
	}
	if len(right) == 0 {
		return len(left)
	}
	if diff := len(left) - len(right); diff > limit || -diff > limit {
		return limit + 1
	}
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, lr := range left {
		current[0] = i + 1
		rowMin := current[0]
		for j, rr := range right {
			cost := 0
			if lr != rr {
				cost = 1
			}
			current[j+1] = min(
				previous[j+1]+1,
				current[j]+1,
				previous[j]+cost,
			)
			if current[j+1] < rowMin {
				rowMin = current[j+1]
			}
		}
		if rowMin > limit {
			return limit + 1
		}
		previous, current = current, previous
	}
	if previous[len(right)] > limit {
		return limit + 1
	}
	return previous[len(right)]
}

func singleMAProvinceMatch(items []openapiit.Province) (openapiit.Province, bool) {
	seen := map[string]openapiit.Province{}
	for _, item := range items {
		if item.Sigla == "" {
			continue
		}
		seen[item.Sigla] = item
	}
	if len(seen) != 1 {
		return openapiit.Province{}, false
	}
	for _, item := range seen {
		return item, true
	}
	return openapiit.Province{}, false
}

func maRegionLookupKeys(value string) []string {
	seen := map[string]struct{}{}
	keys := []string{}
	add := func(key string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	folded := normalizeMAFoldKey(value)
	compact := normalizeMACompactKey(value)
	add(folded)
	add(compact)
	switch {
	case strings.Contains(compact, "valledaosta") || strings.Contains(compact, "valleedaoste"):
		add("valle d aosta")
		add("valledaosta")
	case strings.Contains(compact, "trentinoaltoadige") || strings.Contains(compact, "sudtirol") || strings.Contains(compact, "suedtirol"):
		add("trentino alto adige")
		add("trentinoaltoadige")
		add("trentino alto adige sudtirol")
		add("trentinoaltoadigesudtirol")
	case strings.Contains(compact, "friuliveneziagiulia"):
		add("friuli venezia giulia")
		add("friuliveneziagiulia")
	case strings.Contains(compact, "emiliaromagna"):
		add("emilia romagna")
		add("emiliaromagna")
	}
	return keys
}

func maIntentTerritoryLabel(regions, provinces, excluded, resolved []string) string {
	parts := []string{}
	if len(regions) > 0 {
		parts = append(parts, cleanStringList(regions, 20, 80)...)
	}
	if len(provinces) > 0 {
		parts = append(parts, cleanStringList(provinces, 20, 8)...)
	}
	if len(parts) == 0 && len(resolved) > 0 {
		parts = append(parts, resolved...)
	}
	if len(parts) == 0 {
		return "Italia"
	}
	label := strings.Join(cleanStringList(parts, 30, 80), ", ")
	if len(excluded) > 0 {
		label += " (escluse " + strings.Join(cleanStringList(excluded, 20, 8), ", ") + ")"
	}
	return cleanText(label, 160)
}

func (s *maService) resolveMAIntentAteco(ctx context.Context, promptText string, intent MAIntent, allowed map[string]AtecoCode, subject, email string) ([]MAAtecoCandidate, []MAStrategyConcept, string, []string, []llm.CallAudit, error) {
	if len(intent.AtecoExplicit) > 0 {
		return s.resolveMAIntentExplicitAteco(ctx, intent.AtecoExplicit, intent.Sectors.Exclude, allowed)
	}
	if len(intent.Sectors.Include)+len(intent.Sectors.Exclude) == 0 {
		return nil, nil, "", nil, nil, nil
	}
	if s.ateco == nil {
		return nil, nil, "", nil, nil, errAtecoStoreUnavailable
	}

	// Use case 1: deterministic embedding retrieval over the curated KB replaces the
	// slow/imprecise LLM hierarchy resolver. It is the default; ok=false (KB/embedder
	// unavailable) or a retrieval error degrades gracefully to the resolver below.
	cfg := s.loadAtecoRetrievalConfig(ctx)
	if cfg.Method == maAtecoRetrievalMethodEmbedding {
		candidates, concepts, missing, ok, embErr := s.retrieveMAIntentAtecoEmbedding(ctx, intent, allowed, cfg)
		if ok && embErr == nil {
			return candidates, concepts, "embedding", missing, nil, nil
		}
		fallbackReason := "kb_or_embedder_unavailable"
		fallbackErr := ""
		if embErr != nil {
			fallbackReason = "retrieval_error"
			fallbackErr = embErr.Error()
		}
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_ateco_retrieval_fallback",
			Status:    maTraceEventInfo,
			Error:     fallbackErr,
			Metadata:  maTraceJSON(map[string]any{"reason": fallbackReason}),
		})
	}

	divisions, err := s.ateco.AtecoDivisions(ctx)
	if err != nil {
		return nil, nil, "", nil, nil, err
	}
	for _, division := range divisions {
		rememberAllowedAteco(allowed, division)
	}
	missing := []string{}
	if len(divisions) == 0 {
		missing = append(missing, "Catalogo ATECO senza divisioni disponibili")
	}
	if len(intent.Sectors.Include) == 0 && len(intent.Sectors.Exclude) > 0 {
		missing = append(missing, "Esclusione settoriale senza settore incluso: serve un perimetro positivo")
	}
	divisionCodes := make([]string, 0, len(divisions))
	for _, division := range divisions {
		if code := normalizeAtecoCode(division.Codice); code != "" {
			divisionCodes = append(divisionCodes, code)
		}
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_ateco_hierarchy_initial",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"sector_include":      len(intent.Sectors.Include),
			"sector_exclude":      len(intent.Sectors.Exclude),
			"division_count":      len(divisions),
			"division_codes":      divisionCodes,
			"allowed_ateco_count": len(allowed),
		}),
	}); err != nil {
		return nil, nil, "fallback_llm", nil, nil, err
	}

	output, audit, err := s.rerankMAIntentAteco(ctx, promptText, intent, divisions, allowed, subject, email)
	if err != nil {
		audits := []llm.CallAudit{}
		if audit.Scope != "" || len(audit.Request) > 0 {
			audits = append(audits, audit)
		}
		return nil, nil, "fallback_llm", nil, audits, err
	}
	resolved, constrainedMissing := constrainMAAtecoRerank(output, allowed, intent.Sectors.Include, intent.Sectors.Exclude)
	missing = appendMAMissingCriteria(missing, constrainedMissing...)
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_ateco_hierarchy_result",
		Status:    maTraceEventSucceeded,
		Response:  maTraceJSON(map[string]any{"selected": resolved, "missing": missing}),
		Metadata: maTraceJSON(map[string]any{
			"selected_count":        len(resolved),
			"missing_count":         len(missing),
			"allowed_ateco_count":   len(allowed),
			"model_missing_count":   len(output.MissingCriteria),
			"excluded_prefix_count": len(output.ExcludedPrefixes),
		}),
	}); err != nil {
		return nil, nil, "fallback_llm", nil, nil, err
	}
	return resolved, nil, "fallback_llm", missing, []llm.CallAudit{audit}, nil
}

func (s *maService) resolveMAIntentExplicitAteco(ctx context.Context, constraints []MAIntentAtecoConstraint, excludedSectors []MAIntentTextConstraint, allowed map[string]AtecoCode) ([]MAAtecoCandidate, []MAStrategyConcept, string, []string, []llm.CallAudit, error) {
	if s.ateco == nil {
		return nil, nil, "", nil, nil, errAtecoStoreUnavailable
	}
	candidates := []MAAtecoCandidate{}
	missing := []string{}
	resolvedCodes := []string{}
	seen := map[string]struct{}{}
	for _, constraint := range constraints {
		resolved, err := s.ateco.ResolveAtecoCode(ctx, constraint.Code)
		if errors.Is(err, errAtecoCodeNotFound) {
			missing = append(missing, "ATECO esplicito non trovato: "+maIntentSourceLabel(constraint.Code, constraint.SourceText))
			continue
		}
		if err != nil {
			return nil, nil, "", nil, nil, err
		}
		searchCode := resolved.CodiceSearch
		if searchCode == "" {
			searchCode = atecoSearchCode(resolved.Codice)
		}
		if searchCode == "" {
			missing = append(missing, "ATECO esplicito non utilizzabile: "+maIntentSourceLabel(constraint.Code, constraint.SourceText))
			continue
		}
		rememberAllowedAteco(allowed, resolved)
		resolvedCodes = append(resolvedCodes, resolved.Codice)
		if _, exists := seen[searchCode]; exists {
			continue
		}
		seen[searchCode] = struct{}{}
		candidates = append(candidates, MAAtecoCandidate{
			Code:        resolved.Codice,
			Description: resolved.Titolo,
			Rationale:   cleanText("Codice ATECO esplicito: "+maIntentSourceLabel(constraint.Code, constraint.SourceText), 240),
			Fit:         normalizeMAFit(constraint.Fit),
		})
	}
	for _, excluded := range excludedSectors {
		missing = append(missing, "Esclusione settoriale testuale non applicata con ATECO espliciti: "+maIntentSourceLabel(excluded.Text, excluded.SourceText))
	}
	concepts := []MAStrategyConcept{}
	if s.kb != nil && len(resolvedCodes) > 0 {
		if kbConcepts, err := s.kb.LoadKBConcepts(ctx); err == nil {
			concepts = conceptsCoveringCodes(kbConcepts, resolvedCodes)
		}
	}
	for _, code := range uncoveredExplicitAtecoCodes(concepts, resolvedCodes) {
		missing = append(missing, "Codice ATECO dichiarato fuori dalla base di conoscenza: "+code)
	}
	return candidates, concepts, "explicit_reverse", missing, nil, nil
}

func (s *maService) rerankMAIntentAteco(ctx context.Context, promptText string, intent MAIntent, divisions []AtecoCode, allowed map[string]AtecoCode, subject, email string) (maAtecoRerankOutput, llm.CallAudit, error) {
	if s.llmp == nil {
		return maAtecoRerankOutput{}, llm.CallAudit{}, errMAOpenRouterUnavailable
	}
	modelConfig, err := s.llmp.ResolveModel(ctx, maModelScopeStrategyAtecoHierarchy, "")
	if err != nil {
		return maAtecoRerankOutput{}, llm.CallAudit{}, llmConfigError(err)
	}
	promptConfig, err := s.llmp.ResolvePrompt(ctx, maModelScopeStrategyAtecoHierarchy, "")
	if err != nil {
		return maAtecoRerankOutput{}, llm.CallAudit{}, llmConfigError(err)
	}
	client, err := s.llmp.ClientForModel(ctx, modelConfig)
	if err != nil {
		return maAtecoRerankOutput{}, llm.CallAudit{}, llmConfigError(err)
	}
	reqParams := modelConfig.RawParams()
	if _, ok := reqParams["max_tokens"]; !ok {
		reqParams["max_tokens"] = 2400
	}
	systemPrompt := maAtecoHierarchySystemPrompt(promptConfig.Prompt)
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_ateco_hierarchy_config",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"model_id":       modelConfig.ID,
			"model_scope":    modelConfig.Scope,
			"model":          modelConfig.Model,
			"prompt_id":      promptConfig.ID,
			"prompt_scope":   promptConfig.Scope,
			"prompt_name":    promptConfig.Name,
			"prompt":         systemPrompt,
			"division_count": len(divisions),
		}),
	}); err != nil {
		return maAtecoRerankOutput{}, llm.CallAudit{}, err
	}
	payload := maAtecoHierarchyInput{
		Prompt:         promptText,
		Intent:         intent,
		IncludeSectors: maAtecoRerankSectors(intent.Sectors.Include),
		ExcludeSectors: maAtecoRerankSectors(intent.Sectors.Exclude),
		Divisions:      maAtecoHierarchyCodes(divisions),
	}
	inputRaw, err := json.Marshal(payload)
	if err != nil {
		return maAtecoRerankOutput{}, llm.CallAudit{}, err
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: string(inputRaw)},
	}
	tools := []llm.Tool{maAtecoChildrenTool()}
	var response llm.ChatResponse
	var responseRaw json.RawMessage
	var req llm.ChatRequest
	var duration *int
	resolverStart := time.Now()
	chatRoundCount := 0
	totalToolCallCount := 0
	llmDurationMS := 0
	for round := 0; round <= maMaxToolRounds; round++ {
		req = llm.ChatRequest{
			Model:          modelConfig.Model,
			Params:         reqParams,
			ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
			Messages:       messages,
			Tools:          tools,
			ToolChoice:     "auto",
		}
		start := time.Now()
		response, err = client.Chat(ctx, req)
		duration = maTraceDuration(start)
		if duration != nil {
			llmDurationMS += *duration
		}
		chatRoundCount++
		if err != nil {
			requestBody, _ := llm.BuildRequestBody(req)
			requestRaw, _ := json.Marshal(requestBody)
			responseFull := maTraceJSON(response)
			totalDuration := maTraceDuration(resolverStart)
			audit := llm.CallAudit{
				App:          maApp,
				Scope:        maModelScopeStrategyAtecoHierarchy,
				ProviderID:   modelConfig.ProviderID,
				ModelID:      modelConfig.ID,
				PromptID:     promptConfig.ID,
				Model:        modelConfig.Model,
				Request:      requestRaw,
				Response:     responseFull,
				ActorSubject: subject,
				ActorEmail:   email,
				DurationMS:   totalDuration,
				Status:       "failed",
				ErrorMessage: err.Error(),
			}
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "ma_ateco_hierarchy_chat",
				Round:          maTraceRound(round),
				ExternalSystem: "openrouter",
				Status:         maTraceEventFailed,
				DurationMS:     duration,
				Request:        requestRaw,
				Response:       responseFull,
				Error:          err.Error(),
			})
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:  "ma_ateco_hierarchy_summary",
				Status:     maTraceEventFailed,
				DurationMS: totalDuration,
				Metadata: maTraceJSON(map[string]any{
					"chat_round_count": chatRoundCount,
					"tool_call_count":  totalToolCallCount,
					"llm_duration_ms":  llmDurationMS,
				}),
				Error: err.Error(),
			})
			return maAtecoRerankOutput{}, audit, err
		}
		totalToolCallCount += len(response.ToolCalls)
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "ma_ateco_hierarchy_chat",
			Round:          maTraceRound(round),
			ExternalSystem: "openrouter",
			Status:         maTraceEventSucceeded,
			DurationMS:     duration,
			Request:        maTraceJSON(req),
			Response:       maTraceJSON(response),
			Metadata:       maTraceJSON(map[string]any{"tool_call_count": len(response.ToolCalls), "response_id": response.ID, "response_model": response.Model}),
		}); err != nil {
			return maAtecoRerankOutput{}, llm.CallAudit{}, err
		}
		if len(response.ToolCalls) == 0 {
			responseRaw = json.RawMessage([]byte(strings.TrimSpace(response.Content)))
			break
		}
		if round == maMaxToolRounds {
			err := fmt.Errorf("%w: ateco hierarchy tool loop", errMAStrategyInvalid)
			requestBody, _ := llm.BuildRequestBody(req)
			requestRaw, _ := json.Marshal(requestBody)
			responseFull := maTraceJSON(response)
			usageRaw, _ := json.Marshal(response.Usage)
			totalDuration := maTraceDuration(resolverStart)
			audit := llm.CallAudit{
				App:          maApp,
				Scope:        maModelScopeStrategyAtecoHierarchy,
				ProviderID:   modelConfig.ProviderID,
				ModelID:      modelConfig.ID,
				PromptID:     promptConfig.ID,
				Model:        modelConfig.Model,
				Request:      requestRaw,
				Response:     responseFull,
				Usage:        usageRaw,
				ActorSubject: subject,
				ActorEmail:   email,
				DurationMS:   totalDuration,
				Status:       "failed",
				ErrorMessage: err.Error(),
			}
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType: "ma_ateco_hierarchy_loop_limit",
				Round:     maTraceRound(round),
				Status:    maTraceEventFailed,
				Metadata:  maTraceJSON(map[string]any{"max_tool_rounds": maMaxToolRounds, "tool_call_count": len(response.ToolCalls)}),
				Error:     err.Error(),
			})
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:  "ma_ateco_hierarchy_summary",
				Status:     maTraceEventFailed,
				DurationMS: totalDuration,
				Metadata: maTraceJSON(map[string]any{
					"chat_round_count": chatRoundCount,
					"tool_call_count":  totalToolCallCount,
					"llm_duration_ms":  llmDurationMS,
				}),
				Error: err.Error(),
			})
			return maAtecoRerankOutput{}, audit, err
		}
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})
		for _, call := range response.ToolCalls {
			start := time.Now()
			content := s.executeMAAtecoChildrenTool(ctx, call, allowed)
			parentCode := maAtecoChildrenToolParent(call)
			childCount := maAtecoChildrenToolResultCount(content)
			if err := s.traceEvent(ctx, maTraceEventWrite{
				EventType:  "ma_ateco_hierarchy_tool",
				Round:      maTraceRound(round),
				ToolName:   call.Function.Name,
				Status:     maToolResultStatus(content),
				DurationMS: maTraceDuration(start),
				Request:    maTraceJSON(call),
				Response:   maTraceRawJSON([]byte(content)),
				Metadata: maTraceJSON(map[string]any{
					"parent_code":         parentCode,
					"child_count":         childCount,
					"allowed_ateco_count": len(allowed),
				}),
			}); err != nil {
				return maAtecoRerankOutput{}, llm.CallAudit{}, err
			}
			messages = append(messages, llm.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    content,
			})
		}
	}

	requestBody, err := llm.BuildRequestBody(req)
	if err != nil {
		return maAtecoRerankOutput{}, llm.CallAudit{}, err
	}
	requestRaw, _ := json.Marshal(requestBody)
	usageRaw, _ := json.Marshal(response.Usage)
	responseFull := maTraceJSON(response)
	totalDuration := maTraceDuration(resolverStart)
	audit := llm.CallAudit{
		App:          maApp,
		Scope:        maModelScopeStrategyAtecoHierarchy,
		ProviderID:   modelConfig.ProviderID,
		ModelID:      modelConfig.ID,
		PromptID:     promptConfig.ID,
		Model:        modelConfig.Model,
		Request:      requestRaw,
		Response:     responseFull,
		Usage:        usageRaw,
		ActorSubject: subject,
		ActorEmail:   email,
		DurationMS:   totalDuration,
		Status:       "succeeded",
	}

	var output maAtecoRerankOutput
	if err := json.Unmarshal(responseRaw, &output); err != nil {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_ateco_hierarchy_decode",
			Status:    maTraceEventFailed,
			Response:  responseRaw,
			Error:     err.Error(),
		})
		return maAtecoRerankOutput{}, audit, fmt.Errorf("decode ma ateco hierarchy: %w", err)
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_ateco_hierarchy_decode",
		Status:    maTraceEventSucceeded,
		Response:  maTraceJSON(output),
		Metadata:  maTraceJSON(map[string]any{"allowed_ateco_count": len(allowed)}),
	}); err != nil {
		return maAtecoRerankOutput{}, audit, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType:  "ma_ateco_hierarchy_summary",
		Status:     maTraceEventSucceeded,
		DurationMS: totalDuration,
		Metadata: maTraceJSON(map[string]any{
			"chat_round_count":    chatRoundCount,
			"tool_call_count":     totalToolCallCount,
			"llm_duration_ms":     llmDurationMS,
			"allowed_ateco_count": len(allowed),
		}),
	}); err != nil {
		return maAtecoRerankOutput{}, audit, err
	}
	return output, audit, nil
}

func maAtecoRerankSectors(items []MAIntentTextConstraint) []maAtecoRerankSector {
	out := make([]maAtecoRerankSector, 0, len(items))
	for _, item := range items {
		text := cleanText(item.Text, 180)
		if text == "" {
			continue
		}
		out = append(out, maAtecoRerankSector{Text: text, SourceText: cleanText(item.SourceText, 240)})
	}
	return out
}

func maAtecoHierarchySystemPrompt(base string) string {
	extra := strings.Join([]string{
		"Runtime tool contract override:",
		"- list_ateco_children(code) returns every descendant of the requested ATECO code, not only immediate children.",
		"- The requested code itself is already visible and is not repeated in the tool response.",
		"- Any descendant returned by list_ateco_children may be selected directly; do not call list_ateco_children on returned descendants merely to reach leaves.",
	}, "\n")
	base = strings.TrimSpace(base)
	if base == "" {
		return extra
	}
	return base + "\n\n" + extra
}

func maAtecoHierarchyCodes(items []AtecoCode) []maAtecoHierarchyCode {
	out := make([]maAtecoHierarchyCode, 0, len(items))
	for _, item := range items {
		code := normalizeAtecoCode(item.Codice)
		if code == "" {
			continue
		}
		node := maAtecoHierarchyCode{
			Code:       code,
			SearchCode: item.CodiceSearch,
			Title:      cleanText(item.Titolo, 180),
		}
		if item.Gerarchia != nil {
			node.Hierarchy = *item.Gerarchia
		}
		out = append(out, node)
	}
	return out
}

func maAtecoHierarchyToolCodes(items []AtecoHierarchyCode) []maAtecoHierarchyCode {
	out := make([]maAtecoHierarchyCode, 0, len(items))
	for _, item := range items {
		code := normalizeAtecoCode(item.Codice)
		if code == "" {
			continue
		}
		node := maAtecoHierarchyCode{
			Code:         code,
			SearchCode:   item.CodiceSearch,
			Title:        cleanText(item.Titolo, 180),
			ChildCount:   item.ChildCount,
			SubtreeCount: item.SubtreeCount,
		}
		if item.Gerarchia != nil {
			node.Hierarchy = *item.Gerarchia
		}
		out = append(out, node)
	}
	return out
}

func constrainMAAtecoRerank(output maAtecoRerankOutput, allowed map[string]AtecoCode, includeSectors, excludeSectors []MAIntentTextConstraint) ([]MAAtecoCandidate, []string) {
	bySearch := map[string]AtecoCode{}
	for _, item := range allowed {
		searchCode := item.CodiceSearch
		if searchCode == "" {
			searchCode = atecoSearchCode(item.Codice)
		}
		if searchCode == "" {
			continue
		}
		bySearch[searchCode] = item
	}
	out := []MAAtecoCandidate{}
	missing := []string{}
	seen := map[string]struct{}{}
	selectedCount := 0
	for _, selected := range output.Selected {
		searchCode := atecoSearchCode(selected.Code)
		item, ok := bySearch[searchCode]
		if !ok {
			missing = append(missing, "ATECO selezionato fuori dai codici esplorati: "+cleanText(selected.Code, 40))
			continue
		}
		if _, exists := seen[searchCode]; exists {
			continue
		}
		seen[searchCode] = struct{}{}
		out = append(out, MAAtecoCandidate{
			Code:        item.Codice,
			Description: item.Titolo,
			Rationale:   cleanText(selected.Reason, 240),
			Fit:         maAtecoRerankFit(selected.Fit),
		})
		selectedCount++
	}
	if len(excludeSectors) > 0 {
		exclusionSource := maAtecoExclusionSource(excludeSectors)
		for _, excluded := range output.ExcludedPrefixes {
			prefix := excluded.Prefix
			if prefix == "" {
				prefix = excluded.Code
			}
			code := normalizeAtecoCode(prefix)
			searchCode := atecoSearchCode(code)
			if searchCode == "" || len(searchCode) < 2 {
				continue
			}
			item, ok := bySearch[searchCode]
			if !ok {
				missing = append(missing, "Prefisso ATECO escluso fuori dai codici esplorati: "+cleanText(prefix, 40))
				continue
			}
			key := "excluded:" + searchCode
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, MAAtecoCandidate{
				Code:        item.Codice,
				Description: maAtecoExcludedDescription(code, bySearch),
				Rationale:   maAtecoExcludedRationale(excluded.Reason, exclusionSource),
				Fit:         maFitExcluded,
			})
		}
	}
	acceptedSectorMissing := 0
	for _, item := range output.MissingCriteria {
		if !maAtecoMissingGroundedInSectors(item, includeSectors, excludeSectors) {
			continue
		}
		text := cleanText(item.Text, 100)
		if text == "" {
			continue
		}
		if reason := cleanText(item.Reason, 80); reason != "" {
			text += ": " + reason
		}
		missing = append(missing, text)
		acceptedSectorMissing++
	}
	if selectedCount == 0 && len(includeSectors) > 0 && acceptedSectorMissing == 0 {
		missing = append(missing, "Settore testuale non mappato con sicurezza al catalogo ATECO esplorato")
	}
	return out, missing
}

func maAtecoRerankFit(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case maFitWeak:
		return maFitWeak
	default:
		return maFitCore
	}
}

func maAtecoMissingGroundedInSectors(item maAtecoRerankMissingCriterion, includeSectors, excludeSectors []MAIntentTextConstraint) bool {
	sectorTokens := maSectorConstraintTokens(includeSectors, excludeSectors)
	if len(sectorTokens) == 0 {
		return false
	}
	for _, token := range maTextTokens(item.Text) {
		if _, ok := sectorTokens[token]; ok {
			return true
		}
	}
	return false
}

func maSectorConstraintTokens(includeSectors, excludeSectors []MAIntentTextConstraint) map[string]struct{} {
	out := map[string]struct{}{}
	add := func(item MAIntentTextConstraint) {
		for _, token := range maTextTokens(positiveSectorText(item.Text)) {
			out[token] = struct{}{}
		}
	}
	for _, item := range includeSectors {
		add(item)
	}
	for _, item := range excludeSectors {
		add(item)
	}
	return out
}

func maTextTokens(value string) []string {
	fields := strings.Fields(normalizeMAFoldKey(value))
	out := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, token := range fields {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == "" || maAtecoMissingGroundingStopwords[token] || maNumericToken(token) {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func maNumericToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var maAtecoMissingGroundingStopwords = map[string]bool{
	"attivita": true,
	"azienda":  true,
	"aziende":  true,
	"con":      true,
	"dei":      true,
	"del":      true,
	"della":    true,
	"delle":    true,
	"di":       true,
	"e":        true,
	"gli":      true,
	"il":       true,
	"in":       true,
	"la":       true,
	"le":       true,
	"lo":       true,
	"per":      true,
	"servizi":  true,
	"servizio": true,
	"settore":  true,
	"settori":  true,
}

func maAtecoExcludedDescription(code string, candidates map[string]AtecoCode) string {
	searchCode := atecoSearchCode(code)
	if item, ok := candidates[searchCode]; ok && item.Titolo != "" {
		return item.Titolo
	}
	return cleanText("Esclusione ATECO "+code, 180)
}

func maAtecoExcludedRationale(reason, source string) string {
	parts := []string{}
	if reason := cleanText(reason, 140); reason != "" {
		parts = append(parts, reason)
	}
	if source := cleanText(source, 120); source != "" {
		parts = append(parts, "fonte: "+source)
	}
	if len(parts) == 0 {
		return "Esclusione settoriale esplicita"
	}
	return cleanText(strings.Join(parts, "; "), 240)
}

func maAtecoExclusionSource(items []MAIntentTextConstraint) string {
	parts := []string{}
	for _, item := range items {
		if source := maIntentSourceLabel(item.Text, item.SourceText); source != "" {
			parts = append(parts, source)
		}
	}
	return strings.Join(cleanStringList(parts, 4, 80), "; ")
}

type maLegalFormResolver struct {
	byCode    map[string]maCompanyLegalForm
	byKey     map[string][]string
	byCompact map[string][]string
	rank      map[string]int
}

func (s *maService) resolveMAIntentLegalForms(ctx context.Context, constraints []MAIntentTextConstraint) ([]string, []string, error) {
	if len(constraints) == 0 {
		return nil, nil, nil
	}
	if s.store == nil {
		return nil, nil, errMAStoreUnavailable
	}
	rows, err := s.store.ListMACompanyLegalForms(ctx)
	if err != nil {
		return nil, nil, err
	}
	resolver := newMALegalFormResolver(rows)
	codes := []string{}
	missing := []string{}
	seen := map[string]struct{}{}
	for _, constraint := range constraints {
		resolved := resolver.resolve(constraint.Text)
		if len(resolved) == 0 {
			missing = append(missing, "Forma giuridica non mappabile: "+maIntentSourceLabel(constraint.Text, constraint.SourceText))
			continue
		}
		for _, code := range resolved {
			if _, exists := seen[code]; exists {
				continue
			}
			seen[code] = struct{}{}
			codes = append(codes, code)
		}
	}
	sort.SliceStable(codes, func(i, j int) bool {
		left := resolver.rank[codes[i]]
		right := resolver.rank[codes[j]]
		if left == right {
			return codes[i] < codes[j]
		}
		return left < right
	})
	return normalizeMALegalForms(codes), missing, nil
}

func newMALegalFormResolver(rows []maCompanyLegalForm) maLegalFormResolver {
	resolver := maLegalFormResolver{
		byCode:    map[string]maCompanyLegalForm{},
		byKey:     map[string][]string{},
		byCompact: map[string][]string{},
		rank:      map[string]int{},
	}
	for index, row := range rows {
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if !maLegalFormCodePattern(code) {
			continue
		}
		row.Code = code
		resolver.byCode[code] = row
		resolver.rank[code] = index + 1
		for _, value := range []string{code, row.DescriptionIT, row.DescriptionEN} {
			if key := normalizeMAFoldKey(value); key != "" {
				resolver.byKey[key] = append(resolver.byKey[key], code)
			}
			if key := normalizeMACompactKey(value); key != "" {
				resolver.byCompact[key] = append(resolver.byCompact[key], code)
			}
		}
	}
	return resolver
}

func (r maLegalFormResolver) resolve(value string) []string {
	text := cleanText(value, 160)
	if text == "" {
		return nil
	}
	if code := strings.ToUpper(strings.TrimSpace(text)); maLegalFormCodePattern(code) {
		if _, ok := r.byCode[code]; ok {
			return []string{code}
		}
	}
	key := normalizeMAFoldKey(text)
	compact := normalizeMACompactKey(text)
	if codes := r.filterExisting(maLegalFormSynonymCodes(key, compact)); len(codes) > 0 {
		return codes
	}
	if codes := r.onlyExisting(r.byKey[key]); len(codes) > 0 {
		return codes
	}
	if codes := r.onlyExisting(r.byCompact[compact]); len(codes) > 0 {
		return codes
	}
	if strings.Contains(key, "cooperativ") {
		return r.codesWithDescriptionToken("cooperativ", 8)
	}
	return nil
}

func maLegalFormSynonymCodes(key, compact string) []string {
	switch {
	case strings.Contains(compact, "srlsemplificata") || compact == "srls" || strings.Contains(key, "responsabilita limitata semplificata"):
		return []string{"RS"}
	case strings.Contains(compact, "srlunipersonale") || strings.Contains(key, "srl unico socio") || strings.Contains(key, "responsabilita limitata unico socio") || strings.Contains(key, "responsabilita limitata unipersonale"):
		return []string{"SU"}
	case strings.Contains(key, "capitale ridotto") && strings.Contains(key, "responsabilita limitata"):
		return []string{"RR"}
	case compact == "srl" || strings.Contains(key, "societa a responsabilita limitata"):
		return []string{"SR"}
	case strings.Contains(compact, "spasociounico") || strings.Contains(key, "spa socio unico") || strings.Contains(key, "per azioni socio unico"):
		return []string{"AU"}
	case compact == "spa" || strings.Contains(key, "societa per azioni"):
		return []string{"SP"}
	case compact == "sapa" || strings.Contains(key, "accomandita per azioni"):
		return []string{"AA"}
	case compact == "sas" || strings.Contains(key, "accomandita semplice"):
		return []string{"AS"}
	case compact == "snc" || strings.Contains(key, "nome collettivo"):
		return []string{"SN"}
	case strings.Contains(key, "ditta individuale") || strings.Contains(key, "impresa individuale") || strings.Contains(key, "azienda individuale"):
		return []string{"DI"}
	case strings.Contains(key, "persona fisica"):
		return []string{"PF"}
	case strings.Contains(key, "impresa familiare"):
		return []string{"IF"}
	case strings.Contains(key, "cooperativa sociale"):
		return []string{"OO"}
	case compact == "coop" || strings.Contains(key, "cooperativa") || strings.Contains(key, "cooperative"):
		return []string{"SC"}
	default:
		return nil
	}
}

func (r maLegalFormResolver) filterExisting(codes []string) []string {
	if len(codes) == 0 {
		return nil
	}
	out := []string{}
	for _, code := range codes {
		if _, ok := r.byCode[code]; ok {
			out = append(out, code)
		}
	}
	return out
}

func (r maLegalFormResolver) onlyExisting(codes []string) []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, code := range codes {
		if _, ok := r.byCode[code]; !ok {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func (r maLegalFormResolver) codesWithDescriptionToken(token string, max int) []string {
	out := []string{}
	for code, row := range r.byCode {
		key := normalizeMAFoldKey(row.DescriptionIT + " " + row.DescriptionEN)
		if !strings.Contains(key, token) {
			continue
		}
		out = append(out, code)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := r.rank[out[i]]
		right := r.rank[out[j]]
		if left == right {
			return out[i] < out[j]
		}
		return left < right
	})
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out
}

func (s *maService) draftStrategy(ctx context.Context, prompt string, modelID string, promptID string, subject, email string) (MAStrategySpec, llm.CallAudit, error) {
	if s.llmp == nil {
		return MAStrategySpec{}, llm.CallAudit{}, errMAOpenRouterUnavailable
	}
	if s.store == nil {
		return MAStrategySpec{}, llm.CallAudit{}, errMAStoreUnavailable
	}
	if err := validateOptionalUUID(modelID); err != nil {
		return MAStrategySpec{}, llm.CallAudit{}, err
	}
	if err := validateOptionalUUID(promptID); err != nil {
		return MAStrategySpec{}, llm.CallAudit{}, err
	}
	modelConfig, err := s.llmp.ResolveModel(ctx, maModelScopeStrategy, modelID)
	if err != nil {
		return MAStrategySpec{}, llm.CallAudit{}, llmConfigError(err)
	}
	promptConfig, err := s.llmp.ResolvePrompt(ctx, maModelScopeStrategy, promptID)
	if err != nil {
		return MAStrategySpec{}, llm.CallAudit{}, llmConfigError(err)
	}
	client, err := s.llmp.ClientForModel(ctx, modelConfig)
	if err != nil {
		return MAStrategySpec{}, llm.CallAudit{}, llmConfigError(err)
	}
	// Sampling params are dynamic, from the model's DB config; the scope default
	// max_tokens applies only when the config omits it.
	reqParams := modelConfig.RawParams()
	if _, ok := reqParams["max_tokens"]; !ok {
		reqParams["max_tokens"] = 1800
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_strategy_draft_config",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"model_id":     modelConfig.ID,
			"model_scope":  modelConfig.Scope,
			"model":        modelConfig.Model,
			"prompt_id":    promptConfig.ID,
			"prompt_scope": promptConfig.Scope,
			"prompt_name":  promptConfig.Name,
			"prompt":       promptConfig.Prompt,
		}),
	}); err != nil {
		return MAStrategySpec{}, llm.CallAudit{}, err
	}
	messages := []llm.Message{
		{Role: "system", Content: promptConfig.Prompt},
		{Role: "developer", Content: maStrategyToolInstructions()},
		{Role: "user", Content: prompt},
	}
	allowedAteco := map[string]AtecoCode{}
	allowedProvinces := map[string]openapiit.Province{}
	tools := []llm.Tool{maAtecoSearchTool(), maProvinceRegionTool(), maCompanySurfaceProbeTool()}
	var response llm.ChatResponse
	var responseRaw json.RawMessage
	var req llm.ChatRequest
	for round := 0; round <= maMaxToolRounds; round++ {
		req = llm.ChatRequest{
			Model:          modelConfig.Model,
			Params:         reqParams,
			ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
			Messages:       messages,
			Tools:          tools,
			ToolChoice:     "auto",
		}
		start := time.Now()
		response, err = client.Chat(ctx, req)
		duration := maTraceDuration(start)
		if err != nil {
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "openrouter_chat",
				Round:          maTraceRound(round),
				ExternalSystem: "openrouter",
				Status:         maTraceEventFailed,
				DurationMS:     duration,
				Request:        maTraceJSON(req),
				Error:          err.Error(),
			})
			return MAStrategySpec{}, llm.CallAudit{}, err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "openrouter_chat",
			Round:          maTraceRound(round),
			ExternalSystem: "openrouter",
			Status:         maTraceEventSucceeded,
			DurationMS:     duration,
			Request:        maTraceJSON(req),
			Response:       maTraceJSON(response),
			Metadata:       maTraceJSON(map[string]any{"tool_call_count": len(response.ToolCalls), "response_id": response.ID, "response_model": response.Model}),
		}); err != nil {
			return MAStrategySpec{}, llm.CallAudit{}, err
		}
		if len(response.ToolCalls) == 0 {
			responseRaw = json.RawMessage([]byte(strings.TrimSpace(response.Content)))
			break
		}
		if round == maMaxToolRounds {
			err := fmt.Errorf("%w: ateco tool loop", errMAStrategyInvalid)
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType: "ma_strategy_tool_loop_limit",
				Round:     maTraceRound(round),
				Status:    maTraceEventFailed,
				Metadata:  maTraceJSON(map[string]any{"max_tool_rounds": maMaxToolRounds, "tool_call_count": len(response.ToolCalls)}),
				Error:     err.Error(),
			})
			return MAStrategySpec{}, llm.CallAudit{}, err
		}
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})
		for _, call := range response.ToolCalls {
			start := time.Now()
			content := s.executeMAStrategyTool(ctx, call, allowedAteco, allowedProvinces, subject, email)
			if err := s.traceEvent(ctx, maTraceEventWrite{
				EventType:  "ma_strategy_tool",
				Round:      maTraceRound(round),
				ToolName:   call.Function.Name,
				Status:     maToolResultStatus(content),
				DurationMS: maTraceDuration(start),
				Request:    maTraceJSON(call),
				Response:   maTraceRawJSON([]byte(content)),
				Metadata: maTraceJSON(map[string]any{
					"allowed_ateco_count":    len(allowedAteco),
					"allowed_province_count": len(allowedProvinces),
				}),
			}); err != nil {
				return MAStrategySpec{}, llm.CallAudit{}, err
			}
			messages = append(messages, llm.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    content,
			})
		}
	}
	strategy, err := decodeMAStrategyResponse(responseRaw)
	if err != nil {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_strategy_decode",
			Status:    maTraceEventFailed,
			Response:  maTraceJSON(responseRaw),
			Error:     err.Error(),
		})
		return MAStrategySpec{}, llm.CallAudit{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_strategy_decode",
		Status:    maTraceEventSucceeded,
		Response:  maTraceJSON(strategy),
	}); err != nil {
		return MAStrategySpec{}, llm.CallAudit{}, err
	}
	strategy, err = s.canonicalizeMAStrategyProvinces(strategy, allowedProvinces, true)
	if err != nil {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_strategy_province_canonicalization",
			Status:    maTraceEventFailed,
			Metadata:  maTraceJSON(map[string]any{"allowed_province_count": len(allowedProvinces)}),
			Error:     err.Error(),
		})
		return MAStrategySpec{}, llm.CallAudit{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_strategy_province_canonicalization",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"allowed_province_count": len(allowedProvinces), "provinces": strategy.Provinces}),
	}); err != nil {
		return MAStrategySpec{}, llm.CallAudit{}, err
	}
	strategy, err = s.canonicalizeMAStrategyAteco(ctx, strategy, allowedAteco, true)
	if err != nil {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_strategy_ateco_canonicalization",
			Status:    maTraceEventFailed,
			Metadata:  maTraceJSON(map[string]any{"allowed_ateco_count": len(allowedAteco)}),
			Error:     err.Error(),
		})
		return MAStrategySpec{}, llm.CallAudit{}, err
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_strategy_ateco_canonicalization",
		Status:    maTraceEventSucceeded,
		Response:  maTraceJSON(strategy.AtecoCandidates),
		Metadata:  maTraceJSON(map[string]any{"allowed_ateco_count": len(allowedAteco), "candidate_count": len(strategy.AtecoCandidates)}),
	}); err != nil {
		return MAStrategySpec{}, llm.CallAudit{}, err
	}
	usageRaw, _ := json.Marshal(response.Usage)
	// request is the exact wire body of the final call (BuildRequestBody = what Chat
	// sends). messages grow across tool rounds and the loop breaks on the no-tool-call
	// response, so the last req carries the whole exchange (provider model + full
	// conversation + tools + dynamic params). FKs live in their own columns.
	requestBody, _ := llm.BuildRequestBody(req)
	requestRaw, _ := json.Marshal(requestBody)
	return strategy, llm.CallAudit{
		App:          maApp,
		Scope:        maModelScopeStrategy,
		ProviderID:   modelConfig.ProviderID,
		ModelID:      modelConfig.ID,
		PromptID:     promptConfig.ID,
		Model:        modelConfig.Model,
		Request:      requestRaw,
		Response:     responseRaw,
		Usage:        usageRaw,
		ActorSubject: subject,
		ActorEmail:   email,
	}, nil
}

func decodeMAStrategyResponse(responseRaw json.RawMessage) (MAStrategySpec, error) {
	if len(strings.TrimSpace(string(responseRaw))) == 0 {
		return MAStrategySpec{}, fmt.Errorf("%w: empty strategy response", errMAStrategyInvalid)
	}
	var envelope maStrategyDraftEnvelope
	if err := json.Unmarshal(responseRaw, &envelope); err != nil || strings.TrimSpace(envelope.Strategy.SectorDescription) == "" {
		var direct MAStrategySpec
		if directErr := json.Unmarshal(responseRaw, &direct); directErr != nil {
			if err != nil {
				return MAStrategySpec{}, fmt.Errorf("decode ma strategy: %w", err)
			}
			return MAStrategySpec{}, fmt.Errorf("decode ma strategy: %w", directErr)
		}
		envelope.Strategy = direct
	}
	strategy, err := validateMAStrategy(envelope.Strategy)
	if err != nil {
		return MAStrategySpec{}, err
	}
	return strategy, nil
}

func maStrategyToolInstructions() string {
	return strings.Join([]string{
		"Per compilare atecoCandidates devi usare il tool search_ateco_2025.",
		"Non inventare codici ATECO e non usare codici che non compaiono nei risultati del tool.",
		"Per compilare provinces devi usare il tool list_italian_provinces_regions.",
		"Usa solo sigle provincia restituite dal tool province/regioni; se l'utente indica una regione, espandila nelle province restituite dal tool.",
		"Non inventare appartenenze provincia-regione, macro-aree o sigle non presenti nel tool.",
		"Usa il tool probe_company_search_surface per verificare se una combinazione di provincia, ATECO, fatturato e dipendenti e' praticabile prima di proporla.",
		"Il probe e' solo dry-run e misura la superficie potenziale: non usare searchLimit per restringere questa valutazione.",
		"Se probe_company_search_surface restituisce surfaceStatus=too_broad, restringi territorio, settore o range economici prima della risposta finale.",
		"Se la prima ricerca e' troppo generica, fai piu' chiamate tool mirate con termini italiani come programmazione informatica, consulenza informatica, hosting, elaborazione dati.",
		"Deduci la tesi d'acquisizione (thesis) dalla richiesta: successione, crescita, consolidamento, tuck_in oppure generico se non emerge.",
		"Popola legalForms solo se l'utente richiede esplicitamente una forma societaria; e' un filtro, non un criterio di valutazione.",
		"Non generare scoringCriteria. Per enfatizzare un segnale usa signalWeights solo con id del catalogo (ateco_precision, turnover_proximity, keyword_match, ownership_concentration, company_age, legal_form, turnover_trend, productivity, equity_solidity); non inventare id.",
		"La risposta finale deve restare JSON valido nel formato richiesto dal prompt di sistema.",
	}, "\n")
}

func maAtecoSearchTool() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        maAtecoToolName,
			Description: "Cerca codici ATECO 2025 reali nella tabella Binocolo. Restituisce solo codici validi per la selezione ATECO.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Termini di ricerca in italiano, per esempio 'programmazione consulenza informatica hosting'.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Numero massimo di risultati. Default 50, hard cap backend 100.",
						"minimum":     1,
						"maximum":     atecoSearchHardLimit,
					},
				},
			},
		},
	}
}

func maAtecoChildrenTool() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        maAtecoChildrenToolName,
			Description: "Restituisce tutti i discendenti ATECO di un codice gia' visto nel resolver gerarchico, escluso il codice richiesto. Non fa ricerca testuale.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"code"},
				"properties": map[string]any{
					"code": map[string]any{
						"type":        "string",
						"description": "Codice ATECO gia' presente nelle divisioni iniziali o restituito prima da list_ateco_children.",
					},
				},
			},
		},
	}
}

func maProvinceRegionTool() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        maProvinceRegionToolName,
			Description: "Restituisce province italiane reali e rispettive regioni dai dati OpenAPI.it cacheati in Binocolo.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"region": map[string]any{
						"type":        "string",
						"description": "Nome regione da filtrare, per esempio Lombardia. Ometti per tutte le regioni.",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "Filtro opzionale su sigla, nome provincia o regione.",
					},
				},
			},
		},
	}
}

func maCompanySurfaceProbeTool() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        maCompanySurfaceToolName,
			Description: "Esegue un dry-run Company IT-search senza limit operativo e misura se la superficie e' esatta o troppo ampia.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"province": map[string]any{
						"type":        "string",
						"description": "Sigla provincia italiana, per esempio MI. Ometti per ricerca nazionale.",
					},
					"atecoCode": map[string]any{
						"type":        "string",
						"description": "Codice ATECO restituito prima da search_ateco_2025. Ometti per ricerca espansa.",
					},
					"activityStatus": map[string]any{
						"type":        "string",
						"description": "Stato attivita'. Default ATTIVA.",
					},
					"minTurnover": map[string]any{
						"type":        "integer",
						"description": "Fatturato minimo.",
						"minimum":     0,
					},
					"maxTurnover": map[string]any{
						"type":        "integer",
						"description": "Fatturato massimo.",
						"minimum":     0,
					},
					"minEmployees": map[string]any{
						"type":        "integer",
						"description": "Dipendenti minimi.",
						"minimum":     0,
					},
					"maxEmployees": map[string]any{
						"type":        "integer",
						"description": "Dipendenti massimi.",
						"minimum":     0,
					},
				},
			},
		},
	}
}

type maAtecoToolArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type maAtecoChildrenToolArgs struct {
	Code string `json:"code"`
}

type maProvinceRegionToolArgs struct {
	Region string `json:"region,omitempty"`
	Query  string `json:"query,omitempty"`
}

type maProvinceToolItem struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

type maRegionToolItem struct {
	Name      string   `json:"name"`
	Provinces []string `json:"provinces"`
}

type maCompanySurfaceToolArgs struct {
	Province       string `json:"province,omitempty"`
	AtecoCode      string `json:"atecoCode,omitempty"`
	ActivityStatus string `json:"activityStatus,omitempty"`
	MinTurnover    *int   `json:"minTurnover,omitempty"`
	MaxTurnover    *int   `json:"maxTurnover,omitempty"`
	MinEmployees   *int   `json:"minEmployees,omitempty"`
	MaxEmployees   *int   `json:"maxEmployees,omitempty"`
}

func (s *maService) executeMAStrategyTool(ctx context.Context, call llm.ToolCall, allowedAteco map[string]AtecoCode, allowedProvinces map[string]openapiit.Province, subject, email string) string {
	switch call.Function.Name {
	case maAtecoToolName:
		return s.executeMAAtecoTool(ctx, call, allowedAteco)
	case maProvinceRegionToolName:
		return s.executeMAProvinceRegionTool(ctx, call, allowedProvinces)
	case maCompanySurfaceToolName:
		return s.executeMACompanySurfaceTool(ctx, call, allowedAteco, allowedProvinces, subject, email)
	default:
		return maToolErrorJSON("unsupported_tool")
	}
}

func (s *maService) executeMAAtecoTool(ctx context.Context, call llm.ToolCall, allowed map[string]AtecoCode) string {
	if s.ateco == nil {
		return maToolErrorJSON("ateco_not_configured")
	}
	var args maAtecoToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return maToolErrorJSON("invalid_arguments")
	}
	items, err := s.ateco.SearchAteco(ctx, args.Query, normalizeAtecoSearchLimit(args.Limit))
	if err != nil {
		return maToolErrorJSON("search_failed")
	}
	for _, item := range items {
		rememberAllowedAteco(allowed, item)
	}
	raw, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return maToolErrorJSON("encode_failed")
	}
	return string(raw)
}

func (s *maService) executeMAAtecoChildrenTool(ctx context.Context, call llm.ToolCall, allowed map[string]AtecoCode) string {
	if call.Function.Name != maAtecoChildrenToolName {
		return maToolErrorJSON("unsupported_tool")
	}
	if s.ateco == nil {
		return maToolErrorJSON("ateco_not_configured")
	}
	var args maAtecoChildrenToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return maToolErrorJSON("invalid_arguments")
	}
	searchCode := atecoSearchCode(args.Code)
	if searchCode == "" {
		return maToolErrorJSON("invalid_code")
	}
	parent, ok := allowed[searchCode]
	if !ok {
		return maToolErrorJSON("code_not_allowed")
	}
	children, err := s.ateco.AtecoChildren(ctx, parent.Codice)
	if errors.Is(err, errAtecoCodeNotFound) {
		return maToolErrorJSON("code_not_found")
	}
	if err != nil {
		return maToolErrorJSON("children_failed")
	}
	for _, child := range children {
		rememberAllowedAteco(allowed, child.AtecoCode)
	}
	raw, err := json.Marshal(map[string]any{"items": maAtecoHierarchyToolCodes(children)})
	if err != nil {
		return maToolErrorJSON("encode_failed")
	}
	return string(raw)
}

func (s *maService) executeMAProvinceRegionTool(ctx context.Context, call llm.ToolCall, allowed map[string]openapiit.Province) string {
	var args maProvinceRegionToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return maToolErrorJSON("invalid_arguments")
	}
	envelope, _, err := listProvincesWithCache(ctx, s.provinceCache, s.openapiit, s.now)
	if err != nil {
		return maToolErrorJSON("provinces_unavailable")
	}

	regionFilter := cleanText(args.Region, 80)
	queryFilter := cleanText(args.Query, 80)
	provinces := make([]maProvinceToolItem, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		item.Sigla = strings.ToUpper(strings.TrimSpace(item.Sigla))
		item.Provincia = strings.TrimSpace(item.Provincia)
		item.Regione = strings.TrimSpace(item.Regione)
		if item.Sigla == "" || item.Provincia == "" || item.Regione == "" {
			continue
		}
		if !matchesProvinceToolFilter(item, regionFilter, queryFilter) {
			continue
		}
		rememberAllowedProvince(allowed, item)
		provinces = append(provinces, maProvinceToolItem{
			Code:   item.Sigla,
			Name:   item.Provincia,
			Region: item.Regione,
		})
	}
	sort.SliceStable(provinces, func(i, j int) bool {
		if provinces[i].Region == provinces[j].Region {
			return provinces[i].Code < provinces[j].Code
		}
		return provinces[i].Region < provinces[j].Region
	})

	regionsByName := map[string][]string{}
	for _, item := range provinces {
		regionsByName[item.Region] = append(regionsByName[item.Region], item.Code)
	}
	regionNames := make([]string, 0, len(regionsByName))
	for name := range regionsByName {
		regionNames = append(regionNames, name)
	}
	sort.Strings(regionNames)
	regions := make([]maRegionToolItem, 0, len(regionNames))
	for _, name := range regionNames {
		codes := regionsByName[name]
		sort.Strings(codes)
		regions = append(regions, maRegionToolItem{Name: name, Provinces: codes})
	}

	raw, err := json.Marshal(map[string]any{
		"regions":   regions,
		"provinces": provinces,
	})
	if err != nil {
		return maToolErrorJSON("encode_failed")
	}
	return string(raw)
}

func (s *maService) executeMACompanySurfaceTool(ctx context.Context, call llm.ToolCall, allowed map[string]AtecoCode, allowedProvinces map[string]openapiit.Province, subject, email string) string {
	var args maCompanySurfaceToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return maToolErrorJSON("invalid_arguments")
	}
	province, ok := normalizeProvince(args.Province)
	if !ok {
		return maToolErrorJSON("invalid_province")
	}
	if allowedProvinces != nil && province != "" {
		if _, ok := allowedProvinces[province]; !ok {
			return maToolErrorJSON("province_not_allowed")
		}
	}
	activityStatus, ok := normalizeCompanyActivityStatus(args.ActivityStatus)
	if !ok {
		return maToolErrorJSON("invalid_activity_status")
	}
	if activityStatus == "" {
		activityStatus = "ATTIVA"
	}
	if err := validateOptionalRange(args.MinTurnover, args.MaxTurnover, "turnover"); err != nil {
		return maToolErrorJSON("invalid_turnover_range")
	}
	if err := validateOptionalRange(args.MinEmployees, args.MaxEmployees, "employees"); err != nil {
		return maToolErrorJSON("invalid_employees_range")
	}

	dryRun := 1
	params := openapiit.CompanyITSearchParams{
		DryRun:         &dryRun,
		DataEnrichment: "advanced",
		Province:       province,
		ActivityStatus: activityStatus,
		MinTurnover:    args.MinTurnover,
		MaxTurnover:    args.MaxTurnover,
		MinEmployees:   args.MinEmployees,
		MaxEmployees:   args.MaxEmployees,
	}
	if strings.TrimSpace(args.AtecoCode) != "" {
		item, ok := allowed[atecoSearchCode(args.AtecoCode)]
		if !ok || item.CodiceSearch == "" {
			return maToolErrorJSON("ateco_not_allowed")
		}
		params.AtecoCode = item.CodiceSearch
	}
	result, err := s.probeMASearchSurface(ctx, params, subject, email)
	if err != nil {
		return maToolErrorJSON("surface_probe_failed")
	}
	raw, err := json.Marshal(map[string]any{
		"surfaceStatus":  result.Status,
		"estimatedCount": result.EstimatedCount,
		"lowerBound":     false,
		"tooBroad":       result.Status == maEstimateSurfaceTooBroad,
		"estimatedCost":  result.EstimatedCost,
		"probeCount":     result.ProbeCount,
		"params":         result.Params,
	})
	if err != nil {
		return maToolErrorJSON("encode_failed")
	}
	return string(raw)
}

func matchesProvinceToolFilter(item openapiit.Province, regionFilter string, queryFilter string) bool {
	if regionFilter != "" && !textContainsFold(item.Regione, regionFilter) {
		return false
	}
	if queryFilter == "" {
		return true
	}
	return textContainsFold(item.Sigla, queryFilter) ||
		textContainsFold(item.Provincia, queryFilter) ||
		textContainsFold(item.Regione, queryFilter)
}

func textContainsFold(value, query string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	query = strings.ToLower(strings.TrimSpace(query))
	return query == "" || strings.Contains(value, query)
}

func maToolErrorJSON(code string) string {
	raw, _ := json.Marshal(map[string]string{"error": code})
	return string(raw)
}

func maAtecoChildrenToolParent(call llm.ToolCall) string {
	var args maAtecoChildrenToolArgs
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return ""
	}
	return normalizeAtecoCode(args.Code)
}

func maAtecoChildrenToolResultCount(content string) int {
	var result struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return 0
	}
	return len(result.Items)
}

func rememberAllowedAteco(allowed map[string]AtecoCode, item AtecoCode) {
	if allowed == nil {
		return
	}
	if item.Codice != "" {
		allowed[atecoSearchCode(item.Codice)] = item
	}
	if item.CodiceSearch != "" {
		allowed[atecoSearchCode(item.CodiceSearch)] = item
	}
}

func rememberAllowedProvince(allowed map[string]openapiit.Province, item openapiit.Province) {
	if allowed == nil {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(item.Sigla))
	if code == "" {
		return
	}
	item.Sigla = code
	item.Provincia = strings.TrimSpace(item.Provincia)
	item.Regione = strings.TrimSpace(item.Regione)
	allowed[code] = item
}

func (s *maService) canonicalizeMAStrategyProvinces(strategy MAStrategySpec, allowed map[string]openapiit.Province, requireAllowed bool) (MAStrategySpec, error) {
	if len(strategy.Provinces) == 0 {
		return strategy, nil
	}
	provinces := make([]string, 0, len(strategy.Provinces))
	seen := map[string]struct{}{}
	for _, raw := range strategy.Provinces {
		province, ok := normalizeProvince(raw)
		if !ok {
			return MAStrategySpec{}, fmt.Errorf("%w: province", errMAStrategyInvalid)
		}
		if province == "" {
			continue
		}
		if requireAllowed {
			if _, ok := allowed[province]; !ok {
				return MAStrategySpec{}, fmt.Errorf("%w: province not returned by tool", errMAStrategyInvalid)
			}
		}
		if _, exists := seen[province]; exists {
			continue
		}
		seen[province] = struct{}{}
		provinces = append(provinces, province)
	}
	sort.Strings(provinces)
	strategy.Provinces = provinces
	if strategy.TerritoryLabel == "" && len(provinces) > 0 {
		strategy.TerritoryLabel = strings.Join(provinces, ", ")
	}
	return strategy, nil
}

func (s *maService) canonicalizeMAStrategyAteco(ctx context.Context, strategy MAStrategySpec, allowed map[string]AtecoCode, requireAllowed bool) (MAStrategySpec, error) {
	if len(strategy.AtecoCandidates) == 0 {
		return strategy, nil
	}
	candidates := make([]MAAtecoCandidate, 0, len(strategy.AtecoCandidates))
	seen := map[string]struct{}{}
	for _, candidate := range strategy.AtecoCandidates {
		code := normalizeAtecoCode(candidate.Code)
		if code == "" {
			continue
		}
		fit := normalizeMAFit(candidate.Fit)
		// Excluded entries are used only as subtree PREFIXES (pruning + gate), so they
		// are normalized but NOT resolved against the DB — they may legitimately be a
		// non-leaf prefix (e.g. 63.10.2) that is not an exact ATECO node.
		if fit == maFitExcluded {
			searchCode := atecoSearchCode(code)
			if searchCode == "" {
				continue
			}
			if _, exists := seen[searchCode]; exists {
				continue
			}
			seen[searchCode] = struct{}{}
			candidates = append(candidates, MAAtecoCandidate{
				Code:        code,
				Description: cleanText(candidate.Description, 180),
				Rationale:   cleanText(candidate.Rationale, 240),
				Fit:         maFitExcluded,
				SearchCode:  searchCode,
			})
			continue
		}
		var item AtecoCode
		var ok bool
		if requireAllowed {
			item, ok = allowed[atecoSearchCode(code)]
			if !ok {
				return MAStrategySpec{}, fmt.Errorf("%w: ateco candidate not returned by tool", errMAStrategyInvalid)
			}
		} else {
			if s.ateco == nil {
				return MAStrategySpec{}, errAtecoStoreUnavailable
			}
			resolved, err := s.ateco.ResolveAtecoCode(ctx, code)
			if err != nil {
				return MAStrategySpec{}, err
			}
			item = resolved
		}
		if item.CodiceSearch == "" {
			return MAStrategySpec{}, errAtecoCodeNotFound
		}
		if _, exists := seen[item.CodiceSearch]; exists {
			continue
		}
		seen[item.CodiceSearch] = struct{}{}
		candidates = append(candidates, MAAtecoCandidate{
			Code:        item.Codice,
			Description: item.Titolo,
			Rationale:   cleanText(candidate.Rationale, 240),
			Fit:         fit,
			SearchCode:  item.CodiceSearch,
		})
	}
	strategy.AtecoCandidates = candidates
	// Sector perimeter = the 2-digit divisions of the non-excluded candidates. The
	// curated list (with fit) stays intact for resolveAtecoFit; only the retrieval
	// sets are derived later (expandStrategyAteco / expandStrategyExpansion).
	strategy.SectorDivisions = atecoDivisions(candidates)
	return strategy, nil
}

// expandStrategyAteco replaces each analyst/LLM-selected ATECO sector node with
// the exact codes in its subtree that are actually populated at OpenAPI.it.
// OpenAPI.it matches the ATECO code exactly (no prefix, no descent) and companies
// are tagged at heterogeneous levels per branch, so a selected node (e.g. 62.20)
// is frequently empty while a descendant (62.20.1) holds the companies.
// Discovery is a dry-run probe (count only, €0.01) per subtree code, applying the
// economic filters but neither province nor legal form for maximum cache reuse.
// It is deterministic and cache-backed, so estimate and execution derive the same
// populated set without persisting it onto the stored strategy.
func (s *maService) expandStrategyAteco(ctx context.Context, strategy MAStrategySpec, subject, email string) (MAStrategySpec, error) {
	// Fill the ATECO strategy's retrieval set (AtecoQueryCandidates), leaving the
	// curated AtecoCandidates (with fit) intact for scoring. Default to the core/weak
	// codes as-is; replace with the populated subtree when discovery succeeds.
	strategy.AtecoQueryCandidates = coreWeakQueryFallback(strategy)
	roots := make([]string, 0, len(strategy.AtecoCandidates))
	for _, candidate := range strategy.AtecoCandidates {
		if normalizeMAFit(candidate.Fit) != maFitExcluded {
			roots = append(roots, candidate.Code)
		}
	}
	if s.ateco == nil || len(roots) == 0 {
		return strategy, nil
	}
	populated, subtreeCount, err := s.probePopulatedSubtree(ctx, roots, strategyExcludedSearchCodes(strategy), strategy, subject, email)
	if err != nil {
		return MAStrategySpec{}, err
	}
	if subtreeCount > maAtecoSubtreeProbeCap {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_ateco_discovery_skipped",
			Status:    maTraceEventInfo,
			Metadata:  maTraceJSON(map[string]any{"subtree_codes": subtreeCount, "cap": maAtecoSubtreeProbeCap}),
		})
		return strategy, nil
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_ateco_discovery",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"selected_nodes":  len(roots),
			"subtree_codes":   subtreeCount,
			"populated_codes": len(populated),
		}),
	})
	if len(populated) > 0 {
		strategy.AtecoQueryCandidates = populated
	}
	return strategy, nil
}

// coreWeakQueryFallback returns the non-excluded candidates as a retrieval set
// (their exact codes), used when subtree discovery is unavailable or over the cap.
func coreWeakQueryFallback(strategy MAStrategySpec) []MAAtecoCandidate {
	out := make([]MAAtecoCandidate, 0, len(strategy.AtecoCandidates))
	for _, candidate := range strategy.AtecoCandidates {
		if normalizeMAFit(candidate.Fit) == maFitExcluded {
			continue
		}
		searchCode := candidate.SearchCode
		if searchCode == "" {
			searchCode = atecoSearchCode(candidate.Code)
		}
		out = append(out, MAAtecoCandidate{Code: candidate.Code, Description: candidate.Description, SearchCode: searchCode})
	}
	return out
}

// strategyExcludedSearchCodes returns the dot-stripped search codes of the excluded
// candidates. A descendant/target whose search code has any as a prefix is inside an
// excluded subtree (e.g. excluding 63.10.21 removes it while hosting 63.10.10 stays).
func strategyExcludedSearchCodes(strategy MAStrategySpec) []string {
	out := []string{}
	for _, candidate := range strategy.AtecoCandidates {
		if normalizeMAFit(candidate.Fit) != maFitExcluded {
			continue
		}
		searchCode := candidate.SearchCode
		if searchCode == "" {
			searchCode = atecoSearchCode(candidate.Code)
		}
		if searchCode != "" {
			out = append(out, searchCode)
		}
	}
	return out
}

// probePopulatedSubtree discovers, under the given roots (node + descendants), the
// exact ATECO codes actually populated under the economic filters — dry-run probing
// each (count only, €0.01, cache-backed; province/legal form omitted for cache
// reuse), pruning any excluded subtree. Returns the populated candidates and the
// pre-probe subtree size; when that size exceeds the cap it returns (nil, size, nil)
// so the caller can trace a skip and keep its fallback.
func (s *maService) probePopulatedSubtree(ctx context.Context, roots, excluded []string, strategy MAStrategySpec, subject, email string) ([]MAAtecoCandidate, int, error) {
	type subtreeCode struct{ code, searchCode, titolo string }
	seen := map[string]struct{}{}
	ordered := make([]subtreeCode, 0, len(roots)*4)
	for _, root := range roots {
		descendants, err := s.ateco.SubtreeAtecoCodes(ctx, root)
		if err != nil {
			return nil, 0, err
		}
		for _, descendant := range descendants {
			if descendant.CodiceSearch == "" {
				continue
			}
			if _, exists := seen[descendant.CodiceSearch]; exists {
				continue
			}
			if isExcludedSearchCode(descendant.CodiceSearch, excluded) {
				continue
			}
			seen[descendant.CodiceSearch] = struct{}{}
			ordered = append(ordered, subtreeCode{code: descendant.Codice, searchCode: descendant.CodiceSearch, titolo: descendant.Titolo})
		}
	}
	if len(ordered) == 0 || len(ordered) > maAtecoSubtreeProbeCap {
		return nil, len(ordered), nil
	}
	populated := make([]MAAtecoCandidate, 0, len(ordered))
	for _, item := range ordered {
		params := openapiit.CompanyITSearchParams{
			DataEnrichment: "advanced",
			ActivityStatus: strategy.ActivityStatus,
			MinTurnover:    strategy.TurnoverMin,
			MaxTurnover:    strategy.TurnoverMax,
			MinEmployees:   strategy.EmployeeMin,
			MaxEmployees:   strategy.EmployeeMax,
			AtecoCode:      item.searchCode,
		}
		surface, err := s.probeMASearchSurface(ctx, params, subject, email)
		if err != nil {
			return nil, 0, err
		}
		if surface.EstimatedCount <= 0 {
			continue
		}
		populated = append(populated, MAAtecoCandidate{Code: item.code, Description: item.titolo, SearchCode: item.searchCode})
	}
	return populated, len(ordered), nil
}

func isExcludedSearchCode(searchCode string, excluded []string) bool {
	for _, ex := range excluded {
		if strings.HasPrefix(searchCode, ex) {
			return true
		}
	}
	return false
}

// expandStrategyExpansion computes the "expanded" search's ATECO code set: the
// populated subtree of the sector divisions, minus any excluded subtree. It
// replaces the old expanded behaviour — drop the ATECO filter entirely, which
// retrieved the whole provincial economy — with a sector-bounded net that still
// catches mis-tagged siblings inside the IT divisions (a real MSP tagged 62.09
// rather than 62.20). Like expandStrategyAteco it probes dry-run (count only,
// €0.01, cache-backed) applying the economic filters but neither province nor
// legal form, and respects the subtree probe cap. The result lives only on the
// transient ExpandedAtecoCandidates field; an empty result leaves the legacy
// province-only fallback in place (sector-less strategies).
func (s *maService) expandStrategyExpansion(ctx context.Context, strategy MAStrategySpec, subject, email string) (MAStrategySpec, error) {
	if s.ateco == nil || len(strategy.SectorDivisions) == 0 {
		return strategy, nil
	}
	populated, subtreeCount, err := s.probePopulatedSubtree(ctx, strategy.SectorDivisions, strategyExcludedSearchCodes(strategy), strategy, subject, email)
	if err != nil {
		return MAStrategySpec{}, err
	}
	if subtreeCount > maAtecoSubtreeProbeCap {
		_ = s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_expansion_discovery_skipped",
			Status:    maTraceEventInfo,
			Metadata:  maTraceJSON(map[string]any{"divisions": strategy.SectorDivisions, "subtree_codes": subtreeCount, "cap": maAtecoSubtreeProbeCap}),
		})
		return strategy, nil
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_expansion_discovery",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"divisions":       strategy.SectorDivisions,
			"subtree_codes":   subtreeCount,
			"populated_codes": len(populated),
		}),
	})
	strategy.ExpandedAtecoCandidates = populated
	return strategy, nil
}

func (s *maService) runEstimates(ctx context.Context, sessionID, strategyVersionID string, strategy MAStrategySpec, strategyTypeFilter string, subject, email string) ([]MAEstimate, string, error) {
	if strategyTypeFilter == maStrategyTypeExpanded && len(strategy.SectorDivisions) == 0 {
		return nil, "", fmt.Errorf("%w: perimetro settoriale mancante", errMAStrategyInvalid)
	}
	queries := buildMAEstimateQueries(strategy)
	if strategyTypeFilter != "" {
		filtered := queries[:0]
		for _, query := range queries {
			if query.strategyType == strategyTypeFilter {
				filtered = append(filtered, query)
			}
		}
		queries = filtered
	}
	estimates := make([]MAEstimate, len(queries))
	pricing := s.loadPricing(ctx)

	// Probe the (province × ateco × legal form) surface concurrently with a bounded
	// pool: each combo is an independent dry-run, so the previously-sequential fan-out
	// (the source of the request timeout) collapses to roughly the slowest probe.
	// Results are written by index, so the slice needs no synchronization.
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sem := make(chan struct{}, maEstimateProbeConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	failOnce := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
			cancel() // stop probes still waiting on the semaphore
		}
		mu.Unlock()
	}
	for index := range queries {
		wg.Add(1)
		go func(index int, query maSearchQuery) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-probeCtx.Done():
				return
			}
			defer func() { <-sem }()
			query.params = baseMASurfaceParams(strategy, query.province, query.legalForm, query.params.DryRun)
			if query.atecoSearchCode != "" {
				query.params.AtecoCode = query.atecoSearchCode
			}
			if err := s.traceEvent(probeCtx, maTraceEventWrite{
				EventType: "ma_estimate_query",
				Status:    maTraceEventStarted,
				Request:   maTraceJSON(query.params.Values()),
				Metadata: maTraceJSON(map[string]any{
					"session_id":          sessionID,
					"strategy_version_id": strategyVersionID,
					"strategy_type":       query.strategyType,
					"province":            query.province,
					"ateco_code":          query.atecoCode,
					"legal_form":          query.legalForm,
				}),
			}); err != nil {
				failOnce(err)
				return
			}
			surface, err := s.probeMASearchSurface(probeCtx, query.params, subject, email)
			if err != nil {
				failOnce(err)
				return
			}
			estimates[index] = MAEstimate{
				ID:                uuid.NewString(),
				SessionID:         sessionID,
				StrategyVersionID: strategyVersionID,
				StrategyType:      query.strategyType,
				AtecoCode:         query.atecoCode,
				AtecoDescription:  query.atecoDescription,
				Province:          query.province,
				EstimatedCount:    surface.EstimatedCount,
				EstimatedCost:     float64(surface.EstimatedCount) * pricing.CostAdvanced,
				Selected:          false,
				SurfaceStatus:     surface.Status,
				ExecutionLimit:    strategy.SearchLimit,
				ProbeCount:        surface.ProbeCount,
				Params:            surface.Params,
				VendorResponse:    surface.VendorResponse,
			}
		}(index, queries[index])
	}
	wg.Wait()
	if firstErr != nil {
		return nil, "", firstErr
	}
	estimates = aggregateExpandedEstimates(estimates, strategy, pricing.CostAdvanced)
	selected := chooseSelectedStrategyFromEstimates(estimates, len(strategy.AtecoQueryCandidates) > 0)
	if strategyTypeFilter != "" {
		selected = strategyTypeFilter
	}
	for index := range estimates {
		estimates[index].Selected = selected != "" && estimates[index].StrategyType == selected
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_estimates_completed",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"session_id":          sessionID,
			"strategy_version_id": strategyVersionID,
			"estimate_count":      len(estimates),
			"selected_strategy":   selected,
		}),
	}); err != nil {
		return nil, "", err
	}
	return estimates, selected, nil
}

type maSearchQuery struct {
	strategyType     string
	atecoCode        string
	atecoSearchCode  string
	atecoDescription string
	province         string
	legalForm        string
	params           openapiit.CompanyITSearchParams
}

// maStrategyLegalForms returns the legal-form axis of the search cartesian: the
// explicit perimeter, or a single empty value meaning "any form" (no server-side
// legal-form filter). Because legalFormCode is single-valued at OpenAPI.it, each
// form is a separate query rather than a post-enrichment filter.
func maStrategyLegalForms(strategy MAStrategySpec) []string {
	if len(strategy.LegalForms) == 0 {
		return []string{""}
	}
	return strategy.LegalForms
}

// aggregateExpandedEstimates collapses the per-code expanded estimates into one
// row per province: the sector-bounded net is presented as a single "expanded"
// surface, not a wall of division-code chips. ATECO estimates pass through
// unchanged. Count/probe are summed; cost is recomputed from the sum; the surface
// is too_broad if any part was or the total exceeds the vendor limit.
func aggregateExpandedEstimates(estimates []MAEstimate, strategy MAStrategySpec, costAdvanced float64) []MAEstimate {
	out := make([]MAEstimate, 0, len(estimates))
	byProvince := map[string]int{}
	label := expandedSectorLabel(strategy)
	for _, est := range estimates {
		if est.StrategyType != maStrategyTypeExpanded {
			out = append(out, est)
			continue
		}
		if idx, ok := byProvince[est.Province]; ok {
			out[idx].EstimatedCount += est.EstimatedCount
			out[idx].ProbeCount += est.ProbeCount
			if est.SurfaceStatus == maEstimateSurfaceTooBroad {
				out[idx].SurfaceStatus = maEstimateSurfaceTooBroad
			}
			continue
		}
		merged := est
		merged.AtecoCode = ""
		merged.AtecoDescription = label
		byProvince[est.Province] = len(out)
		out = append(out, merged)
	}
	for i := range out {
		if out[i].StrategyType != maStrategyTypeExpanded {
			continue
		}
		out[i].EstimatedCost = float64(out[i].EstimatedCount) * costAdvanced
		if out[i].EstimatedCount > maVendorLimit {
			out[i].SurfaceStatus = maEstimateSurfaceTooBroad
		}
	}
	return out
}

// expandedSectorLabel describes the expanded net's perimeter for the estimate UI.
func expandedSectorLabel(strategy MAStrategySpec) string {
	if len(strategy.SectorDivisions) == 0 {
		return "Ricerca espansa"
	}
	return "Divisioni ATECO " + strings.Join(strategy.SectorDivisions, ", ")
}

func buildMAEstimateQueries(strategy MAStrategySpec) []maSearchQuery {
	dryRun := 1
	provinces := strategy.Provinces
	if len(provinces) == 0 {
		provinces = []string{""}
	}
	forms := maStrategyLegalForms(strategy)
	queries := make([]maSearchQuery, 0, len(provinces)*(len(strategy.AtecoCandidates)+1)*len(forms))
	for _, province := range provinces {
		for _, form := range forms {
			for _, candidate := range strategy.AtecoQueryCandidates {
				searchCode := candidate.SearchCode
				if searchCode == "" {
					searchCode = atecoSearchCode(candidate.Code)
				}
				params := baseMASurfaceParams(strategy, province, form, &dryRun)
				params.AtecoCode = searchCode
				queries = append(queries, maSearchQuery{
					strategyType:     maStrategyTypeATECO,
					atecoCode:        candidate.Code,
					atecoSearchCode:  searchCode,
					atecoDescription: candidate.Description,
					province:         province,
					legalForm:        form,
					params:           params,
				})
			}
			if len(strategy.ExpandedAtecoCandidates) > 0 {
				// Sector-bounded expanded net: one dry-run per populated division
				// code instead of a single ATECO-less query over the whole province.
				for _, candidate := range strategy.ExpandedAtecoCandidates {
					searchCode := candidate.SearchCode
					if searchCode == "" {
						searchCode = atecoSearchCode(candidate.Code)
					}
					params := baseMASurfaceParams(strategy, province, form, &dryRun)
					params.AtecoCode = searchCode
					queries = append(queries, maSearchQuery{
						strategyType:     maStrategyTypeExpanded,
						atecoCode:        candidate.Code,
						atecoSearchCode:  searchCode,
						atecoDescription: candidate.Description,
						province:         province,
						legalForm:        form,
						params:           params,
					})
				}
			} else {
				// Legacy fallback for sector-less strategies (no divisions to widen):
				// the province-only surface, kept so such searches still run.
				params := baseMASurfaceParams(strategy, province, form, &dryRun)
				queries = append(queries, maSearchQuery{
					strategyType: maStrategyTypeExpanded,
					province:     province,
					legalForm:    form,
					params:       params,
				})
			}
		}
	}
	return queries
}

func (s *maService) probeMASearchSurface(ctx context.Context, params openapiit.CompanyITSearchParams, subject, email string) (maSurfaceProbeResult, error) {
	dryRun := 1
	params.DryRun = &dryRun
	params.Limit = nil
	params.Skip = nil

	first, err := s.runMASurfaceProbe(ctx, params, subject, email)
	if err != nil {
		return maSurfaceProbeResult{}, err
	}
	probes := []maSurfaceProbeAudit{first}
	status := maEstimateSurfaceExact
	estimatedCount := first.Count
	// Projected advanced-enrichment cost of fetching these companies — NOT the
	// dry-run's own spend (first.Cost, ~€0.01), which is tracked in the probe audit.
	estimatedCost := float64(estimatedCount) * maCostPerCompanyEUR
	if first.Count > maVendorLimit {
		status = maEstimateSurfaceTooBroad
	}

	paramsRaw, err := companySearchParamsJSON(params.Values())
	if err != nil {
		return maSurfaceProbeResult{}, err
	}
	responseRaw, err := json.Marshal(map[string]any{"probes": probes})
	if err != nil {
		return maSurfaceProbeResult{}, fmt.Errorf("marshal ma surface probe response: %w", err)
	}
	return maSurfaceProbeResult{
		Status:         status,
		EstimatedCount: estimatedCount,
		EstimatedCost:  estimatedCost,
		ProbeCount:     len(probes),
		Params:         paramsRaw,
		VendorResponse: responseRaw,
	}, nil
}

func (s *maService) runMASurfaceProbe(ctx context.Context, params openapiit.CompanyITSearchParams, subject, email string) (maSurfaceProbeAudit, error) {
	params.Limit = nil
	response, raw, err := s.cachedCompanySearch(ctx, params, subject, email)
	if err != nil {
		return maSurfaceProbeAudit{}, err
	}
	paramsRaw, err := companySearchParamsJSON(params.Values())
	if err != nil {
		return maSurfaceProbeAudit{}, err
	}
	skip := 0
	if params.Skip != nil {
		skip = *params.Skip
	}
	return maSurfaceProbeAudit{
		Skip:     skip,
		Count:    envelopeCount(response),
		Cost:     envelopeCost(response),
		Params:   paramsRaw,
		Response: raw,
	}, nil
}

// maExecutionCombo is one cell of the (province × ateco code × legal form)
// search cartesian. A company matches exactly one combo (one registered
// province, one exact ateco code, one legal form), so the union of combo results
// is distinct and the per-combo dry-run count feeds a fair budget allocation.
type maExecutionCombo struct {
	province   string
	atecoCode  string
	searchCode string
	legalForm  string
	count      int
}

func (s *maService) runExecution(ctx context.Context, strategy MAStrategySpec, strategyType string, limit int, subject, email, enrichment string) ([]MATarget, error) {
	if enrichment == "" {
		enrichment = maEnrichmentAdvanced
	}
	provinces := strategy.Provinces
	if len(provinces) == 0 {
		provinces = []string{""}
	}
	forms := maStrategyLegalForms(strategy)

	combos := []maExecutionCombo{}
	for _, province := range provinces {
		for _, form := range forms {
			if strategyType == maStrategyTypeATECO {
				for _, candidate := range strategy.AtecoQueryCandidates {
					searchCode := candidate.SearchCode
					if searchCode == "" {
						searchCode = atecoSearchCode(candidate.Code)
					}
					combos = append(combos, maExecutionCombo{province: province, atecoCode: candidate.Code, searchCode: searchCode, legalForm: form})
				}
			} else if len(strategy.ExpandedAtecoCandidates) > 0 {
				// Sector-bounded expanded net: enrich only companies under the
				// populated division codes (minus exclusions), never the whole
				// province. Dedup downstream collapses cross-code overlaps.
				for _, candidate := range strategy.ExpandedAtecoCandidates {
					searchCode := candidate.SearchCode
					if searchCode == "" {
						searchCode = atecoSearchCode(candidate.Code)
					}
					combos = append(combos, maExecutionCombo{province: province, atecoCode: candidate.Code, searchCode: searchCode, legalForm: form})
				}
			} else {
				// Legacy province-only fallback (sector-less strategies).
				combos = append(combos, maExecutionCombo{province: province, legalForm: form})
			}
		}
	}

	// Count per combo (dry-run, count only). These hit the cache populated by the
	// estimate phase, so the allocation costs effectively nothing.
	for index := range combos {
		surfaceParams := baseMASurfaceParams(strategy, combos[index].province, combos[index].legalForm, nil)
		if combos[index].searchCode != "" {
			surfaceParams.AtecoCode = combos[index].searchCode
		}
		surface, err := s.probeMASearchSurface(ctx, surfaceParams, subject, email)
		if err != nil {
			return nil, err
		}
		combos[index].count = surface.EstimatedCount
	}

	counts := make([]int, len(combos))
	for index, combo := range combos {
		counts[index] = combo.count
	}
	allocation := allocateExecutionLimit(counts, limit)

	dryRun := 0
	rawTargets := []MATarget{}
	for index, combo := range combos {
		slot := allocation[index]
		if slot <= 0 {
			continue
		}
		params := baseMASearchParams(strategy, combo.province, combo.legalForm, &dryRun, slot, enrichment)
		if combo.searchCode != "" {
			params.AtecoCode = combo.searchCode
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType: "ma_execution_query",
			Status:    maTraceEventStarted,
			Request:   maTraceJSON(params.Values()),
			Metadata:  maTraceJSON(map[string]any{"strategy_type": strategyType, "province": combo.province, "ateco_code": combo.atecoCode, "legal_form": combo.legalForm, "allocated": slot, "available": combo.count}),
		}); err != nil {
			return nil, err
		}
		targets, err := s.executeCompanySearch(ctx, params, subject, email)
		if err != nil {
			return nil, err
		}
		rawTargets = append(rawTargets, targets...)
	}
	targets := dedupeMATargets(rawTargets)
	if len(targets) > limit {
		targets = targets[:limit]
	}
	// Tag each target with the enrichment level it was fetched at, so persistence
	// knows whether it is a scored ("advanced") or identity-only ("address") row.
	for index := range targets {
		targets[index].EnrichmentLevel = enrichment
	}
	if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_execution_queries_completed",
		Status:    maTraceEventSucceeded,
		Metadata:  maTraceJSON(map[string]any{"strategy_type": strategyType, "limit": limit, "combo_count": len(combos), "raw_target_count": len(rawTargets), "deduped_target_count": len(targets)}),
	}); err != nil {
		return nil, err
	}
	return targets, nil
}

// allocateExecutionLimit distributes a fetch limit across search combos using a
// floor (equal guaranteed minimum per non-empty combo) plus proportional shares
// of the remainder (largest-remainder method). When the combos together hold no
// more than the limit, every company is fetched. The result is aligned with the
// input counts and sums to min(limit, totalAvailable). Deterministic: ties break
// by lowest index.
func allocateExecutionLimit(counts []int, limit int) []int {
	slots := make([]int, len(counts))
	if limit <= 0 {
		return slots
	}
	total, nonEmpty := 0, 0
	for _, count := range counts {
		if count > 0 {
			total += count
			nonEmpty++
		}
	}
	if total == 0 {
		return slots
	}
	if total <= limit {
		for index, count := range counts {
			if count > 0 {
				slots[index] = count
			}
		}
		return slots
	}
	// Stage 1 — floor: a small guaranteed minimum per non-empty combo so a named
	// province/form is never fully starved by a larger one. Only when affordable.
	floor := 0
	if limit >= nonEmpty {
		floor = 1
	}
	used := 0
	for index, count := range counts {
		if count <= 0 {
			continue
		}
		give := floor
		if give > count {
			give = count
		}
		slots[index] = give
		used += give
	}
	// Stage 2 — proportional remainder over residual demand (largest-remainder).
	remaining := limit - used
	residualTotal := 0
	for index, count := range counts {
		if count > slots[index] {
			residualTotal += count - slots[index]
		}
	}
	remainder := make([]float64, len(counts))
	if residualTotal > 0 {
		for index, count := range counts {
			capacity := count - slots[index]
			if capacity <= 0 {
				continue
			}
			ideal := float64(remaining) * float64(capacity) / float64(residualTotal)
			give := int(ideal)
			if give > capacity {
				give = capacity
			}
			slots[index] += give
			used += give
			remainder[index] = ideal - float64(int(ideal))
		}
	}
	// Distribute any leftover one unit at a time by largest remainder; on ties
	// prefer the least-filled combo so units spread rather than pile on index 0.
	for used < limit {
		best := -1
		for index, count := range counts {
			if count <= slots[index] {
				continue
			}
			if best == -1 {
				best = index
				continue
			}
			if remainder[index] > remainder[best] ||
				(remainder[index] == remainder[best] && slots[index] < slots[best]) {
				best = index
			}
		}
		if best == -1 {
			break
		}
		slots[best]++
		used++
	}
	return slots
}

func (s *maService) executeCompanySearch(ctx context.Context, params openapiit.CompanyITSearchParams, subject, email string) ([]MATarget, error) {
	response, _, err := s.cachedCompanySearch(ctx, params, subject, email)
	if err != nil {
		return nil, err
	}
	return parseMATargetsFromVendorData(response.Data)
}

func (s *maService) cachedCompanySearch(ctx context.Context, params openapiit.CompanyITSearchParams, subject, email string) (openapiit.Envelope[openapiit.CompanyDataset], json.RawMessage, error) {
	cacheKey, paramsJSON, err := companySearchCacheKey(params)
	if err != nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
	}
	now := s.now()
	if s.searchCache != nil {
		entry, err := s.searchCache.GetValidCompanySearch(ctx, cacheKey, now)
		if err != nil {
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
		if entry != nil {
			envelope, err := decodeCompanySearchEnvelope(entry.Response)
			status := maTraceEventSucceeded
			errMessage := ""
			if err != nil {
				status = maTraceEventFailed
				errMessage = err.Error()
			}
			if traceErr := s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "company_search_cache_hit",
				ExternalSystem: "binocolo_cache",
				Status:         status,
				Request:        maTraceJSON(paramsJSON),
				Response:       maTraceJSON(entry.Response),
				Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey, "count": envelopeCount(envelope), "cost": envelopeCost(envelope)}),
				Error:          errMessage,
			}); traceErr != nil {
				return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, traceErr
			}
			return envelope, entry.Response, err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "company_search_cache_miss",
			ExternalSystem: "binocolo_cache",
			Status:         maTraceEventInfo,
			Request:        maTraceJSON(paramsJSON),
			Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
		}); err != nil {
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
	} else if err := s.traceEvent(ctx, maTraceEventWrite{
		EventType:      "company_search_cache_disabled",
		ExternalSystem: "binocolo_cache",
		Status:         maTraceEventInfo,
		Request:        maTraceJSON(paramsJSON),
		Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
	}); err != nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
	}
	if s.openapiit == nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, errMAOpenAPIITUnavailable
	}

	fetch := func(ctx context.Context) (openapiit.Envelope[openapiit.CompanyDataset], json.RawMessage, error) {
		start := time.Now()
		response, err := s.openapiit.Company().SearchITRaw(ctx, params)
		if err != nil {
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "company_search_upstream",
				ExternalSystem: "openapiit",
				Status:         maTraceEventFailed,
				DurationMS:     maTraceDuration(start),
				Request:        maTraceJSON(paramsJSON),
				Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
				Error:          err.Error(),
			})
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
		raw, err := json.Marshal(response)
		if err != nil {
			_ = s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "company_search_upstream",
				ExternalSystem: "openapiit",
				Status:         maTraceEventFailed,
				DurationMS:     maTraceDuration(start),
				Request:        maTraceJSON(paramsJSON),
				Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
				Error:          err.Error(),
			})
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "company_search_upstream",
			ExternalSystem: "openapiit",
			Status:         maTraceEventSucceeded,
			DurationMS:     maTraceDuration(start),
			Request:        maTraceJSON(paramsJSON),
			Response:       maTraceJSON(raw),
			Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey, "count": envelopeCount(response), "cost": envelopeCost(response)}),
		}); err != nil {
			return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
		}
		return response, raw, nil
	}

	if s.searchCache == nil {
		response, raw, err := fetch(ctx)
		return response, raw, err
	}

	var response openapiit.Envelope[openapiit.CompanyDataset]
	var raw json.RawMessage
	var upstreamErr error
	err = s.searchCache.WithCompanySearchCacheLock(ctx, cacheKey, func(ctx context.Context) error {
		entry, err := s.searchCache.GetValidCompanySearch(ctx, cacheKey, s.now())
		if err != nil {
			return err
		}
		if entry != nil {
			response, err = decodeCompanySearchEnvelope(entry.Response)
			raw = entry.Response
			status := maTraceEventSucceeded
			errMessage := ""
			if err != nil {
				status = maTraceEventFailed
				errMessage = err.Error()
			}
			if traceErr := s.traceEvent(ctx, maTraceEventWrite{
				EventType:      "company_search_cache_hit_after_lock",
				ExternalSystem: "binocolo_cache",
				Status:         status,
				Request:        maTraceJSON(paramsJSON),
				Response:       maTraceJSON(entry.Response),
				Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey, "count": envelopeCount(response), "cost": envelopeCost(response)}),
				Error:          errMessage,
			}); traceErr != nil {
				return traceErr
			}
			return err
		}
		response, raw, upstreamErr = fetch(ctx)
		if upstreamErr != nil {
			return nil
		}
		fetchedAt := s.now()
		dryRun := params.DryRun != nil && *params.DryRun == 1
		if err := s.searchCache.UpsertCompanySearch(ctx, companySearchCacheWrite{
			CacheKey:           cacheKey,
			Params:             paramsJSON,
			Response:           raw,
			DryRun:             dryRun,
			DataEnrichment:     params.DataEnrichment,
			FetchedAt:          fetchedAt,
			ExpiresAt:          fetchedAt.Add(companySearchCacheTTL),
			RefreshedBySubject: subject,
			RefreshedByEmail:   email,
		}); err != nil {
			return err
		}
		if err := s.traceEvent(ctx, maTraceEventWrite{
			EventType:      "company_search_cache_write",
			ExternalSystem: "binocolo_cache",
			Status:         maTraceEventSucceeded,
			Request:        maTraceJSON(paramsJSON),
			Metadata:       maTraceJSON(map[string]any{"cache_key": cacheKey}),
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, err
	}
	if upstreamErr != nil {
		return openapiit.Envelope[openapiit.CompanyDataset]{}, nil, upstreamErr
	}
	return response, raw, nil
}

func decodeCompanySearchEnvelope(raw json.RawMessage) (openapiit.Envelope[openapiit.CompanyDataset], error) {
	var response openapiit.Envelope[openapiit.CompanyDataset]
	if err := json.Unmarshal(raw, &response); err != nil {
		return response, fmt.Errorf("decode cached company search: %w", err)
	}
	return response, nil
}

// baseMASearchParams builds an execution search for one cell of the
// (province × ateco code × legal form) iteration. Province, atecoCode and
// legalFormCode are single-valued at OpenAPI.it, so each is one axis of the
// cartesian; the caller iterates and passes a single value per call. An empty
// legalForm means "any form" (no server-side legal-form filter).
func baseMASearchParams(strategy MAStrategySpec, province, legalForm string, dryRun *int, limit int, enrichment string) openapiit.CompanyITSearchParams {
	if limit <= 0 {
		limit = maDefaultSearchLimit
	}
	if limit > maVendorLimit {
		limit = maVendorLimit
	}
	if enrichment == "" {
		enrichment = maEnrichmentAdvanced
	}
	params := openapiit.CompanyITSearchParams{
		DryRun:         dryRun,
		DataEnrichment: enrichment,
		Province:       province,
		LegalFormCode:  legalForm,
		ActivityStatus: strategy.ActivityStatus,
		MinTurnover:    strategy.TurnoverMin,
		MaxTurnover:    strategy.TurnoverMax,
		MinEmployees:   strategy.EmployeeMin,
		MaxEmployees:   strategy.EmployeeMax,
		Limit:          &limit,
	}
	return params
}

func baseMASurfaceParams(strategy MAStrategySpec, province, legalForm string, dryRun *int) openapiit.CompanyITSearchParams {
	return openapiit.CompanyITSearchParams{
		DryRun:         dryRun,
		DataEnrichment: "advanced",
		Province:       province,
		LegalFormCode:  legalForm,
		ActivityStatus: strategy.ActivityStatus,
		MinTurnover:    strategy.TurnoverMin,
		MaxTurnover:    strategy.TurnoverMax,
		MinEmployees:   strategy.EmployeeMin,
		MaxEmployees:   strategy.EmployeeMax,
	}
}

func envelopeCount(envelope openapiit.Envelope[openapiit.CompanyDataset]) int {
	if envelope.Count != nil {
		return *envelope.Count
	}
	return 0
}

func envelopeCost(envelope openapiit.Envelope[openapiit.CompanyDataset]) float64 {
	if envelope.Cost != nil {
		return *envelope.Cost
	}
	return 0
}

func maStrategyBudget(strategy MAStrategySpec, defaultBudget float64) float64 {
	if strategy.MaxBudgetEUR != nil && *strategy.MaxBudgetEUR > 0 {
		return *strategy.MaxBudgetEUR
	}
	return defaultBudget
}

// maProjectedSpend is the advanced-enrichment cost of a run: at most `limit`
// companies out of `available` are fetched, each priced at maCostPerCompanyEUR.
func maProjectedSpend(available, limit int, costPerCompany float64) float64 {
	fetched := available
	if limit > 0 && fetched > limit {
		fetched = limit
	}
	if fetched < 0 {
		fetched = 0
	}
	return float64(fetched) * costPerCompany
}

func estimateTotal(estimates []MAEstimate, strategyType string) int {
	total := 0
	for _, estimate := range estimates {
		if estimate.StrategyType == strategyType {
			total += estimate.EstimatedCount
		}
	}
	return total
}

func estimatesForStrategy(estimates []MAEstimate, strategyType string) int {
	total := 0
	for _, estimate := range estimates {
		if estimate.StrategyType == strategyType {
			total++
		}
	}
	return total
}

func estimatesTooBroad(estimates []MAEstimate, strategyType string) bool {
	for _, estimate := range estimates {
		if estimate.StrategyType == strategyType && estimate.SurfaceStatus == maEstimateSurfaceTooBroad {
			return true
		}
	}
	return false
}

func estimatesMatchSearchLimit(estimates []MAEstimate, strategyType string, limit int) bool {
	seen := false
	for _, estimate := range estimates {
		if estimate.StrategyType != strategyType {
			continue
		}
		seen = true
		value := estimateExecutionLimit(estimate)
		if value <= 0 {
			return false
		}
		if value != limit {
			return false
		}
	}
	return seen
}

func estimateExecutionLimit(estimate MAEstimate) int {
	if estimate.ExecutionLimit > 0 {
		return estimate.ExecutionLimit
	}
	return estimateParamLimit(estimate.Params)
}

func estimateParamLimit(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var params map[string]string
	if err := json.Unmarshal(raw, &params); err != nil {
		return 0
	}
	value, ok := params["limit"]
	if !ok {
		return 0
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func buildMAXLSX(rows [][]any) ([]byte, error) {
	file := excelize.NewFile()
	defer file.Close()
	sheet := file.GetSheetName(0)
	if sheet == "" {
		sheet = "Sheet1"
	}
	_ = file.SetSheetName(sheet, "Target")
	sheet = "Target"
	for rowIndex, row := range rows {
		for colIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err != nil {
				return nil, err
			}
			if err := file.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	if len(rows) > 0 {
		lastCol, _ := excelize.ColumnNumberToName(len(rows[0]))
		_ = file.AutoFilter(sheet, "A1:"+lastCol+"1", nil)
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func titleFromPrompt(prompt string) string {
	title := cleanText(prompt, 80)
	if title == "" {
		return "Nuova ricerca"
	}
	return title
}

func safeFilenamePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "ricerca"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
		if b.Len() >= 48 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "ricerca"
	}
	return out
}

func maErrorCode(err error) string {
	var apiErr *openapiit.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusPaymentRequired {
			return "openapiit_credit_required"
		}
		return "openapiit_upstream_error"
	}
	if errors.Is(err, errMAEstimateTooLarge) {
		return "estimate_too_large"
	}
	return "execution_error"
}

func validateOptionalUUID(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if _, err := uuid.Parse(value); err != nil {
		return fmt.Errorf("%w: llm selection", errMAStrategyInvalid)
	}
	return nil
}

func llmConfigError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errMALLMConfigUnavailable
	}
	return err
}
