package llm

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
)

// OCRModel è una riga risolta da mrsmith.ocr_model (mig 116). L'OCR dei bilanci è
// un servizio a sé (NON una chat completion): ha un registro dedicato, risoluzione
// per (app, scope) e NESSUN fallback sulla cascata chat. Il provider è agganciato
// per id come per gli altri modelli.
type OCRModel struct {
	ID         string
	App        string
	Scope      string
	ProviderID string
	Name       string
	Model      string
	Params     json.RawMessage
	IsDefault  bool
}

// ErrNoOCRModel è restituito quando non esiste un default per l'esatta coppia
// (app, scope). Nessuna cascata: un fascicolo si legge con l'OCR dichiarato, mai
// con un modello di ripiego.
var ErrNoOCRModel = errors.New("llm: no default ocr model for (app, scope)")

// Parametri OCR pinnati come COSTANTI: sono requisiti di correttezza del parser a
// valle (F5), non tuning. Il markdown tabellare e i blocchi/confidence per pagina
// sono ciò che il parser CEE si aspetta di trovare.
const (
	OCRTableFormat                 = "markdown"
	OCRIncludeBlocks               = true
	OCRConfidenceScoresGranularity = "page"
)

const ocrModelColumns = `id::text, app, scope, provider_id::text, name, model, params, is_default`

// ResolveOCRModel risolve l'OCR per l'ESATTA coppia (app, scope) con is_default.
// Nessuna cascata su scope 'default' né su '_global': l'assenza è un errore
// esplicito (ErrNoOCRModel).
func (s *Service) ResolveOCRModel(ctx context.Context, app, scope string) (OCRModel, error) {
	if s == nil || s.db == nil {
		return OCRModel{}, ErrNotConfigured
	}
	app, scope = strings.TrimSpace(app), strings.TrimSpace(scope)
	m, err := scanOCRModelInto(s.db.QueryRowContext(ctx, `SELECT `+ocrModelColumns+`
FROM mrsmith.ocr_model WHERE is_default AND app = $1 AND scope = $2 LIMIT 1`, app, scope))
	if errors.Is(err, sql.ErrNoRows) {
		return OCRModel{}, fmt.Errorf("%w: app=%s scope=%s", ErrNoOCRModel, app, scope)
	}
	return m, err
}

// OCRPage è una pagina del risultato OCR. Extras porta grezzi i campi extra della
// pagina (blocks/confidence/dimensions se presenti), letti dal parser a valle.
type OCRPage struct {
	Index    int
	Markdown string
	Extras   json.RawMessage
}

// OCRUsageInfo riporta il consumo: per l'OCR l'unità è la PAGINA, non i token.
type OCRUsageInfo struct {
	PagesProcessed int
	DocSizeBytes   int64
}

// OCRResponse è la risposta OCR decodificata in modo difensivo.
type OCRResponse struct {
	Model     string
	Pages     []OCRPage
	UsageInfo OCRUsageInfo
}

