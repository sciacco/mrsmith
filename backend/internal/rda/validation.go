package rda

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func validateCreatePO(req createPORequest) error {
	if req.Type != "STANDARD" && req.Type != "ECOMMERCE" {
		return errors.New("Seleziona il tipo PO")
	}
	if req.BudgetID <= 0 {
		return errors.New("Seleziona un budget")
	}
	if req.ProviderID <= 0 {
		return errors.New("Seleziona un fornitore")
	}
	if strings.TrimSpace(req.Project) == "" {
		return errors.New("Inserisci il progetto")
	}
	if len([]rune(strings.TrimSpace(req.Project))) > 50 {
		return errors.New("Il progetto puo avere al massimo 50 caratteri")
	}
	if strings.TrimSpace(req.Object) == "" {
		return errors.New("Inserisci l'oggetto")
	}
	hasCostCenter := strings.TrimSpace(req.CostCenter) != ""
	hasBudgetUser := req.BudgetUserID > 0
	if hasCostCenter == hasBudgetUser {
		return errors.New("Il budget deve indicare un solo centro di costo o utente")
	}
	return nil
}

func (h *Handler) handleCreateRow(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	if !isRequester(po, email) || po.State != "DRAFT" {
		httputil.Error(w, http.StatusForbidden, "Le righe possono essere modificate solo dal richiedente in bozza")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return
	}
	if err := validateRow(body); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	upstreamBody := buildRowCreateBody(body, email)
	encoded, err := encodeJSONBody(upstreamBody)
	if err != nil {
		httputil.InternalError(w, r, err, "rda row body encode failed")
		return
	}
	h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/row", "", encoded, mergeHeaders(requesterHeaders(email), jsonHeaders()))
}

func validateRow(body map[string]any) error {
	rowType := strings.TrimSpace(stringValue(body["type"]))
	if rowType != "good" && rowType != "service" {
		return errors.New("Seleziona il tipo riga")
	}
	if strings.TrimSpace(stringValue(body["description"])) == "" {
		return errors.New("Inserisci la descrizione")
	}
	if numberValue(body["qty"]) <= 0 {
		return errors.New("Inserisci una quantita maggiore di zero")
	}
	if strings.TrimSpace(stringValue(body["product_code"])) == "" || strings.TrimSpace(stringValue(body["product_description"])) == "" {
		return errors.New("Seleziona un articolo")
	}
	paymentDetail, _ := body["payment_detail"].(map[string]any)
	startAt := strings.TrimSpace(stringValue(paymentDetail["start_at"]))
	if startAt == "" {
		return errors.New("Seleziona la decorrenza")
	}
	if startAt == "specific_date" && strings.TrimSpace(stringValue(paymentDetail["start_at_date"])) == "" {
		return errors.New("Inserisci la data di decorrenza")
	}
	if rowType == "good" {
		if startAt != "activation_date" && startAt != "advance_payment" && startAt != "specific_date" {
			return errors.New("Decorrenza non valida")
		}
		if numberValue(body["price"]) <= 0 {
			return errors.New("Inserisci un costo unitario maggiore di zero")
		}
		return nil
	}
	if startAt != "activation_date" && startAt != "specific_date" {
		return errors.New("Decorrenza non valida")
	}
	mrc := numberValue(body["monthly_fee"])
	if mrc == 0 {
		mrc = numberValue(body["montly_fee"])
	}
	nrc := numberValue(body["activation_price"])
	if nrc == 0 {
		nrc = numberValue(body["activation_fee"])
	}
	if mrc <= 0 && nrc <= 0 {
		return errors.New("Inserisci MRC o NRC")
	}
	renewDetail, _ := body["renew_detail"].(map[string]any)
	if numberValue(renewDetail["initial_subscription_months"]) <= 0 {
		return errors.New("Inserisci la durata iniziale")
	}
	if numberValue(paymentDetail["month_recursion"]) <= 0 {
		return errors.New("Seleziona la ricorrenza")
	}
	if boolValue(renewDetail["automatic_renew"]) && strings.TrimSpace(stringValue(renewDetail["cancellation_advice"])) == "" {
		return errors.New("Inserisci il preavviso di disdetta")
	}
	if boolValue(renewDetail["automatic_renew"]) && numberValue(renewDetail["cancellation_advice"]) <= 0 {
		return errors.New("Inserisci il preavviso di disdetta")
	}
	return nil
}

