package binocolo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// Deposited-filing ingest job (issue #78, Fase 5). filing_ingest is the durable OCR →
// identity → parse → NI pipeline that runs off the request path on the shared ma_job
// queue, enqueued by the acquire (F4) once a PDF is downloaded. It RESUMES from the
// filing.status: queued→ocr→parse→ni_reading→ready|degraded (F6 closes the last two),
// with failed / identity_blocked as off-ramps.
//
// Contract (orchestrator):
//   - queued        : OCR pending; the OCR call is NEVER blocked by the identity gate.
//   - ocr           : OCR done, pages persisted; identity gate acts from here on.
//   - parse         : parse + checks (+ optional DocAI) done.
//   - ni_reading    : NI reading in progress/queued (F6 completes it).
//   - ready|degraded: closed by F6. failed: terminal. identity_blocked: awaits override.
//
// identity_status (a separate column, NOT a status) drives the gate: validated/override
// let parse run; mismatch = hard block (never overridable); unreadable = soft block
// (explicit override possible, only from pending_validation — enforced in the store).

const (
	// maFilingOCRScope is the (app, scope) the filing OCR/DocAI resolve against in the
	// registry (mrsmith.ocr_model row seeded by mig 116).
	maFilingOCRScope = "ma_filing_ocr"
)

// maFilingDocAISchema is the minimal document-annotation schema for the DocAI control
// branch: per-exercise SP/CE totals + the declared A−B. Kept small on purpose — DocAI is
// a cross-check on the deterministic parse, not a replacement.
const maFilingDocAISchema = `{
  "type": "object",
  "properties": {
    "exercises": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "exerciseDate": {"type": "string", "description": "closing date YYYY-MM-DD"},
          "totaleAttivo": {"type": "number"},
          "totalePassivo": {"type": "number"},
          "valoreProduzione": {"type": "number"},
          "costiProduzione": {"type": "number"},
          "differenzaAB": {"type": "number"}
        }
      }
    }
  }
}`

// runFilingIngestJob owns the operation trace for one tick of a filing_ingest job (one
// trace per run, completed here). The trace id becomes the OCR call's RequestID so the
// trace correlates to the llm_call_audit row. Returns the work error verbatim so the
// worker sees errMAFilingPollPending unchanged.
func (s *maService) runFilingIngestJob(ctx context.Context, job maJob) (string, error) {
	trace, err := s.startTrace(ctx, maTraceStart{
		Operation:        "ma_filing_ingest",
		CreatedBySubject: job.CreatedBySubject,
		CreatedByEmail:   job.CreatedByEmail,
		Request:          job.Payload,
	})
	if err != nil {
		return "", err
	}
	ctx = withMATrace(ctx, trace)
	workErr := s.filingIngestWork(ctx, job, trace.id)
	s.completeFilingTrace(ctx, workErr)
	return trace.id, workErr
}

// filingIngestTrace records one ordered ingest trace event with an accurate external
// system (the shared filingTrace hardcodes "docuengine", right for search/acquire but not
// for the ingest, whose stages hit Mistral OCR or run purely in-process). Best-effort.
func (s *maService) filingIngestTrace(ctx context.Context, eventType, externalSystem, status string, metadata map[string]any, errMsg string) {
	event := maTraceEventWrite{
		EventType:      eventType,
		ExternalSystem: externalSystem,
		Status:         status,
		Error:          errMsg,
	}
	if len(metadata) > 0 {
		event.Metadata = maTraceJSON(metadata)
	}
	_ = s.traceEvent(ctx, event)
}

// maFilingSysMistral / maFilingSysInternal / maFilingSysLLM label ingest trace events by where
// the work happens: the OCR/DocAI calls hit Mistral; the NI reading hits the chat LLM provider;
// parse/identity/dispatch run in-process.
const (
	maFilingSysMistral  = "mistral"
	maFilingSysInternal = "binocolo"
	maFilingSysLLM      = "llm"
)

