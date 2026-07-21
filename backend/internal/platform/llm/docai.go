package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	openai "github.com/openai/openai-go/v3"
)

// Document AI (annotated OCR) è una variante dell'OCR bilanci (issue #78, Fase 5):
// stesso endpoint `ocr` e stesso registro modelli (mrsmith.ocr_model), con in più il
// parametro `document_annotation_format` che chiede al vendor una lettura STRUTTURATA
// del documento secondo uno schema JSON. Serve come RAMO DI CONTROLLO CIRCOSCRITTO
// dietro flag: l'ingest confronta i totali estratti dal parse markdown deterministico
// con quelli annotati dal vendor (docai_diff), ma il dato di record resta il parse.
//
// Contratto vendor ASSUNTO (da validare nello smoke con l'utente, come l'OCR):
// request con gli stessi campi dell'OCR più
//   "document_annotation_format": {"type":"json_schema","json_schema":{"schema":{...}}}
// response con, oltre a "pages"/"usage_info"/"model", un campo
// "document_annotation" (stringa JSON o oggetto) con l'annotazione secondo lo schema.
// Il parsing è difensivo: un'annotazione mancante/malformata non è un errore fatale
// (il ramo DocAI è best-effort e non blocca mai la pipeline).

// DocAICall è l'input di Service.DocAI. Schema è lo schema JSON di annotazione del
// documento (proprietà "schema" del json_schema vendor). RequestID/Context correlano
// trace<->audit come per l'OCR.
type DocAICall struct {
	App          string
	Scope        string
	PDF          []byte
	Schema       json.RawMessage
	RequestID    string
	Context      map[string]any
	ActorSubject string
	ActorEmail   string
}

// OCRAnnotatedResponse è la risposta dell'OCR annotato: le pagine markdown come
// l'OCR normale più Annotation, il JSON grezzo dell'annotazione strutturata.
type OCRAnnotatedResponse struct {
	Model      string
	Pages      []OCRPage
	Annotation json.RawMessage
	UsageInfo  OCRUsageInfo
}

// OCRAnnotated esegue l'OCR con annotazione documentale sullo stesso endpoint `ocr`.
// Il body è quello dell'OCR (stessi parametri pinnati) più document_annotation_format.
func (c *Client) OCRAnnotated(ctx context.Context, model string, pdf []byte, schema json.RawMessage) (OCRAnnotatedResponse, error) {
	if len(pdf) == 0 {
		return OCRAnnotatedResponse{}, fmt.Errorf("%s: docai called with empty document", c.name())
	}
	dataURL := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(pdf)
	annotationFormat := map[string]any{"type": "json_schema"}
	if len(schema) > 0 {
		annotationFormat["json_schema"] = map[string]any{
			"name":   "sp_ce_totals",
			"schema": json.RawMessage(schema),
		}
	}
	body := map[string]any{
		"model":                         model,
		"table_format":                  OCRTableFormat,
		"include_blocks":                OCRIncludeBlocks,
		"confidence_scores_granularity": OCRConfidenceScoresGranularity,
		"document_annotation_format":    annotationFormat,
		"document": map[string]any{
			"type":         "document_url",
			"document_url": dataURL,
		},
	}
	var decoded struct {
		Model string            `json:"model"`
		Pages []json.RawMessage `json:"pages"`
		// document_annotation può arrivare come stringa JSON o come oggetto: RawMessage
		// assorbe entrambe le forme, il chiamante la normalizza.
		DocumentAnnotation json.RawMessage `json:"document_annotation"`
		Usage              struct {
			PagesProcessed int   `json:"pages_processed"`
			DocSizeBytes   int64 `json:"doc_size_bytes"`
		} `json:"usage_info"`
	}
	if err := c.sdk.Post(ctx, "ocr", body, &decoded); err != nil {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			return OCRAnnotatedResponse{}, &APIError{Provider: c.name(), StatusCode: apiErr.StatusCode, Body: errorBody(apiErr)}
		}
		return OCRAnnotatedResponse{}, fmt.Errorf("%s: docai request failed: %w", c.name(), err)
	}
	out := OCRAnnotatedResponse{
		Model:      decoded.Model,
		Pages:      make([]OCRPage, 0, len(decoded.Pages)),
		Annotation: normalizeDocAIAnnotation(decoded.DocumentAnnotation),
		UsageInfo: OCRUsageInfo{
			PagesProcessed: decoded.Usage.PagesProcessed,
			DocSizeBytes:   decoded.Usage.DocSizeBytes,
		},
	}
	for _, raw := range decoded.Pages {
		out.Pages = append(out.Pages, parseOCRPage(raw))
	}
	return out, nil
}

