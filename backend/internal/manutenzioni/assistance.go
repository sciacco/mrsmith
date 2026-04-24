package manutenzioni

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openrouter"
)

var errAssistanceDecode = errors.New("decode assistance response")

type assistanceFailure struct {
	err            error
	scope          string
	requestedModel string
}

func (e *assistanceFailure) Error() string {
	return e.err.Error()
}

func (e *assistanceFailure) Unwrap() error {
	return e.err
}

func wrapAssistanceFailure(err error, scope, requestedModel string) error {
	if err == nil {
		return nil
	}
	return &assistanceFailure{
		err:            err,
		scope:          scope,
		requestedModel: requestedModel,
	}
}

func logAssistanceFailure(ctx context.Context, message string, err error, attrs ...any) {
	args := []any{
		"component", "manutenzioni",
		"request_id", logging.RequestID(ctx),
	}
	args = append(args, attrs...)
	var failure *assistanceFailure
	if errors.As(err, &failure) {
		if failure.scope != "" {
			args = append(args, "model_scope", failure.scope)
		}
		if failure.requestedModel != "" {
			args = append(args, "requested_model", failure.requestedModel)
		}
	}
	args = append(args, "error", err)
	logging.FromContext(ctx).Error(message, args...)
}

type assistanceAIOutput struct {
	Texts           assistanceTextProposal       `json:"texts"`
	ServiceTaxonomy []assistanceAIClassification `json:"service_taxonomy"`
	ReasonClasses   []assistanceAIClassification `json:"reason_classes"`
	ImpactEffects   []assistanceAIClassification `json:"impact_effects"`
	QualityFlags    []assistanceAIClassification `json:"quality_flags"`
	Summary         string                       `json:"summary"`
}

type assistanceAIClassification struct {
	ReferenceID int64    `json:"reference_id"`
	Confidence  *float64 `json:"confidence"`
	Rationale   *string  `json:"rationale"`
}

type assistanceReferenceBundle struct {
	ServiceTaxonomy []ReferenceItem `json:"service_taxonomy"`
	ReasonClasses   []ReferenceItem `json:"reason_classes"`
	ImpactEffects   []ReferenceItem `json:"impact_effects"`
	QualityFlags    []ReferenceItem `json:"quality_flags"`
}

