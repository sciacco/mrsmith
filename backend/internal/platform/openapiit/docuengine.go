package openapiit

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// DocuEngine è il servizio openapi.it che consegna i documenti camerali (per il
// binocolo: "Bilancio Ottico", il bilancio depositato in PDF). Vive su un host a
// sé (docuengine.openapi.com) ma condivide auth Bearer, envelope {data,success,
// message,error} e la macchina degli errori con gli altri servizi del package.
//
// Flusso a 2 fasi (progettato in F4, qui il client resta thin e stateless):
// POST /requests (state NEW) -> PATCH /requests/{id} (state SEARCH) -> poll
// GET /requests/{id} finché compaiono i results -> PATCH resultId -> poll DONE
// -> GET /requests/{id}/documents. NESSUN loop di polling con sleep dentro il
// client: lo pilota il job worker tra i tick.

// Stati osservabili di una richiesta DocuEngine.
const (
	DocuStateNew       = "NEW"
	DocuStateSearch    = "SEARCH"
	DocuStateWait      = "WAIT"
	DocuStateDone      = "DONE"
	DocuStateCancelled = "CANCELLED"
)

// docuEngineMaxDownloadBytes è il tetto difensivo su un singolo download firmato:
// un bilancio depositato è tipicamente < 5 MB, 50 MiB copre casi estremi senza
// esporre il processo a un body ostile.
const docuEngineMaxDownloadBytes = 50 << 20

// DocuEngineError è un errore di business restituito nell'envelope anche con
// HTTP 200 (success=false oppure error!=nil). Distinto da APIError (che porta uno
// status HTTP non-2xx) perché il chiamante può volerli trattare diversamente.
type DocuEngineError struct {
	Path    string
	Code    *int
	Message string
}

func (e *DocuEngineError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "docuengine business error"
	}
	if e.Code != nil {
		return fmt.Sprintf("openapi.it %s: %s (code %d)", docuEngineServiceName, message, *e.Code)
	}
	return fmt.Sprintf("openapi.it %s: %s", docuEngineServiceName, message)
}

// DocuEngineClient è il sotto-client per il servizio DocuEngine, gemello di
// CompanyClient. Nil-safe come Company()/CAP().
type DocuEngineClient struct {
	client *Client
}

func (c *Client) DocuEngine() *DocuEngineClient {
	if c == nil {
		return nil
	}
	return &DocuEngineClient{client: c}
}

// DocuDocument è una voce del catalogo documenti (GET /documents). Per il binocolo
// interessa scoprire il documentId del "Bilancio Ottico" (name esatto), che NON è
// hardcodato lato codice: arriva dalla config del modulo.
type DocuDocument struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	HasSearch     bool    `json:"hasSearch"`
	IsSync        bool    `json:"isSync"`
	SearchPrice   float64 `json:"searchPrice"`
	DocumentPrice float64 `json:"documentPrice"`
	TotalPrice    float64 `json:"totalPrice"`
}

// DocuRequestCreate è il body di POST /requests. Search è la mappa dei campi di
// ricerca ({"field0": <taxCode>} per il Bilancio Ottico). NESSUNA callback: la
// pipeline usa polling.
type DocuRequestCreate struct {
	DocumentID string            `json:"documentId"`
	State      string            `json:"state"`
	Search     map[string]string `json:"search,omitempty"`
}

// DocuResult è un candidato tra i results di una richiesta. ID è un handle OPACO
// (32-hex per-request) usato SOLO nel PATCH resultId. Data è l'oggetto non
// tipizzato del risultato: si decodifica in modo difensivo con BalanceSheet().
type DocuResult struct {
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}

// DocuBalanceSheet sono i campi utili del result data per un bilancio. Tutti
// opzionali: BalanceSheet() non fallisce e non va mai in panic su payload parziali.
type DocuBalanceSheet struct {
	BalanceSheetID              string
	BalanceSheetDate            string
	BalanceSheetTypeCode        string
	BalanceSheetTypeDescription string
}

