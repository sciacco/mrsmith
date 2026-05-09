package grappadcim

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListXcon(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	tab := strings.TrimSpace(r.URL.Query().Get("tab"))
	where := []string{}
	if tab == "ceased" {
		where = append(where, "LOWER(TRIM(x.stato)) = 'cessata'")
	} else {
		where = append(where, "LOWER(TRIM(x.stato)) <> 'cessata'")
	}
	args := []any{}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(x.ticket LIKE ? OR x.ticket_esteso LIKE ? OR x.num_ordine LIKE ? OR x.riga_ordine LIKE ? OR x.tipo LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like, like)
	}
	rows, err := h.grappa.QueryContext(r.Context(), xconSelectSQL()+" WHERE "+strings.Join(where, " AND ")+" ORDER BY x.created_at DESC, x.id DESC LIMIT 300", args...)
	if err != nil {
		h.dbFailure(w, r, "list_xcon", err)
		return
	}
	defer rows.Close()
	items := []Xcon{}
	for rows.Next() {
		item, err := scanXcon(rows)
		if err != nil {
			h.dbFailure(w, r, "list_xcon_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_xcon") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetXcon(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_xcon_id")
		return
	}
	item, found, err := h.getXcon(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_xcon", err, "xcon_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "xcon_not_found")
		return
	}
	hops, err := h.listXconHops(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_xcon_hops", err, "xcon_id", id)
		return
	}
	item.Hops = hops
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateXcon(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body XconInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_xcon_payload")
		return
	}
	if code := validateXconInput(body); code != "" {
		invalidRequest(w, code)
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `
		INSERT INTO xcon
			(ticket, pa, cliente, stato, num_ordine, riga_ordine, tipo, data_attivazione, data_cessazione,
			 aend_unita_app, aend_slot, aend_fibre, aend_apparato,
			 zend_unita_app, zend_slot, zend_fibre, zend_apparato,
			 note, ticket_esteso, note_cliente, sorgente, created_at, aend_rack_id, zend_rack_id, loa_name, loa_id, mmr_port)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), ?, ?, ?, ?, ?)`,
		strings.TrimSpace(body.Ticket), optionalTrimmed(body.PA), body.CustomerID, strings.TrimSpace(body.Status),
		optionalTrimmed(body.OrderCode), optionalTrimmed(body.SerialNumber), strings.TrimSpace(body.Type),
		optionalTrimmed(body.ActivatedAt), optionalTrimmed(body.CeasedAt),
		strings.TrimSpace(body.AEndUnit), optionalTrimmed(body.AEndSlot), strings.TrimSpace(body.AEndFibers), strings.TrimSpace(body.AEndEquipment),
		strings.TrimSpace(body.ZEndUnit), optionalTrimmed(body.ZEndSlot), strings.TrimSpace(body.ZEndFibers), strings.TrimSpace(body.ZEndEquipment),
		optionalTrimmed(body.Note), optionalTrimmed(body.ExtendedTicket), optionalTrimmed(body.CustomerNote), optionalTrimmed(body.Source),
		body.AEndRackID, body.ZEndRackID, optionalTrimmed(body.LoaName), body.LoaID, optionalTrimmed(body.MMRPort),
	)
	if err != nil {
		h.dbFailure(w, r, "create_xcon", err)
		return
	}
	id, _ := result.LastInsertId()
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Cross connect creato."})
}

