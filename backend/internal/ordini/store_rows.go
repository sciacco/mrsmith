package ordini

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const orderRowsSelect = `
SELECT id,
       orders_id,
       cdlan_systemodv_row,
       cdlan_codice_kit,
       index_kit,
       CASE WHEN COALESCE(cdlan_codice_kit, '') <> '' THEN CONCAT(cdlan_codice_kit, '-', index_kit) ELSE NULL END AS bundle_code,
       cdlan_codart,
       cdlan_descart,
       cdlan_qta,
       cdlan_prezzo,
       cdlan_prezzo_attivazione,
       cdlan_prezzo_cessazione,
       cdlan_ragg_fatturazione,
       cdlan_data_attivazione,
       cdlan_serialnumber,
       confirm_data_attivazione,
       data_annullamento
FROM orders_rows
`

func (h *Handler) listOrderRows(r *http.Request, orderID int64) ([]OrderRow, error) {
	rows, err := h.deps.Vodka.QueryContext(r.Context(), orderRowsSelect+`
WHERE orders_id = ?
ORDER BY id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]OrderRow, 0)
	for rows.Next() {
		var row OrderRow
		if err := scanOrderRow(rows, &row); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (h *Handler) getOrderRow(r *http.Request, orderID, rowID int64) (*OrderRow, error) {
	row := h.deps.Vodka.QueryRowContext(r.Context(), orderRowsSelect+`
WHERE orders_id = ? AND id = ?
LIMIT 1`, orderID, rowID)
	var item OrderRow
	if err := scanOrderRow(row, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanOrderRow(scanner interface{ Scan(dest ...any) error }, row *OrderRow) error {
	return scanner.Scan(
		&row.ID,
		&row.OrderID,
		&row.CdlanSystemODVRow,
		&row.CdlanCodiceKit,
		&row.IndexKit,
		&row.BundleCode,
		&row.CdlanCodart,
		&row.CdlanDescart,
		&row.CdlanQta,
		&row.Canone,
		&row.ActivationPrice,
		&row.TerminationPrice,
		&row.CdlanRaggFatturazione,
		&row.CdlanDataAttivazione,
		&row.CdlanSerialNumber,
		&row.ConfirmDataAttivazione,
		&row.DataAnnullamento,
	)
}

func (h *Handler) listTechnicalRows(r *http.Request, orderID int64) ([]TechnicalRow, error) {
	rows, err := h.deps.Vodka.QueryContext(r.Context(), `
SELECT id,
       cdlan_systemodv_row,
       CASE WHEN COALESCE(cdlan_codice_kit, '') <> '' THEN CONCAT(cdlan_codice_kit, '-', index_kit) ELSE NULL END AS bundle_code,
       cdlan_codart,
       cdlan_descart,
       CONVERT(note_tecnici USING UTF8) AS note_tecnici,
       data_annullamento
FROM orders_rows
WHERE orders_id = ?
ORDER BY id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]TechnicalRow, 0)
	for rows.Next() {
		var row TechnicalRow
		if err := scanTechnicalRow(rows, &row); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (h *Handler) getTechnicalRow(r *http.Request, orderID, rowID int64) (*TechnicalRow, error) {
	row := h.deps.Vodka.QueryRowContext(r.Context(), `
SELECT id,
       cdlan_systemodv_row,
       CASE WHEN COALESCE(cdlan_codice_kit, '') <> '' THEN CONCAT(cdlan_codice_kit, '-', index_kit) ELSE NULL END AS bundle_code,
       cdlan_codart,
       cdlan_descart,
       CONVERT(note_tecnici USING UTF8) AS note_tecnici,
       data_annullamento
FROM orders_rows
WHERE orders_id = ? AND id = ?
LIMIT 1`, orderID, rowID)
	var item TechnicalRow
	if err := scanTechnicalRow(row, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanTechnicalRow(scanner interface{ Scan(dest ...any) error }, row *TechnicalRow) error {
	return scanner.Scan(
		&row.ID,
		&row.CdlanSystemODVRow,
		&row.BundleCode,
		&row.CdlanCodart,
		&row.CdlanDescart,
		&row.NoteTecnici,
		&row.DataAnnullamento,
	)
}

func (h *Handler) handleListRows(w http.ResponseWriter, r *http.Request) {
	if !h.requireVodka(w) {
		return
	}
	orderID, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	items, err := h.listOrderRows(r, orderID)
	if err != nil {
		h.dbFailure(w, r, "list_order_rows", err, "order_id", orderID)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleListTechnicalRows(w http.ResponseWriter, r *http.Request) {
	if !h.requireVodka(w) {
		return
	}
	orderID, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	items, err := h.listTechnicalRows(r, orderID)
	if err != nil {
		h.dbFailure(w, r, "list_technical_rows", err, "order_id", orderID)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handlePatchSerialNumber(w http.ResponseWriter, r *http.Request) {
	if !h.requireVodka(w) {
		return
	}
	orderID, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	rowID, ok := h.parseRowID(w, r)
	if !ok {
		return
	}
	payload, ok := decodeJSON[UpdateSerialRequest](w, r)
	if !ok {
		return
	}
	order, err := h.getOrderWithoutOrigin(r, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "order_not_found")
			return
		}
		h.dbFailure(w, r, "get_order_for_serial_patch", err, "order_id", orderID)
		return
	}
	if !requireState(w, stateOf(order), OrderStateBozza) {
		return
	}
	if _, err := h.getOrderRow(r, orderID, rowID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "row_not_found")
			return
		}
		h.dbFailure(w, r, "get_row_for_serial_patch", err, "order_id", orderID, "row_id", rowID)
		return
	}
	_, err = h.deps.Vodka.ExecContext(r.Context(), `
UPDATE orders_rows
SET cdlan_serialnumber = ?
WHERE id = ? AND orders_id = ?`, nullIfBlank(payload.SerialNumber), rowID, orderID)
	if err != nil {
		h.dbFailure(w, r, "patch_row_serial", err, "order_id", orderID, "row_id", rowID)
		return
	}
	row, err := h.getOrderRow(r, orderID, rowID)
	if err != nil {
		h.dbFailure(w, r, "get_row_after_serial_patch", err, "order_id", orderID, "row_id", rowID)
		return
	}
	httputil.JSON(w, http.StatusOK, row)
}

func (h *Handler) handlePatchTechnicalNotes(w http.ResponseWriter, r *http.Request) {
	if !h.requireVodka(w) {
		return
	}
	orderID, ok := h.parseOrderID(w, r)
	if !ok {
		return
	}
	rowID, ok := h.parseRowID(w, r)
	if !ok {
		return
	}
	payload, ok := decodeJSON[UpdateTechnicalNotesRequest](w, r)
	if !ok {
		return
	}
	if _, err := h.getOrderRow(r, orderID, rowID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "row_not_found")
			return
		}
		h.dbFailure(w, r, "get_row_for_technical_notes_patch", err, "order_id", orderID, "row_id", rowID)
		return
	}
	_, err := h.deps.Vodka.ExecContext(r.Context(), `
UPDATE orders_rows
SET note_tecnici = ?
WHERE id = ? AND orders_id = ?`, nullIfBlank(payload.TechnicalNotes), rowID, orderID)
	if err != nil {
		h.dbFailure(w, r, "patch_row_technical_notes", err, "order_id", orderID, "row_id", rowID)
		return
	}
	row, err := h.getTechnicalRow(r, orderID, rowID)
	if err != nil {
		h.dbFailure(w, r, "get_technical_row_after_patch", err, "order_id", orderID, "row_id", rowID)
		return
	}
	httputil.JSON(w, http.StatusOK, row)
}