// BalanceSheet decodifica difensivamente il result data. balanceSheetId è un int
// nel payload vendor (json.Number -> string); balanceSheetTypeCode può essere
// stringa ("710") o numero. Ritorna ok=false solo se non c'è alcun campo utile.
func (r DocuResult) BalanceSheet() (DocuBalanceSheet, bool) {
	if len(r.Data) == 0 {
		return DocuBalanceSheet{}, false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(r.Data, &obj); err != nil {
		return DocuBalanceSheet{}, false
	}
	bs := DocuBalanceSheet{
		BalanceSheetID:              scalarString(obj["balanceSheetId"]),
		BalanceSheetDate:            scalarString(obj["balanceSheetDate"]),
		BalanceSheetTypeCode:        scalarString(obj["balanceSheetTypeCode"]),
		BalanceSheetTypeDescription: scalarString(obj["balanceSheetTypeDescription"]),
	}
	ok := bs.BalanceSheetID != "" || bs.BalanceSheetDate != "" || bs.BalanceSheetTypeCode != ""
	return bs, ok
}

// DocuDownload è un file scaricabile (GET /requests/{id}/documents). md5 arriva in
// BASE64 (usa CanonicalMD5FromBase64 per il confronto); fileSize è una STRINGA nel
// payload vendor; downloadUrl è un URL GCS pre-firmato con validità ~24h.
type DocuDownload struct {
	FileName    string `json:"fileName"`
	MimeType    string `json:"mimeType"`
	FileSize    string `json:"fileSize"`
	MD5         string `json:"md5"`
	URLExpire   int64  `json:"urlExpire"`
	DownloadURL string `json:"downloadUrl"`
}

// DocuRequest è la vista completa di una richiesta (GET /requests/{id}).
type DocuRequest struct {
	ID                 string            `json:"id"`
	DocumentID         string            `json:"documentId"`
	Name               string            `json:"name"`
	State              string            `json:"state"`
	ReadableSearch     map[string]string `json:"readableSearch"`
	Results            []DocuResult      `json:"results"`
	ResultID           *string           `json:"resultId"`
	Documents          []DocuDownload    `json:"documents"`
	CancellationReason string            `json:"cancellationReason"`
	Timestamps         DocuTimestamps    `json:"timestamps"`
	SearchPrice        float64           `json:"searchPrice"`
	DocumentPrice      float64           `json:"documentPrice"`
	TotalPrice         float64           `json:"totalPrice"`
}

// DocuRequestSummary è la vista sintetica di GET /requests (la lista NON ha filtri
// server-side: la riconciliazione unknown la usa per finestrare sui timestamps e
// poi GET /requests/{id} per il match su readableSearch).
type DocuRequestSummary struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	State      string         `json:"state"`
	Timestamps DocuTimestamps `json:"timestamps"`
}

// DocuTimestamps sono i timestamp unix per-richiesta. Il set esatto delle chiavi
// non è fissato dalla spec, quindi si decodifica in modo permissivo: ogni scalare
// (numero/stringa/null) diventa int64, le chiavi ignote sono preservate e un
// valore mancante/vuoto vale 0. Non fallisce mai il decode dell'intera richiesta.
type DocuTimestamps map[string]int64

func (t *DocuTimestamps) UnmarshalJSON(b []byte) error {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &raw); err != nil {
		*t = DocuTimestamps{}
		return nil
	}
	out := make(DocuTimestamps, len(raw))
	for k, v := range raw {
		if n, ok := parseUnix(v); ok {
			out[k] = n
		}
	}
	*t = out
	return nil
}

// Max ritorna il timestamp più recente tra quelli presenti (0 se vuoto): utile per
// finestrare i candidati nella riconciliazione unknown.
func (t DocuTimestamps) Max() int64 {
	var max int64
	for _, v := range t {
		if v > max {
			max = v
		}
	}
	return max
}

// ListDocuments enumera il catalogo documenti (GET /documents).
func (dc *DocuEngineClient) ListDocuments(ctx context.Context) ([]DocuDocument, error) {
	return docuEngineCall[[]DocuDocument](ctx, dc, http.MethodGet, "/documents", nil, nil)
}

// CreateRequest apre una richiesta (POST /requests). Se State è vuoto usa "NEW"
// (unico stato ammesso in POST). Ritorna la richiesta creata (con id).
func (dc *DocuEngineClient) CreateRequest(ctx context.Context, body DocuRequestCreate) (DocuRequest, error) {
	if strings.TrimSpace(body.State) == "" {
		body.State = DocuStateNew
	}
	return docuEngineCall[DocuRequest](ctx, dc, http.MethodPost, "/requests", nil, body)
}

// GetRequest legge la vista completa di una richiesta (GET /requests/{id}).
func (dc *DocuEngineClient) GetRequest(ctx context.Context, id string) (DocuRequest, error) {
	return docuEngineCall[DocuRequest](ctx, dc, http.MethodGet, "/requests/"+pathSegment(id), nil, nil)
}

// SubmitSearch avvia la ricerca (PATCH /requests/{id}) impostando il tax code e lo
// stato SEARCH. La chiave del body è LETTERALE "search.field0" (non un oggetto
// annidato): è il contratto PatchBody della spec.
func (dc *DocuEngineClient) SubmitSearch(ctx context.Context, id, taxCode string) (DocuRequest, error) {
	body := map[string]any{
		"search.field0": taxCode,
		"state":         DocuStateSearch,
	}
	return docuEngineCall[DocuRequest](ctx, dc, http.MethodPatch, "/requests/"+pathSegment(id), nil, body)
}

// SelectResult sceglie il result da acquistare (PATCH /requests/{id}). resultID è
// l'handle opaco di DocuResult.ID.
func (dc *DocuEngineClient) SelectResult(ctx context.Context, id, resultID string) (DocuRequest, error) {
	body := map[string]any{"resultId": resultID}
	return docuEngineCall[DocuRequest](ctx, dc, http.MethodPatch, "/requests/"+pathSegment(id), nil, body)
}

