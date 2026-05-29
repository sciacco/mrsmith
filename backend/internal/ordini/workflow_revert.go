package ordini

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/hubspot"
)

const alyanteOrderRowsCountQuery = `
SELECT COUNT(1)
FROM Tsmi_Ordini_Esteso
WHERE ANNO_DOCUMENTO = @p2
  AND (
    NUM_DOC_GAMMA = @p1
    OR LTRIM(RTRIM(ISNULL(NUM_DOCUMENTO, ''))) = @p3
  )`

const alyanteOrderRowsCountByDocumentQuery = `
SELECT COUNT(1)
FROM Tsmi_Ordini_Esteso
WHERE ANNO_DOCUMENTO = @p1
  AND LTRIM(RTRIM(ISNULL(NUM_DOCUMENTO, ''))) = @p2`

func (h *Handler) handleRevertConversion(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if !h.requireCustomerRelations(w, r) || !h.requireVodka(w) || !h.requireMistra(w) || !h.requireAlyante(w) {
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
		h.dbFailure(w, r, "revert_conversion_load_order", err, "order_id", orderID)
		return
	}
	if !requireState(w, stateOf(order), OrderStateBozza) {
		return
	}
	if strings.TrimSpace(ptrStringValue(order.ArxDocNumber)) != "" {
		httputil.Error(w, http.StatusConflict, "order_has_signed_pdf")
		return
	}

	bridge, err := h.loadConvertedQuoteBridge(r, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusConflict, "not_converted_from_quote")
			return
		}
		h.dbFailure(w, r, "revert_conversion_load_bridge", err, "order_id", orderID)
		return
	}
	quoteID := bridge.QuoteID

	erpRows, err := h.countAlyanteOrderRows(r, order)
	if err != nil {
		if errors.Is(err, errGatewayPreconditionMissing) {
			httputil.Error(w, http.StatusUnprocessableEntity, "precondition_missing")
			return
		}
		h.dbFailure(w, r, "revert_conversion_alyante_check", err, "order_id", orderID)
		return
	}
	if erpRows > 0 {
		httputil.Error(w, http.StatusConflict, "order_has_erp_rows")
		return
	}

	deletedRows, err := h.deleteVodkaConvertedOrder(r, orderID)
	if err != nil {
		if errors.Is(err, errOrderStateChanged) {
			httputil.Error(w, http.StatusConflict, "wrong_state")
			return
		}
		h.dbFailure(w, r, "revert_conversion_delete_vodka", err, "order_id", orderID)
		return
	}

	hubSpotCleanup := h.cleanupConvertedOrderHubSpot(r, bridge, orderID, start)

	bridgeDeleted := true
	warnings := append([]string(nil), hubSpotCleanup.Warnings...)
	if err := h.deleteLegacyOrderBridge(r, quoteID, orderID); err != nil {
		bridgeDeleted = false
		warnings = appendWarning(warnings, "bridge_delete_failed")
		h.logFailure(r, slog.LevelWarn, "legacy bridge cleanup failed after conversion revert", "revert_conversion_delete_bridge", start, "order_id", orderID, "quote_id", quoteID, "error", err)
	}

	response := RevertConversionResponse{
		Reverted:      true,
		OrderID:       orderID,
		QuoteID:       quoteID,
		OrderCode:     order.CodiceOrdine,
		DeletedRows:   deletedRows,
		BridgeDeleted: bridgeDeleted,
		HubSpot:       hubSpotCleanup,
		Warnings:      warnings,
		Warning:       firstWarning(warnings),
	}
	httputil.JSON(w, http.StatusOK, response)
}

type convertedOrderBridge struct {
	QuoteID int64
	JData   sql.NullString
}

type convertedOrderHubSpotMetadata struct {
	DealID     string `json:"deal_id,omitempty"`
	DealURL    string `json:"deal_url,omitempty"`
	FileID     string `json:"file_id,omitempty"`
	NoteID     int64  `json:"note_id,omitempty"`
	Filename   string `json:"filename,omitempty"`
	FolderPath string `json:"folder_path,omitempty"`
}

func (b *convertedOrderBridge) hubSpotMetadata() *convertedOrderHubSpotMetadata {
	if b == nil || !b.JData.Valid || strings.TrimSpace(b.JData.String) == "" {
		return nil
	}
	var payload struct {
		HubSpot *convertedOrderHubSpotMetadata `json:"hubspot"`
	}
	if err := json.Unmarshal([]byte(b.JData.String), &payload); err != nil || payload.HubSpot == nil {
		return nil
	}
	payload.HubSpot.DealID = strings.TrimSpace(payload.HubSpot.DealID)
	payload.HubSpot.DealURL = strings.TrimSpace(payload.HubSpot.DealURL)
	payload.HubSpot.FileID = strings.TrimSpace(payload.HubSpot.FileID)
	payload.HubSpot.Filename = strings.TrimSpace(payload.HubSpot.Filename)
	payload.HubSpot.FolderPath = strings.TrimSpace(payload.HubSpot.FolderPath)
	return payload.HubSpot
}