func (s *maService) filingIngestWork(ctx context.Context, job maJob, requestID string) error {
	if s.filing == nil {
		return errMAStoreUnavailable
	}
	var payload maFilingIngestJobPayload
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("decode ma filing ingest payload: %w", err)
		}
	}
	filingID := strings.TrimSpace(payload.FilingID)
	if filingID == "" {
		return fmt.Errorf("%w: filing id", errMAStrategyInvalid)
	}
	filing, err := s.filing.GetMAFiling(ctx, filingID)
	if err != nil {
		return err
	}
	if filing == nil {
		return fmt.Errorf("%w: filing %s not found", errMAStrategyInvalid, filingID)
	}
	// Resume ALWAYS from the durable filing status.
	switch filing.Status {
	case maFilingStatusQueued:
		return s.filingIngestOCR(ctx, job, payload, filing, requestID)
	case maFilingStatusOCR, maFilingStatusIdentityBlocked:
		// ocr = OCR done, identity gate pending; identity_blocked = re-entered after an
		// override was applied (or still blocked). Both re-run the identity → parse stage.
		return s.filingIngestIdentityAndParse(ctx, job, payload, filing, requestID)
	case maFilingStatusParse:
		return s.filingIngestToNIReading(ctx, filing, requestID)
	case maFilingStatusNIReading:
		return s.filingIngestNIReading(ctx, filing, requestID)
	case maFilingStatusReady, maFilingStatusDegraded:
		// Terminal (ready/degraded closed by F6). A HARD CRASH of a prior tick of THIS job, mid
		// narrative (issue #80), can leave the narrative run orphaned at 'running' (the filing was
		// already closed, so this re-lease early-returns). Mark it failed (regenerable) before
		// returning — best-effort, no-op when there is no running run.
		s.failMANINarrativeRunIfRunning(ctx, filing.ID)
		return nil
	case maFilingStatusFailed:
		return nil // terminal
	default:
		return fmt.Errorf("%w: unexpected filing status %q", errMAStrategyInvalid, filing.Status)
	}
}

// filingIngestOCR runs stage 1 (queued → ocr): load the blob, OCR it (NEVER gated by
// identity), persist an immutable processing run + pages, record the page count, and move
// to 'ocr'. A retryable OCR error (5xx/429/network) is returned raw so the worker retries
// (cap maJobMaxAttempts); a definitive 4xx fails the filing. On success it continues in
// the same tick to identity + parse (no extra tick), per the contract.
func (s *maService) filingIngestOCR(ctx context.Context, job maJob, payload maFilingIngestJobPayload, filing *maFiling, requestID string) error {
	if s.llmp == nil {
		return errMAStoreUnavailable
	}
	md5Hex := strings.TrimSpace(filing.BlobMD5)
	if md5Hex == "" {
		return s.failFilingIngest(ctx, filing.ID, "blob del fascicolo assente")
	}
	pdf, _, _, err := s.filing.GetMAFilingBlob(ctx, md5Hex)
	if err != nil {
		return err
	}
	if len(pdf) == 0 {
		return s.failFilingIngest(ctx, filing.ID, "pdf del fascicolo non disponibile")
	}
	s.filingIngestTrace(ctx, "ocr", maFilingSysMistral, maTraceEventStarted, map[string]any{"filing_id": filing.ID}, "")
	resp, model, ocrErr := s.llmp.OCR(ctx, llm.OCRCall{
		App:          maApp,
		Scope:        maFilingOCRScope,
		PDF:          pdf,
		RequestID:    requestID,
		Context:      map[string]any{"filingId": filing.ID},
		ActorSubject: job.CreatedBySubject,
		ActorEmail:   job.CreatedByEmail,
	})
	if ocrErr != nil {
		s.filingIngestTrace(ctx, "ocr", maFilingSysMistral, maTraceEventFailed, nil, ocrErr.Error())
		if maOCRErrorDefinitive(ocrErr) {
			return s.failFilingIngest(ctx, filing.ID, "ocr non riuscito: "+ocrErr.Error())
		}
		return ocrErr // retryable → worker retryOrFail (attempt bump, cap 5)
	}
	pages := maNormalizeOCRPages(resp.Pages)
	if len(pages) == 0 {
		return s.failFilingIngest(ctx, filing.ID, "ocr non ha prodotto pagine")
	}
	runID, err := s.filing.CreateMAFilingProcessingRun(ctx, filing.ID, model.Model, maFilingOCRParamsVersion, maFilingParseVersion, requestID)
	if err != nil {
		return err
	}
	if err := s.filing.InsertMAFilingPages(ctx, runID, pages); err != nil {
		return err
	}
	if err := s.filing.SetMAFilingPageCount(ctx, filing.ID, len(pages)); err != nil {
		return err
	}
	if err := s.filing.UpdateMAFilingStatus(ctx, filing.ID, maFilingStatusOCR, ""); err != nil {
		return err
	}
	s.filingIngestTrace(ctx, "ocr", maFilingSysMistral, maTraceEventSucceeded, map[string]any{"pages": len(pages), "run_id": runID}, "")
	// Continue in the SAME run to identity + parse (no extra tick).
	filing.Status = maFilingStatusOCR
	filing.ActiveProcessingRunID = runID
	return s.filingIngestIdentityAndParse(ctx, job, payload, filing, requestID)
}

