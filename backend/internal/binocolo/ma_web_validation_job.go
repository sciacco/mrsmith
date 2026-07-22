package binocolo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/sciacco/mrsmith/internal/platform/brave"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"golang.org/x/sync/errgroup"
)

const (
	maWebValidationDefaultLimit       = 100
	maWebValidationMaxLimit           = 100
	maWebValidationDefaultDomainCount = 20
	// maWebValidationConcurrency is how many targets the batch validation processes at
	// once. The loop body fans out to external services that have NO internal
	// rate-limiter (Brave domain + probe queries, the self-hosted scrape service,
	// Fireworks embed+rerank), so this is the throttle — 4 balances wall-clock against
	// tripping Brave's rate limit / saturating the scraper. The shared state it touches
	// is safe: the trace is mutex-guarded, the DB pool (50) covers the fan-out, targets
	// upsert distinct rows, and the counters are atomic.
	maWebValidationConcurrency = 4
	// maWebValidationEvidenceCount is how many results each NEUTRAL self-description
	// probe pulls from the company's own site (no strategy terms — UC2 gathers what
	// the company says about itself, then classifies, instead of confirming the
	// strategy's expectations).
	maWebValidationEvidenceCount = 5
	// maNeutralEvidenceSnippetCap bounds how many self-description snippets feed the
	// distiller. Without it a content-heavy site (blog feeds, JSON-LD) bloats the
	// distiller input to thousands of tokens and tips cheap models into rambling
	// chain-of-thought instead of producing the description.
	maNeutralEvidenceSnippetCap = 12
	// maDomainVerifyCandidateCap bounds how many ranked candidates the scrape-based
	// entity verification will fetch (aggressive but cost-bounded). The chosen domain
	// is normally among the top few, and a P.IVA hit short-circuits the loop.
	maDomainVerifyCandidateCap = 5
	// maScrapeEvidenceChunkCap bounds how many prose blocks from the scraped homepage
	// markdown seed the evidence corpus (the rest of the snippet cap is left to Brave).
	maScrapeEvidenceChunkCap = 8
	// Post-resolution crawl bounds (async, runs in the background job — not bound by
	// the 60s HTTP write timeout). Shallow + few pages keeps it a respectful,
	// cost-bounded site job that still reaches /chi-siamo, /servizi, /contatti.
	maCrawlMaxDepth = 2
	maCrawlMaxPages = 8
	maCrawlTimeout  = 90 * time.Second
	// maCrawlPageEvidenceCap bounds evidence chunks taken per crawled page when there
	// is more than one, so a content-heavy homepage doesn't crowd out the other pages.
	maCrawlPageEvidenceCap = 4
	// Crawl-before-reject bounds. Before the aggressive wrong-entity reject fires,
	// the most credible candidate(s) get a sitemap-first map (URL discovery only,
	// cheap) plus targeted scrapes of the few identity-bearing pages (/contatti,
	// /note-legali, ...) where Italian sites actually publish the P.IVA the
	// homepage omits. Eval 2026-07-02 (365 validated companies): retrieval NEVER
	// failed, while this reject path held 15% of companies in manual review — 2/3
	// of them with a brand-compatible top candidate, i.e. very likely the right
	// site rejected for a homepage-only identity check.
	maDeepVerifyCandidateCap = 2
	maDeepVerifyPageCap      = 3
	maMapTimeout             = 30 * time.Second
)

// maNeutralEvidenceProbes are the deliberately strategy-agnostic site queries used
// to surface a company's self-description. They never mention the strategy sector,
// so the evidence cannot be confirmation-biased toward the expected answer.
var maNeutralEvidenceProbes = []string{"chi siamo", "servizi soluzioni", "cosa facciamo"}

// maUnrenderedPlaceholderPattern matches templating leaks such as "{vendor_count}"
// (or "{{var}}") that scrapers capture when a page's template variable was never
// interpolated server-side. These mark non-content fragments (typically consent
// banners) generically, regardless of language or CMS.
var maUnrenderedPlaceholderPattern = regexp.MustCompile(`\{\{?[a-zA-Z0-9_]+\}?\}`)

// isLowValueEvidence drops snippets that are NOT company self-description prose:
// raw structured-data (JSON-LD) blobs and fragments carrying unrendered template
// placeholders. These are content-type/structural filters with no domain-vocabulary
// matching. Consent/cookie boilerplate PROSE is intentionally NOT pattern-matched
// here — enumerating consent managers is brittle (language/plugin-specific) and risks
// dropping real business vocabulary; the distiller is instructed to ignore it instead.
func isLowValueEvidence(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return true
	}
	if strings.HasPrefix(lower, "{") ||
		strings.Contains(lower, "\"headline\"") ||
		strings.Contains(lower, "\"articlebody\"") ||
		strings.Contains(lower, "\"datepublished\"") {
		return true
	}
	return maUnrenderedPlaceholderPattern.MatchString(text)
}

type maWebValidationJobPayload struct {
	Limit              int  `json:"limit"`
	Force              bool `json:"force"`
	IncludeIdentifiers bool `json:"includeIdentifiers"`
	AnalyzeWithLLM     bool `json:"analyzeWithLLM"`
	// LLMOnAll forces the LLM analyst to run on EVERY company, not only on the
	// ambiguous/no_signal deterministic verdicts. Used by the sector-eval harness to
	// learn what the LLM would have chosen even where embed+rerank was already terminal.
	// It does NOT change the production decision (sectorFinalDecision still ignores the
	// analyst for confirm/reject/weak) — it only records the extra opinion.
	LLMOnAll     bool `json:"llmOnAll"`
	DomainCount  int  `json:"domainCount"`
	KeywordCount int  `json:"keywordCount"`
	Rank         bool `json:"rank"`
	Inline       bool `json:"inline"`
}

func (s *maService) enqueueWebValidation(ctx context.Context, sessionID string, req MAWebValidationEnrichRequest, subject, email string) (MASessionDetail, error) {
	if s.store == nil {
		return MASessionDetail{}, errMAStoreUnavailable
	}
	if s.brave == nil {
		return MASessionDetail{}, errMABraveUnavailable
	}
	detail, err := s.store.GetMASession(ctx, sessionID)
	if err != nil {
		return MASessionDetail{}, err
	}
	if err := ensureMASessionOperational(detail.Session); err != nil {
		return MASessionDetail{}, err
	}
	if detail.Strategy == nil {
		return MASessionDetail{}, fmt.Errorf("%w: strategy", errMAStrategyInvalid)
	}
	if len(detail.Targets) == 0 {
		return MASessionDetail{}, fmt.Errorf("%w: targets required", errMAStrategyInvalid)
	}
	payload := normalizeMAWebValidationPayload(req)
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return MASessionDetail{}, fmt.Errorf("marshal ma web validation payload: %w", err)
	}
	// Inline (dev-only, off-queue): on a shared DB the ma_job queue can be claimed by a
	// foreign worker running stale code, so a locally-enqueued job is processed by the
	// wrong binary. Inline mode runs the work in THIS process via a detached goroutine and
	// never writes an ma_job row — nothing for another worker to steal. No durability/resume
	// (the reason the queue exists); acceptable for a dev eval run. See
	// project_binocolo_shared_job_queue.
	if payload.Inline {
		job := maJob{
			JobType:           maJobTypeWebValidation,
			SessionID:         sessionID,
			StrategyVersionID: detail.Strategy.ID,
			Status:            maJobStatusRunning,
			Payload:           rawPayload,
			CreatedBySubject:  subject,
			CreatedByEmail:    email,
		}
		go func() {
			bg := context.WithoutCancel(ctx)
			if _, err := s.runWebValidationJob(bg, job); err != nil {
				logging.FromContext(bg).Error("binocolo inline web validation failed",
					"component", "binocolo", "operation", "ma_web_validation_inline",
					"session_id", sessionID, "error", err)
			}
		}()
		return s.getSession(ctx, sessionID)
	}
	if _, _, err := s.store.EnqueueMAJob(ctx, maJobEnqueue{
		JobType:           maJobTypeWebValidation,
		SessionID:         sessionID,
		StrategyVersionID: detail.Strategy.ID,
		Status:            maJobStatusPending,
		Subject:           subject,
		Email:             email,
		Payload:           rawPayload,
		Owner:             s.owner,
	}); err != nil {
		return MASessionDetail{}, err
	}
	return s.getSession(ctx, sessionID)
}

func (s *maService) runWebValidationJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_session_web_validation",
		SessionID:        job.SessionID,
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	if workErr := s.webValidationJobWork(ctx, job); workErr != nil {
		_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusFailed, ErrorMessage: workErr.Error()})
		return trace.id, workErr
	}
	_ = s.completeTrace(ctx, maTraceComplete{Status: maTraceStatusSucceeded})
	return trace.id, nil
}