func (h *Handler) handleDraftAssistance(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	if h.ai == nil {
		appError(w, http.StatusServiceUnavailable, "assistance_not_configured")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	var body assistanceDraftRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	detail, err := h.loadMaintenanceDetail(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "maintenance_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "assistance_load_maintenance", err, "maintenance_id", id)
		return
	}
	references, err := h.loadAssistanceReferences(r, id)
	if err != nil {
		h.dbFailure(w, r, "assistance_reference_data", err, "maintenance_id", id)
		return
	}
	response, err := h.generateAssistanceDraft(r, detail, references, body)
	if err != nil {
		logAssistanceFailure(
			r.Context(),
			"maintenance assistance failed",
			err,
			"maintenance_id", id,
		)
		appError(w, http.StatusBadGateway, "assistance_generation_failed")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *Handler) handlePreviewAssistance(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	if h.ai == nil {
		appError(w, http.StatusServiceUnavailable, "assistance_not_configured")
		return
	}
	var body assistancePreviewRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	brief := strings.TrimSpace(body.Brief)
	if len(brief) < 30 {
		appError(w, http.StatusBadRequest, "brief_too_short")
		return
	}
	if len(brief) > 2000 {
		appError(w, http.StatusBadRequest, "brief_too_long")
		return
	}
	references, err := h.loadPreviewReferences(r.Context())
	if err != nil {
		h.dbFailure(w, r, "assistance_preview_refs", err)
		return
	}
	detail := buildPreviewDetail(body)
	draft := assistanceDraftRequest{Note: &brief, Regenerate: false}
	response, err := h.generateAssistanceDraft(r, detail, references, draft)
	if err != nil {
		logAssistanceFailure(
			r.Context(),
			"maintenance assistance preview failed",
			err,
		)
		appError(w, http.StatusBadGateway, "assistance_generation_failed")
		return
	}
	httputil.JSON(w, http.StatusOK, mapPreviewResponse(response))
}

func (h *Handler) loadPreviewReferences(ctx context.Context) (assistanceReferenceBundle, error) {
	var bundle assistanceReferenceBundle
	var err error
	if bundle.ServiceTaxonomy, err = h.loadReferenceItems(ctx, resourceMetas["service-taxonomy"], true, nil); err != nil {
		return bundle, err
	}
	if bundle.ReasonClasses, err = h.loadReferenceItems(ctx, resourceMetas["reason-classes"], true, nil); err != nil {
		return bundle, err
	}
	if bundle.ImpactEffects, err = h.loadReferenceItems(ctx, resourceMetas["impact-effects"], true, nil); err != nil {
		return bundle, err
	}
	if bundle.QualityFlags, err = h.loadReferenceItems(ctx, resourceMetas["quality-flags"], true, nil); err != nil {
		return bundle, err
	}
	return bundle, nil
}

func buildPreviewDetail(body assistancePreviewRequest) MaintenanceDetail {
	detail := MaintenanceDetail{}
	if body.MaintenanceKindID != nil {
		detail.MaintenanceKind = ReferenceItem{ID: *body.MaintenanceKindID}
	}
	if body.TechnicalDomainID != nil {
		detail.TechnicalDomain = ReferenceItem{ID: *body.TechnicalDomainID}
	}
	if body.CustomerScopeID != nil {
		detail.CustomerScope = &ReferenceItem{ID: *body.CustomerScopeID}
	}
	return detail
}

func mapPreviewResponse(draft assistanceDraftResponse) assistancePreviewResponse {
	return assistancePreviewResponse{
		Texts:              draft.Texts,
		ServiceTaxonomyIDs: proposalIDs(draft.ServiceTaxonomy),
		ReasonClassIDs:     proposalIDs(draft.ReasonClasses),
		ImpactEffectIDs:    proposalIDs(draft.ImpactEffects),
		QualityFlagIDs:     proposalIDs(draft.QualityFlags),
		ServiceTaxonomy:    draft.ServiceTaxonomy,
		ReasonClasses:      draft.ReasonClasses,
		ImpactEffects:      draft.ImpactEffects,
		QualityFlags:       draft.QualityFlags,
		Audit:              draft.Audit,
		Usage:              draft.Usage,
	}
}

func proposalIDs(items []assistanceClassificationProposal) []int64 {
	if len(items) == 0 {
		return []int64{}
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ReferenceID)
	}
	return ids
}

func (h *Handler) loadAssistanceReferences(r *http.Request, maintenanceID int64) (assistanceReferenceBundle, error) {
	selected, err := h.selectedReferenceIDs(r.Context(), maintenanceID)
	if err != nil {
		return assistanceReferenceBundle{}, err
	}
	var bundle assistanceReferenceBundle
	if bundle.ServiceTaxonomy, err = h.loadReferenceItems(r.Context(), resourceMetas["service-taxonomy"], true, selected["service-taxonomy"]); err != nil {
		return bundle, err
	}
	if bundle.ReasonClasses, err = h.loadReferenceItems(r.Context(), resourceMetas["reason-classes"], true, selected["reason-classes"]); err != nil {
		return bundle, err
	}
	if bundle.ImpactEffects, err = h.loadReferenceItems(r.Context(), resourceMetas["impact-effects"], true, selected["impact-effects"]); err != nil {
		return bundle, err
	}
	if bundle.QualityFlags, err = h.loadReferenceItems(r.Context(), resourceMetas["quality-flags"], true, selected["quality-flags"]); err != nil {
		return bundle, err
	}
	return bundle, nil
}

func (h *Handler) generateAssistanceDraft(r *http.Request, detail MaintenanceDetail, refs assistanceReferenceBundle, body assistanceDraftRequest) (assistanceDraftResponse, error) {
	modelScope := llmModelScopeAssistanceDraft
	requestedModel, err := h.resolveLLMModel(r.Context(), modelScope)
	if err != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(fmt.Errorf("resolve assistance model: %w", err), modelScope, "")
	}
	payload, err := json.MarshalIndent(buildAssistancePromptPayload(detail, refs, body), "", "  ")
	if err != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(fmt.Errorf("marshal assistance payload: %w", err), modelScope, requestedModel)
	}
	request := openrouter.ChatRequest{
		Model:       requestedModel,
		Temperature: 0.2,
		MaxTokens:   4096,
		ResponseFormat: &openrouter.ResponseFormat{
			Type: "json_object",
		},
		Messages: []openrouter.Message{
			{Role: "system", Content: maintenanceAssistanceSystemPrompt},
			{Role: "user", Content: string(payload)},
		},
	}

	start := time.Now()
	aiResponse, err := h.ai.Chat(r.Context(), request)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(err, modelScope, requestedModel)
	}
	logging.FromContext(r.Context()).Info(
		"maintenance assistance completion succeeded",
		"component", "manutenzioni",
		"request_id", logging.RequestID(r.Context()),
		"maintenance_id", detail.MaintenanceID,
		"model_scope", modelScope,
		"requested_model", requestedModel,
		"model", aiResponse.Model,
		"latency_ms", latencyMs,
		"prompt_tokens", aiResponse.Usage.PromptTokens,
		"completion_tokens", aiResponse.Usage.CompletionTokens,
		"total_tokens", aiResponse.Usage.TotalTokens,
	)

	parsed, err := decodeAssistanceAIOutput(aiResponse.Content)
	if err != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(err, modelScope, requestedModel)
	}
	parsed.Texts.TitleIT = cleanTextOrFallback(parsed.Texts.TitleIT, detail.TitleIT)
	parsed.Texts.TitleEN = cleanText(parsed.Texts.TitleEN)
	parsed.Texts.DescriptionIT = cleanText(parsed.Texts.DescriptionIT)
	parsed.Texts.DescriptionEN = cleanText(parsed.Texts.DescriptionEN)
	if strings.TrimSpace(stringValue(detail.ReasonIT)) == "" {
		parsed.Texts.ReasonEN = nil
	} else {
		parsed.Texts.ReasonEN = cleanText(parsed.Texts.ReasonEN)
	}
	if strings.TrimSpace(stringValue(detail.ResidualServiceIT)) == "" {
		parsed.Texts.ResidualServiceEN = nil
	} else {
		parsed.Texts.ResidualServiceEN = cleanText(parsed.Texts.ResidualServiceEN)
	}

	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		summary = "Proposte generate dal contesto disponibile."
	}
	return assistanceDraftResponse{
		Texts:           parsed.Texts,
		ServiceTaxonomy: sanitizeAssistanceClassifications(parsed.ServiceTaxonomy, refs.ServiceTaxonomy, true, detail.TechnicalDomain.ID),
		ReasonClasses:   sanitizeAssistanceClassifications(parsed.ReasonClasses, refs.ReasonClasses, true, 0),
		ImpactEffects:   sanitizeAssistanceClassifications(parsed.ImpactEffects, refs.ImpactEffects, true, 0),
		QualityFlags:    sanitizeAssistanceClassifications(parsed.QualityFlags, refs.QualityFlags, false, 0),
		Audit: assistanceAudit{
			GeneratedAt: time.Now().UTC(),
			Model:       aiResponse.Model,
			Summary:     summary,
		},
		Usage: assistanceUsage{
			PromptTokens:     aiResponse.Usage.PromptTokens,
			CompletionTokens: aiResponse.Usage.CompletionTokens,
			TotalTokens:      aiResponse.Usage.TotalTokens,
		},
	}, nil
}

