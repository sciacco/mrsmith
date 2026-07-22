package binocolo

// Nota-integrativa reading (issue #78, Fase 6). The last ingest stage: an LLM reads the NI
// sections of a processing run to extract the facts that are INVISIBLE to the CEE prospetti /
// PFN bridge (already computed upstream), and proposes normalizing adjustments to EBITDA / PFN
// or flags due-diligence themes. Every proposal is mechanically grounded — its verbatim quote
// must exist on the cited page and its amount must appear in that quote — so a hallucinated
// figure is discarded or demoted, never stored as fact. Proposals are IMMUTABLE and analyst
// decisions APPEND-ONLY; a re-read carries the analyst's standing decisions forward
// (auto-reconfirmation) and the reducer collapses the decision history into effective
// adjustments (consumed by F7). This file owns the maService orchestration plus the pure,
// unit-tested helpers (fingerprint, citation validation, chunking, reducer, auto-reconfirm
// matcher); the SQLStore methods live in ma_ni_store.go.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// NI vocabulary — the stored enum values (English; the DB CHECK constraints in mig 114 are the
// canonical source). Copy in the UI is Italian B2B (F9).
const (
	maNIActionRatify = "ratify"
	maNIActionReject = "reject"
	maNIActionRevoke = "revoke"

	maNITrattamentoEbitda = "ebitda"
	maNITrattamentoPFN    = "pfn"
	maNITrattamentoDDOnly = "dd_only"

	maNIDirectionIncrease  = "increase"
	maNIDirectionDecrease  = "decrease"
	maNIDirectionUncertain = "uncertain"

	maNIIncertezzaLow    = "low"
	maNIIncertezzaMedium = "medium"
	maNIIncertezzaHigh   = "high"

	// maNISectionCharBudget is the char ceiling for accreting small NI sections into a single
	// LLM call: one section per call by default, but consecutive small sections are batched
	// until adding the next would exceed this budget (a section larger than the budget is its
	// own chunk — never split, so a citation is never severed across calls). ~12k chars keeps a
	// chunk well within the reasoning-model context while collapsing the many tiny abbreviato
	// sections into a handful of calls.
	maNISectionCharBudget = 12000
)

var (
	maNIValidTrattamento = map[string]bool{maNITrattamentoEbitda: true, maNITrattamentoPFN: true, maNITrattamentoDDOnly: true}
	maNIValidDirection   = map[string]bool{maNIDirectionIncrease: true, maNIDirectionDecrease: true, maNIDirectionUncertain: true}
	maNIValidIncertezza  = map[string]bool{maNIIncertezzaLow: true, maNIIncertezzaMedium: true, maNIIncertezzaHigh: true}

	// maNIAmountTokenRe extracts number-like tokens from a quote for the amount-in-quote check
	// (Italian formats: 250.000 / 250.000,00 / €250.000 — the € is not captured, parsing strips
	// currency markers anyway). The tokens are parsed with parseItalianNumber, so the match is
	// NUMERIC (value equality within the euro half-slack), never textual.
	maNIAmountTokenRe = regexp.MustCompile(`[0-9][0-9.,]*`)
)

// maNIProposalLLM is one proposal exactly as the mig-117 prompt emits it (camelCase). Parsed
// defensively, then validated + normalized into a stored maNIProposal.
type maNIProposalLLM struct {
	ExerciseDate         string   `json:"exerciseDate"`
	FattoOsservato       string   `json:"fattoOsservato"`
	ImportoLordo         *float64 `json:"importoLordo"`
	TrattamentoCandidato string   `json:"trattamentoCandidato"`
	Direction            string   `json:"direction"`
	ImportoRettifica     *float64 `json:"importoRettifica"`
	Incertezza           string   `json:"incertezza"`
	Quote                string   `json:"quote"`
	PageNo               *int     `json:"pageNo"`
	Section              string   `json:"section"`
	Label                string   `json:"label"`
	Rationale            string   `json:"rationale"`
}

// maNIEffectiveAdjustment is one adjustment the analyst's decisions make EFFECTIVE for the
// active reading run (the reducer's output). Treatment is ebitda|pfn (dd_only never surfaces
// here); Amount is SIGNED. The reducer does NOT sum — F7 groups/filters by exercise alignment.
type maNIEffectiveAdjustment struct {
	ProposalID   string
	Treatment    string
	ExerciseDate *time.Time
	Amount       float64
}