func (h *Handler) handleUpdateXcon(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_xcon_id")
		return
	}
	var body XconPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_xcon_payload")
		return
	}
	if code := validateXconInput(body); code != "" {
		invalidRequest(w, code)
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `
		UPDATE xcon
		SET ticket = ?, pa = ?, cliente = ?, stato = ?, num_ordine = ?, riga_ordine = ?, tipo = ?,
		    data_attivazione = ?, data_cessazione = ?,
		    aend_unita_app = ?, aend_slot = ?, aend_fibre = ?, aend_apparato = ?,
		    zend_unita_app = ?, zend_slot = ?, zend_fibre = ?, zend_apparato = ?,
		    note = ?, ticket_esteso = ?, note_cliente = ?, sorgente = ?,
		    aend_rack_id = ?, zend_rack_id = ?, loa_name = ?, loa_id = ?, mmr_port = ?
		WHERE id = ?`,
		strings.TrimSpace(body.Ticket), optionalTrimmed(body.PA), body.CustomerID, strings.TrimSpace(body.Status),
		optionalTrimmed(body.OrderCode), optionalTrimmed(body.SerialNumber), strings.TrimSpace(body.Type),
		optionalTrimmed(body.ActivatedAt), optionalTrimmed(body.CeasedAt),
		strings.TrimSpace(body.AEndUnit), optionalTrimmed(body.AEndSlot), strings.TrimSpace(body.AEndFibers), strings.TrimSpace(body.AEndEquipment),
		strings.TrimSpace(body.ZEndUnit), optionalTrimmed(body.ZEndSlot), strings.TrimSpace(body.ZEndFibers), strings.TrimSpace(body.ZEndEquipment),
		optionalTrimmed(body.Note), optionalTrimmed(body.ExtendedTicket), optionalTrimmed(body.CustomerNote), optionalTrimmed(body.Source),
		body.AEndRackID, body.ZEndRackID, optionalTrimmed(body.LoaName), body.LoaID, optionalTrimmed(body.MMRPort), id,
	)
	if err != nil {
		h.dbFailure(w, r, "update_xcon", err, "xcon_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "xcon_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Cross connect aggiornato."})
}

func (h *Handler) handleReplaceXconHops(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_xcon_id")
		return
	}
	var body XconHopsInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_xcon_hops_payload")
		return
	}
	for i, hop := range body.Items {
		if strings.TrimSpace(hop.Room) == "" || strings.TrimSpace(hop.Rack) == "" || strings.TrimSpace(hop.Unit) == "" || strings.TrimSpace(hop.Fibers) == "" || hop.RackID <= 0 {
			invalidRequest(w, "xcon_hop_required_fields")
			return
		}
		body.Items[i].Order = i + 1
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		var lockedID int
		if err := tx.QueryRowContext(r.Context(), `SELECT id FROM xcon WHERE id = ? FOR UPDATE`, id).Scan(&lockedID); err != nil {
			if err == sql.ErrNoRows {
				return sql.ErrNoRows
			}
			return err
		}
		if lockedID == 0 {
			return sql.ErrNoRows
		}
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM xcon_hop WHERE xcon_id = ?`, id); err != nil {
			return err
		}
		for _, hop := range body.Items {
			if _, err := tx.ExecContext(r.Context(), `
				INSERT INTO xcon_hop (xcon_id, hop_room, hop_rack, hop_unita_app, hop_slot, hop_fibre, hop_num, rack_id)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				id, strings.TrimSpace(hop.Room), strings.TrimSpace(hop.Rack), strings.TrimSpace(hop.Unit),
				optionalTrimmed(hop.Slot), strings.TrimSpace(hop.Fibers), hop.Order, hop.RackID,
			); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "xcon_not_found")
			return
		}
		h.dbFailure(w, r, "replace_xcon_hops", err, "xcon_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Percorso aggiornato."})
}

func (h *Handler) handleXconProductOptions(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	rows, err := h.grappa.QueryContext(r.Context(), `SELECT DISTINCT TRIM(tipo) FROM xcon WHERE TRIM(tipo) <> '' ORDER BY TRIM(tipo) ASC`)
	if err != nil {
		h.dbFailure(w, r, "xcon_product_options", err)
		return
	}
	defer rows.Close()
	items := []LookupItem{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			h.dbFailure(w, r, "xcon_product_options_scan", err)
			return
		}
		items = append(items, LookupItem{ID: value, Label: value})
	}
	if !h.rowsDone(w, r, rows, "xcon_product_options") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) getXcon(r *http.Request, id int) (Xcon, bool, error) {
	row := h.grappa.QueryRowContext(r.Context(), xconSelectSQL()+` WHERE x.id = ?`, id)
	item, err := scanXcon(row)
	if err == sql.ErrNoRows {
		return Xcon{}, false, nil
	}
	return item, err == nil, err
}