func (s *maService) webValidationJobWork(ctx context.Context, job maJob) error {
	if s.brave == nil {
		return errMABraveUnavailable
	}
	payload := maWebValidationJobPayload{
		Limit:          maWebValidationDefaultLimit,
		AnalyzeWithLLM: true,
		DomainCount:    maWebValidationDefaultDomainCount,
		KeywordCount:   maWebValidationEvidenceCount,
		Rank:           true,
	}
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma web validation payload: %w", err)
		}
	}
	payload = normalizeMAWebValidationPayload(MAWebValidationEnrichRequest{
		Limit:              payload.Limit,
		Force:              payload.Force,
		IncludeIdentifiers: payload.IncludeIdentifiers,
		AnalyzeWithLLM:     boolPtr(payload.AnalyzeWithLLM),
		LLMOnAll:           boolPtr(payload.LLMOnAll),
		DomainCount:        payload.DomainCount,
		KeywordCount:       payload.KeywordCount,
		Rank:               boolPtr(payload.Rank),
	})

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
	if err := s.linkTrace(ctx, maTraceLink{SessionID: job.SessionID, StrategyVersionID: version.ID}); err != nil {
		return err
	}

	return s.validateMATargetsBatch(ctx, job.SessionID, version.ID, version.Strategy, detail.Targets, payload, job.CreatedBySubject, job.CreatedByEmail)
}

// validateMATargetsBatch runs the UC2 concept gate over a candidate set, bounded to
// maWebValidationConcurrency, and upserts one ma_target_web_validation row per target.
// It is the shared core of both the standalone web-validation job and the gated-search
// pipeline's gate stage. Idempotent: a target with a still-fresh validation (and no
// Force) is skipped, so a retry re-charges only the un-validated tail. The errgroup's
// shared ctx keeps the abort-on-first-error semantics (a failed upsert cancels peers).
func (s *maService) validateMATargetsBatch(ctx context.Context, sessionID, strategyVersionID string, strategy MAStrategySpec, candidates []MATarget, payload maWebValidationJobPayload, subject, email string) error {
	targets := selectMAWebValidationTargets(candidates, strategy, payload, s.now(), payload.Limit)
	var processed, skipped, failed atomic.Int64
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_web_validation_started",
		Status:    maTraceEventStarted,
		Metadata: maTraceJSON(map[string]any{
			"session_id":          sessionID,
			"strategy_version_id": strategyVersionID,
			"candidate_count":     len(targets),
			"force":               payload.Force,
			"limit":               payload.Limit,
			"method":              "concept",
			"concurrency":         maWebValidationConcurrency,
		}),
	})

	group, gctx := errgroup.WithContext(ctx)
	group.SetLimit(maWebValidationConcurrency)
	for _, work := range targets {
		group.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			target := work.Target
			if !payload.Force && reusableMAWebValidation(target.WebValidation, work.InputHash, work.InputHash, s.now()) {
				skipped.Add(1)
				return nil
			}

			body, buildErr := s.buildMAWebValidation(gctx, target, strategy, work.InputHash, payload, subject, email)
			if buildErr != nil {
				body = failedMAWebValidationRequest(target, work.InputHash, buildErr)
				failed.Add(1)
			}
			if _, err := s.upsertTargetWebValidation(gctx, sessionID, body, subject, email); err != nil {
				return err
			}
			processed.Add(1)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}

	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_web_validation_completed",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"session_id":   sessionID,
			"processed":    processed.Load(),
			"skipped":      skipped.Load(),
			"failed":       failed.Load(),
			"target_count": len(targets),
		}),
	})
	return nil
}

// buildMAWebValidation runs the UC2 concept pipeline for one target: resolve the
// official domain (kept), gather NEUTRAL self-description evidence, classify it
// against the curated KB concepts (embedding + reranker, Step 2), and — only when
// the deterministic verdict is ambiguous/no_signal — escalate to the LLM analyst.
// The result maps onto the existing MAWebValidation contract.
func (s *maService) buildMAWebValidation(
	ctx context.Context,
	target MATarget,
	strategy MAStrategySpec,
	inputHash string,
	payload maWebValidationJobPayload,
	subject string,
	email string,
) (MAWebValidationUpsertRequest, error) {
	// Cross-session domain registry first: a company whose official domain was
	// already identity-verified (or manually associated) skips retrieval and
	// verification entirely — zero Brave/scrape spend, and a manual association
	// done once holds for every future session.
	if known := s.lookupCompanyDomain(ctx, target); known != nil {
		if known.Method == maDomainMethodNoWebsite {
			return s.noWebsiteWebValidation(target, inputHash), nil
		}
		reason := "dominio dal registro (verificato in pagina)"
		identityState := maIdentityStateVerified
		if known.Method == maDomainMethodManual {
			reason = "dominio dal registro (associato manualmente)"
			identityState = maIdentityStateVouched
		}
		if known.GroupSite {
			reason = "dominio dal registro (sito di gruppo confermato)"
			identityState = maIdentityStateVouched
		}
		selectedDomain := &DomainResolutionCandidate{
			Domain:     known.Domain,
			Score:      100,
			Confidence: "alta",
			Reasons:    []string{reason},
		}
		scrapedMarkdown, _ := s.scrapeHomepage(ctx, known.Domain) // best-effort; crawl+evidence tolerate empty
		domainResponse := DomainResolutionResponse{
			Query:      "registry:" + known.Domain,
			Candidates: []DomainResolutionCandidate{*selectedDomain},
		}
		return s.classifyResolvedDomain(ctx, target, strategy, inputHash, payload, domainResponse, selectedDomain, scrapedMarkdown, identityState, false, subject, email), nil
	}

	domainResponse, err := s.resolveDomainCandidates(ctx, DomainResolutionRequest{
		CompanyName: target.CompanyName,
		VATCode:     optionalIdentifier(payload.IncludeIdentifiers, target.VATCode),
		TaxCode:     optionalIdentifier(payload.IncludeIdentifiers, target.TaxCode),
		Town:        target.Town,
		Province:    target.Province,
		Count:       payload.DomainCount,
	})
	if err != nil {
		return MAWebValidationUpsertRequest{}, fmt.Errorf("domain resolution: %w", err)
	}
	selectedDomain, scrapedMarkdown, identityVerified, groupHint := s.verifyDomainByScrape(ctx, domainResponse.Candidates, target)
	// Fact-at-write: the group-site suspicion rides the persisted domain_response
	// JSONB on resolved and unresolved validations alike (policy B1 reads it as the
	// queue reason + remedy precondition).
	domainResponse.GroupSiteHint = groupHint
	if selectedDomain == nil {
		decision := &CandidateMatchFinalDecision{
			InitialMatchState:  target.MatchState,
			DeterministicScore: target.Score,
			WebValidationState: "domain_unresolved",
			FinalAction:        "needs_domain_review",
			Confidence:         "bassa",
			Reason:             "Nessun dominio ufficiale verificato.",
			Reasons:            []string{"Nessun dominio ufficiale verificato (punteggio insufficiente o identità non confermata in pagina)."},
		}
		if groupHint != nil {
			decision.Reasons = append(decision.Reasons, "Possibile sito di gruppo: "+groupHint.Domain+" dichiara la P.IVA di un'altra società ("+groupHint.Identifier+").")
		}
		return s.assembleWebValidationRequest(target, inputHash, domainResponse, nil, nil, maSectorClassification{}, nil, "", decision), nil
	}

	identityState := maIdentityStateAssumed
	if identityVerified {
		identityState = maIdentityStateVerified
	}
	return s.classifyResolvedDomain(ctx, target, strategy, inputHash, payload, domainResponse, selectedDomain, scrapedMarkdown, identityState, true, subject, email), nil
}