// ---------------------------------------------------------------------------
// Ingest stage: NI reading (replaces the F5 poll-pending sentinel).
// ---------------------------------------------------------------------------

// filingIngestNIReading runs the final ingest stage (ni_reading → ready|degraded): resolve the
// registry model/prompt, open (or reuse) an active reading run, call the LLM once per NI-section
// chunk, mechanically validate + persist the proposals, auto-reconfirm the analyst's standing
// decisions, and close the filing. A retryable LLM error is returned RAW so the worker retries
// with the infra backoff (cap maJobMaxAttempts — "NI = backoff"); the filing NEVER stays at
// ni_reading after a successful run. `ready` when a usable deep baseline exists for the fiscal
// identity, else `degraded` (the scheda declares the adjusted analysis unavailable).
func (s *maService) filingIngestNIReading(ctx context.Context, filing *maFiling, requestID string) error {
	if s.llmp == nil {
		return errMAStoreUnavailable
	}
	runID := strings.TrimSpace(filing.ActiveProcessingRunID)
	if runID == "" {
		return s.failFilingIngest(ctx, filing.ID, "processing run assente: rilanciare l'ocr")
	}
	// Registry model/prompt: their uuids are the reading run's soft-refs (contract QA-F1).
	model, err := s.llmp.ResolveModel(ctx, maModelScopeNIReading, "")
	if err != nil {
		return llmConfigError(err)
	}
	prompt, err := s.llmp.ResolvePrompt(ctx, maModelScopeNIReading, "")
	if err != nil {
		return llmConfigError(err)
	}
	client, err := s.llmp.ClientForModel(ctx, model)
	if err != nil {
		return err
	}

	pages, err := s.filing.GetMAFilingPages(ctx, runID)
	if err != nil {
		return err
	}
	sections := sectionMAFilingNI(pages)
	extracts, err := s.filing.GetMAFilingExtracts(ctx, runID)
	if err != nil {
		return err
	}
	exerciseSet := map[string]bool{}
	exerciseISO := make([]string, 0, len(extracts))
	var baseline *time.Time
	for i := range extracts {
		key := maDateValue(extracts[i].ExerciseDate)
		if !exerciseSet[key] {
			exerciseSet[key] = true
			exerciseISO = append(exerciseISO, key)
		}
		d := extracts[i].ExerciseDate
		if baseline == nil || d.After(*baseline) {
			baseline = &d
		}
	}
	if filing.ClosingDate != nil {
		baseline = filing.ClosingDate
	}

	// Reuse the active reading run when it belongs to THIS processing run (a retry after an LLM
	// error), else open a new one (a re-OCR produced a new processing run — supersede the old).
	var niRunID string
	active, err := s.filing.GetActiveMANIReadingRun(ctx, filing.ID)
	if err != nil {
		return err
	}
	if active != nil && active.ProcessingRunID == runID {
		niRunID = active.ID
	} else {
		niRunID, err = s.filing.CreateMANIReadingRun(ctx, filing.ID, runID, prompt.ID, model.ID, requestID)
		if err != nil {
			return err
		}
	}
	s.filingIngestTrace(ctx, "ni_run", maFilingSysInternal, maTraceEventSucceeded, map[string]any{"filing_id": filing.ID, "run_id": niRunID, "sections": len(sections)}, "")

	company := map[string]any{}
	if filing.VATClean != "" {
		company["vat"] = filing.VATClean
	}
	if filing.TaxClean != "" {
		company["tax"] = filing.TaxClean
	}
	if baseline != nil {
		company["baselineExercise"] = maDateValue(*baseline)
	}

	// One LLM call per chunk of NI sections.
	var llmProps []maNIProposalLLM
	for _, chunk := range chunkMANISections(sections, maNISectionCharBudget) {
		chunkProps, cerr := s.readMANIChunk(ctx, model, prompt, client, requestID, filing.ID, niRunID, company, chunk, exerciseISO)
		if cerr != nil {
			return cerr // raw LLM error → worker retryOrFail (backoff, cap maJobMaxAttempts)
		}
		llmProps = append(llmProps, chunkProps...)
	}

	// Mechanical validation (enum / exercise-range / citation-grounding) then immutable persist.
	idx := buildMANIPageIndex(pages)
	proposals, stats := s.buildMANIProposals(ctx, llmProps, exerciseSet, idx)
	inserted, err := s.filing.InsertMANIProposals(ctx, niRunID, proposals)
	if err != nil {
		return err
	}
	s.filingIngestTrace(ctx, "ni_validated", maFilingSysInternal, maTraceEventInfo, map[string]any{
		"run_id": niRunID, "candidates": len(llmProps), "kept": len(proposals),
		"inserted": inserted, "discarded": stats.Discarded, "demoted": stats.Demoted,
	}, "")

	// Carry the analyst's standing decisions forward onto identical facts (best-effort).
	s.autoReconfirmMANIDecisions(ctx, filing.ID, niRunID)

	// Close the filing: ready when a usable deep baseline exists, else degraded.
	status := s.resolveMANIFinalStatus(ctx, filing)
	if err := s.filing.UpdateMAFilingStatus(ctx, filing.ID, status, ""); err != nil {
		return err
	}
	s.filingIngestTrace(ctx, "ni_done", maFilingSysInternal, maTraceEventSucceeded, map[string]any{"filing_id": filing.ID, "status": status, "proposals": len(proposals)}, "")

	// NI NARRATIVE reading (issue #80, Fase N1): a SEPARATE analysis run AFTER the proposals
	// reading in the SAME tick. Best-effort by contract — its failure NEVER changes the filing
	// outcome (ready|degraded already decided by the proposals above), it only logs and leaves a
	// regenerable 'failed' run.
	s.runMANINarrativeBestEffort(ctx, filing, requestID)
	return nil
}

