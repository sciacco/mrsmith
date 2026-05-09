package grappadcim

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListStorage(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status == "active" || status == "" {
		where = append(where, "LOWER(TRIM(s.status)) <> 'chiuso'")
	} else if status != "all" {
		where = append(where, "s.status = ?")
		args = append(args, status)
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(a.name LIKE ? OR s.codice_ordine LIKE ? OR s.serial_number LIKE ? OR s.note LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}
	rows, err := h.grappa.QueryContext(r.Context(), storageSelectSQL()+` WHERE `+strings.Join(where, " AND ")+` ORDER BY s.id DESC`, args...)
	if err != nil {
		h.dbFailure(w, r, "list_storage", err)
		return
	}
	defer rows.Close()
	items := []StorageItem{}
	for rows.Next() {
		item, err := scanStorage(rows)
		if err != nil {
			h.dbFailure(w, r, "list_storage_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_storage") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetStorage(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_storage_id")
		return
	}
	item, found, err := h.getStorage(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_storage", err, "storage_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "storage_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateStorage(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body StorageInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_storage_payload")
		return
	}
	if body.CustomerID <= 0 || body.EquipmentID <= 0 {
		invalidRequest(w, "invalid_storage_payload")
		return
	}
	if err := validateStorageWritableStatus(body.Status); err != nil {
		invalidRequest(w, err.Error())
		return
	}
	status := "Attivo"
	if body.Status != nil && strings.TrimSpace(*body.Status) != "" {
		status = strings.TrimSpace(*body.Status)
	}
	result, err := h.grappa.ExecContext(r.Context(), `
		INSERT INTO storage
			(access_protocol, size, cli_fatturazione_id, apparato_id_apparato, note, size_type, status, created_at, codice_ordine, serial_number)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), ?, ?)`,
		optionalTrimmed(body.Protocol), body.Size, body.CustomerID, body.EquipmentID, optionalTrimmed(body.Note),
		optionalTrimmed(body.SizeType), status, optionalTrimmed(body.OrderCode), optionalTrimmed(body.SerialNumber),
	)
	if err != nil {
		h.dbFailure(w, r, "create_storage", err)
		return
	}
	id, _ := result.LastInsertId()
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Storage creato."})
}

func (h *Handler) handleUpdateStorage(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_storage_id")
		return
	}
	var currentStatus string
	if err := h.grappa.QueryRowContext(r.Context(), `SELECT status FROM storage WHERE id = ?`, id).Scan(&currentStatus); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "storage_not_found")
			return
		}
		h.dbFailure(w, r, "get_storage_status", err, "storage_id", id)
		return
	}
	if strings.EqualFold(strings.TrimSpace(currentStatus), "Chiuso") {
		httputil.Error(w, http.StatusConflict, "storage_closed_read_only")
		return
	}
	var body StoragePatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_storage_payload")
		return
	}
	sets, args, err := storagePatch(body)
	if err != nil {
		invalidRequest(w, err.Error())
		return
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE storage SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_storage", err, "storage_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "storage_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Storage aggiornato."})
}

func (h *Handler) handleArchiveStorage(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_storage_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE storage SET status = 'Chiuso', closed_at = COALESCE(closed_at, NOW()) WHERE id = ?`, id)
	if err != nil {
		h.dbFailure(w, r, "archive_storage", err, "storage_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "storage_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Storage archiviato."})
}

func (h *Handler) handleDeleteStorage(w http.ResponseWriter, _ *http.Request) {
	httputil.Error(w, http.StatusNotImplemented, "storage_delete_deferred")
}

func (h *Handler) getStorage(r *http.Request, id int) (StorageItem, bool, error) {
	row := h.grappa.QueryRowContext(r.Context(), storageSelectSQL()+` WHERE s.id = ?`, id)
	item, err := scanStorage(row)
	if err == sql.ErrNoRows {
		return StorageItem{}, false, nil
	}
	return item, err == nil, err
}

func storageSelectSQL() string {
	return `
		SELECT s.id, s.access_protocol, s.size, s.cli_fatturazione_id, s.apparato_id_apparato, a.name,
		       s.note, s.size_type, s.status, s.created_at, s.closed_at, s.codice_ordine, s.serial_number
		FROM storage s
		LEFT JOIN apparato a ON a.id_apparato = s.apparato_id_apparato`
}

type storageScanner interface {
	Scan(dest ...any) error
}

func scanStorage(scanner storageScanner) (StorageItem, error) {
	var item StorageItem
	var protocol, equipment, note, sizeType, orderCode, serial sql.NullString
	var size sql.NullInt64
	var createdAt, closedAt sql.NullTime
	if err := scanner.Scan(&item.ID, &protocol, &size, &item.CustomerID, &item.EquipmentID, &equipment, &note, &sizeType, &item.Status, &createdAt, &closedAt, &orderCode, &serial); err != nil {
		return item, err
	}
	item.Protocol = nullableString(protocol)
	item.Size = nullableInt(size)
	item.Equipment = nullableString(equipment)
	item.Note = nullableString(note)
	item.SizeType = nullableString(sizeType)
	item.CreatedAt = nullableTime(createdAt)
	item.ClosedAt = nullableTime(closedAt)
	item.OrderCode = nullableString(orderCode)
	item.SerialNumber = nullableString(serial)
	item.ReadOnly = strings.EqualFold(strings.TrimSpace(item.Status), "Chiuso")
	return item, nil
}

func storagePatch(body StoragePatch) ([]string, []any, error) {
	sets := []string{}
	args := []any{}
	if err := validateStorageWritableStatus(body.Status); err != nil {
		return nil, nil, err
	}
	if body.CustomerID != nil {
		if *body.CustomerID <= 0 {
			return nil, nil, fmt.Errorf("invalid_storage_customer")
		}
		sets = append(sets, "cli_fatturazione_id = ?")
		args = append(args, *body.CustomerID)
	}
	if body.EquipmentID != nil {
		if *body.EquipmentID <= 0 {
			return nil, nil, fmt.Errorf("invalid_storage_equipment")
		}
		sets = append(sets, "apparato_id_apparato = ?")
		args = append(args, *body.EquipmentID)
	}
	if body.Size != nil {
		sets = append(sets, "size = ?")
		args = append(args, *body.Size)
	}
	fields := []struct {
		column string
		value  *string
	}{
		{"access_protocol", body.Protocol},
		{"note", body.Note},
		{"size_type", body.SizeType},
		{"status", body.Status},
		{"codice_ordine", body.OrderCode},
		{"serial_number", body.SerialNumber},
	}
	for _, field := range fields {
		if field.value != nil {
			sets = append(sets, field.column+" = ?")
			args = append(args, optionalTrimmed(field.value))
		}
	}
	return sets, args, nil
}

func validateStorageWritableStatus(status *string) error {
	if status != nil && strings.EqualFold(strings.TrimSpace(*status), "Chiuso") {
		return fmt.Errorf("storage_close_requires_archive")
	}
	return nil
}
