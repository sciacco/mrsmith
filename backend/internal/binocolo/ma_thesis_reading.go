package binocolo

// Lettura di tesi context-scoped (Fase 5, DEEP-DIVE-IMPLEMENTATION-PLAN.md): il
// dossier deep resta thesis-neutral e globale; questo è il secondo strato — il
// memo per (iniziativa, azienda) che applica la TESI DELLA SESSIONE DI
// PROVENIENZA al dossier. Generazione on-demand (pattern B6), nessun automatismo
// sulla state machine del board; rigenerazione esplicita quando la tesi cambia
// (staleness dal confronto con lo snapshot).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/llm"
)

// resolveCardThesis risolve la sessione di provenienza (rating più recente da
// MACardProvenance, fallback CreatedFromSession) e ne compone la tesi. Le
// iniziative NON hanno tesi propria (decisione E: la ricerca è centrica).
func (s *maService) resolveCardThesis(ctx context.Context, initiativeID string, card MAInitiativeCard) (string, string, error) {
	sessionID := card.CreatedFromSession
	if provenances, err := s.store.ListMACardProvenances(ctx, initiativeID, []string{card.CompanyKey}); err == nil {
		var latest time.Time
		for _, provenance := range provenances[card.CompanyKey] {
			if provenance.SessionID != "" && provenance.RatedAt.After(latest) {
				latest = provenance.RatedAt
				sessionID = provenance.SessionID
			}
		}
	}
	if sessionID == "" {
		return "", "", fmt.Errorf("%w: card senza sessione di provenienza, lettura di tesi non disponibile", errMAStrategyInvalid)
	}
	session, err := s.store.GetMASessionState(ctx, sessionID)
	if err != nil {
		return "", "", err
	}
	version, err := s.store.GetMAStrategyVersion(ctx, sessionID, session.ActiveStrategyID)
	if err != nil {
		return "", "", err
	}
	return sessionID, composeMAThesisText(version.Strategy), nil
}

// composeMAThesisText: la tesi esplicita quando c'è; altrimenti il frame della
// ricerca (settore + razionale), dichiarato come tale.
func composeMAThesisText(strategy MAStrategySpec) string {
	if thesis := strings.TrimSpace(strategy.Thesis); thesis != "" {
		return thesis
	}
	parts := make([]string, 0, 2)
	if sector := strings.TrimSpace(strategy.SectorDescription); sector != "" {
		parts = append(parts, "Perimetro della ricerca: "+sector+".")
	}
	if rationale := strings.TrimSpace(strategy.Rationale); rationale != "" {
		parts = append(parts, "Razionale: "+rationale)
	}
	return strings.Join(parts, " ")
}

// getCardThesisReading ritorna la lettura persistita (nil se mai generata) con
// la staleness calcolata rispetto alla tesi corrente della provenienza.
func (s *maService) getCardThesisReading(ctx context.Context, initiativeID, companyKey string) (*MACardThesisReading, error) {
	if s.store == nil {
		return nil, errMAStoreUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.store.GetMAInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, errMACardNotFound
	}
	record, err := s.store.GetMACardThesisReading(ctx, initiativeID, companyKey)
	if err != nil || record == nil {
		return record, err
	}
	if _, currentThesis, err := s.resolveCardThesis(ctx, initiativeID, *card); err == nil {
		record.StaleThesis = strings.TrimSpace(currentThesis) != strings.TrimSpace(record.ThesisSnapshot)
	}
	return record, nil
}