// readMANIChunk performs one NI-section-chunk LLM call: builds the {company, sections, exercises}
// input, calls chat in json_object mode, records the audit (RequestID = the ingest trace id;
// Context correlates filing/run/section, mirroring the OCR call), and defensively parses the
// proposals. A chat/parse error is returned raw (retryable).
func (s *maService) readMANIChunk(ctx context.Context, model llm.Model, prompt llm.Prompt, client maAIClient, requestID, filingID, runID string, company map[string]any, chunk []maNISection, exercises []string) ([]maNIProposalLLM, error) {
	sections := make([]map[string]any, 0, len(chunk))
	labels := make([]string, 0, len(chunk))
	for _, sec := range chunk {
		sections = append(sections, map[string]any{
			"label": sec.Label,
			"pages": maNISectionPagesLabel(sec),
			"text":  sec.Markdown,
		})
		labels = append(labels, sec.Label)
	}
	payload, err := json.Marshal(map[string]any{"company": company, "sections": sections, "exercises": exercises})
	if err != nil {
		return nil, err
	}
	reqParams := model.RawParams()
	if _, ok := reqParams["max_tokens"]; !ok {
		reqParams["max_tokens"] = 5000
	}
	chatReq := llm.ChatRequest{
		Model:          model.Model,
		Params:         reqParams,
		ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
		Messages: []llm.Message{
			{Role: "system", Content: prompt.Prompt},
			{Role: "user", Content: string(payload)},
		},
	}
	s.filingIngestTrace(ctx, "ni_section", maFilingSysLLM, maTraceEventStarted, map[string]any{"run_id": runID, "sections": labels}, "")
	resp, chatErr := client.Chat(ctx, chatReq)
	usageRaw, _ := json.Marshal(resp.Usage)
	requestBody, _ := llm.BuildRequestBody(chatReq)
	requestRaw, _ := json.Marshal(requestBody)
	audit := llm.CallAudit{
		App:        maApp,
		Scope:      maModelScopeNIReading,
		ProviderID: model.ProviderID,
		ModelID:    model.ID,
		PromptID:   prompt.ID,
		Model:      model.Model,
		Request:    requestRaw,
		Usage:      usageRaw,
		RequestID:  requestID,
		Context:    maTraceJSON(map[string]any{"filingId": filingID, "runId": runID, "section": strings.Join(labels, ", ")}),
	}
	if chatErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = chatErr.Error()
	} else if respRaw, mErr := json.Marshal(map[string]any{"content": resp.Content}); mErr == nil {
		audit.Response = respRaw
	}
	_ = s.llmp.RecordAudit(ctx, audit)
	if chatErr != nil {
		s.filingIngestTrace(ctx, "ni_section", maFilingSysLLM, maTraceEventFailed, map[string]any{"run_id": runID}, chatErr.Error())
		return nil, chatErr
	}
	props, err := parseMANIProposals(resp.Content)
	if err != nil {
		s.filingIngestTrace(ctx, "ni_section", maFilingSysLLM, maTraceEventFailed, map[string]any{"run_id": runID}, err.Error())
		return nil, err
	}
	s.filingIngestTrace(ctx, "ni_section", maFilingSysLLM, maTraceEventSucceeded, map[string]any{"run_id": runID, "proposals": len(props)}, "")
	return props, nil
}