// classifyResolvedDomain runs the evidence → classification → decision tail of the gate
// once an official domain has been chosen — whether auto-resolved+verified
// (buildMAWebValidation) or MANUALLY associated (buildMAWebValidationForDomain). It crawls
// the site for neutral self-description, classifies against the KB concepts, and escalates
// ambiguous verdicts to the LLM analyst, producing the web-validation request. A classifier
// failure (embedder/KB down) is surfaced as analysis_unavailable, never as a hard error.
//
// identityState is the domain-identity certainty established so far (verified /
// vouched / assumed); the crawl below can upgrade it to verified via an on-page
// P.IVA. registerDomain gates the write to the cross-session domain registry (auto
// path only — the manual path registers in associateDomainWork with method
// 'manual', and a registry hit needs no re-write).
func (s *maService) classifyResolvedDomain(
	ctx context.Context,
	target MATarget,
	strategy MAStrategySpec,
	inputHash string,
	payload maWebValidationJobPayload,
	domainResponse DomainResolutionResponse,
	selectedDomain *DomainResolutionCandidate,
	scrapedMarkdown string,
	identityState string,
	registerDomain bool,
	subject string,
	email string,
) MAWebValidationUpsertRequest {
	// Enrich evidence with a shallow crawl of the resolved site: real multi-page
	// self-description (services / about / contacts) classifies far better than the
	// homepage alone, and an on-page P.IVA on a legal/contact page confirms identity
	// the homepage often omits. Degrades to homepage-only evidence when disabled/failing.
	evidencePages, crawlVerified := s.crawlResolvedSite(ctx, selectedDomain.Domain, scrapedMarkdown, target)
	if crawlVerified {
		identityState = maIdentityStateVerified
	}
	if identityState == maIdentityStateVerified && selectedDomain.Confidence != "alta" {
		selectedDomain.Confidence = "alta" // on-page P.IVA is the strongest identity signal
	}
	if registerDomain && identityState == maIdentityStateVerified {
		s.registerCompanyDomain(ctx, target, selectedDomain.Domain, maDomainMethodAutoVerified, subject, email)
	}
	evidence, evidenceRuns := s.gatherNeutralEvidence(ctx, selectedDomain.Domain, payload.KeywordCount, subject, email, evidencePages)

	classification, classErr := s.classifyCompanySector(ctx, evidence, strategy, subject, email)
	if classErr != nil {
		// Embedder/KB unavailable: cannot classify. Surface as analysis_unavailable so
		// the analyst reviews it; never silently confirm/reject.
		classError := cleanText(classErr.Error(), 180)
		decision := &CandidateMatchFinalDecision{
			InitialMatchState:  target.MatchState,
			DeterministicScore: target.Score,
			WebValidationState: "analysis_unavailable",
			FinalAction:        "needs_business_validation",
			Confidence:         "bassa",
			Reason:             "Classificazione settore non disponibile (embedder/KB).",
			Reasons:            []string{"Classificazione settore non disponibile: " + classError},
		}
		out := s.assembleWebValidationRequest(target, inputHash, domainResponse, selectedDomain, evidenceRuns, maSectorClassification{}, nil, classError, decision)
		out.IdentityState = identityState
		return out
	}

	var analysis *CandidateMatchAnalysisResponse
	analysisErr := ""
	if payload.AnalyzeWithLLM && (payload.LLMOnAll || sectorVerdictNeedsLLM(classification.Verdict)) {
		result, err := s.analyzeSectorAmbiguity(ctx, target, strategy, evidence, classification, subject, email)
		if err != nil {
			analysisErr = cleanText(err.Error(), 180)
		} else {
			analysis = &result
		}
	}

	decision := sectorFinalDecision(target, classification, analysis, analysisErr)
	s.recordSectorClassificationTrace(ctx, target, classification, decision, analysis != nil)
	out := s.assembleWebValidationRequest(target, inputHash, domainResponse, selectedDomain, evidenceRuns, classification, analysis, analysisErr, decision)
	out.IdentityState = identityState
	return out
}

// buildMAWebValidationForDomain re-runs the gate for ONE target with a MANUALLY associated
// domain, skipping automatic resolution+verification — the operator vouches for the
// identity. It scrapes/crawls the given site, classifies it, and produces the web-validation
// request, so a company previously HELD in manual_review (official domain unresolved) gets a
// real keep/forse/scarta verdict and can then be auto-enriched.
func (s *maService) buildMAWebValidationForDomain(
	ctx context.Context,
	target MATarget,
	strategy MAStrategySpec,
	inputHash string,
	payload maWebValidationJobPayload,
	domain string,
	reason string,
	subject string,
	email string,
) (MAWebValidationUpsertRequest, error) {
	normalized, ok := normalizeDomain(domain)
	if !ok {
		return MAWebValidationUpsertRequest{}, fmt.Errorf("%w: domain", errMAStrategyInvalid)
	}
	// The operator's chosen domain IS the identity — high confidence, no resolution
	// ranking. classifyResolvedDomain still tries to confirm the P.IVA on-page via the
	// crawl, but the verdict is no longer gated on domain resolution succeeding.
	selectedDomain := &DomainResolutionCandidate{
		Domain:     normalized,
		Score:      100,
		Confidence: "alta",
		Reasons:    []string{defaultString(reason, "dominio associato manualmente dall'operatore")},
	}
	scrapedMarkdown, _ := s.scrapeHomepage(ctx, normalized) // best-effort; crawl+evidence tolerate empty
	domainResponse := DomainResolutionResponse{
		Query:      "manual:" + normalized,
		Candidates: []DomainResolutionCandidate{*selectedDomain},
	}
	return s.classifyResolvedDomain(ctx, target, strategy, inputHash, payload, domainResponse, selectedDomain, scrapedMarkdown, maIdentityStateVouched, false, subject, email), nil
}

// lookupCompanyDomain consults the cross-session verified-domain registry for a
// target. Soft dependency: a lookup failure logs and falls through to normal
// resolution (nil), never blocking the gate.
func (s *maService) lookupCompanyDomain(ctx context.Context, target MATarget) *maCompanyDomain {
	if s.store == nil {
		return nil
	}
	companyKey := normalizeMACompanyKey(target.CompanyKey)
	if companyKey == "" {
		companyKey = normalizeMACompanyKey(maTargetDedupeKey(target))
	}
	vat := strings.ToUpper(strings.TrimSpace(target.VATCode))
	tax := strings.ToUpper(strings.TrimSpace(target.TaxCode))
	record, err := s.store.GetMACompanyDomain(ctx, companyKey, vat, tax)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo domain registry lookup failed",
			"component", "binocolo", "operation", "ma_company_domain_lookup",
			"company", target.CompanyName, "error", err)
		return nil
	}
	if record == nil {
		return nil
	}
	if record.Method != maDomainMethodNoWebsite && strings.TrimSpace(record.Domain) == "" {
		return nil
	}
	return record
}

func validMADomainIdentityState(state string) bool {
	return state == maIdentityStateVerified || state == maIdentityStateVouched || state == maIdentityStateAssumed
}

// registerCompanyDomainWithIdentityState is the strict registry write used by workflows
// whose success requires the session validation and global identity to stay synchronized.
func (s *maService) registerCompanyDomainWithIdentityState(ctx context.Context, target MATarget, domain, method, identityState string, groupSite bool, subject, email string) error {
	if s.store == nil {
		return fmt.Errorf("domain registry store unavailable")
	}
	if !validMADomainIdentityState(identityState) {
		return fmt.Errorf("invalid domain identity state %q", identityState)
	}
	companyKey := normalizeMACompanyKey(target.CompanyKey)
	if companyKey == "" {
		companyKey = normalizeMACompanyKey(maTargetDedupeKey(target))
	}
	normalized, ok := normalizeDomain(domain)
	if companyKey == "" || !ok {
		return fmt.Errorf("invalid company domain registry record")
	}
	return s.store.UpsertMACompanyDomain(ctx, maCompanyDomain{
		CompanyKey: companyKey, VATCode: strings.ToUpper(strings.TrimSpace(target.VATCode)),
		TaxCode: strings.ToUpper(strings.TrimSpace(target.TaxCode)), CompanyName: strings.TrimSpace(target.CompanyName),
		Domain: normalized, Method: method, GroupSite: groupSite, IdentityState: identityState,
		CreatedBySubject: subject, CreatedByEmail: email,
	})
}

// registerCompanyDomain upserts a registry entry as a soft dependency for automatic paths.
func (s *maService) registerCompanyDomain(ctx context.Context, target MATarget, domain, method, subject, email string) {
	if s.store == nil {
		return
	}
	companyKey := normalizeMACompanyKey(target.CompanyKey)
	if companyKey == "" {
		companyKey = normalizeMACompanyKey(maTargetDedupeKey(target))
	}
	normalized, ok := normalizeDomain(domain)
	if companyKey == "" || !ok {
		return
	}
	identityState := maIdentityStateAssumed
	if method == maDomainMethodAutoVerified {
		identityState = maIdentityStateVerified
	} else if method == maDomainMethodManual {
		identityState = maIdentityStateVouched
	}
	err := s.registerCompanyDomainWithIdentityState(ctx, target, normalized, method, identityState, false, subject, email)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo domain registry write failed",
			"component", "binocolo", "operation", "ma_company_domain_upsert",
			"company", target.CompanyName, "domain", normalized, "method", method, "error", err)
	}
}

func (s *maService) registerCompanyDomainRecord(ctx context.Context, target MATarget, domain, method string, groupSite bool, subject, email string) {
	if s.store == nil {
		return
	}
	companyKey := normalizeMACompanyKey(target.CompanyKey)
	if companyKey == "" {
		companyKey = normalizeMACompanyKey(maTargetDedupeKey(target))
	}
	if companyKey == "" {
		return
	}
	normalized := ""
	if method != maDomainMethodNoWebsite {
		var ok bool
		normalized, ok = normalizeDomain(domain)
		if !ok {
			return
		}
	}
	identityState := maIdentityStateAssumed
	if method == maDomainMethodAutoVerified {
		identityState = maIdentityStateVerified
	} else if method == maDomainMethodManual {
		identityState = maIdentityStateVouched
	}
	err := s.store.UpsertMACompanyDomain(ctx, maCompanyDomain{
		CompanyKey:       companyKey,
		VATCode:          strings.ToUpper(strings.TrimSpace(target.VATCode)),
		TaxCode:          strings.ToUpper(strings.TrimSpace(target.TaxCode)),
		CompanyName:      strings.TrimSpace(target.CompanyName),
		Domain:           normalized,
		Method:           method,
		GroupSite:        groupSite,
		IdentityState:    identityState,
		CreatedBySubject: subject,
		CreatedByEmail:   email,
	})
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo domain registry write failed",
			"component", "binocolo", "operation", "ma_company_domain_upsert",
			"company", target.CompanyName, "domain", normalized, "method", method, "error", err)
	}
}