// ListRequestDocuments elenca i file scaricabili di una richiesta completata
// (GET /requests/{id}/documents).
func (dc *DocuEngineClient) ListRequestDocuments(ctx context.Context, id string) ([]DocuDownload, error) {
	return docuEngineCall[[]DocuDownload](ctx, dc, http.MethodGet, "/requests/"+pathSegment(id)+"/documents", nil, nil)
}

// ListRequests elenca in forma sintetica tutte le richieste (GET /requests). NON ha
// filtri server-side: la riconciliazione unknown filtra client-side.
func (dc *DocuEngineClient) ListRequests(ctx context.Context) ([]DocuRequestSummary, error) {
	return docuEngineCall[[]DocuRequestSummary](ctx, dc, http.MethodGet, "/requests", nil, nil)
}

// DownloadFile scarica un URL GCS pre-firmato. NESSUN header Authorization: l'URL
// porta già la firma; aggiungere l'header può invalidarla. Applica il tetto
// dimensione difensivo.
func (dc *DocuEngineClient) DownloadFile(ctx context.Context, signedURL string) ([]byte, error) {
	if dc == nil || dc.client == nil {
		return nil, fmt.Errorf("openapi.it %s: client not configured", docuEngineServiceName)
	}
	signedURL = strings.TrimSpace(signedURL)
	if signedURL == "" {
		return nil, fmt.Errorf("openapi.it %s: empty download url", docuEngineServiceName)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("openapi.it %s: create download request: %w", docuEngineServiceName, err)
	}
	resp, err := dc.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openapi.it %s: download: %w", docuEngineServiceName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		// Path generico: l'URL firmato contiene un token e non va conservato.
		return nil, &APIError{Service: docuEngineServiceName, StatusCode: resp.StatusCode, Path: "download", Body: string(snippet)}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, docuEngineMaxDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("openapi.it %s: read download: %w", docuEngineServiceName, err)
	}
	if len(data) > docuEngineMaxDownloadBytes {
		return nil, fmt.Errorf("openapi.it %s: download exceeds %d bytes cap", docuEngineServiceName, docuEngineMaxDownloadBytes)
	}
	return data, nil
}

// CanonicalMD5FromBase64 normalizza l'md5 nel formato canonico (hex lowercase, 32
// char). Il valore DocuEngine è base64 (16 byte). Tollerante: se s è GIÀ 32-hex
// (qualunque case) lo ritorna in lowercase senza ridecodificarlo.
func CanonicalMD5FromBase64(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("openapi.it %s: empty md5", docuEngineServiceName)
	}
	if len(s) == 32 && isHex(s) {
		return strings.ToLower(s), nil
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		// Alcune API omettono il padding: prova la variante senza padding.
		raw, err = base64.RawStdEncoding.DecodeString(s)
		if err != nil {
			return "", fmt.Errorf("openapi.it %s: decode md5 base64: %w", docuEngineServiceName, err)
		}
	}
	if len(raw) != 16 {
		return "", fmt.Errorf("openapi.it %s: md5 has %d bytes, want 16", docuEngineServiceName, len(raw))
	}
	return hex.EncodeToString(raw), nil
}

// docuEngineCall esegue una chiamata DocuEngine e srotola l'envelope: un errore di
// business (success=false o error!=nil) diventa un *DocuEngineError anche con HTTP
// 200. Assunzione (da validare nello smoke): ogni 200 porta l'envelope canonico.
func docuEngineCall[T any](ctx context.Context, dc *DocuEngineClient, method, path string, query url.Values, body any) (T, error) {
	var zero T
	if dc == nil || dc.client == nil {
		return zero, fmt.Errorf("openapi.it %s: client not configured", docuEngineServiceName)
	}
	var env Envelope[T]
	if err := dc.client.doJSON(ctx, docuEngineServiceName, dc.client.docuEngineBaseURL, method, path, query, body, &env); err != nil {
		return zero, err
	}
	if !env.Success || env.Error != nil {
		return zero, &DocuEngineError{Path: path, Code: env.Error, Message: env.Message}
	}
	return env.Data, nil
}

// scalarString estrae uno scalare JSON (stringa/numero/bool) come stringa. Ritorna
// "" su null, vuoto o valori strutturati. Non va mai in panic.
func scalarString(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			return str
		}
		return ""
	}
	if s[0] == '{' || s[0] == '[' {
		return ""
	}
	return s
}

// parseUnix legge un timestamp unix da uno scalare JSON (numero o stringa
// numerica). Ritorna ok=false se non è interpretabile.
func parseUnix(raw json.RawMessage) (int64, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, false
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return 0, false
		}
		s = strings.TrimSpace(str)
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f), true
	}
	return 0, false
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