func buildRowCreateBody(body map[string]any, email string) map[string]any {
	rowType := strings.TrimSpace(stringValue(body["type"]))
	paymentDetail, _ := body["payment_detail"].(map[string]any)

	out := map[string]any{
		"type":                rowType,
		"description":         strings.TrimSpace(stringValue(body["description"])),
		"requester_email":     email,
		"product_code":        strings.TrimSpace(stringValue(body["product_code"])),
		"product_description": strings.TrimSpace(stringValue(body["product_description"])),
		"qty":                 int64Value(body["qty"]),
		"payment_detail": map[string]any{
			"start_at": strings.TrimSpace(stringValue(paymentDetail["start_at"])),
		},
	}

	outPaymentDetail := out["payment_detail"].(map[string]any)
	if startAtDate := strings.TrimSpace(stringValue(paymentDetail["start_at_date"])); startAtDate != "" {
		outPaymentDetail["start_at_date"] = startAtDate
	}

	if rowType == "good" {
		out["price"] = decimalString(body["price"])
		return out
	}

	mrc := body["monthly_fee"]
	if numberValue(mrc) == 0 {
		mrc = body["montly_fee"]
	}
	nrc := body["activation_price"]
	if numberValue(nrc) == 0 {
		nrc = body["activation_fee"]
	}
	renewDetail, _ := body["renew_detail"].(map[string]any)

	out["price"] = decimalString(mrc)
	out["activation_price"] = decimalString(nrc)
	outPaymentDetail["is_recurrent"] = numberValue(mrc) > 0
	outPaymentDetail["month_recursion"] = int64Value(paymentDetail["month_recursion"])

	autoRenew := boolValue(renewDetail["automatic_renew"])
	outRenewDetail := map[string]any{
		"initial_subscription_months": int64Value(renewDetail["initial_subscription_months"]),
		"automatic_renew":             autoRenew,
	}
	if autoRenew {
		outRenewDetail["cancellation_advice"] = int64Value(renewDetail["cancellation_advice"])
	}
	out["renew_detail"] = outRenewDetail
	return out
}

func decimalString(value any) string {
	return strconv.FormatFloat(numberValue(value), 'f', -1, 64)
}

func (h *Handler) handleDeleteRow(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	if !isRequester(po, email) || po.State != "DRAFT" {
		httputil.Error(w, http.StatusForbidden, "Le righe possono essere modificate solo dal richiedente in bozza")
		return
	}
	h.forwardArak(w, r, http.MethodDelete, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/row/"+url.PathEscape(r.PathValue("rowId")), "", nil, requesterHeaders(email))
}

func (h *Handler) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	if po.State != "DRAFT" && po.State != "PENDING_VERIFICATION" {
		httputil.Error(w, http.StatusForbidden, "Allegato non caricabile nello stato attuale")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Il file supera la dimensione massima o non puo essere letto")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil || header == nil || header.Size == 0 {
		httputil.Error(w, http.StatusBadRequest, "Seleziona un file da caricare")
		return
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeQuotes(header.Filename)))
	if contentType := multipartContentType(header); contentType != "" {
		partHeader.Set("Content-Type", contentType)
	}
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		httputil.InternalError(w, r, err, "rda attachment part create failed")
		return
	}
	if _, err := io.Copy(part, file); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Il file non puo essere letto")
		return
	}
	attachmentType := "transport_document"
	if po.State == "DRAFT" {
		attachmentType = "quote"
	}
	if err := writer.WriteField("attachment_type", attachmentType); err != nil {
		httputil.InternalError(w, r, err, "rda attachment field write failed")
		return
	}
	if err := writer.Close(); err != nil {
		httputil.InternalError(w, r, err, "rda attachment body close failed")
		return
	}
	headers := mergeHeaders(requesterHeaders(email), http.Header{"Content-Type": []string{writer.FormDataContentType()}})
	h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/attachment", "", bytes.NewReader(buf.Bytes()), headers)
}

func (h *Handler) handleDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	path := arakRDARoot + "/po/" + url.PathEscape(r.PathValue("id")) + "/attachment/" + url.PathEscape(r.PathValue("aid")) + "/download"
	h.forwardArak(w, r, http.MethodGet, path, "", nil, requesterHeaders(email))
}

func (h *Handler) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	if !isRequester(po, email) || po.State != "DRAFT" {
		httputil.Error(w, http.StatusForbidden, "Gli allegati possono essere eliminati solo dal richiedente in bozza")
		return
	}
	path := arakRDARoot + "/po/" + url.PathEscape(r.PathValue("id")) + "/attachment/" + url.PathEscape(r.PathValue("aid"))
	h.forwardArak(w, r, http.MethodDelete, path, "", nil, requesterHeaders(email))
}

