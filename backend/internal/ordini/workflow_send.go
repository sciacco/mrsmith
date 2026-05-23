package ordini

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const maxSendToERPPDFBytes = 32 << 20

func (h *Handler) handleSendToERP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !h.requireCustomerRelations(w, r) || !h.requireVodka(w) || !h.requireGateway(w) {
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
			h.logFailure(r, slog.LevelWarn, "send to ERP row failed", "send_to_erp", start, gatewayFailureAttrs("/orders/v1/erp", err, "order_id", orderID, "row_id", row.ID)...)
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
		h.logFailure(r, slog.LevelWarn, "arxivar upload failed after send state transition", "send_to_erp_arxivar", start, gatewayFailureAttrs("/orders/v1/send-to-arxivar", err, "order_id", orderID)...)
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
	if errors.Is(err, errGatewayPreconditionMissing) {
		return "precondition_missing"
	}
	return "gateway_error"
}