// OCR esegue l'OCR di un PDF sul provider (Mistral in mig 116: base_url
// https://api.mistral.ai/v1, path relativo "ocr"). SEMPRE sincrono, MAI batch. Il
// documento è inline come data-URL base64.
//
// Contratto vendor ASSUNTO (da validare nello smoke con l'utente): request con
// "model" + i tre parametri costanti + "document":{"type":"document_url",
// "document_url":"data:application/pdf;base64,..."}; response con "pages"
// ([{index, markdown, ...}]), "usage_info":{pages_processed, doc_size_bytes} e
// "model". I punti assunti sono commentati nei tag JSON qui sotto.
func (c *Client) OCR(ctx context.Context, model string, pdf []byte) (OCRResponse, error) {
	if len(pdf) == 0 {
		return OCRResponse{}, fmt.Errorf("%s: ocr called with empty document", c.name())
	}
	dataURL := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(pdf)
	body := map[string]any{
		"model":                         model,
		"table_format":                  OCRTableFormat,
		"include_blocks":                OCRIncludeBlocks,
		"confidence_scores_granularity": OCRConfidenceScoresGranularity,
		"document": map[string]any{
			"type":         "document_url",
			"document_url": dataURL,
		},
	}
	var decoded struct {
		Model string            `json:"model"`
		Pages []json.RawMessage `json:"pages"`
		Usage struct {
			PagesProcessed int   `json:"pages_processed"`
			DocSizeBytes   int64 `json:"doc_size_bytes"`
		} `json:"usage_info"`
	}
	if err := c.sdk.Post(ctx, "ocr", body, &decoded); err != nil {
		var apiErr *openai.Error
		if errors.As(err, &apiErr) {
			return OCRResponse{}, &APIError{Provider: c.name(), StatusCode: apiErr.StatusCode, Body: errorBody(apiErr)}
		}
		return OCRResponse{}, fmt.Errorf("%s: ocr request failed: %w", c.name(), err)
	}
	out := OCRResponse{
		Model: decoded.Model,
		Pages: make([]OCRPage, 0, len(decoded.Pages)),
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

// parseOCRPage estrae index/markdown e conserva il resto della pagina come Extras.
// Difensivo: campi mancanti restano a zero, non fallisce mai.
func parseOCRPage(raw json.RawMessage) OCRPage {
	var page OCRPage
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return page
	}
	if v, ok := fields["index"]; ok {
		_ = json.Unmarshal(v, &page.Index)
	}
	if v, ok := fields["markdown"]; ok {
		_ = json.Unmarshal(v, &page.Markdown)
	}
	delete(fields, "index")
	delete(fields, "markdown")
	if len(fields) > 0 {
		if extras, err := json.Marshal(fields); err == nil {
			page.Extras = extras
		}
	}
	return page
}

// OCRCall è l'input di Service.OCR. RequestID e Context correlano trace<->audit
// (a differenza del brief, l'OCR li valorizza).
type OCRCall struct {
	App          string
	Scope        string
	PDF          []byte
	RequestID    string
	Context      map[string]any
	ActorSubject string
	ActorEmail   string
}

// OCR risolve il modello OCR, esegue la chiamata e registra l'audit su
// mrsmith.llm_call_audit. usage = PAGINE ({"pages": N}); la request è REDATTA (mai
// il base64 del PDF: solo dimensione e parametri). Ritorna anche il modello
// risolto perché il chiamante ne salva lo snapshot testuale nel processing run.
func (s *Service) OCR(ctx context.Context, call OCRCall) (OCRResponse, OCRModel, error) {
	if s == nil || s.db == nil {
		return OCRResponse{}, OCRModel{}, ErrNotConfigured
	}
	model, err := s.ResolveOCRModel(ctx, call.App, call.Scope)
	if err != nil {
		return OCRResponse{}, OCRModel{}, err
	}
	client, _, err := s.clientForProvider(ctx, model.ProviderID)
	if err != nil {
		return OCRResponse{}, model, err
	}

	start := time.Now()
	resp, ocrErr := client.OCR(ctx, model.Model, call.PDF)
	durMS := int(time.Since(start).Milliseconds())

	pages := resp.UsageInfo.PagesProcessed
	if pages == 0 {
		pages = len(resp.Pages)
	}
	usageRaw, _ := json.Marshal(map[string]any{"pages": pages})
	// Request redatta: NIENTE base64 del PDF, solo dimensione + parametri pinnati.
	reqRaw, _ := json.Marshal(map[string]any{
		"model":                         model.Model,
		"table_format":                  OCRTableFormat,
		"include_blocks":                OCRIncludeBlocks,
		"confidence_scores_granularity": OCRConfidenceScoresGranularity,
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
	if ocrErr != nil {
		audit.Status = "failed"
		audit.ErrorMessage = ocrErr.Error()
		var apiErr *APIError
		if errors.As(ocrErr, &apiErr) {
			audit.ErrorCode = fmt.Sprintf("http_%d", apiErr.StatusCode)
		}
	} else {
		// Response redatta a metadati: il markdown OCR è grande e può contenere PII
		// (compensi/anagrafiche dei prospetti). Il testo integrale vive nel filing,
		// non nell'audit.
		respRaw, _ := json.Marshal(map[string]any{"model": resp.Model, "pages": pages})
		audit.Response = respRaw
	}
	// Best-effort: un audit fallito non deve abortire l'ingest (come nel brief).
	// WithoutCancel: se l'OCR va in timeout/cancellazione dobbiamo comunque
	// registrare la chiamata pagata — il ctx originale è già cancellato e farebbe
	// fallire subito l'INSERT proprio nel caso più importante da tracciare.
	_ = s.RecordAudit(context.WithoutCancel(ctx), audit)

	if ocrErr != nil {
		return OCRResponse{}, model, ocrErr
	}
	return resp, model, nil
}

func scanOCRModelInto(sc rowScanner) (OCRModel, error) {
	var m OCRModel
	var params []byte
	if err := sc.Scan(&m.ID, &m.App, &m.Scope, &m.ProviderID, &m.Name, &m.Model, &params, &m.IsDefault); err != nil {
		return OCRModel{}, err
	}
	if len(params) > 0 {
		m.Params = json.RawMessage(params)
	}
	return m, nil
}