func (h *Handler) handleComments(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	h.forwardArak(w, r, http.MethodGet, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/comment", queryWithDefaults(r), nil, requesterHeaders(email))
}

func (h *Handler) handlePostComment(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	var body struct {
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Comment) == "" {
		httputil.Error(w, http.StatusBadRequest, "Inserisci un commento")
		return
	}
	encoded, err := encodeJSONBody(map[string]string{"comment": strings.TrimSpace(body.Comment)})
	if err != nil {
		httputil.InternalError(w, r, err, "rda comment body encode failed")
		return
	}
	h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/comment", "", encoded, mergeHeaders(requesterHeaders(email), jsonHeaders()))
}

func (h *Handler) handleSubmitPO(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	if !isRequester(po, email) || po.State != "DRAFT" {
		httputil.Error(w, http.StatusForbidden, "La bozza puo essere inviata solo dal richiedente")
		return
	}
	if len(po.Rows) == 0 {
		httputil.Error(w, http.StatusBadRequest, "Aggiungi almeno una riga PO")
		return
	}
	if parseTotalPrice(po.TotalPrice) >= 3000 && len(po.Attachments) < 2 {
		httputil.Error(w, http.StatusBadRequest, "Per importi superiori a 3.000 euro sono necessari almeno 2 preventivi")
		return
	}
	h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/submit", "", nil, requesterHeaders(email))
}

func (h *Handler) handleApprovePO(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	if po.State != "PENDING_APPROVAL" || !h.hasAnyRole(r, applaunch.RDAApproverL1L2Roles()...) || !isApprover(po, email) {
		httputil.Error(w, http.StatusForbidden, "Operazione riservata agli approvatori assegnati")
		return
	}
	h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/approve", "", nil, requesterHeaders(email))
}

func (h *Handler) handleRejectPO(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	switch po.State {
	case "PENDING_APPROVAL":
		if !h.hasAnyRole(r, applaunch.RDAApproverL1L2Roles()...) || !isApprover(po, email) {
			httputil.Error(w, http.StatusForbidden, "Operazione riservata agli approvatori assegnati")
			return
		}
	case "PENDING_APPROVAL_PAYMENT_METHOD":
		if !h.hasAnyRole(r, applaunch.RDAApproverAFCRoles()...) {
			httputil.Error(w, http.StatusForbidden, "Operazione riservata agli utenti abilitati")
			return
		}
	case "PENDING_APPROVAL_NO_LEASING":
		if !h.hasAnyRole(r, applaunch.RDAApproverNoLeasingRoles()...) {
			httputil.Error(w, http.StatusForbidden, "Operazione riservata agli utenti abilitati")
			return
		}
	default:
		httputil.Error(w, http.StatusConflict, "Azione non disponibile nello stato attuale")
		return
	}
	h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/reject", "", nil, requesterHeaders(email))
}

func (h *Handler) handlePatchPaymentMethod(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, po, ok := h.loadPOForWrite(w, r)
	if !ok {
		return
	}
	if !isRequester(po, email) || po.State != "PENDING_APPROVAL_PAYMENT_METHOD" {
		httputil.Error(w, http.StatusForbidden, "Il metodo di pagamento puo essere aggiornato solo dal richiedente")
		return
	}
	var body struct {
		PaymentMethod string `json:"payment_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.PaymentMethod) == "" {
		httputil.Error(w, http.StatusBadRequest, "Seleziona un metodo di pagamento")
		return
	}
	encoded, err := encodeJSONBody(map[string]string{"payment_method": strings.TrimSpace(body.PaymentMethod)})
	if err != nil {
		httputil.InternalError(w, r, err, "rda payment method body encode failed")
		return
	}
	h.forwardArak(w, r, http.MethodPatch, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+"/payment-method", "", encoded, mergeHeaders(requesterHeaders(email), jsonHeaders()))
}

func (h *Handler) handleRoleTransition(roles []string, suffix string, operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.requireArak(w) {
			return
		}
		email, ok := currentEmail(r)
		if !ok {
			httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
			return
		}
		if !h.hasAnyRole(r, roles...) {
			httputil.Error(w, http.StatusForbidden, "Operazione riservata agli utenti abilitati")
			return
		}
		h.requestLogger(r, operation)
		h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+suffix, "", nil, requesterHeaders(email))
	}
}

func (h *Handler) handleStateTransition(requiredState, suffix, operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.requireArak(w) {
			return
		}
		email, po, ok := h.loadPOForWrite(w, r)
		if !ok {
			return
		}
		if po.State != requiredState {
			httputil.Error(w, http.StatusConflict, "Azione non disponibile nello stato attuale")
			return
		}
		h.requestLogger(r, operation)
		h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+suffix, "", nil, requesterHeaders(email))
	}
}

func (h *Handler) handleBudgetIncrement(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.requireArak(w) {
			return
		}
		email, ok := currentEmail(r)
		if !ok {
			httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
			return
		}
		if !h.hasAnyRole(r, applaunch.RDAApproverExtraBudgetRoles()...) {
			httputil.Error(w, http.StatusForbidden, "Operazione riservata agli utenti abilitati")
			return
		}
		suffix := "/reject-budget-increment"
		if approve {
			suffix = "/approve-budget-increment"
		}
		h.forwardArak(w, r, http.MethodPost, arakRDARoot+"/po/"+url.PathEscape(r.PathValue("id"))+suffix, "", r.Body, mergeHeaders(requesterHeaders(email), jsonHeaders()))
	}
}

func parseTotalPrice(value string) float64 {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9':
			return r
		case r == ',' || r == '.' || r == '-':
			return r
		default:
			return -1
		}
	}, value)
	if strings.Count(clean, ",") > 0 && strings.Count(clean, ".") > 0 {
		clean = strings.ReplaceAll(clean, ".", "")
		clean = strings.ReplaceAll(clean, ",", ".")
	} else {
		clean = strings.ReplaceAll(clean, ",", ".")
	}
	parsed, err := strconv.ParseFloat(clean, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0
	}
	return parsed
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

func numberValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v), ",", "."), 64)
		return n
	default:
		return 0
	}
}

func int64Value(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, _ := strconv.ParseBool(v)
		return parsed
	default:
		return false
	}
}

func escapeQuotes(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}