// resolveMANIFinalStatus decides the filing's terminal ingest state: ready when a deep-dive
// baseline exists for the fiscal identity and is usable (ready/queued/running), else degraded
// (absent or failed). A lookup error degrades (never blocks the close).
func (s *maService) resolveMANIFinalStatus(ctx context.Context, filing *maFiling) string {
	if s.store == nil {
		return maFilingStatusDegraded
	}
	deep, err := s.store.GetMADeepByFiscalIdentity(ctx, filing.VATClean, filing.TaxClean)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo filing ni deep baseline lookup failed",
			"component", "binocolo", "operation", "ma_filing_ingest", "filing_id", filing.ID, "error", err)
		return maFilingStatusDegraded
	}
	if deep != nil {
		switch deep.Status {
		case maDeepStatusReady, maDeepStatusQueued, maDeepStatusRunning:
			return maFilingStatusReady
		}
	}
	return maFilingStatusDegraded
}

// autoReconfirmMANIDecisions clones the analyst's standing decision from a prior run onto each
// identical-fingerprint proposal of the new run (best-effort: a failure never blocks the close —
// the proposals are already persisted and the analyst can re-decide).
func (s *maService) autoReconfirmMANIDecisions(ctx context.Context, filingID, runID string) {
	candidates, err := s.filing.ListMANIAutoReconfirmCandidates(ctx, filingID, runID)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo filing ni auto-reconfirm lookup failed",
			"component", "binocolo", "operation", "ma_filing_ingest", "filing_id", filingID, "error", err)
		return
	}
	reconfirmed := 0
	for _, c := range candidates {
		if _, err := s.filing.InsertMANIDecision(ctx, maNIDecisionCreate{
			ProposalID:          c.NewProposalID,
			Action:              c.Action,
			RatifiedAmount:      c.RatifiedAmount,
			RatifiedTreatment:   c.RatifiedTreatment,
			Reason:              c.Reason,
			ActorSubject:        c.ActorSubject,
			ActorEmail:          c.ActorEmail,
			AutoReconfirmedFrom: c.HistoricalDecisionID,
		}); err != nil {
			logging.FromContext(ctx).Warn("binocolo filing ni auto-reconfirm clone failed",
				"component", "binocolo", "operation", "ma_filing_ingest", "filing_id", filingID, "proposal_id", c.NewProposalID, "error", err)
			continue
		}
		reconfirmed++
	}
	if reconfirmed > 0 {
		s.filingIngestTrace(ctx, "ni_auto_reconfirm", maFilingSysInternal, maTraceEventInfo, map[string]any{"run_id": runID, "reconfirmed": reconfirmed}, "")
	}
}

// maNIValidationStats counts the mechanical outcomes of one reading (surfaced in the trace).
type maNIValidationStats struct {
	Discarded int // enum out-of-vocabulary OR quote not grounded on the cited page(s)
	Demoted   int // amount not in the quote OR exercise outside the fascicolo ⇒ dd_only
}