func (h *Handler) loadConvertedQuoteBridge(r *http.Request, orderID int64) (*convertedOrderBridge, error) {
	var bridge convertedOrderBridge
	err := h.deps.Mistra.QueryRowContext(r.Context(), `
	SELECT quote_id, jdata::text
	FROM orders.legacy_orders
	WHERE vodka_id = $1
	LIMIT 1`, orderID).Scan(&bridge.QuoteID, &bridge.JData)
	return &bridge, err
}

func (h *Handler) cleanupConvertedOrderHubSpot(r *http.Request, bridge *convertedOrderBridge, orderID int64, start time.Time) RevertConversionHubSpotCleanup {
	metadata := bridge.hubSpotMetadata()
	cleanup := RevertConversionHubSpotCleanup{}
	if metadata == nil || (metadata.FileID == "" && metadata.NoteID == 0) {
		return cleanup
	}

	cleanup.Attempted = true
	if metadata.FileID != "" {
		cleanup.FileID = &metadata.FileID
	}
	if metadata.NoteID > 0 {
		cleanup.NoteID = &metadata.NoteID
	}
	if h.deps.HubSpot == nil {
		cleanup.Warnings = appendWarning(cleanup.Warnings, "hubspot_not_configured")
		h.logFailure(r, slog.LevelWarn, "hubspot cleanup skipped without client", "revert_conversion_hubspot_cleanup", start, "order_id", orderID, "quote_id", bridge.QuoteID, "hubspot_file_id", metadata.FileID, "hubspot_note_id", metadata.NoteID)
		return cleanup
	}

	if metadata.NoteID > 0 {
		err := h.deps.HubSpot.DeleteNote(r.Context(), metadata.NoteID)
		if err != nil && !hubspot.IsNotFound(err) {
			cleanup.Warnings = appendWarning(cleanup.Warnings, "hubspot_cleanup_failed")
			h.logFailure(r, slog.LevelWarn, "hubspot note cleanup failed after conversion revert", "revert_conversion_hubspot_note", start, "order_id", orderID, "quote_id", bridge.QuoteID, "hubspot_note_id", metadata.NoteID, "error", err)
		} else {
			cleanup.NoteDeleted = true
		}
	}

	if metadata.FileID != "" {
		err := h.deps.HubSpot.DeleteFile(r.Context(), metadata.FileID)
		if err != nil && !hubspot.IsNotFound(err) {
			cleanup.Warnings = appendWarning(cleanup.Warnings, "hubspot_cleanup_failed")
			h.logFailure(r, slog.LevelWarn, "hubspot file cleanup failed after conversion revert", "revert_conversion_hubspot_file", start, "order_id", orderID, "quote_id", bridge.QuoteID, "hubspot_file_id", metadata.FileID, "error", err)
		} else {
			cleanup.FileDeleted = true
		}
	}

	return cleanup
}

func appendWarning(warnings []string, code string) []string {
	for _, warning := range warnings {
		if warning == code {
			return warnings
		}
	}
	return append(warnings, code)
}

func firstWarning(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	return warnings[0]
}

func (h *Handler) countAlyanteOrderRows(r *http.Request, order *OrderDetail) (int64, error) {
	ndoc := strings.TrimSpace(ptrStringValue(order.CdlanNdoc))
	if ndoc == "" || order.CdlanAnno == nil {
		return 0, errGatewayPreconditionMissing
	}
	var numericDoc int64
	var hasNumericDoc bool
	if parsed, err := strconv.ParseInt(ndoc, 10, 64); err == nil {
		numericDoc = parsed
		hasNumericDoc = true
	}

	var count int64
	var err error
	if hasNumericDoc {
		err = h.deps.Alyante.QueryRowContext(r.Context(), alyanteOrderRowsCountQuery, numericDoc, *order.CdlanAnno, ndoc).Scan(&count)
	} else {
		err = h.deps.Alyante.QueryRowContext(r.Context(), alyanteOrderRowsCountByDocumentQuery, *order.CdlanAnno, ndoc).Scan(&count)
	}
	return count, err
}

var errOrderStateChanged = errors.New("order state changed")

func (h *Handler) deleteVodkaConvertedOrder(r *http.Request, orderID int64) (int64, error) {
	tx, err := h.deps.Vodka.BeginTx(r.Context(), nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rowsResult, err := tx.ExecContext(r.Context(), `DELETE FROM orders_rows WHERE orders_id = ?`, orderID)
	if err != nil {
		return 0, err
	}
	deletedRows, _ := rowsResult.RowsAffected()

	orderResult, err := tx.ExecContext(r.Context(), `DELETE FROM orders WHERE id = ? AND cdlan_stato = 'BOZZA'`, orderID)
	if err != nil {
		return 0, err
	}
	if affected, err := orderResult.RowsAffected(); err == nil && affected == 0 {
		return 0, errOrderStateChanged
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return deletedRows, nil
}

func (h *Handler) deleteLegacyOrderBridge(r *http.Request, quoteID, orderID int64) error {
	_, err := h.deps.Mistra.ExecContext(r.Context(), `
DELETE FROM orders.legacy_orders
WHERE quote_id = $1 AND vodka_id = $2`, quoteID, orderID)
	return err
}