func (s *maService) noWebsiteWebValidation(target MATarget, inputHash string) MAWebValidationUpsertRequest {
	decision := &CandidateMatchFinalDecision{
		InitialMatchState:  target.MatchState,
		DeterministicScore: target.Score,
		WebValidationState: maWebValidationStateNoWebsite,
		FinalAction:        maFinalActionNoWebsite,
		Confidence:         "alta",
		Reason:             "Nessun sito ufficiale (dichiarato dall'operatore)",
		Reasons:            []string{"Nessun sito ufficiale (dichiarato dall'operatore)"},
	}
	out := s.assembleWebValidationRequest(target, inputHash, DomainResolutionResponse{Query: "registry:no_website"}, nil, nil, maSectorClassification{}, nil, "", decision)
	out.IdentityState = maIdentityStateVouched
	return out
}

func sectorVerdictNeedsLLM(verdict maSectorVerdict) bool {
	// confirm escalates too: a confident deterministic "confirm" had 0% precision in
	// the UC2 eval (every fire was a non-keeper, and short-circuiting it silenced a
	// better LLM, e.g. MDOTM/AGSDIGITAL). reject keeps short-circuiting — it is safe
	// (high precision) and cheap. Asymmetric by design: never auto-confirm.
	return verdict == maSectorAmbiguous || verdict == maSectorNoSignal || verdict == maSectorConfirm
}

// crawlResolvedSite shallow-crawls a resolved company domain to (1) gather
// multi-page self-description evidence for classification and (2) confirm the
// target's P.IVA / codice fiscale on pages the homepage omits (/contatti,
// /note-legali). homepageMarkdown — already fetched during verification — is
// always returned first so evidence degrades gracefully when the crawl is
// disabled or fails. Returns the page markdowns and whether identity was confirmed.
func (s *maService) crawlResolvedSite(ctx context.Context, domain, homepageMarkdown string, target MATarget) ([]string, bool) {
	vat := normalizeIdentifierForPageMatch(target.VATCode)
	tax := normalizeIdentifierForPageMatch(target.TaxCode)
	verified := false
	checkIdentity := func(md string) {
		if verified || md == "" {
			return
		}
		if (vat != "" && pageContainsIdentifier(md, vat)) || (tax != "" && pageContainsIdentifier(md, tax)) {
			verified = true
		}
	}

	pages := []string{}
	if md := strings.TrimSpace(homepageMarkdown); md != "" {
		pages = append(pages, md)
		checkIdentity(md)
	}
	if s.scrape == nil {
		return pages, verified
	}

	hosts := companyPageHosts(domain)
	if len(hosts) == 0 {
		return pages, verified
	}
	cctx, cancel := context.WithTimeout(ctx, maCrawlTimeout)
	defer cancel()
	crawled, err := s.scrape.Crawl(cctx, "https://"+hosts[0], maCrawlMaxDepth, maCrawlMaxPages)
	if err != nil {
		return pages, verified // soft dependency: keep homepage-only evidence
	}
	for _, p := range crawled {
		md := strings.TrimSpace(p.Markdown)
		if md == "" {
			continue
		}
		if len(pages) > 0 && md == pages[0] {
			continue // the crawl re-fetched the homepage we already have
		}
		pages = append(pages, md)
		checkIdentity(md)
	}
	return pages, verified
}

// gatherNeutralEvidence collects a company's self-description from its own site
// using strategy-agnostic probes. Returns the evidence corpus (for classification)
// and per-probe runs (for the persisted contract / UI).
func (s *maService) gatherNeutralEvidence(ctx context.Context, domain string, count int, subject, email string, prefetchedPages []string) (maCompanyEvidence, []CandidateMatchEvidenceRun) {
	evidence := maCompanyEvidence{Domain: domain}
	runs := make([]CandidateMatchEvidenceRun, 0, len(maNeutralEvidenceProbes)+1)
	seen := map[string]struct{}{}
	add := func(text string) {
		if len(evidence.Snippets) >= maNeutralEvidenceSnippetCap {
			return
		}
		cleaned := cleanText(text, 360)
		if cleaned == "" || isLowValueEvidence(cleaned) {
			return
		}
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		evidence.Snippets = append(evidence.Snippets, cleaned)
	}
	// Seed from the scraped/crawled page markdown when available: real page content
	// beats thin search snippets (fixes Apache-placeholder / unrelated-snippet
	// evidence). Markdown leads (best slots); the Brave probes below fill what
	// remains. With multiple crawled pages, cap per page so a content-heavy homepage
	// doesn't crowd out /chi-siamo, /servizi, etc.
	perPageCap := maScrapeEvidenceChunkCap
	if len(prefetchedPages) > 1 {
		perPageCap = maCrawlPageEvidenceCap
	}
	for i, pageMarkdown := range prefetchedPages {
		chunks := markdownToEvidenceSnippets(pageMarkdown)
		if len(chunks) == 0 {
			continue
		}
		term := "page"
		if i == 0 {
			term = "homepage"
		}
		run := CandidateMatchEvidenceRun{Bucket: "scrape", Term: term}
		before := len(evidence.Snippets)
		for taken, chunk := range chunks {
			if taken >= perPageCap {
				break
			}
			add(chunk)
		}
		run.ResultCount = len(evidence.Snippets) - before
		run.Matched = run.ResultCount > 0
		runs = append(runs, run)
		if len(evidence.Snippets) >= maNeutralEvidenceSnippetCap {
			break
		}
	}
	for _, probe := range maNeutralEvidenceProbes {
		run := CandidateMatchEvidenceRun{Bucket: "neutral", Term: probe}
		response, err := s.searchDomainEvidence(ctx, WebSearchRequest{
			Domain:   domain,
			Keywords: strings.Fields(probe),
			Count:    count,
			Rank:     false,
		}, subject, email)
		if err != nil {
			run.Error = cleanText(err.Error(), 180)
		} else {
			responseCopy := response
			run.Response = &responseCopy
			run.ResultCount = len(response.Results)
			run.Matched = run.ResultCount > 0
			for _, result := range response.Results {
				add(result.Title)
				for _, snippet := range result.Snippets {
					add(snippet)
				}
			}
		}
		runs = append(runs, run)
	}
	return evidence, runs
}

