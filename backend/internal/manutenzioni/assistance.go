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
	"github.com/sciacco/mrsmith/internal/platform/llm"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

var errAssistanceDecode = errors.New("decode assistance response")

const (
	manutenzioniApp      = "manutenzioni"
	assistanceDraftScope = "assistance_draft"
)

// recordAssistanceAudit appends the assistance LLM call (success or failure) to
// the centralized audit, best-effort.
func (h *Handler) recordAssistanceAudit(ctx context.Context, detail MaintenanceDetail, model llm.Model, prompt llm.Prompt, request llm.ChatRequest, resp llm.ChatResponse, latencyMs int, callErr error) {
	if h.llmSvc == nil {
		return
	}
	requestRaw, _ := json.Marshal(request)
	usageRaw, _ := json.Marshal(resp.Usage)
	ctxRaw, _ := json.Marshal(map[string]any{"maintenance_id": detail.MaintenanceID})
	audit := llm.CallAudit{
		App:        manutenzioniApp,
		Scope:      assistanceDraftScope,
		ProviderID: model.ProviderID,
		ModelID:    model.ID,
		PromptID:   prompt.ID,
		Model:      model.Model,
		Request:    requestRaw,
		Usage:      usageRaw,
		Context:    ctxRaw,
		DurationMS: &latencyMs,
	}
	if callErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = callErr.Error()
	} else if respRaw, mErr := json.Marshal(map[string]any{"content": resp.Content}); mErr == nil {
		audit.Response = respRaw
	}
	_ = h.llmSvc.RecordAudit(ctx, audit)
}

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
	if h.llmSvc == nil {
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
	modelScope := assistanceDraftScope
	model, err := h.llmSvc.ResolveModel(r.Context(), manutenzioniApp, modelScope, "")
	if err != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(fmt.Errorf("resolve assistance model: %w", err), modelScope, "")
	}
	prompt, err := h.llmSvc.ResolvePrompt(r.Context(), manutenzioniApp, modelScope, "")
	if err != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(fmt.Errorf("resolve assistance prompt: %w", err), modelScope, model.Model)
	}
	client, _, err := h.llmSvc.ClientForModel(r.Context(), model)
	if err != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(fmt.Errorf("build assistance client: %w", err), modelScope, model.Model)
	}
	payload, err := json.MarshalIndent(buildAssistancePromptPayload(detail, refs, body), "", "  ")
	if err != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(fmt.Errorf("marshal assistance payload: %w", err), modelScope, model.Model)
	}
	params := model.DecodedParams()
	temperature := 0.2
	if params.Temperature != nil {
		temperature = *params.Temperature
	}
	maxTokens := 4096
	if params.MaxTokens != nil {
		maxTokens = *params.MaxTokens
	}
	request := llm.ChatRequest{
		Model:       model.Model,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		ResponseFormat: &llm.ResponseFormat{
			Type: "json_object",
		},
		Messages: []llm.Message{
			{Role: "system", Content: prompt.Prompt},
			{Role: "user", Content: string(payload)},
		},
	}

	start := time.Now()
	aiResponse, chatErr := client.Chat(r.Context(), request)
	latencyMs := time.Since(start).Milliseconds()
	h.recordAssistanceAudit(r.Context(), detail, model, prompt, request, aiResponse, int(latencyMs), chatErr)
	if chatErr != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(chatErr, modelScope, model.Model)
	}
	logging.FromContext(r.Context()).Info(
		"maintenance assistance completion succeeded",
		"component", "manutenzioni",
		"request_id", logging.RequestID(r.Context()),
		"maintenance_id", detail.MaintenanceID,
		"model_scope", modelScope,
		"requested_model", model.Model,
		"model", aiResponse.Model,
		"latency_ms", latencyMs,
		"prompt_tokens", aiResponse.Usage.PromptTokens,
		"completion_tokens", aiResponse.Usage.CompletionTokens,
		"total_tokens", aiResponse.Usage.TotalTokens,
	)

	parsed, err := decodeAssistanceAIOutput(aiResponse.Content)
	if err != nil {
		return assistanceDraftResponse{}, wrapAssistanceFailure(err, modelScope, model.Model)
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