// normalizeDocAIAnnotation ritorna l'annotazione come JSON grezzo. Se il vendor la
// consegna come STRINGA JSON (json.RawMessage che è una stringa quotata), la de-quota
// una volta così il chiamante trova sempre un oggetto/array direttamente parsabile.
func normalizeDocAIAnnotation(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return json.RawMessage(asString)
	}
	return raw
}

// DocAI risolve il modello OCR (stesso registro), esegue l'OCR annotato e registra
// l'audit su mrsmith.llm_call_audit IDENTICAMENTE all'OCR: usage = pagine, request
// REDATTA (mai il base64 del PDF), response redatta a metadati. Ritorna anche il
// modello risolto (snapshot testuale come per l'OCR).
func (s *Service) DocAI(ctx context.Context, call DocAICall) (OCRAnnotatedResponse, OCRModel, error) {
	if s == nil || s.db == nil {
		return OCRAnnotatedResponse{}, OCRModel{}, ErrNotConfigured
	}
	model, err := s.ResolveOCRModel(ctx, call.App, call.Scope)
	if err != nil {
		return OCRAnnotatedResponse{}, OCRModel{}, err
	}
	client, _, err := s.clientForProvider(ctx, model.ProviderID)
	if err != nil {
		return OCRAnnotatedResponse{}, model, err
	}

	start := time.Now()
	resp, docErr := client.OCRAnnotated(ctx, model.Model, call.PDF, call.Schema)
	durMS := int(time.Since(start).Milliseconds())

	pages := resp.UsageInfo.PagesProcessed
	if pages == 0 {
		pages = len(resp.Pages)
	}
	usageRaw, _ := json.Marshal(map[string]any{"pages": pages})
	reqRaw, _ := json.Marshal(map[string]any{
		"model":                         model.Model,
		"table_format":                  OCRTableFormat,
		"include_blocks":                OCRIncludeBlocks,
		"confidence_scores_granularity": OCRConfidenceScoresGranularity,
		"document_annotation":           true,
		"document": map[string]any{
			"type":  "document_url",
			"bytes": len(call.PDF),
		},
	})
	var ctxRaw json.RawMessage
	if len(call.Context) > 0 {
		if b, mErr := json.Marshal(call.Context); mErr == nil {
			ctxRaw = b
		}
	}
	audit := CallAudit{
		App:          call.App,
		Scope:        call.Scope,
		ProviderID:   model.ProviderID,
		ModelID:      model.ID,
		Model:        model.Model,
		Request:      reqRaw,
		Usage:        usageRaw,
		Context:      ctxRaw,
		ActorSubject: call.ActorSubject,
		ActorEmail:   call.ActorEmail,
		RequestID:    call.RequestID,
		DurationMS:   &durMS,
	}
	if docErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = docErr.Error()
		var apiErr *APIError
		if errors.As(docErr, &apiErr) {
			audit.ErrorCode = fmt.Sprintf("http_%d", apiErr.StatusCode)
		}
	} else {
		respRaw, _ := json.Marshal(map[string]any{"model": resp.Model, "pages": pages, "annotated": len(resp.Annotation) > 0})
		audit.Response = respRaw
	}
	// Best-effort e WithoutCancel come l'OCR: registrare comunque la chiamata pagata.
	_ = s.RecordAudit(context.WithoutCancel(ctx), audit)

	if docErr != nil {
		return OCRAnnotatedResponse{}, model, docErr
	}
	return resp, model, nil
}