// verifyDomainByScrape implements the aggressive entity-verification policy. It
// scrapes ranked candidates and accepts the first whose page carries the target's
// P.IVA / codice fiscale (strong identity match) — a low-ranked candidate can win
// this way, rescuing domains the score-only heuristic under-rated. Failing that it
// accepts the first page carrying the company name (medium). It returns the chosen
// candidate, its homepage markdown (reused as evidence), whether the target's
// identity was confirmed ON-PAGE — the only signal strong enough to feed the
// cross-session domain registry — and the group-site hint: the fact that a
// brand-compatible candidate advertised ANOTHER entity's P.IVA (the subsidiary
// whose web presence is the group's site). The hint is captured regardless of
// the final outcome; a deep-verify hit (legal page) overwrites a homepage one.
//
// Degradation / recall safety:
//   - scraper disabled (s.scrape == nil)  -> chooseMAWebValidationDomain (today's pick), no markdown.
//   - an identifier was available AND >=1 page was read AND none matched -> deep
//     identity pass first (map + identity pages, see deepVerifyIdentity), THEN
//     reject (nil): the target lands in needs_domain_review (forse), never scarta.
//     This is the wrong-entity kill (a cinema / turbine maker won't carry the
//     target's P.IVA) and it is recall-safe — flagged for review, not rejected.
//   - no identifier to check, or every scrape failed (transport/4xx) -> fall back to
//     the score-only pick (with its markdown when we managed to fetch it).
func (s *maService) verifyDomainByScrape(ctx context.Context, candidates []DomainResolutionCandidate, target MATarget) (*DomainResolutionCandidate, string, bool, *MAGroupSiteHint) {
	chosen := chooseMAWebValidationDomain(candidates)
	if s.scrape == nil {
		return chosen, "", false, nil
	}

	vat := normalizeIdentifierForPageMatch(target.VATCode)
	tax := normalizeIdentifierForPageMatch(target.TaxCode)
	nameTokens := companyResolutionTokens(target.CompanyName)

	markdownByDomain := map[string]string{}
	anyPageRead := false
	var nameMatch *DomainResolutionCandidate
	var groupHint *MAGroupSiteHint

	for i := range candidates {
		if i >= maDomainVerifyCandidateCap {
			break
		}
		cand := candidates[i]
		md, ok := s.scrapeHomepage(ctx, cand.Domain)
		if !ok {
			continue
		}
		anyPageRead = true
		markdownByDomain[cand.Domain] = md
		if (vat != "" && pageContainsIdentifier(md, vat)) || (tax != "" && pageContainsIdentifier(md, tax)) {
			winner := cand
			return &winner, md, true, nil // strong identity match short-circuits
		}
		if nameMatch == nil && pageContainsCompanyName(md, nameTokens) {
			winner := cand
			nameMatch = &winner
		}
	}

	// A name-match may override the score order only when the score-top (`chosen`)
	// was itself read and didn't match — i.e. it is genuinely a different entity
	// (e.g. makemu.it for CARDNOLOGY, beaten by tessere-online.com which carried
	// the target's identity). If `chosen` FAILED to scrape, a name-match on a
	// lower-ranked candidate is too weak to demote it: the score-top may be the
	// correct site that simply didn't render, while the lower one false-matched on
	// generic tokens (e.g. consorzioarsenal.it failing to render while ioveneto.it
	// matched "veneto"/"ricerca" for ARSENAL). In that case fall through to the
	// reject / score-only path rather than trusting the speculative name-match.
	chosenRead := chosen != nil && markdownByDomain[chosen.Domain] != ""
	if nameMatch != nil && chosen != nil && (chosenRead || nameMatch.Domain == chosen.Domain) {
		// Foreign-identifier guard on the name-match path too (it only guarded
		// brand-trust): same-brand-different-company is exactly what name-match
		// can't tell apart — greenteam.it carries the name "Green Team" AND
		// another entity's P.IVA (05313430968 vs the target's 04332290370), and
		// accepting it classifies the target on someone else's business (false
		// scarta = recall loss). A name-matched page advertising a different
		// P.IVA falls through to brand-trust/deep-verify/reject instead.
		foreignID, foreign := pageForeignIdentifier(markdownByDomain[nameMatch.Domain])
		if !foreign {
			return nameMatch, markdownByDomain[nameMatch.Domain], false, groupHint
		}
		groupHint = &MAGroupSiteHint{Domain: nameMatch.Domain, Identifier: foreignID, Source: "name_match"}
	}
	// Recall-safe brand-label trust: when the score-top candidate's domain LABEL
	// matches the company name (adawen.it for ADAWEN) and it ranked with high
	// confidence, resolve it even though on-page identity wasn't confirmed, and let
	// classification run on whatever page/snippets we have (pre-scrape behaviour).
	// This covers both the site that never rendered (blocked / 404 / renderer error)
	// and the site that rendered thin or with a mistokenised brand (edisoftware.it ->
	// "edi"+"software", so name-match can't fire). The one case we still refuse: a
	// page that DID render and advertises a DIFFERENT P.IVA — that foreign identifier
	// is the genuine wrong-entity signal (greenteam.it), so leave it for review.
	if chosen != nil && chosen.Confidence == "alta" && domainLooksCompanyOwned(chosen.Domain, nameTokens) {
		foreignID, foreign := "", false
		if chosenRead {
			foreignID, foreign = pageForeignIdentifier(markdownByDomain[chosen.Domain])
		}
		if !foreign {
			return chosen, markdownByDomain[chosen.Domain], false, groupHint
		}
		if groupHint == nil {
			groupHint = &MAGroupSiteHint{Domain: chosen.Domain, Identifier: foreignID, Source: "brand_trust"}
		}
	}
	// Aggressive: an available identifier absent from every page we actually read
	// means the resolved site is a different entity -> reject. But the homepage is
	// the wrong place to look for a P.IVA — crawl-before-reject gives the credible
	// candidates one deep identity pass (map + /contatti-class pages) first, and an
	// on-page match there both rescues the company AND certifies it for the registry.
	if (vat != "" || tax != "") && anyPageRead {
		for _, cand := range deepVerifyCandidates(chosen, nameMatch, nameTokens) {
			verified, foreignID := s.deepVerifyIdentity(ctx, cand.Domain, vat, tax)
			if verified {
				winner := cand
				winner.Reasons = append(winner.Reasons, "identità confermata su pagina interna (P.IVA/CF)")
				return &winner, markdownByDomain[cand.Domain], true, groupHint
			}
			// A foreign P.IVA on a LEGAL page of a brand-compatible candidate is the
			// strongest group-site evidence (art. 35: that's where the owner publishes
			// it) — it overwrites a homepage-level hint. Brand-alien candidates stay
			// out: a wrong entity is not a group.
			if foreignID != "" && (domainLooksCompanyOwned(cand.Domain, nameTokens) || (nameMatch != nil && cand.Domain == nameMatch.Domain)) {
				groupHint = &MAGroupSiteHint{Domain: cand.Domain, Identifier: foreignID, Source: "deep_verify"}
			}
		}
		return nil, "", false, groupHint
	}
	if chosen != nil {
		return chosen, markdownByDomain[chosen.Domain], false, groupHint
	}
	return nil, "", false, groupHint
}

// deepVerifyCandidates selects which rejected candidates deserve the deep
// identity pass: the score-top pick and the name-match (when distinct), each
// only when credible — ranker confidence above "bassa" or a brand-compatible
// domain label. The eval's junk best-candidates (registries, unrelated sites,
// all low-confidence and brand-alien) stay excluded, so the extra spend
// concentrates exactly on the likely-right-site population. Pure, cap 2.
func deepVerifyCandidates(chosen, nameMatch *DomainResolutionCandidate, nameTokens []string) []DomainResolutionCandidate {
	out := make([]DomainResolutionCandidate, 0, maDeepVerifyCandidateCap)
	seen := map[string]struct{}{}
	for _, cand := range []*DomainResolutionCandidate{chosen, nameMatch} {
		if cand == nil || len(out) >= maDeepVerifyCandidateCap {
			continue
		}
		if _, dup := seen[cand.Domain]; dup {
			continue
		}
		if cand.Confidence == "bassa" && !domainLooksCompanyOwned(cand.Domain, nameTokens) {
			continue
		}
		seen[cand.Domain] = struct{}{}
		out = append(out, *cand)
	}
	return out
}

// deepVerifyIdentity maps a candidate site (sitemap-first URL discovery, no page
// content) and scrapes only its identity-bearing pages, reporting whether any
// carries the target's P.IVA / codice fiscale. A page advertising a DIFFERENT
// 11-digit identifier is the wrong-entity signal — stop reading that site and
// return the foreign identifier (on a brand-compatible candidate it names the
// group entity — group-site hint). Soft on every failure: map/scrape errors
// just mean "not verified".
func (s *maService) deepVerifyIdentity(ctx context.Context, domain, vat, tax string) (bool, string) {
	if s.scrape == nil || (vat == "" && tax == "") {
		return false, ""
	}
	hosts := companyPageHosts(domain)
	if len(hosts) == 0 {
		return false, ""
	}
	mctx, cancel := context.WithTimeout(ctx, maMapTimeout)
	links, err := s.scrape.Map(mctx, "https://"+hosts[0], maCrawlMaxDepth)
	cancel()
	if err != nil || len(links) == 0 {
		return false, ""
	}
	for _, link := range identityPageLinks(links, domain, maDeepVerifyPageCap) {
		res, err := s.scrape.ScrapeFull(ctx, link) // full page: the P.IVA lives in the footer
		if err != nil {
			continue
		}
		if res.StatusCode != 0 && (res.StatusCode < 200 || res.StatusCode >= 400) {
			continue
		}
		md := strings.TrimSpace(res.Markdown)
		if md == "" {
			continue
		}
		if (vat != "" && pageContainsIdentifier(md, vat)) || (tax != "" && pageContainsIdentifier(md, tax)) {
			return true, ""
		}
		if foreignID, foreign := pageForeignIdentifier(md); foreign {
			return false, foreignID // a legal/contact page with someone else's P.IVA = different entity
		}
	}
	return false, ""
}

// identityPagePriority orders the URL-path markers of pages that carry a
// company's legal identity in Italy, most likely first: contact pages, then
// legal/privacy boilerplate (P.IVA is mandatory there), then about pages.
var identityPagePriority = []string{
	"contatti", "contact", "note-legali", "notelegali", "legal", "privacy",
	"termini", "terms", "impressum", "chi-siamo", "chisiamo", "about", "azienda", "company",
}