// buildMANIProposals turns the raw LLM proposals into the immutable stored shape, applying the
// mechanical guards in order: (1) enum out-of-vocabulary ⇒ discard; (2) quote not grounded on
// the cited page (±1) ⇒ discard; (3) amount not in the quote, or exercise outside the fascicolo
// ⇒ demote to dd_only (importoRettifica nil) with a note. The fingerprint is computed on the
// FINAL (post-demotion) values, and the batch is deduplicated by fingerprint (belt-and-suspenders
// with the run's UNIQUE index).
func (s *maService) buildMANIProposals(ctx context.Context, llmProps []maNIProposalLLM, exerciseSet map[string]bool, idx maNIPageIndex) ([]maNIProposal, maNIValidationStats) {
	var out []maNIProposal
	var stats maNIValidationStats
	log := logging.FromContext(ctx)
	for _, lp := range llmProps {
		trattamento := strings.ToLower(strings.TrimSpace(lp.TrattamentoCandidato))
		direction := strings.ToLower(strings.TrimSpace(lp.Direction))
		incertezza := strings.ToLower(strings.TrimSpace(lp.Incertezza))
		if !maNIValidTrattamento[trattamento] || !maNIValidDirection[direction] || !maNIValidIncertezza[incertezza] {
			stats.Discarded++
			log.Warn("binocolo ni proposal discarded: enum out of vocabulary",
				"component", "binocolo", "operation", "ma_filing_ingest",
				"trattamento", trattamento, "direction", direction, "incertezza", incertezza)
			continue
		}

		var exDate *time.Time
		if d, ok := maParseISODate(lp.ExerciseDate); ok {
			exDate = &d
		}
		outOfFascicolo := exDate == nil || !exerciseSet[maDateValue(*exDate)]

		verdict := validateMANICitation(lp, idx)
		if verdict.Discard {
			stats.Discarded++
			log.Warn("binocolo ni proposal discarded: quote not grounded on cited page",
				"component", "binocolo", "operation", "ma_filing_ingest", "page_no", lp.PageNo)
			continue
		}

		pageNo := lp.PageNo
		if verdict.CorrectedPage != nil {
			pageNo = verdict.CorrectedPage
		}
		notes := make([]string, 0, 3)
		if verdict.Note != "" {
			notes = append(notes, verdict.Note)
		}

		importoLordo := lp.ImportoLordo
		importoRettifica := lp.ImportoRettifica
		demoted := false
		if outOfFascicolo {
			notes = append(notes, "esercizio non presente nel fascicolo: declassato a due-diligence")
			demoted = true
		}
		if verdict.AmountMissing && (trattamento == maNITrattamentoEbitda || trattamento == maNITrattamentoPFN) {
			notes = append(notes, "importo non presente nella citazione: declassato a due-diligence")
			demoted = true
		}
		if demoted {
			trattamento = maNITrattamentoDDOnly
			stats.Demoted++
		}
		// dd_only carries NO quantified effect: clear importo_rettifica for EVERY dd_only —
		// whether native (the LLM emitted dd_only) or demoted — so the stored column never
		// shows a rettifica the reducer ignores but the UI might surface as a proposal.
		if trattamento == maNITrattamentoDDOnly {
			importoRettifica = nil
		}

		rationale := strings.TrimSpace(lp.Rationale)
		if len(notes) > 0 {
			rationale = strings.TrimSpace(rationale + " [" + strings.Join(notes, "; ") + "]")
		}
		fingerprint := maNIProposalFingerprint(trattamento, exDate, importoLordo, importoRettifica, lp.Quote)
		out = append(out, maNIProposal{
			ExerciseDate:         exDate,
			Fingerprint:          fingerprint,
			FattoOsservato:       cleanText(lp.FattoOsservato, 500),
			ImportoLordo:         importoLordo,
			TrattamentoCandidato: trattamento,
			Direction:            direction,
			ImportoRettifica:     importoRettifica,
			Incertezza:           incertezza,
			Quote:                cleanText(lp.Quote, 1000),
			PageNo:               pageNo,
			Section:              cleanText(lp.Section, 200),
			Label:                cleanText(lp.Label, 200),
			Rationale:            cleanText(rationale, 800),
		})
	}
	return dedupMANIProposalsByFingerprint(out), stats
}

// dedupMANIProposalsByFingerprint keeps the first proposal per fingerprint within one batch
// (the run's UNIQUE(run_id, fingerprint) is the durable guard; this keeps the inserted count
// deterministic even against an in-memory/fake store).
func dedupMANIProposalsByFingerprint(proposals []maNIProposal) []maNIProposal {
	seen := make(map[string]bool, len(proposals))
	out := make([]maNIProposal, 0, len(proposals))
	for _, p := range proposals {
		if seen[p.Fingerprint] {
			continue
		}
		seen[p.Fingerprint] = true
		out = append(out, p)
	}
	return out
}

// ---------------------------------------------------------------------------
// Citation validation (mechanical anti-hallucination).
// ---------------------------------------------------------------------------

