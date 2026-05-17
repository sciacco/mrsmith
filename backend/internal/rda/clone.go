package rda

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleClonePO(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	email, ok := currentEmail(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "Accesso richiesto")
		return
	}
	sourceID, err := positivePathID(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return
	}

	var req clonePORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return
	}
	if err := validateClonePO(req); err != nil {
		httputil.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	source, err := h.fetchPODetail(r, email, sourceID)
	if err != nil {
		h.handleFetchPOError(w, r, err)
		return
	}
	if source.Provider.ID <= 0 {
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Richiesta sorgente non duplicabile in questo momento",
			"code":  codeUpstreamUnavailable,
		})
		return
	}
	if !h.requireArakDB(w) {
		return
	}

	provider := mergeCloneProvider(source.Provider, h.fetchProviderForCreate(r, source.Provider.ID))
	createBody, warnings, err := h.buildCloneCreateBody(r.Context(), req, source, provider)
	if err != nil {
		switch {
		case errors.Is(err, errPaymentMethodNotAllowed):
			httputil.Error(w, http.StatusBadRequest, "Seleziona un metodo di pagamento valido")
		default:
			httputil.Error(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	encoded, err := encodeJSONBody(createBody)
	if err != nil {
		httputil.InternalError(w, r, err, "rda clone create body encode failed")
		return
	}
	createResponse, err := h.createPOUpstream(email, encoded)
	if err != nil {
		h.requestLogger(r, "rda_clone_po", "source_po_id", sourceID, "upstream_path", arakRDARoot+"/po").Error("upstream create failed", "error", err)
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Servizio RDA temporaneamente non disponibile",
			"code":  codeUpstreamUnavailable,
		})
		return
	}
	if createResponse.status == http.StatusUnauthorized || createResponse.status == http.StatusForbidden {
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Autorizzazione verso il servizio RDA non riuscita",
			"code":  codeUpstreamAuthFailed,
		})
		return
	}
	if createResponse.status < 200 || createResponse.status >= 300 {
		h.writeUpstreamBodyRejected(w, r, createResponse, arakRDARoot+"/po")
		return
	}

	newPOID, err := poIDInt64FromResponse(createResponse.body)
	if err != nil {
		h.requestLogger(r, "rda_clone_po", "source_po_id", sourceID).Error("created clone response missing id", "error", err, "body", string(createResponse.body))
		httputil.JSON(w, http.StatusBadGateway, map[string]string{
			"error": "Bozza duplicata non riapribile in questo momento",
			"code":  codeUpstreamUnavailable,
		})
		return
	}
	newPOIDString := strconv.FormatInt(newPOID, 10)

	copiedRows, skippedRows := 0, 0
	if req.includeRows() {
		rowWarnings := []string{}
		copiedRows, skippedRows, rowWarnings = h.copyPORows(r, email, newPOIDString, source.Rows)
		warnings = append(warnings, rowWarnings...)
	}

	copiedRecipients := 0
	if req.includeRecipients() {
		recipientWarnings := []string{}
		copiedRecipients, recipientWarnings = h.copyPORecipients(r, email, newPOIDString, source.Recipients)
		warnings = append(warnings, recipientWarnings...)
	}

	h.requestLogger(
		r,
		"rda_clone_po",
		"source_po_id", sourceID,
		"new_po_id", newPOIDString,
		"copied_rows", copiedRows,
		"skipped_rows", skippedRows,
		"copied_recipients", copiedRecipients,
	).Info("po cloned")

	httputil.JSON(w, http.StatusOK, clonePOResponse{
		ID:               newPOID,
		CopiedRows:       copiedRows,
		SkippedRows:      skippedRows,
		CopiedRecipients: copiedRecipients,
		Warnings:         warnings,
	})
}

func positivePathID(raw string) (string, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return "", errors.New("invalid id")
	}
	return strconv.FormatInt(id, 10), nil
}

func validateClonePO(req clonePORequest) error {
	if req.BudgetID <= 0 {
		return errors.New("Seleziona un budget")
	}
	hasCostCenter := strings.TrimSpace(req.CostCenter) != ""
	hasBudgetUser := req.BudgetUserID > 0
	if hasCostCenter == hasBudgetUser {
		return errors.New("Il budget deve indicare un solo centro di costo o utente")
	}
	return nil
}