// identityPageLinks filters a mapped URL list down to the few same-site pages
// worth scraping for an identity check, ranked by identityPagePriority and, on
// ties, by URL length (top-level pages beat deep articles). Pure.
func identityPageLinks(links []string, domain string, limit int) []string {
	type scored struct {
		url      string
		priority int
	}
	seen := map[string]struct{}{}
	matches := []scored{}
	for _, link := range links {
		trimmed := strings.TrimSpace(link)
		if trimmed == "" {
			continue
		}
		host, ok := normalizeDomain(trimmed)
		if !ok || !hostMatchesDomain(host, domain) {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasSuffix(lower, ".pdf") || strings.HasSuffix(lower, ".jpg") ||
			strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".xml") {
			continue
		}
		priority := -1
		for i, marker := range identityPagePriority {
			if strings.Contains(lower, marker) {
				priority = i
				break
			}
		}
		if priority < 0 {
			continue
		}
		if _, dup := seen[lower]; dup {
			continue
		}
		seen[lower] = struct{}{}
		matches = append(matches, scored{url: trimmed, priority: priority})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].priority != matches[j].priority {
			return matches[i].priority < matches[j].priority
		}
		return len(matches[i].url) < len(matches[j].url)
	})
	out := make([]string, 0, limit)
	for _, m := range matches {
		if len(out) >= limit {
			break
		}
		out = append(out, m.url)
	}
	return out
}

// scrapeHomepage fetches a candidate company homepage as markdown, trying the
// www host first (almost always configured, and often the only variant that
// renders — the bare apex frequently 404s or fails the renderer) then falling
// back to the bare apex. Returns the first non-empty, 2xx markdown, or ok=false
// when neither variant yields usable content.
func (s *maService) scrapeHomepage(ctx context.Context, domain string) (string, bool) {
	if s.scrape == nil {
		return "", false
	}
	for _, host := range companyPageHosts(domain) {
		// Full page, not main-content: the homepage markdown feeds the identity
		// checks (P.IVA short-circuit, foreign-identifier guard, name-match) and
		// Italian sites put the P.IVA in the footer the main-content mode strips.
		res, err := s.scrape.ScrapeFull(ctx, "https://"+host)
		if err != nil {
			continue
		}
		if res.StatusCode != 0 && (res.StatusCode < 200 || res.StatusCode >= 400) {
			continue
		}
		if md := strings.TrimSpace(res.Markdown); md != "" {
			return md, true
		}
	}
	return "", false
}

// companyPageHosts returns the host variants to try for a resolved domain,
// www-first then bare. Candidates are apex company domains, so prefixing www is
// safe; the bare fallback covers the rare www-less site. A wrong www guess just
// costs one failed call before the fallback.
func companyPageHosts(domain string) []string {
	d := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), "www.")
	if d == "" {
		return nil
	}
	return []string{"www." + d, d}
}

// normalizeIdentifierForPageMatch reduces a P.IVA / codice fiscale to bare
// alphanumerics for substring matching against page text. Returns "" for values
// too short to match reliably (avoids spurious hits on stray digit runs).
func normalizeIdentifierForPageMatch(value string) string {
	compact := compactAlnum(strings.ToLower(value))
	if len([]rune(compact)) < 8 {
		return ""
	}
	return compact
}

func pageContainsIdentifier(markdown, identifier string) bool {
	if identifier == "" {
		return false
	}
	return strings.Contains(compactAlnum(strings.ToLower(markdown)), identifier)
}

// foreignIdentifierRe matches a standalone 11-digit run — the shape of an Italian
// P.IVA. It is evaluated on a page only AFTER the target's own P.IVA/CF failed to
// match, so any hit is by definition a DIFFERENT entity's identifier.
var foreignIdentifierRe = regexp.MustCompile(`\b\d{11}\b`)

// pageForeignIdentifier returns the first P.IVA-shaped identifier the markdown
// advertises — the wrong-entity signal that withholds brand-label trust from an
// otherwise brand-compatible rendered page (e.g. greenteam.it carrying someone
// else's P.IVA). Callers use it only past the target-identity short-circuit, so a
// match here is a foreign identifier, never the target's. The identifier itself
// is kept: on a brand-compatible site it names the group/other entity (policy B1).
func pageForeignIdentifier(markdown string) (string, bool) {
	id := foreignIdentifierRe.FindString(markdown)
	return id, id != ""
}

// pageContainsCompanyName checks the page carries the company's distinctive name
// tokens as whole words (not substrings — "safe" must not match "creditsafe").
// Requires all tokens for 1-2 token names, >=2 for longer ones. Used only as a
// positive signal (promote a candidate), never to reject.
func pageContainsCompanyName(markdown string, nameTokens []string) bool {
	if len(nameTokens) == 0 {
		return false
	}
	words := pageWordSet(markdown)
	matched := 0
	for _, token := range nameTokens {
		if _, ok := words[token]; ok {
			matched++
		}
	}
	need := min(len(nameTokens), 2)
	return matched >= need
}

func pageWordSet(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(w)) >= 2 {
			out[w] = struct{}{}
		}
	}
	return out
}

var (
	markdownImageRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	markdownLinkRe  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
)

// markdownToEvidenceSnippets turns scraped homepage markdown into a handful of
// prose blocks suitable as classification evidence: link/image syntax is reduced to
// its visible text and short / navigation-like lines are dropped. It is not a full
// markdown parser — the downstream LLM distiller handles the rest.
func markdownToEvidenceSnippets(markdown string) []string {
	plain := strings.TrimSpace(markdown)
	if plain == "" {
		return nil
	}
	plain = markdownImageRe.ReplaceAllString(plain, " ")
	plain = markdownLinkRe.ReplaceAllString(plain, "$1")
	plain = strings.ReplaceAll(plain, "`", " ")
	out := []string{}
	for _, line := range strings.Split(plain, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "#*->| "))
		if len([]rune(line)) < 40 {
			continue // nav items, headings, list bullets, fragments
		}
		out = append(out, line)
		if len(out) >= maScrapeEvidenceChunkCap {
			break
		}
	}
	return out
}

// sectorFinalDecision maps the concept verdict (+ optional analyst override) onto
// the MAWebValidation lifecycle, with NO magic thresholds — the verdict already
// carries the calibrated decision. The analyst only matters when the deterministic
// verdict was ambiguous/no_signal.
func sectorFinalDecision(target MATarget, class maSectorClassification, analysis *CandidateMatchAnalysisResponse, analysisErr string) *CandidateMatchFinalDecision {
	webScore := clampScore(class.TopProb * 100)
	reasons := []string{}
	if class.Reason != "" {
		reasons = append(reasons, class.Reason)
	}
	reasons = append(reasons, sectorConceptReasons(class)...)

	state, action, confidence := "unclear", "needs_business_validation", class.Confidence
	switch class.Verdict {
	case maSectorReject:
		state, action = "rejected", "reject"
	case maSectorWeak:
		state, action = "deprioritized", "deprioritize"
	case maSectorConfirm, maSectorAmbiguous, maSectorNoSignal:
		// confirm no longer auto-confirms: it escalates to the LLM like ambiguous (det
		// `confirm` had 0% precision in eval). The only difference is the no-analyst
		// fallback — confirm honors the deterministic confirm (prior behavior, e.g. LLM
		// disabled); ambiguous/no_signal degrade to needs_business_validation.
		switch {
		case analysis != nil:
			state, action, confidence = analystToLifecycle(analysis)
			if analysis.Verdict != "" {
				reasons = append([]string{"Giudizio LLM: " + analysis.Verdict + " (" + analysis.RecommendedAction + ")."}, reasons...)
			}
		case class.Verdict == maSectorConfirm:
			state, action = "confirmed", "confirm"
		default:
			state = "analysis_unavailable"
			action = "needs_business_validation"
			if confidence == "" {
				confidence = "bassa"
			}
			if strings.TrimSpace(analysisErr) != "" {
				reasons = append([]string{"Analyst non disponibile: " + analysisErr}, reasons...)
			} else {
				reasons = append([]string{"Classificazione non decisiva: validazione business richiesta."}, reasons...)
			}
		}
	}
	if confidence == "" {
		confidence = "bassa"
	}

	decision := &CandidateMatchFinalDecision{
		InitialMatchState:  target.MatchState,
		DeterministicScore: target.Score,
		WebScore:           webScore,
		WebValidationState: state,
		FinalAction:        action,
		Confidence:         confidence,
		Reason:             firstNonEmpty(class.Reason, "Classificazione settore concept-based."),
		Reasons:            cleanStringList(reasons, 12, 280),
	}
	if analysis != nil {
		decision.AnalystVerdict = analysis.Verdict
		decision.AnalystAction = analysis.RecommendedAction
	}
	return decision
}

func analystToLifecycle(analysis *CandidateMatchAnalysisResponse) (state, action, confidence string) {
	confidence = analysis.Confidence
	if confidence == "" {
		confidence = "media"
	}
	switch {
	case analysis.RecommendedAction == "confirm" && (analysis.Verdict == "strong_match" || analysis.Verdict == "match"):
		return "confirmed", "confirm", confidence
	case analysis.RecommendedAction == "reject" || analysis.Verdict == "no_match":
		return "rejected", "reject", confidence
	case analysis.RecommendedAction == "downgrade" || analysis.Verdict == "weak_match":
		return "deprioritized", "deprioritize", confidence
	default:
		return "unclear", "needs_business_validation", confidence
	}
}