// filingIngestIdentityAndParse runs stage 2 (identity gate) then stage 3 (parse). The
// gate acts ONLY here (never on the OCR). A DocuEngine-channel filing has its identity
// asserted by the search's tax-code match; an upload is validated from page 1. On
// validated/override it enqueues the deep-dive baseline (best-effort) and parses in the
// same run. A hard mismatch or an unreadable identity blocks (status identity_blocked)
// and the job completes — a later override (F8) re-enqueues to resume here.
func (s *maService) filingIngestIdentityAndParse(ctx context.Context, job maJob, payload maFilingIngestJobPayload, filing *maFiling, requestID string) error {
	runID := strings.TrimSpace(filing.ActiveProcessingRunID)
	if runID == "" {
		return s.failFilingIngest(ctx, filing.ID, "processing run assente: rilanciare l'ocr")
	}
	identityStatus := filing.IdentityStatus
	if identityStatus != maFilingIdentityValidated && identityStatus != maFilingIdentityOverride {
		fromChannel, err := s.filing.HasMAFilingDocuEngineAcquisition(ctx, filing.ID)
		if err != nil {
			return err
		}
		if fromChannel {
			if err := s.filing.SetMAFilingIdentityStatus(ctx, filing.ID, maFilingIdentityValidated); err != nil {
				return err
			}
			s.filingIngestTrace(ctx, "identity", maFilingSysInternal, maTraceEventSucceeded, map[string]any{"filing_id": filing.ID, "source": "docuengine"}, "")
		} else {
			pages, err := s.filing.GetMAFilingPages(ctx, runID)
			if err != nil {
				return err
			}
			res := validateMAFilingIdentity(pages, filing.VATClean, filing.TaxClean)
			switch res.Outcome {
			case maIdentityMatch:
				if err := s.filing.SetMAFilingIdentityStatus(ctx, filing.ID, maFilingIdentityValidated); err != nil {
					return err
				}
				s.filingIngestTrace(ctx, "identity", maFilingSysInternal, maTraceEventSucceeded, map[string]any{"filing_id": filing.ID, "source": "page1"}, "")
			case maIdentityMismatch:
				if err := s.filing.SetMAFilingIdentityStatus(ctx, filing.ID, maFilingIdentityMismatch); err != nil {
					return err
				}
				if err := s.filing.UpdateMAFilingStatus(ctx, filing.ID, maFilingStatusIdentityBlocked, "identità fiscale a pagina 1 non corrisponde a quella attesa"); err != nil {
					return err
				}
				s.filingIngestTrace(ctx, "identity", maFilingSysInternal, maTraceEventFailed, map[string]any{"filing_id": filing.ID, "found": res.FoundCodes}, "mismatch")
				return nil // hard block, NOT overridable
			default: // unreadable
				if err := s.filing.UpdateMAFilingStatus(ctx, filing.ID, maFilingStatusIdentityBlocked, "identità fiscale non leggibile a pagina 1"); err != nil {
					return err
				}
				s.filingIngestTrace(ctx, "identity", maFilingSysInternal, maTraceEventInfo, map[string]any{"filing_id": filing.ID}, "unreadable")
				return nil // soft block: an explicit override (from pending_validation) can resume
			}
		}
	}
	// Identity validated/override: dispatch the deep-dive baseline (best-effort, never a
	// spend on failed — EnqueueMADeepAnalysisIfAbsent guarantees it) and parse.
	s.enqueueFilingDeepBaseline(ctx, payload, filing, job)
	return s.filingIngestParse(ctx, filing, runID, requestID)
}