func buildAssistancePromptPayload(detail MaintenanceDetail, refs assistanceReferenceBundle, body assistanceDraftRequest) map[string]any {
	return map[string]any{
		"maintenance": map[string]any{
			"id":                  detail.MaintenanceID,
			"code":                detail.Code,
			"title_it":            detail.TitleIT,
			"title_en":            detail.TitleEN,
			"description_it":      detail.DescriptionIT,
			"description_en":      detail.DescriptionEN,
			"maintenance_kind":    detail.MaintenanceKind,
			"technical_domain":    detail.TechnicalDomain,
			"customer_scope":      detail.CustomerScope,
			"site":                detail.Site,
			"reason_it":           detail.ReasonIT,
			"reason_en":           detail.ReasonEN,
			"residual_service_it": detail.ResidualServiceIT,
			"residual_service_en": detail.ResidualServiceEN,
			"current_window":      detail.CurrentWindow,
			"metadata":            detail.Metadata,
		},
		"current_classifications": map[string]any{
			"service_taxonomy": classificationPromptItems(detail.ServiceTaxonomy),
			"reason_classes":   classificationPromptItems(detail.ReasonClasses),
			"impact_effects":   classificationPromptItems(detail.ImpactEffects),
			"quality_flags":    classificationPromptItems(detail.QualityFlags),
		},
		"reference_options": map[string]any{
			"service_taxonomy": assistanceReferenceOptions(refs.ServiceTaxonomy),
			"reason_classes":   assistanceReferenceOptions(refs.ReasonClasses),
			"impact_effects":   assistanceReferenceOptions(refs.ImpactEffects),
			"quality_flags":    assistanceReferenceOptions(refs.QualityFlags),
		},
		"user_note":  strings.TrimSpace(stringValue(body.Note)),
		"regenerate": body.Regenerate,
	}
}