func sectorConceptReasons(class maSectorClassification) []string {
	out := []string{}
	for i, c := range class.Concepts {
		if i >= 3 {
			break
		}
		tag := "target"
		if c.Kind == "distractor" {
			tag = "distrattore"
		}
		scope := ""
		if c.InStrategy {
			scope = ", in perimetro"
		}
		out = append(out, fmt.Sprintf("Concetto %s (%s%s): prob %.2f", c.Name, tag, scope, c.RerankProb))
	}
	return out
}

// assembleWebValidationRequest maps the classification + evidence onto the existing
// MAWebValidation contract. The KeywordSet field is repurposed honestly: matched
// target concepts -> CoreTerms, matched distractors -> NegativeTerms (no hardcoded
// negative dogma). Full concept provenance lives in the trace event.
func (s *maService) assembleWebValidationRequest(
	target MATarget,
	inputHash string,
	domainResponse DomainResolutionResponse,
	selectedDomain *DomainResolutionCandidate,
	evidenceRuns []CandidateMatchEvidenceRun,
	class maSectorClassification,
	analysis *CandidateMatchAnalysisResponse,
	analysisErr string,
	decision *CandidateMatchFinalDecision,
) MAWebValidationUpsertRequest {
	if evidenceRuns == nil {
		evidenceRuns = []CandidateMatchEvidenceRun{}
	}
	keywordSet := classificationToKeywordSet(class)
	summary := classificationToSummary(class, decision, analysis)
	return MAWebValidationUpsertRequest{
		PipelineVersion:        maWebValidationPipelineVersion,
		InputHash:              inputHash,
		KeywordSetHash:         inputHash,
		Target:                 target,
		KeywordSet:             keywordSet,
		DomainResponse:         domainResponse,
		SelectedDomain:         selectedDomain,
		EvidenceRuns:           evidenceRuns,
		Summary:                summary,
		CandidateMatchAnalysis: analysis,
		CandidateMatchError:    analysisErr,
		FinalDecision:          *decision,
	}
}

func classificationToKeywordSet(class maSectorClassification) CandidateMatchKeywordSet {
	core := []string{}
	negative := []string{}
	for _, c := range class.Concepts {
		if c.Kind == "distractor" {
			negative = append(negative, c.Name)
		} else {
			core = append(core, c.Name)
		}
	}
	intent := strings.TrimSpace(class.CompanyDescription)
	if intent == "" {
		intent = "Classificazione settore concept-based"
	}
	sources := []string{}
	if len(class.StrategyConcepts) > 0 {
		sources = append(sources, "perimetro strategia: "+strings.Join(class.StrategyConcepts, ", "))
	}
	return CandidateMatchKeywordSet{
		IntentLabel:   cleanText(intent, 220),
		CoreTerms:     cleanStringList(core, 12, 120),
		AdjacentTerms: []string{},
		NegativeTerms: cleanStringList(negative, 12, 120),
		Sources:       cleanStringList(sources, 6, 220),
	}
}

func classificationToSummary(class maSectorClassification, decision *CandidateMatchFinalDecision, analysis *CandidateMatchAnalysisResponse) CandidateMatchEvidenceSummary {
	coreMatches, negativeMatches := 0, 0
	for _, c := range class.Concepts {
		if c.Kind == "distractor" {
			negativeMatches++
		} else {
			coreMatches++
		}
	}
	score := 0
	if decision != nil {
		score = decision.WebScore
	}
	concepts := class.Concepts
	if len(concepts) > 8 {
		concepts = concepts[:8]
	}
	summary := CandidateMatchEvidenceSummary{
		Score:                score,
		Confidence:           class.Confidence,
		CoreMatches:          coreMatches,
		NegativeMatches:      negativeMatches,
		TotalCoreTerms:       coreMatches,
		CompanyDescription:   cleanText(class.CompanyDescription, 1000),
		Concepts:             concepts,
		DeterministicVerdict: string(class.Verdict),
	}
	if analysis != nil {
		summary.LLMVerdictAll = analysis.Verdict
		summary.LLMActionAll = analysis.RecommendedAction
	}
	return summary
}

func (s *maService) recordSectorClassificationTrace(ctx context.Context, target MATarget, class maSectorClassification, decision *CandidateMatchFinalDecision, analystUsed bool) {
	concepts := make([]map[string]any, 0, len(class.Concepts))
	for i, c := range class.Concepts {
		if i >= 6 {
			break
		}
		concepts = append(concepts, map[string]any{
			"concept_id":  c.ID,
			"name":        c.Name,
			"kind":        c.Kind,
			"cosine":      math.Round(c.Cosine*10000) / 10000,
			"rerank_prob": math.Round(c.RerankProb*10000) / 10000,
			"in_strategy": c.InStrategy,
		})
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_sector_classification",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"company_key":       target.CompanyKey,
			"company_name":      target.CompanyName,
			"verdict":           string(class.Verdict),
			"final_action":      decision.FinalAction,
			"top_prob":          math.Round(class.TopProb*10000) / 10000,
			"rerank_applied":    class.RerankApplied,
			"analyst_used":      analystUsed,
			"strategy_concepts": class.StrategyConcepts,
		}),
		Response: maTraceJSON(map[string]any{
			"description": class.CompanyDescription,
			"concepts":    concepts,
		}),
	})
}

func (s *maService) resolveDomainCandidates(ctx context.Context, body DomainResolutionRequest) (DomainResolutionResponse, error) {
	if s.brave == nil {
		return DomainResolutionResponse{}, errMABraveUnavailable
	}
	companyName := strings.Join(strings.Fields(strings.TrimSpace(body.CompanyName)), " ")
	if companyName == "" {
		return DomainResolutionResponse{}, fmt.Errorf("%w: company name", errMAStrategyInvalid)
	}
	count := body.Count
	if count <= 0 {
		count = domainResolutionDefaultCount
	}
	if count > domainResolutionMaxCount {
		count = domainResolutionMaxCount
	}

	queries := buildDomainResolutionQueries(body, companyName)
	results := []WebSearchResult{}
	seenResults := map[string]struct{}{}
	executedQueries := []string{}
	candidates := []DomainResolutionCandidate{}
	for _, query := range queries {
		if len(query) > webSearchMaxQueryLen || len(strings.Fields(query)) > webSearchMaxWords {
			continue
		}
		executedQueries = append(executedQueries, query)
		res, err := s.brave.LLMContext(ctx, brave.LLMContextParams{
			Query:      query,
			Count:      count,
			Country:    "it",
			SearchLang: "it",
		})
		if err != nil {
			return DomainResolutionResponse{}, err
		}
		for _, g := range res.Generic {
			host := resultHostname(g.URL, res.Sources)
			if strings.TrimSpace(host) == "" {
				continue
			}
			if _, exists := seenResults[g.URL]; exists {
				continue
			}
			seenResults[g.URL] = struct{}{}
			results = append(results, WebSearchResult{
				Title:    g.Title,
				URL:      g.URL,
				Hostname: host,
				Age:      firstAge(res.Sources[g.URL].Age),
				Snippets: g.Snippets,
			})
		}
		candidates = rankDomainResolutionCandidates(body, companyName, results)
		if hasCredibleDomainResolutionCandidate(candidates) {
			break
		}
	}

	// Retrieval fallback: Brave surfaced no credible official-site candidate (only
	// registries/aggregators, or nothing). Try the fastcrw web search — it finds
	// distinctive-brand company sites Brave misses (weytec.com, direte.it,
	// digitalvirgo.com). Results feed the SAME ranker, so registries stay filtered
	// and brand-match still decides. Fires only on Brave failure, so it never
	// overrides a Brave success (avoids same-brand-wrong-company regressions). No
	// P.IVA in the query (same anti-registry rule as Brave).
	if s.scrape != nil && !hasCredibleDomainResolutionCandidate(candidates) {
		for _, query := range buildFastcrwDomainQueries(body, companyName) {
			if len(query) > webSearchMaxQueryLen || len(strings.Fields(query)) > webSearchMaxWords {
				continue
			}
			hits, err := s.scrape.Search(ctx, query, count)
			if err != nil {
				break // soft dependency: keep whatever Brave returned
			}
			executedQueries = append(executedQueries, "fastcrw:"+query)
			for _, h := range hits {
				host, ok := normalizeDomain(h.URL)
				if !ok {
					continue
				}
				if _, exists := seenResults[h.URL]; exists {
					continue
				}
				seenResults[h.URL] = struct{}{}
				results = append(results, WebSearchResult{
					Title:    h.Title,
					URL:      h.URL,
					Hostname: host,
					Snippets: dedupNonEmpty([]string{h.Description, h.Snippet}),
				})
			}
			candidates = rankDomainResolutionCandidates(body, companyName, results)
			if hasCredibleDomainResolutionCandidate(candidates) {
				break
			}
		}
	}

	if len(executedQueries) == 0 {
		return DomainResolutionResponse{}, fmt.Errorf("%w: query too long", errMAStrategyInvalid)
	}
	return DomainResolutionResponse{
		Query:      strings.Join(executedQueries, " | "),
		Count:      count,
		Candidates: candidates,
		Results:    results,
	}, nil
}