func mergeCloneProvider(source providerDetail, fetched providerDetail) providerDetail {
	if fetched.ID <= 0 {
		fetched.ID = source.ID
	}
	if strings.TrimSpace(fetched.Language) == "" {
		fetched.Language = source.Language
	}
	if strings.TrimSpace(fetched.VATNumber) == "" {
		fetched.VATNumber = source.VATNumber
	}
	if strings.TrimSpace(fetched.PostalCode) == "" {
		fetched.PostalCode = source.PostalCode
	}
	if strings.TrimSpace(fetched.CAP) == "" {
		fetched.CAP = source.CAP
	}
	if len(bytes.TrimSpace(fetched.DefaultPaymentMethod)) == 0 {
		fetched.DefaultPaymentMethod = source.DefaultPaymentMethod
	}
	return fetched
}

func (h *Handler) buildCloneCreateBody(ctx context.Context, req clonePORequest, source poDetail, provider providerDetail) (map[string]any, []string, error) {
	warnings := []string{}
	currency, err := createPOCurrency(source.Currency)
	if err != nil {
		currency = defaultPOCurrency
		warnings = append(warnings, "Valuta sorgente non valida: la nuova bozza usa EUR.")
	}

	paymentMethod, paymentWarnings, err := h.clonePaymentMethod(ctx, source, provider)
	if err != nil {
		return nil, warnings, err
	}
	warnings = append(warnings, paymentWarnings...)

	createReq := createPORequest{
		Type:          clonePOType(source.Type),
		BudgetID:      req.BudgetID,
		CostCenter:    strings.TrimSpace(req.CostCenter),
		BudgetUserID:  req.BudgetUserID,
		ProviderID:    source.Provider.ID,
		PaymentMethod: paymentMethod,
		Currency:      currency,
		Project:       strings.TrimSpace(source.Project),
		Object:        strings.TrimSpace(source.Object),
		Description:   strings.TrimSpace(source.Description),
		Note:          strings.TrimSpace(source.Note),
	}
	if err := validateCreatePO(createReq); err != nil {
		return nil, warnings, err
	}

	body := map[string]any{
		"type":                createReq.Type,
		"budget_id":           createReq.BudgetID,
		"provider_id":         createReq.ProviderID,
		"project":             createReq.Project,
		"object":              createReq.Object,
		"reference_warehouse": "MILANO",
		"currency":            currency,
		"language":            providerLanguage(provider),
		"payment_method":      paymentMethod,
		"recipient_ids":       []int64{},
	}
	if createReq.CostCenter != "" {
		body["cost_center"] = createReq.CostCenter
	} else {
		body["budget_user_id"] = createReq.BudgetUserID
	}
	if createReq.Description != "" {
		body["description"] = createReq.Description
	}
	if createReq.Note != "" {
		body["note"] = createReq.Note
	}
	if req.IncludeOfferFields {
		if code := strings.TrimSpace(source.ProviderOfferCode); code != "" {
			body["provider_offer_code"] = code
		}
		if date := cloneDate(source.ProviderOfferDate); date != "" {
			body["provider_offer_date"] = date
		}
	}
	if cap := providerCAP(provider); cap != "" {
		body["cap"] = cap
	}
	if vat := strings.TrimSpace(provider.VATNumber); vat != "" {
		body["vat"] = vat
	}
	return body, warnings, nil
}

func clonePOType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ECOMMERCE":
		return "ECOMMERCE"
	default:
		return "STANDARD"
	}
}

func cloneDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if index := strings.Index(value, "T"); index > 0 {
		return value[:index]
	}
	if len(value) > 10 {
		return value[:10]
	}
	return value
}

func (h *Handler) clonePaymentMethod(ctx context.Context, source poDetail, provider providerDetail) (string, []string, error) {
	requested := poPaymentMethodCode(source)
	paymentMethod, providerDefault, err := h.resolvePaymentMethod(ctx, requested, provider)
	if err != nil {
		return "", nil, err
	}
	if err := h.validateEffectivePaymentMethod(ctx, paymentMethod, providerDefault); err != nil {
		if !errors.Is(err, errPaymentMethodNotAllowed) {
			return "", nil, err
		}
		fallback, fallbackProviderDefault, fallbackErr := h.resolvePaymentMethod(ctx, "", provider)
		if fallbackErr != nil {
			return "", nil, fallbackErr
		}
		if fallback != "" && h.validateEffectivePaymentMethod(ctx, fallback, fallbackProviderDefault) == nil {
			return fallback, []string{"Metodo di pagamento sorgente non piu valido: e stato impostato il metodo predefinito disponibile."}, nil
		}
		return "", nil, errPaymentMethodNotAllowed
	}
	return paymentMethod, nil, nil
}

func (h *Handler) createPOUpstream(email string, body io.Reader) (upstreamBodyResponse, error) {
	resp, err := h.arak.DoWithHeaders(http.MethodPost, arakRDARoot+"/po", "", body, mergeHeaders(requesterHeaders(email), jsonHeaders()))
	if err != nil {
		return upstreamBodyResponse{}, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return upstreamBodyResponse{}, err
	}
	return upstreamBodyResponse{status: resp.StatusCode, header: resp.Header.Clone(), body: responseBody}, nil
}