// filingIngestParse runs stage 3: parse the prospetti, compute arithmetic + cross checks
// (+ adapted-comparative against peer filings), persist one extract per exercise, record
// the discovered closing date / balance-sheet type / taxonomy, run the DocAI control
// branch when enabled (best-effort), and move to 'parse'. A parse failure fails the
// filing with a legible reason.
func (s *maService) filingIngestParse(ctx context.Context, filing *maFiling, runID, requestID string) error {
	pages, err := s.filing.GetMAFilingPages(ctx, runID)
	if err != nil {
		return err
	}
	s.filingIngestTrace(ctx, "parse", maFilingSysInternal, maTraceEventStarted, map[string]any{"filing_id": filing.ID, "pages": len(pages)}, "")
	result, perr := parseMAFilingProspetti(pages)
	if perr != nil {
		s.filingIngestTrace(ctx, "parse", maFilingSysInternal, maTraceEventFailed, nil, perr.Error())
		return s.failFilingIngest(ctx, filing.ID, perr.Error())
	}
	adapted := s.detectFilingAdaptedComparative(ctx, filing, result)
	for i := range result.Exercises {
		ex := result.Exercises[i]
		checks := ex.Checks
		if ac, ok := adapted[maDateValue(ex.ExerciseDate)]; ok && ac.Diverged {
			checks = append(checks, maFilingCheck{Name: "adapted_comparative", Passed: false, Expected: ac.Expected, Actual: ac.Actual})
		}
		if err := s.filing.UpsertMAFilingExtract(ctx, runID, ex.ExerciseDate,
			maMarshalJSON(ex.SP), maMarshalJSON(ex.CE), maMarshalJSON(checks), nil, nil); err != nil {
			return err
		}
	}
	if err := s.filing.SetMAFilingParsed(ctx, filing.ID, result.ClosingDate(), result.BalanceSheetType, result.TaxonomyVersion, len(pages)); err != nil {
		return err
	}
	s.filingIngestTrace(ctx, "parse", maFilingSysInternal, maTraceEventSucceeded, map[string]any{"exercises": len(result.Exercises), "balance_sheet_type": result.BalanceSheetType}, "")

	if s.filingDocAICompare {
		s.filingIngestDocAI(ctx, filing, runID, requestID, result)
	}
	if err := s.filing.UpdateMAFilingStatus(ctx, filing.ID, maFilingStatusParse, ""); err != nil {
		return err
	}
	filing.Status = maFilingStatusParse
	return s.filingIngestToNIReading(ctx, filing, requestID)
}

// filingIngestToNIReading moves parse → ni_reading and delegates to the real NI reading stage
// in the same tick (no extra poll), so a parsed filing closes to ready|degraded without waiting.
func (s *maService) filingIngestToNIReading(ctx context.Context, filing *maFiling, requestID string) error {
	if err := s.filing.UpdateMAFilingStatus(ctx, filing.ID, maFilingStatusNIReading, ""); err != nil {
		return err
	}
	filing.Status = maFilingStatusNIReading
	return s.filingIngestNIReading(ctx, filing, requestID)
}

// enqueueFilingDeepBaseline dispatches the IT-full deep-dive baseline for the filing's
// fiscal identity, if-absent (never re-charges a failed row). Best-effort: a failure logs
// and traces but never blocks the ingest.
func (s *maService) enqueueFilingDeepBaseline(ctx context.Context, payload maFilingIngestJobPayload, filing *maFiling, job maJob) {
	if s.store == nil {
		return
	}
	created, status, err := s.store.EnqueueMADeepAnalysisIfAbsent(ctx, payload.ContextCompanyKey, filing.VATClean, filing.TaxClean, job.CreatedByEmail)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo filing deep baseline enqueue failed",
			"component", "binocolo", "operation", "ma_filing_ingest", "filing_id", filing.ID, "error", err)
		s.filingIngestTrace(ctx, "deep_dispatch", maFilingSysInternal, maTraceEventFailed, map[string]any{"filing_id": filing.ID}, err.Error())
		return
	}
	s.filingIngestTrace(ctx, "deep_dispatch", maFilingSysInternal, maTraceEventSucceeded, map[string]any{"filing_id": filing.ID, "created": created, "existing_status": status}, "")
}