func (s *maService) searchDomainEvidence(ctx context.Context, body WebSearchRequest, subject, email string) (WebSearchResponse, error) {
	if s.brave == nil {
		return WebSearchResponse{}, errMABraveUnavailable
	}
	domain, ok := normalizeDomain(body.Domain)
	if !ok {
		return WebSearchResponse{}, fmt.Errorf("%w: domain", errMAStrategyInvalid)
	}
	keywords := cleanStringList(body.Keywords, 20, 120)
	if len(keywords) == 0 {
		return WebSearchResponse{}, fmt.Errorf("%w: keywords", errMAStrategyInvalid)
	}
	count := body.Count
	if count <= 0 {
		count = webSearchDefaultCount
	}
	if count > webSearchMaxCount {
		count = webSearchMaxCount
	}
	query := "site:" + domain + " " + strings.Join(keywords, " ")
	if len(query) > webSearchMaxQueryLen || len(strings.Fields(query)) > webSearchMaxWords {
		return WebSearchResponse{}, fmt.Errorf("%w: query too long", errMAStrategyInvalid)
	}

	res, err := s.brave.LLMContext(ctx, brave.LLMContextParams{
		Query:      query,
		Count:      count,
		Country:    "it",
		SearchLang: "it",
	})
	if err != nil {
		return WebSearchResponse{}, err
	}
	results := make([]WebSearchResult, 0, len(res.Generic))
	for _, g := range res.Generic {
		host := resultHostname(g.URL, res.Sources)
		if !hostMatchesDomain(host, domain) {
			continue
		}
		results = append(results, WebSearchResult{
			Title:    g.Title,
			URL:      g.URL,
			Hostname: host,
			Age:      firstAge(res.Sources[g.URL].Age),
			Snippets: g.Snippets,
		})
	}

	ranked := false
	rankErr := ""
	if body.Rank && len(results) > 0 {
		scores, scoreErr := s.scoreWebSearchResults(ctx, strings.Join(keywords, " "), results, subject, email)
		if scoreErr != nil {
			rankErr = cleanText(scoreErr.Error(), 180)
		} else {
			for i := range results {
				if sc, ok := scores[i]; ok {
					value := sc
					results[i].Score = &value
				}
			}
			sort.SliceStable(results, func(a, b int) bool {
				return scoreOf(results[a]) > scoreOf(results[b])
			})
			ranked = true
		}
	}
	return WebSearchResponse{Query: query, Count: count, Ranked: ranked, RankError: rankErr, Results: results}, nil
}

func normalizeMAWebValidationPayload(req MAWebValidationEnrichRequest) maWebValidationJobPayload {
	analyzeWithLLM := true
	if req.AnalyzeWithLLM != nil {
		analyzeWithLLM = *req.AnalyzeWithLLM
	}
	llmOnAll := false
	if req.LLMOnAll != nil {
		llmOnAll = *req.LLMOnAll
	}
	rank := true
	if req.Rank != nil {
		rank = *req.Rank
	}
	limit := req.Limit
	if limit <= 0 {
		limit = maWebValidationDefaultLimit
	}
	if limit > maWebValidationMaxLimit {
		limit = maWebValidationMaxLimit
	}
	domainCount := req.DomainCount
	if domainCount <= 0 {
		domainCount = maWebValidationDefaultDomainCount
	}
	if domainCount > domainResolutionMaxCount {
		domainCount = domainResolutionMaxCount
	}
	keywordCount := req.KeywordCount
	if keywordCount <= 0 {
		keywordCount = maWebValidationEvidenceCount
	}
	if keywordCount > webSearchMaxCount {
		keywordCount = webSearchMaxCount
	}
	return maWebValidationJobPayload{
		Limit:              limit,
		Force:              req.Force,
		IncludeIdentifiers: req.IncludeIdentifiers,
		AnalyzeWithLLM:     analyzeWithLLM,
		LLMOnAll:           llmOnAll,
		DomainCount:        domainCount,
		KeywordCount:       keywordCount,
		Rank:               rank,
		Inline:             req.Inline,
	}
}

type maWebValidationWorkTarget struct {
	Target    MATarget
	InputHash string
}

func selectMAWebValidationTargets(targets []MATarget, strategy MAStrategySpec, payload maWebValidationJobPayload, now time.Time, limit int) []maWebValidationWorkTarget {
	out := make([]maWebValidationWorkTarget, 0, len(targets))
	for _, target := range targets {
		if target.MatchState == maMatchStateOutside {
			continue
		}
		inputHash := maWebValidationInputHash(target, strategy, payload)
		if !payload.Force && reusableMAWebValidation(target.WebValidation, inputHash, inputHash, now) {
			continue
		}
		out = append(out, maWebValidationWorkTarget{
			Target:    target,
			InputHash: inputHash,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri := targetRatingValue(out[i].Target)
		rj := targetRatingValue(out[j].Target)
		if ri != rj {
			return ri > rj
		}
		if out[i].Target.Score != out[j].Target.Score {
			return out[i].Target.Score > out[j].Target.Score
		}
		return out[i].Target.CompanyName < out[j].Target.CompanyName
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func targetRatingValue(target MATarget) int {
	if target.Rating == nil {
		return 0
	}
	return *target.Rating
}

func reusableMAWebValidation(validation *MAWebValidation, inputHash, keywordSetHash string, now time.Time) bool {
	return validation != nil &&
		validation.PipelineVersion == maWebValidationPipelineVersion &&
		validation.InputHash == inputHash &&
		validation.KeywordSetHash == keywordSetHash &&
		maWebValidationFreshness(now, validation.StaleAfter, validation.ExpiresAt) == maWebValidationFresh
}

func failedMAWebValidationRequest(target MATarget, inputHash string, err error) MAWebValidationUpsertRequest {
	errorText := cleanText(err.Error(), 180)
	decision := &CandidateMatchFinalDecision{
		InitialMatchState:  target.MatchState,
		DeterministicScore: target.Score,
		WebValidationState: "analysis_unavailable",
		FinalAction:        "needs_business_validation",
		Confidence:         "bassa",
		Reason:             "Validazione web fallita.",
		Reasons:            []string{"Errore pipeline: " + errorText},
	}
	return MAWebValidationUpsertRequest{
		PipelineVersion:     maWebValidationPipelineVersion,
		InputHash:           inputHash,
		KeywordSetHash:      inputHash,
		Target:              target,
		KeywordSet:          CandidateMatchKeywordSet{IntentLabel: "Validazione web fallita"},
		DomainResponse:      DomainResolutionResponse{},
		EvidenceRuns:        []CandidateMatchEvidenceRun{},
		Summary:             CandidateMatchEvidenceSummary{Confidence: "bassa"},
		CandidateMatchError: errorText,
		FinalDecision:       *decision,
	}
}

func chooseMAWebValidationDomain(candidates []DomainResolutionCandidate) *DomainResolutionCandidate {
	credibleReasons := map[string]struct{}{
		"host compatibile con ragione sociale": {},
		"dominio citato come sameAs":           {},
		"dominio citato come sito ufficiale":   {},
		"email aziendale su dominio":           {},
	}
	for _, candidate := range candidates {
		if candidate.Confidence == "bassa" || candidate.Score < 45 {
			continue
		}
		for _, reason := range candidate.Reasons {
			if _, ok := credibleReasons[reason]; ok {
				copyCandidate := candidate
				return &copyCandidate
			}
		}
	}
	return nil
}

func maWebValidationInputHash(target MATarget, strategy MAStrategySpec, payload maWebValidationJobPayload) string {
	return maWebValidationHash(map[string]any{
		"pipelineVersion": maWebValidationPipelineVersion,
		"target":          maWebValidationTargetFingerprint(target),
		"sector":          cleanText(strategy.SectorDescription, 400),
		"options":         maWebValidationInputOptions(payload),
	})
}

func maWebValidationInputOptions(payload maWebValidationJobPayload) map[string]any {
	return map[string]any{
		"analyzeWithLLM":     payload.AnalyzeWithLLM,
		"llmOnAll":           payload.LLMOnAll,
		"domainCount":        payload.DomainCount,
		"includeIdentifiers": payload.IncludeIdentifiers,
		"keywordCount":       payload.KeywordCount,
		"rank":               payload.Rank,
	}
}

func clampScore(value float64) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return int(value + 0.5)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func optionalIdentifier(enabled bool, value string) string {
	if !enabled {
		return ""
	}
	return value
}

func boolPtr(value bool) *bool {
	return &value
}