func (h *Handler) writeUpstreamBodyRejected(w http.ResponseWriter, r *http.Request, response upstreamBodyResponse, path string) {
	resp := &http.Response{
		StatusCode: response.status,
		Header:     response.header,
		Body:       io.NopCloser(bytes.NewReader(response.body)),
	}
	h.writeUpstreamRejected(w, r, resp, path, "")
}

func poIDInt64FromResponse(body []byte) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return 0, err
	}
	if object, ok := value.(map[string]any); ok {
		return int64FromAny(object["id"])
	}
	return int64FromAny(value)
}

func int64FromAny(value any) (int64, error) {
	switch typed := value.(type) {
	case json.Number:
		id, err := typed.Int64()
		if err != nil || id <= 0 {
			return 0, errors.New("invalid id")
		}
		return id, nil
	case string:
		id, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil || id <= 0 {
			return 0, errors.New("invalid id")
		}
		return id, nil
	case float64:
		id := int64(typed)
		if id <= 0 {
			return 0, errors.New("invalid id")
		}
		return id, nil
	case int64:
		if typed <= 0 {
			return 0, errors.New("invalid id")
		}
		return typed, nil
	case int:
		if typed <= 0 {
			return 0, errors.New("invalid id")
		}
		return int64(typed), nil
	default:
		return 0, errors.New("invalid id")
	}
}

func (h *Handler) copyPORows(r *http.Request, email string, newPOID string, rows []json.RawMessage) (int, int, []string) {
	copied := 0
	skipped := 0
	for index, raw := range rows {
		payload, err := cloneRowPayloadFromSource(raw)
		if err != nil {
			skipped++
			h.requestLogger(r, "rda_clone_rows", "new_po_id", newPOID, "row_index", index).Warn("source row not copyable", "error", err)
			continue
		}
		upstreamBody := buildRowCreateBody(payload, email)
		encoded, err := encodeJSONBody(upstreamBody)
		if err != nil {
			skipped++
			h.requestLogger(r, "rda_clone_rows", "new_po_id", newPOID, "row_index", index).Warn("row body encode failed", "error", err)
			continue
		}
		response, err := h.createRowUpstream(email, newPOID, encoded)
		if err != nil {
			skipped++
			h.requestLogger(r, "rda_clone_rows", "new_po_id", newPOID, "row_index", index).Warn("upstream row copy failed", "error", err)
			continue
		}
		if response.status < 200 || response.status >= 300 {
			skipped++
			h.requestLogger(r, "rda_clone_rows", "new_po_id", newPOID, "row_index", index, "upstream_status", response.status).Warn("upstream row copy rejected", "body", string(response.body))
			continue
		}
		copied++
	}
	if skipped == 0 {
		return copied, skipped, nil
	}
	return copied, skipped, []string{fmt.Sprintf("%d righe non sono state copiate: verifica la nuova bozza.", skipped)}
}