// maNIPageIndex is the per-page normalized markdown of a processing run, plus the sorted page
// order, used to ground a proposal's quote on its cited page (with a ±1 tolerance).
type maNIPageIndex struct {
	norm  map[int]string
	order []int
}

// buildMANIPageIndex normalizes every OCR page once (lowercase + whitespace-collapsed) so the
// quote-grounding check is a cheap substring test per proposal. It indexes the REASSEMBLED
// page text (tables inlined from extras), so a quote that cites a figure living in a table
// still anchors — otherwise every table-sourced citation would be discarded.
func buildMANIPageIndex(pages []maFilingPage) maNIPageIndex {
	idx := maNIPageIndex{norm: make(map[int]string, len(pages))}
	for _, p := range pages {
		idx.norm[p.PageNo] = maNormalizeQuote(maFilingPageText(p))
		idx.order = append(idx.order, p.PageNo)
	}
	sort.Ints(idx.order)
	return idx
}

// maNICitationVerdict is the mechanical citation outcome for one proposal.
type maNICitationVerdict struct {
	Discard       bool   // (a) the quote is not grounded on the cited page (±1) ⇒ discard
	AmountMissing bool   // (b) the declared amount is not present in the quote ⇒ demote dd_only
	CorrectedPage *int   // the neighbor page the quote was actually found on (page_no correction)
	Note          string // human-readable page-correction note ("" when none)
}

// validateMANICitation enforces the two mechanical grounding rules WITHOUT ever fabricating
// text: (a) the normalized quote must occur in the normalized markdown of the cited page — or,
// tolerating an OCR off-by-one, page ±1 (recording the correction); a quote found nowhere is a
// DISCARD. (b) the amount(s) (importoLordo, and importoRettifica when set) must appear IN the
// quote as an Italian-formatted number; a missing amount is flagged for DEMOTION by the caller.
func validateMANICitation(prop maNIProposalLLM, idx maNIPageIndex) maNICitationVerdict {
	var v maNICitationVerdict
	quoteNorm := maNormalizeQuote(prop.Quote)
	if quoteNorm == "" {
		v.Discard = true
		return v
	}
	grounded, correctedPage, note := maNIQuoteGrounding(quoteNorm, prop.PageNo, idx)
	if !grounded {
		v.Discard = true
		return v
	}
	v.CorrectedPage = correctedPage
	v.Note = note

	if !maNICitationAmountGrounded(prop.Quote, prop) {
		v.AmountMissing = true
	}
	return v
}

// maNIQuoteGrounding is the shared mechanical anchor of a citation to the REASSEMBLED OCR text
// (tables inlined via maFilingPageText in buildMANIPageIndex) — used by BOTH the proposals reading
// (validateMANICitation) and the narrative reading (issue #80). The normalized quote must occur in
// the normalized markdown of the cited page — or, tolerating an OCR off-by-one, page ±1 — else,
// when no page is cited, the first page it occurs on in page order. It NEVER fabricates text.
// Returns: grounded; the corrected page (the neighbor/deduced page the quote was actually found
// on, nil when the cited page matched); and a human-readable correction note ("" when none).
func maNIQuoteGrounding(quoteNorm string, pageNo *int, idx maNIPageIndex) (grounded bool, correctedPage *int, note string) {
	if quoteNorm == "" {
		return false, nil, ""
	}
	if pageNo != nil {
		pn := *pageNo
		for _, cand := range []int{pn, pn - 1, pn + 1} {
			txt, ok := idx.norm[cand]
			if !ok {
				continue
			}
			if strings.Contains(txt, quoteNorm) {
				if cand != pn {
					c := cand
					return true, &c, fmt.Sprintf("pagina citazione corretta da %d a %d", pn, cand)
				}
				return true, nil, ""
			}
		}
		return false, nil, ""
	}
	// No page cited: search in page order (deterministic) and adopt the first match.
	for _, p := range idx.order {
		if strings.Contains(idx.norm[p], quoteNorm) {
			c := p
			return true, &c, fmt.Sprintf("pagina citazione dedotta: %d", p)
		}
	}
	return false, nil, ""
}

