package ordini

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const maxSendToERPPDFBytes = 32 << 20

func (h *Handler) handleSendToERP(w http.ResponseWriter, r *http.Request) {
	if !h.requireVodka(w) || !h.requireGateway(w) || !h.requireCustomerRelations(w, r) {
		return
	}
	orderID, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	order, err := h.getOrderWithoutOrigin(r, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "order_not_found")
			return
		}
		h.dbFailure(w, r, "send_to_erp_load_order", err, "order_id", orderID)
		return
	}
	if !requireState(w, stateOf(order), OrderStateBozza) {
		return
	}
	if !order.CdlanDataconferma.Valid {
		httputil.Error(w, http.StatusUnprocessableEntity, "missing_confirmation_date")
		return
	}
	if strings.TrimSpace(ptrStringValue(order.CdlanCliente)) == "" && order.CdlanClienteID == nil {
		httputil.Error(w, http.StatusUnprocessableEntity, "missing_customer")
		return
	}
	pdf, ok := h.readSendPDF(w, r)
	if !ok {
		return
	}
	rows, err := h.listOrderRows(r, orderID)
	if err != nil {
		h.dbFailure(w, r, "send_to_erp_list_rows", err, "order_id", orderID)
		return
	}
	if len(rows) == 0 {
		httputil.Error(w, http.StatusUnprocessableEntity, "precondition_missing")
		return
	}

	response := SendToERPResponse{Rows: make([]SendToERPRowOutcome, 0, len(rows))}
	failed := false
	for _, row := range rows {
		outcome := SendToERPRowOutcome{RowID: row.ID, CdlanSystemODVRow: row.CdlanSystemODVRow, Status: "ok"}
		if err := h.gatewaySendToERP(order, row); err != nil {
			failed = true
			outcome.Status = "error"
			code := sanitizeGatewayError(err)
			outcome.Error = &code
			h.logger.Warn("send to ERP row failed", "operation", "send_to_erp", "order_id", orderID, "row_id", row.ID, "error", err)
		}
		response.Rows = append(response.Rows, outcome)
	}
	if failed {
		httputil.JSON(w, http.StatusOK, response)
		return
	}

	res, err := h.deps.Vodka.ExecContext(r.Context(), `
UPDATE orders
SET cdlan_stato = 'INVIATO', cdlan_evaso = 1
WHERE id = ? AND cdlan_stato = 'BOZZA'`, orderID)
	if err != nil {
		h.dbFailure(w, r, "send_to_erp_update_state", err, "order_id", orderID)
		return
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		httputil.Error(w, http.StatusConflict, "wrong_state")
		return
	}
	response.StateTransitioned = true

	filename := orderCodeForFilename(order) + "_firmato.pdf"
	if err := h.gatewayUploadToArxivar(order, pdf, filename); err != nil {
		response.Warning = "arxivar_upload_failed"
		h.logger.Warn("arxivar upload failed after send state transition", "operation", "send_to_erp_arxivar", "order_id", orderID, "error", err)
		httputil.JSON(w, http.StatusOK, response)
		return
	}
	response.ArxivarUploaded = true
	httputil.JSON(w, http.StatusOK, response)
}

func (h *Handler) readSendPDF(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSendToERPPDFBytes)
	if err := r.ParseMultipartForm(maxSendToERPPDFBytes); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_pdf")
		return nil, false
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httputil.Error(w, http.StatusUnprocessableEntity, "missing_pdf")
		return nil, false
	}
	defer file.Close()
	pdf, err := io.ReadAll(io.LimitReader(file, maxSendToERPPDFBytes))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_pdf")
		return nil, false
	}
	if !bytes.HasPrefix(bytes.TrimSpace(pdf), []byte("%PDF")) {
		httputil.Error(w, http.StatusBadRequest, "invalid_pdf")
		return nil, false
	}
	return pdf, true
}

func sanitizeGatewayError(err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(err.Error(), "precondition_missing") {
		return "precondition_missing"
	}
	return "gateway_error"
}