func (h *Handler) listXconHops(r *http.Request, id int) ([]XconHop, error) {
	rows, err := h.grappa.QueryContext(r.Context(), `
		SELECT id, xcon_id, hop_room, hop_rack, hop_unita_app, hop_slot, hop_fibre, hop_num, rack_id
		FROM xcon_hop
		WHERE xcon_id = ?
		ORDER BY hop_num ASC, id ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []XconHop{}
	for rows.Next() {
		var item XconHop
		var slot sql.NullString
		if err := rows.Scan(&item.ID, &item.XconID, &item.Room, &item.Rack, &item.Unit, &slot, &item.Fibers, &item.Order, &item.RackID); err != nil {
			return nil, err
		}
		item.Slot = nullableString(slot)
		items = append(items, item)
	}
	return items, rows.Err()
}

func validateXconInput(body XconInput) string {
	if strings.TrimSpace(body.Status) == "" {
		return "invalid_xcon_status"
	}
	if strings.TrimSpace(body.Ticket) == "" || body.CustomerID <= 0 || strings.TrimSpace(body.Type) == "" {
		return "xcon_required_fields"
	}
	if strings.TrimSpace(body.AEndUnit) == "" || strings.TrimSpace(body.AEndFibers) == "" || strings.TrimSpace(body.AEndEquipment) == "" {
		return "xcon_aend_required_fields"
	}
	if strings.TrimSpace(body.ZEndUnit) == "" || strings.TrimSpace(body.ZEndFibers) == "" || strings.TrimSpace(body.ZEndEquipment) == "" {
		return "xcon_zend_required_fields"
	}
	return ""
}

func xconSelectSQL() string {
	return `
		SELECT x.id, x.ticket, x.pa, x.cliente, x.stato, x.num_ordine, x.riga_ordine, x.tipo,
		       x.data_attivazione, x.data_cessazione,
		       x.aend_unita_app, x.aend_slot, x.aend_fibre, x.aend_apparato,
		       x.zend_unita_app, x.zend_slot, x.zend_fibre, x.zend_apparato,
		       x.note, x.ticket_esteso, x.note_cliente, x.sorgente, x.created_at,
		       x.aend_rack_id, x.zend_rack_id, x.loa_name, x.loa_id, x.mmr_port
		FROM xcon x`
}

type xconScanner interface {
	Scan(dest ...any) error
}

func scanXcon(scanner xconScanner) (Xcon, error) {
	var item Xcon
	var pa, orderCode, serialNumber, aEndSlot, zEndSlot sql.NullString
	var activatedAt, ceasedAt sql.NullTime
	var note, extendedTicket, customerNote, source, loaName, mmrPort sql.NullString
	var createdAt sql.NullTime
	var aRack, zRack, loaID sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.Ticket, &pa, &item.CustomerID, &item.Status, &orderCode, &serialNumber, &item.Type,
		&activatedAt, &ceasedAt,
		&item.AEndUnit, &aEndSlot, &item.AEndFibers, &item.AEndEquipment,
		&item.ZEndUnit, &zEndSlot, &item.ZEndFibers, &item.ZEndEquipment,
		&note, &extendedTicket, &customerNote, &source, &createdAt,
		&aRack, &zRack, &loaName, &loaID, &mmrPort,
	); err != nil {
		return item, err
	}
	item.PA = nullableString(pa)
	item.OrderCode = nullableString(orderCode)
	item.SerialNumber = nullableString(serialNumber)
	item.ActivatedAt = nullableTime(activatedAt)
	item.CeasedAt = nullableTime(ceasedAt)
	item.AEndSlot = nullableString(aEndSlot)
	item.ZEndSlot = nullableString(zEndSlot)
	item.Note = nullableString(note)
	item.ExtendedTicket = nullableString(extendedTicket)
	item.CustomerNote = nullableString(customerNote)
	item.Source = nullableString(source)
	item.CreatedAt = nullableTime(createdAt)
	item.AEndRackID = nullableInt(aRack)
	item.ZEndRackID = nullableInt(zRack)
	item.LoaName = nullableString(loaName)
	item.LoaID = nullableInt(loaID)
	item.MMRPort = nullableString(mmrPort)
	return item, nil
}