// generateCardThesisReading genera (o rigenera) la lettura di tesi: azione
// esplicita dell'analista, costo LLM in centesimi, nessun cancello di spesa.
// Richiede il dossier deep pronto: senza fatti il fit sarebbe vuoto.
func (s *maService) generateCardThesisReading(ctx context.Context, initiativeID, companyKey, subject, email string) (*MACardThesisReading, error) {
	if s.llmp == nil {
		return nil, errMAOpenRouterUnavailable
	}
	companyKey = normalizeMACompanyKey(companyKey)
	card, err := s.requireOperationalInitiativeCard(ctx, initiativeID, companyKey)
	if err != nil {
		return nil, err
	}
	sessionID, thesis, err := s.resolveCardThesis(ctx, initiativeID, card)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(thesis) == "" {
		return nil, fmt.Errorf("%w: la sessione di provenienza non ha una tesi", errMAStrategyInvalid)
	}
	deep, err := s.store.ListMADeepAnalysis(ctx, []string{card.CompanyKey})
	if err != nil {
		return nil, err
	}
	record, ok := deep[card.CompanyKey]
	if !ok || record.Status != maDeepStatusReady || record.Scorecard == nil {
		return nil, fmt.Errorf("%w: analisi approfondita non pronta — la lettura di tesi si appoggia al dossier", errMAStrategyInvalid)
	}

	input := map[string]any{
		"thesis":    thesis,
		"scorecard": record.Scorecard,
		"valuation": record.Valuation,
		"brief":     record.Brief,
	}
	var webEvidenceDate *time.Time
	// Evidenza web (best-effort, già per company_key): curata — solo il verdetto
	// semantico e la sua DATA, mai i payload grezzi.
	if validation, err := s.store.GetMAWebValidationForCompany(ctx, sessionID, card.CompanyKey); err == nil && validation != nil {
		ts := validation.UpdatedAt
		webEvidenceDate = &ts
		input["webEvidence"] = map[string]any{
			"date":            validation.UpdatedAt.Format("2006-01-02"),
			"selectedDomain":  validation.SelectedDomain,
			"identityState":   validation.IdentityState,
			"validationState": validation.WebValidationState,
			"finalAction":     validation.FinalAction,
			"analystVerdict":  validation.AnalystVerdict,
			"summary":         validation.Summary,
		}
	}

	model, err := s.llmp.ResolveModel(ctx, maModelScopeThesisReading, "")
	if err != nil {
		return nil, llmConfigError(err)
	}
	prompt, err := s.llmp.ResolvePrompt(ctx, maModelScopeThesisReading, "")
	if err != nil {
		return nil, llmConfigError(err)
	}
	client, err := s.llmp.ClientForModel(ctx, model)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	// 5000 come per il brief: sui reasoning model il budget copre anche i token
	// di ragionamento e un tetto stretto tronca il JSON.
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
	resp, chatErr := client.Chat(ctx, chatReq)
	usageRaw, _ := json.Marshal(resp.Usage)
	requestBody, _ := llm.BuildRequestBody(chatReq)
	requestRaw, _ := json.Marshal(requestBody)
	audit := llm.CallAudit{
		App:        maApp,
		Scope:      maModelScopeThesisReading,
		ProviderID: model.ProviderID,
		ModelID:    model.ID,
		PromptID:   prompt.ID,
		Model:      model.Model,
		Request:    requestRaw,
		Usage:      usageRaw,
	}
	if chatErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = chatErr.Error()
	} else if respRaw, mErr := json.Marshal(map[string]any{"content": resp.Content}); mErr == nil {
		audit.Response = respRaw
	}
	_ = s.llmp.RecordAudit(ctx, audit)
	if chatErr != nil {
		return nil, chatErr
	}
	reading, err := parseMAThesisReading(resp.Content)
	if err != nil {
		return nil, err
	}

	out := &MACardThesisReading{
		InitiativeID:     initiativeID,
		CompanyKey:       card.CompanyKey,
		SessionID:        sessionID,
		ThesisSnapshot:   thesis,
		Reading:          reading,
		WebEvidenceDate:  webEvidenceDate,
		GeneratedByEmail: email,
	}
	if err := s.store.UpsertMACardThesisReading(ctx, out, model.ID, prompt.ID, subject); err != nil {
		return nil, err
	}
	_ = s.traceEvent(ctx, maTraceEventWrite{
		EventType: "ma_thesis_reading_generated",
		Status:    maTraceEventSucceeded,
		Metadata: maTraceJSON(map[string]any{
			"initiative_id": initiativeID, "company_key": card.CompanyKey,
			"session_id": sessionID, "by": email,
		}),
	})
	return s.getCardThesisReading(ctx, initiativeID, card.CompanyKey)
}

func parseMAThesisReading(content string) (*MAThesisReading, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil, errors.New("empty thesis reading response")
	}
	var reading MAThesisReading
	if err := json.Unmarshal([]byte(trimmed), &reading); err != nil {
		return nil, fmt.Errorf("decode thesis reading: %w", err)
	}
	reading.FitLevel = strings.ToLower(strings.TrimSpace(reading.FitLevel))
	switch reading.FitLevel {
	case "alto", "medio", "basso", "non_valutabile":
	default:
		reading.FitLevel = ""
	}
	reading.Fit = cleanText(reading.Fit, 800)
	reading.ValuationStance = cleanText(reading.ValuationStance, 500)
	capList := func(items []string, max int, each int) []string {
		if len(items) > max {
			items = items[:max]
		}
		for i := range items {
			items[i] = cleanText(items[i], each)
		}
		return items
	}
	reading.BlockingFlags = capList(reading.BlockingFlags, 5, 300)
	reading.TolerableFlags = capList(reading.TolerableFlags, 5, 300)
	reading.ThesisDDQuestions = capList(reading.ThesisDDQuestions, 6, 300)
	reading.SynergyHypotheses = capList(reading.SynergyHypotheses, 4, 300)
	reading.NotAddressed = capList(reading.NotAddressed, 4, 200)
	return &reading, nil
}