// detectFilingAdaptedComparative fetches peer extracts (other filings, same fiscal key,
// overlapping exercises) and flags adapted comparatives. Best-effort: a fetch failure
// yields no flags (never blocks parse). Dormant until multiple filings exist for one
// identity, but wired end-to-end.
func (s *maService) detectFilingAdaptedComparative(ctx context.Context, filing *maFiling, result maFilingParseResult) map[string]maFilingAdaptedComparative {
	out := map[string]maFilingAdaptedComparative{}
	if len(result.Exercises) == 0 {
		return out
	}
	dates := make([]time.Time, 0, len(result.Exercises))
	for _, ex := range result.Exercises {
		dates = append(dates, ex.ExerciseDate)
	}
	peers, err := s.filing.GetMAFilingExtractsByFiscalKey(ctx, filing.FiscalKey, filing.ID, dates)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo filing adapted-comparative peers fetch failed",
			"component", "binocolo", "operation", "ma_filing_ingest", "filing_id", filing.ID, "error", err)
		return out
	}
	if len(peers) == 0 {
		return out
	}
	currentClosing := filing.ClosingDate
	if currentClosing == nil {
		currentClosing = result.ClosingDate()
	}
	for _, ac := range detectAdaptedComparative(result.Exercises, currentClosing, peers, maFilingAdaptedTolerance) {
		out[maDateValue(ac.ExerciseDate)] = ac
	}
	return out
}

// filingIngestDocAI is the DocAI control branch (behind Deps.FilingDocAICompare). It
// annotates the same PDF, maps the per-exercise totals onto the parse shape, and writes a
// field-by-field docai_diff (with the arithmetic-check arbitration). CIRCOSCRITTO and
// best-effort: any error only logs + records a docai_diff error marker — never blocks the
// pipeline, and the markdown parse stays the record.
func (s *maService) filingIngestDocAI(ctx context.Context, filing *maFiling, runID, requestID string, result maFilingParseResult) {
	log := logging.FromContext(ctx)
	pdf, _, _, err := s.filing.GetMAFilingBlob(ctx, strings.TrimSpace(filing.BlobMD5))
	if err != nil || len(pdf) == 0 {
		log.Warn("binocolo filing docai skipped: blob unavailable", "component", "binocolo", "operation", "ma_filing_ingest", "filing_id", filing.ID)
		return
	}
	s.filingIngestTrace(ctx, "docai", maFilingSysMistral, maTraceEventStarted, map[string]any{"filing_id": filing.ID}, "")
	resp, _, derr := s.llmp.DocAI(ctx, llm.DocAICall{
		App:          maApp,
		Scope:        maFilingOCRScope,
		PDF:          pdf,
		Schema:       json.RawMessage(maFilingDocAISchema),
		RequestID:    requestID,
		Context:      map[string]any{"filingId": filing.ID, "branch": "docai"},
		ActorSubject: filing.CreatedBySubject,
		ActorEmail:   filing.CreatedByEmail,
	})
	if derr != nil {
		s.filingIngestTrace(ctx, "docai", maFilingSysMistral, maTraceEventFailed, nil, derr.Error())
		log.Warn("binocolo filing docai failed", "component", "binocolo", "operation", "ma_filing_ingest", "filing_id", filing.ID, "error", derr)
		s.writeDocAIError(ctx, runID, result, derr.Error())
		return
	}
	byDate := maParseDocAIExercises(resp.Annotation)
	for i := range result.Exercises {
		ex := result.Exercises[i]
		annotated, ok := byDate[maDateValue(ex.ExerciseDate)]
		if !ok {
			continue
		}
		diff := maDocAIDiff(ex, annotated)
		if err := s.filing.UpsertMAFilingExtract(ctx, runID, ex.ExerciseDate, nil, nil, nil, maMarshalJSON(annotated), maMarshalJSON(diff)); err != nil {
			log.Warn("binocolo filing docai upsert failed", "component", "binocolo", "operation", "ma_filing_ingest", "filing_id", filing.ID, "error", err)
			return
		}
	}
	s.filingIngestTrace(ctx, "docai", maFilingSysMistral, maTraceEventSucceeded, map[string]any{"annotated_exercises": len(byDate)}, "")
}