// maNICitationAmountGrounded reports whether the proposal's declared amount(s) appear in its
// quote. importoLordo must always appear (a proposal without a text-grounded gross figure has an
// ungrounded number); importoRettifica must appear too when set. A missing importoLordo counts
// as ungrounded (the caller demotes to dd_only).
func maNICitationAmountGrounded(quote string, prop maNIProposalLLM) bool {
	if prop.ImportoLordo == nil {
		return false
	}
	if !maQuoteContainsAmount(quote, *prop.ImportoLordo) {
		return false
	}
	if prop.ImportoRettifica != nil && !maQuoteContainsAmount(quote, *prop.ImportoRettifica) {
		return false
	}
	return true
}

// maQuoteContainsAmount reports whether an Italian-formatted number equal (within the euro
// half-slack) to |amount| appears in the quote. Numeric, not textual: each token is parsed with
// parseItalianNumber so "250.000", "250.000,00" and "€250.000" all match 250000.
func maQuoteContainsAmount(quote string, amount float64) bool {
	target := math.Abs(amount)
	for _, tok := range maNIAmountTokenRe.FindAllString(quote, -1) {
		if v, ok := parseItalianNumber(tok); ok {
			if math.Abs(math.Abs(v)-target) <= maFilingCheckTolerance {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Fingerprint (stable dedup + cross-run auto-reconfirm key).
// ---------------------------------------------------------------------------

// maNIProposalFingerprint is the deterministic dedup/reconfirm key of a proposal:
// sha256(trattamento | exercise_date ISO | importo_lordo | importo_rettifica | sha256(quoteNorm)),
// truncated to 32 hex chars. Stable across whitespace/case differences in the quote (the quote is
// normalized before hashing) and sensitive to the amount (a different figure ⇒ a different key).
func maNIProposalFingerprint(trattamento string, exerciseDate *time.Time, importoLordo, importoRettifica *float64, quote string) string {
	exISO := ""
	if exerciseDate != nil {
		exISO = exerciseDate.Format("2006-01-02")
	}
	quoteHash := sha256.Sum256([]byte(maNormalizeQuote(quote)))
	parts := strings.Join([]string{
		trattamento,
		exISO,
		maNormalizeAmountKey(importoLordo),
		maNormalizeAmountKey(importoRettifica),
		hex.EncodeToString(quoteHash[:]),
	}, "|")
	sum := sha256.Sum256([]byte(parts))
	return hex.EncodeToString(sum[:])[:32]
}

// maNormalizeAmountKey renders a nullable amount as a stable fixed-precision key ("null" for nil,
// two decimals otherwise) so 250000 and 250000.004 collapse but 250000 and 250001 do not.
func maNormalizeAmountKey(v *float64) string {
	if v == nil {
		return "null"
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

// maNormalizeQuote lowercases and collapses all whitespace to single spaces — the canonical form
// used for both quote grounding and the fingerprint's quote hash.
func maNormalizeQuote(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// ---------------------------------------------------------------------------
// Chunking + parsing.
// ---------------------------------------------------------------------------

// chunkMANISections groups NI sections into per-call chunks: consecutive sections are batched
// until adding the next would exceed budget (a lone section larger than budget is its own chunk —
// never split, so a citation is never severed).
func chunkMANISections(sections []maNISection, budget int) [][]maNISection {
	var chunks [][]maNISection
	var cur []maNISection
	curLen := 0
	for _, sec := range sections {
		secLen := len(sec.Markdown)
		if len(cur) > 0 && curLen+secLen > budget {
			chunks = append(chunks, cur)
			cur = nil
			curLen = 0
		}
		cur = append(cur, sec)
		curLen += secLen
	}
	if len(cur) > 0 {
		chunks = append(chunks, cur)
	}
	return chunks
}

// maNISectionPagesLabel renders a section's page range for the LLM input ("10" or "10-11").
func maNISectionPagesLabel(sec maNISection) string {
	if sec.StartPage == sec.EndPage {
		return strconv.Itoa(sec.StartPage)
	}
	return fmt.Sprintf("%d-%d", sec.StartPage, sec.EndPage)
}

// maParseISODate parses a YYYY-MM-DD date (UTC), ok=false when malformed.
func maParseISODate(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// parseMANIProposals defensively decodes the prompt's {"proposals":[...]} output (tolerating
// ```json fences). An empty body yields no proposals (not an error — a truncated/empty
// generation surfaces zero facts rather than failing the whole filing); malformed JSON is an
// error so the section call retries.
func parseMANIProposals(content string) ([]maNIProposalLLM, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil, nil
	}
	var decoded struct {
		Proposals []maNIProposalLLM `json:"proposals"`
	}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, fmt.Errorf("decode ni reading proposals: %w", err)
	}
	return decoded.Proposals, nil
}

// ---------------------------------------------------------------------------
// Decision reducer (pure — the contract F7 consumes).
// ---------------------------------------------------------------------------

// reduceMANIEffectiveAdjustments collapses a run's proposals + decisions into the effective
// adjustments. Per proposal the LATEST decision (by created_at, id tie-break) decides:
//   - not ratify (reject/revoke) or no decision ⇒ NOT effective;
//   - effective treatment = ratified_treatment when set, else the candidate; a treatment that is
//     not ebitda|pfn (dd_only, un-promoted) is NEVER effective;
//   - effective amount = ratified_amount when set (signed); else, only when direction is CERTAIN,
//     the proposal's importo_rettifica; an uncertain verso with no ratified amount, or a certain
//     verso with no amount at all, is NOT effective (a verso without a signed amount has no effect).
//
// It does NOT sum — F7 groups/filters by exercise alignment. The caller passes ONLY the active
// run's proposals+decisions, so historical runs never double-count.
func reduceMANIEffectiveAdjustments(proposals []maNIProposal, decisions []maNIDecision) []maNIEffectiveAdjustment {
	latest := make(map[string]maNIDecision, len(decisions))
	for _, d := range decisions {
		cur, ok := latest[d.ProposalID]
		if !ok || d.CreatedAt.After(cur.CreatedAt) || (d.CreatedAt.Equal(cur.CreatedAt) && d.ID > cur.ID) {
			latest[d.ProposalID] = d
		}
	}
	var out []maNIEffectiveAdjustment
	for _, p := range proposals {
		d, ok := latest[p.ID]
		if !ok || d.Action != maNIActionRatify {
			continue
		}
		treatment := p.TrattamentoCandidato
		if d.RatifiedTreatment != "" {
			treatment = d.RatifiedTreatment
		}
		if treatment != maNITrattamentoEbitda && treatment != maNITrattamentoPFN {
			continue
		}
		var amount float64
		switch {
		case d.RatifiedAmount != nil:
			amount = *d.RatifiedAmount
		case p.Direction != maNIDirectionUncertain && p.ImportoRettifica != nil:
			amount = *p.ImportoRettifica
		default:
			continue
		}
		out = append(out, maNIEffectiveAdjustment{
			ProposalID:   p.ID,
			Treatment:    treatment,
			ExerciseDate: p.ExerciseDate,
			Amount:       amount,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Cross-run auto-reconfirm matcher (pure — the store loads, this decides).
// ---------------------------------------------------------------------------

// matchMANIAutoReconfirm decides which new-run proposals inherit a prior standing decision. For
// each fingerprint it takes the MOST RECENT prior proposal (any decision), and carries its latest
// decision ONLY when that decision is ratify or reject — a revoke (or no decision) leaves the fact
// un-decided, so nothing is carried. This mirrors the analyst's current standing state.
func matchMANIAutoReconfirm(newProps []maNINewProposal, prior []maNIPriorProposal) []maNIAutoReconfirm {
	best := make(map[string]maNIPriorProposal, len(prior))
	for _, p := range prior {
		cur, ok := best[p.Fingerprint]
		if !ok || p.CreatedAt.After(cur.CreatedAt) || (p.CreatedAt.Equal(cur.CreatedAt) && p.ProposalID > cur.ProposalID) {
			best[p.Fingerprint] = p
		}
	}
	var out []maNIAutoReconfirm
	for _, np := range newProps {
		b, ok := best[np.Fingerprint]
		if !ok || b.Decision == nil {
			continue
		}
		if b.Decision.Action != maNIActionRatify && b.Decision.Action != maNIActionReject {
			continue // latest standing decision is revoke (or other): do not carry
		}
		out = append(out, maNIAutoReconfirm{
			NewProposalID:        np.ProposalID,
			HistoricalDecisionID: b.Decision.ID,
			Action:               b.Decision.Action,
			RatifiedAmount:       b.Decision.RatifiedAmount,
			RatifiedTreatment:    b.Decision.RatifiedTreatment,
			Reason:               b.Decision.Reason,
			ActorSubject:         b.Decision.ActorSubject,
			ActorEmail:           b.Decision.ActorEmail,
		})
	}
	return out
}