func classificationPromptItems(items []ClassificationItem) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"reference_id": item.Reference.ID,
			"label":        item.Reference.NameIT,
			"source":       item.Source,
			"confidence":   item.Confidence,
			"is_primary":   item.IsPrimary,
		})
	}
	return result
}

func assistanceReferenceOptions(items []ReferenceItem) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"reference_id":          item.ID,
			"code":                  item.Code,
			"name_it":               item.NameIT,
			"name_en":               item.NameEN,
			"description":           item.Description,
			"technical_domain_id":   item.TechnicalDomainID,
			"technical_domain_name": item.TechnicalDomainName,
			"target_type_id":        item.TargetTypeID,
			"target_type_name":      item.TargetTypeName,
			"audience":              item.Audience,
		})
	}
	return result
}

func decodeAssistanceAIOutput(content string) (assistanceAIOutput, error) {
	var parsed assistanceAIOutput
	if err := json.Unmarshal([]byte(content), &parsed); err == nil {
		return parsed, nil
	}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return parsed, errAssistanceDecode
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &parsed); err != nil {
		return parsed, fmt.Errorf("%w: %v", errAssistanceDecode, err)
	}
	return parsed, nil
}

func sanitizeAssistanceClassifications(items []assistanceAIClassification, refs []ReferenceItem, hasPrimary bool, technicalDomainID int64) []assistanceClassificationProposal {
	byID := map[int64]ReferenceItem{}
	for _, ref := range refs {
		if technicalDomainID > 0 && ref.TechnicalDomainID != nil && *ref.TechnicalDomainID != technicalDomainID {
			continue
		}
		byID[ref.ID] = ref
	}

	result := make([]assistanceClassificationProposal, 0, len(items))
	seen := map[int64]struct{}{}
	for _, item := range items {
		ref, ok := byID[item.ReferenceID]
		if !ok {
			continue
		}
		if _, ok := seen[item.ReferenceID]; ok {
			continue
		}
		seen[item.ReferenceID] = struct{}{}
		confidence := cleanConfidence(item.Confidence)
		result = append(result, assistanceClassificationProposal{
			ReferenceID: item.ReferenceID,
			Label:       ref.NameIT,
			Source:      "ai_extracted",
			Confidence:  confidence,
			IsPrimary:   hasPrimary && len(result) == 0,
			Rationale:   cleanText(item.Rationale),
		})
	}
	return result
}

func cleanText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func cleanTextOrFallback(value *string, fallback string) *string {
	if cleaned := cleanText(value); cleaned != nil {
		return cleaned
	}
	return stringPtr(fallback)
}

func cleanConfidence(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cleaned := *value
	if cleaned < 0 {
		cleaned = 0
	}
	if cleaned > 1 {
		cleaned = 1
	}
	return &cleaned
}

var maintenanceAssistanceSystemPrompt = strings.TrimSpace(`
Sei un assistente per manutenzioni tecniche interne. Devi proporre testi e classificazioni a partire dal contesto disponibile e dalle opzioni di riferimento.

Se la manutenzione non ha ancora id ne titolo e user_note contiene un brief libero, usa user_note come fonte primaria per inferire titolo, descrizione e classificazioni.
Se la manutenzione esiste gia con dati propri, integra e affina senza inventare informazioni non supportate.

Restituisci solo un oggetto JSON valido con questa forma:
{
  "texts": {
    "title_it": "titolo operativo in italiano",
    "title_en": "English title",
    "description_it": "descrizione operativa in italiano",
    "description_en": "English description",
    "reason_en": "English reason, only if reason_it exists",
    "residual_service_en": "English residual service, only if residual_service_it exists"
  },
  "service_taxonomy": [{"reference_id": 1, "confidence": 0.85, "rationale": "motivo sintetico"}],
  "reason_classes": [{"reference_id": 1, "confidence": 0.85, "rationale": "motivo sintetico"}],
  "impact_effects": [{"reference_id": 1, "confidence": 0.85, "rationale": "motivo sintetico"}],
  "quality_flags": [{"reference_id": 1, "confidence": 0.85, "rationale": "motivo sintetico"}],
  "summary": "sintesi sintetica delle proposte"
}

Regole:
- Usa solo reference_id presenti in reference_options.
- Per service_taxonomy scegli solo servizi coerenti con il technical_domain della manutenzione.
- Non inventare clienti, target, ordini, circuiti o asset puntuali.
- Non applicare automaticamente nulla: produci solo proposte.
- Mantieni un tono operativo, sintetico e adatto a comunicazioni interne.
- Se un dato non e supportato dal contesto, lascia il campo vuoto o ometti la proposta.
`)