// writeDocAIError records a docai_diff error marker on each exercise so the failure is
// visible without blocking the parse-of-record.
func (s *maService) writeDocAIError(ctx context.Context, runID string, result maFilingParseResult, msg string) {
	marker := maMarshalJSON(map[string]any{"error": msg})
	for _, ex := range result.Exercises {
		_ = s.filing.UpsertMAFilingExtract(ctx, runID, ex.ExerciseDate, nil, nil, nil, nil, marker)
	}
}

// maFilingDocAIExercise is one exercise of the DocAI annotation (mirrors the schema).
type maFilingDocAIExercise struct {
	ExerciseDate     string   `json:"exerciseDate"`
	TotaleAttivo     *float64 `json:"totaleAttivo"`
	TotalePassivo    *float64 `json:"totalePassivo"`
	ValoreProduzione *float64 `json:"valoreProduzione"`
	CostiProduzione  *float64 `json:"costiProduzione"`
	DifferenzaAB     *float64 `json:"differenzaAB"`
}

// maParseDocAIExercises defensively parses the DocAI annotation into a by-date index.
func maParseDocAIExercises(annotation json.RawMessage) map[string]maFilingDocAIExercise {
	out := map[string]maFilingDocAIExercise{}
	if len(annotation) == 0 {
		return out
	}
	var decoded struct {
		Exercises []maFilingDocAIExercise `json:"exercises"`
	}
	if err := json.Unmarshal(annotation, &decoded); err != nil {
		return out
	}
	for _, ex := range decoded.Exercises {
		if d, ok := maParseDMY(ex.ExerciseDate); ok {
			out[maDateValue(d)] = ex
			continue
		}
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(ex.ExerciseDate)); err == nil {
			out[maDateValue(t)] = ex
		}
	}
	return out
}

// maDocAIFieldDiff is one field's parse-vs-docai comparison.
type maDocAIFieldDiff struct {
	Field          string   `json:"field"`
	Parse          *float64 `json:"parse,omitempty"`
	DocAI          *float64 `json:"docai,omitempty"`
	Delta          *float64 `json:"delta,omitempty"`
	ParseSatisfies bool     `json:"parseSatisfies"` // the parse figure satisfies the arithmetic check
}

// maDocAIDiff compares the deterministic parse of one exercise against the DocAI
// annotation, field by field. The arbitration note (parseSatisfies) records that the
// markdown parse — the record — is the arithmetically-consistent source; DocAI is only a
// signal to flag divergence for review.
func maDocAIDiff(ex maFilingExerciseExtract, ai maFilingDocAIExercise) []maDocAIFieldDiff {
	abProven := ex.CE.ValoreProduzione != nil && ex.CE.CostiProduzione != nil && ex.CE.DifferenzaAB != nil &&
		floatAbs((*ex.CE.ValoreProduzione-*ex.CE.CostiProduzione)-*ex.CE.DifferenzaAB) <= maFilingCheckTolerance
	spProven := ex.SP.TotaleAttivo != nil && ex.SP.TotalePassivo != nil &&
		floatAbs(*ex.SP.TotaleAttivo-*ex.SP.TotalePassivo) <= maFilingCheckTolerance
	fields := []struct {
		name   string
		parse  *float64
		docai  *float64
		proven bool
	}{
		{"totaleAttivo", ex.SP.TotaleAttivo, ai.TotaleAttivo, spProven},
		{"totalePassivo", ex.SP.TotalePassivo, ai.TotalePassivo, spProven},
		{"valoreProduzione", ex.CE.ValoreProduzione, ai.ValoreProduzione, abProven},
		{"costiProduzione", ex.CE.CostiProduzione, ai.CostiProduzione, abProven},
		{"differenzaAB", ex.CE.DifferenzaAB, ai.DifferenzaAB, abProven},
	}
	var out []maDocAIFieldDiff
	for _, f := range fields {
		if f.parse == nil && f.docai == nil {
			continue
		}
		d := maDocAIFieldDiff{Field: f.name, Parse: f.parse, DocAI: f.docai, ParseSatisfies: f.proven}
		if f.parse != nil && f.docai != nil {
			delta := *f.parse - *f.docai
			d.Delta = &delta
		}
		out = append(out, d)
	}
	return out
}

func floatAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// maNormalizeOCRPages converts OCR pages to filing pages with 1-based page numbers.
// Mistral OCR emits a 0-based `index`; normalizing by (index - minIndex + 1) yields
// 1-based page numbers whether the vendor is 0- or 1-based, preserving order and gaps.
func maNormalizeOCRPages(ocrPages []llm.OCRPage) []maFilingPage {
	if len(ocrPages) == 0 {
		return nil
	}
	minIdx := ocrPages[0].Index
	for _, p := range ocrPages {
		if p.Index < minIdx {
			minIdx = p.Index
		}
	}
	out := make([]maFilingPage, 0, len(ocrPages))
	for _, p := range ocrPages {
		out = append(out, maFilingPage{
			PageNo:   p.Index - minIdx + 1,
			Markdown: p.Markdown,
			Extras:   p.Extras,
		})
	}
	return out
}

// maOCRErrorDefinitive classifies an OCR error: a 4xx (other than 429) means the request
// cannot succeed as-is ⇒ definitive (fail the filing). 429/5xx/network are transient ⇒
// retryable (the worker's attempt budget covers them).
func maOCRErrorDefinitive(err error) bool {
	var apiErr *llm.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests {
			return false
		}
		return apiErr.StatusCode >= 400 && apiErr.StatusCode < 500
	}
	return false
}

// failFilingIngest marks the filing failed with a legible reason and completes the job
// (returns nil): a definitive ingest failure is terminal, not an infra retry.
func (s *maService) failFilingIngest(ctx context.Context, filingID, reason string) error {
	if err := s.filing.UpdateMAFilingStatus(ctx, filingID, maFilingStatusFailed, reason); err != nil {
		return err
	}
	s.filingIngestTrace(ctx, "ingest_failed", maFilingSysInternal, maTraceEventFailed, map[string]any{"filing_id": filingID}, reason)
	return nil
}

// failFilingIngestIfNotBlocked is the worker's terminal cleanup for an infrastructure
// give-up (attempts exhausted): fail the filing with the code UNLESS it is already
// identity_blocked (awaits a manual override) or failed. `unknown` does not exist for the
// ingest — there is no paid vendor state machine to reconcile here.
func (s *maService) failFilingIngestIfNotBlocked(ctx context.Context, job maJob, code string) {
	if s.filing == nil {
		return
	}
	var payload maFilingIngestJobPayload
	if len(job.Payload) > 0 {
		_ = json.Unmarshal(job.Payload, &payload)
	}
	filingID := strings.TrimSpace(payload.FilingID)
	if filingID == "" {
		return
	}
	filing, err := s.filing.GetMAFiling(ctx, filingID)
	if err != nil || filing == nil {
		return
	}
	if filing.Status == maFilingStatusIdentityBlocked || filing.Status == maFilingStatusFailed {
		return
	}
	_ = s.filing.UpdateMAFilingStatus(ctx, filingID, maFilingStatusFailed, code)
}

// sweepFilingIngestOrphans re-enqueues filings stuck in a non-terminal ingest state with
// no job working them (QA-F4 gap: a post-acquire ingest enqueue that failed). Idempotent
// via the mig-115 inflight index; runs periodically from the worker tick.
func (s *maService) sweepFilingIngestOrphans(ctx context.Context) {
	if s.filing == nil {
		return
	}
	orphans, err := s.filing.SweepMAFilingIngestOrphans(ctx, maFilingIngestOrphanAge)
	if err != nil {
		logging.FromContext(ctx).Warn("binocolo filing ingest orphan sweep failed",
			"component", "binocolo", "operation", "ma_filing_ingest", "error", err)
		return
	}
	for _, o := range orphans {
		if err := s.enqueueFilingIngest(ctx, o.FilingID, o.FiscalKey, o.ContextCompanyKey, o.ActorSubject, o.ActorEmail); err != nil {
			logging.FromContext(ctx).Warn("binocolo filing ingest orphan re-enqueue failed",
				"component", "binocolo", "operation", "ma_filing_ingest", "filing_id", o.FilingID, "error", err)
			continue
		}
		logging.FromContext(ctx).Info("binocolo filing ingest orphan re-enqueued",
			"component", "binocolo", "operation", "ma_filing_ingest", "filing_id", o.FilingID)
	}
}