func cloneRowPayloadFromSource(raw json.RawMessage) (map[string]any, error) {
	row, err := decodeRawMap(raw)
	if err != nil {
		return nil, err
	}
	rowType := strings.ToLower(strings.TrimSpace(stringValue(row["type"])))
	if rowType != "good" && rowType != "service" {
		return nil, errors.New("invalid row type")
	}
	qty := numberValue(row["qty"])
	if qty <= 0 {
		return nil, errors.New("invalid row quantity")
	}
	productCode := strings.TrimSpace(stringValue(row["product_code"]))
	productDescription := strings.TrimSpace(firstString(row["product_description"], row["description"], row["product_code"]))
	if productCode == "" || productDescription == "" {
		return nil, errors.New("missing row product")
	}

	paymentDetail := mapValue(row["payment_detail"])
	startAt := normalizeCloneStartAt(firstString(paymentDetail["start_at"], paymentDetail["start_pay_at_activation_date"]), rowType)
	startAtDate := strings.TrimSpace(stringValue(paymentDetail["start_at_date"]))
	if startAt == "specific_date" && startAtDate == "" {
		return nil, errors.New("missing row start date")
	}

	payloadPaymentDetail := map[string]any{"start_at": startAt}
	if startAtDate != "" {
		payloadPaymentDetail["start_at_date"] = startAtDate
	}
	payload := map[string]any{
		"type":                rowType,
		"description":         strings.TrimSpace(firstString(row["description"], row["product_description"])),
		"qty":                 qty,
		"product_code":        productCode,
		"product_description": productDescription,
		"payment_detail":      payloadPaymentDetail,
	}

	if rowType == "good" {
		price := firstPositiveMoney(row["price"])
		if price <= 0 {
			if total := firstPositiveMoney(row["total_price"], row["total"]); total > 0 {
				price = total / qty
			}
		}
		if price <= 0 {
			return nil, errors.New("missing good row price")
		}
		payload["price"] = price
		if err := validateRow(payload); err != nil {
			return nil, err
		}
		return payload, nil
	}

	mrc := firstPositiveMoney(row["monthly_fee"], row["montly_fee"], row["mrc"], row["price"])
	nrc := firstPositiveMoney(row["activation_price"], row["activation_fee"], row["nrc"])
	if mrc <= 0 && nrc <= 0 {
		return nil, errors.New("missing service row economics")
	}
	recurrence := firstPositiveMoney(paymentDetail["month_recursion"])
	if recurrence <= 0 {
		return nil, errors.New("missing service row recurrence")
	}
	renewDetail := mapValue(row["renew_detail"])
	duration := firstPositiveMoney(renewDetail["initial_subscription_months"])
	if duration <= 0 {
		return nil, errors.New("missing service row duration")
	}
	autoRenew := boolValue(renewDetail["automatic_renew"])
	payload["monthly_fee"] = mrc
	payload["activation_price"] = nrc
	payloadPaymentDetail["month_recursion"] = int64(recurrence)
	payloadRenewDetail := map[string]any{
		"initial_subscription_months": int64(duration),
		"automatic_renew":             autoRenew,
	}
	if autoRenew {
		cancellationAdvice := int64Value(renewDetail["cancellation_advice"])
		if cancellationAdvice <= 0 {
			return nil, errors.New("missing service row cancellation advice")
		}
		payloadRenewDetail["cancellation_advice"] = cancellationAdvice
	}
	payload["renew_detail"] = payloadRenewDetail
	if err := validateRow(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func decodeRawMap(raw json.RawMessage) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var row map[string]any
	if err := decoder.Decode(&row); err != nil {
		return nil, err
	}
	return row, nil
}

func mapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func firstString(values ...any) string {
	for _, value := range values {
		if text := strings.TrimSpace(stringValue(value)); text != "" {
			return text
		}
	}
	return ""
}

func normalizeCloneStartAt(value string, rowType string) string {
	value = strings.TrimSpace(value)
	if rowType == "good" {
		if value == "activation_date" || value == "advance_payment" || value == "specific_date" {
			return value
		}
		return "activation_date"
	}
	if value == "activation_date" || value == "specific_date" {
		return value
	}
	return "activation_date"
}

func (h *Handler) copyPORecipients(r *http.Request, email string, newPOID string, recipients []json.RawMessage) (int, []string) {
	ids := recipientIDsFromRaw(recipients)
	if len(ids) == 0 {
		return 0, nil
	}
	encoded, err := encodeJSONBody(map[string]any{"recipient_ids": ids})
	if err != nil {
		h.requestLogger(r, "rda_clone_recipients", "new_po_id", newPOID).Warn("recipient body encode failed", "error", err)
		return 0, []string{"I destinatari non sono stati copiati: verifica la nuova bozza."}
	}
	response, err := h.patchPORecipientsUpstream(email, newPOID, encoded)
	if err != nil {
		h.requestLogger(r, "rda_clone_recipients", "new_po_id", newPOID).Warn("upstream recipient copy failed", "error", err)
		return 0, []string{"I destinatari non sono stati copiati: verifica la nuova bozza."}
	}
	if response.status < 200 || response.status >= 300 {
		h.requestLogger(r, "rda_clone_recipients", "new_po_id", newPOID, "upstream_status", response.status).Warn("upstream recipient copy rejected", "body", string(response.body))
		return 0, []string{"Alcuni destinatari non sono piu disponibili per il fornitore."}
	}
	return len(ids), nil
}

func recipientIDsFromRaw(recipients []json.RawMessage) []int64 {
	seen := map[int64]struct{}{}
	ids := make([]int64, 0, len(recipients))
	for _, raw := range recipients {
		row, err := decodeRawMap(raw)
		if err != nil {
			continue
		}
		id := int64Value(row["id"])
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func (h *Handler) patchPORecipientsUpstream(email string, poID string, body io.Reader) (upstreamBodyResponse, error) {
	path := arakRDARoot + "/po/" + url.PathEscape(poID) + "/recipients"
	resp, err := h.arak.DoWithHeaders(http.MethodPatch, path, "", body, mergeHeaders(requesterHeaders(email), jsonHeaders()))
	if err != nil {
		return upstreamBodyResponse{}, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return upstreamBodyResponse{}, err
	}
	return upstreamBodyResponse{status: resp.StatusCode, header: resp.Header.Clone(), body: responseBody}, nil
}
