package grappadcim

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListCameras(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(code LIKE ? OR model LIKE ? OR brand LIKE ? OR position LIKE ? OR ipaddr LIKE ? OR serial LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like, like, like)
	}
	rows, err := h.grappa.QueryContext(r.Context(), `SELECT id, code, model, brand, position, ipaddr, status, serial FROM cams WHERE `+strings.Join(where, " AND ")+` ORDER BY code ASC, id ASC`, args...)
	if err != nil {
		h.dbFailure(w, r, "list_cameras", err)
		return
	}
	defer rows.Close()
	items := []CameraItem{}
	for rows.Next() {
		item, err := scanCamera(rows)
		if err != nil {
			h.dbFailure(w, r, "list_cameras_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_cameras") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetCamera(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_camera_id")
		return
	}
	var item CameraItem
	item, err = h.getCamera(r, id)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "camera_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_camera", err, "camera_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateCamera(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body CameraInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_camera_payload")
		return
	}
	if err := validateCameraRequired(body.Code, body.Model, body.Brand, body.Position, body.IPAddr); err != nil {
		invalidRequest(w, err.Error())
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `
		INSERT INTO cams (code, model, brand, position, ipaddr, status, serial)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(body.Code), strings.TrimSpace(body.Model), strings.TrimSpace(body.Brand), strings.TrimSpace(body.Position),
		optionalTrimmed(body.IPAddr), optionalTrimmed(body.Status), optionalTrimmed(body.Serial),
	)
	if err != nil {
		h.dbFailure(w, r, "create_camera", err)
		return
	}
	id, _ := result.LastInsertId()
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Telecamera creata."})
}

func (h *Handler) handleUpdateCamera(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_camera_id")
		return
	}
	var body CameraPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_camera_payload")
		return
	}
	sets, args, err := cameraPatch(body)
	if err != nil {
		invalidRequest(w, err.Error())
		return
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE cams SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_camera", err, "camera_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "camera_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Telecamera aggiornata."})
}

func (h *Handler) getCamera(r *http.Request, id int) (CameraItem, error) {
	return scanCamera(h.grappa.QueryRowContext(r.Context(), `SELECT id, code, model, brand, position, ipaddr, status, serial FROM cams WHERE id = ?`, id))
}

type cameraScanner interface {
	Scan(dest ...any) error
}

func scanCamera(scanner cameraScanner) (CameraItem, error) {
	var item CameraItem
	var code, model, brand, position, ipaddr, status, serial sql.NullString
	if err := scanner.Scan(&item.ID, &code, &model, &brand, &position, &ipaddr, &status, &serial); err != nil {
		return item, err
	}
	item.Code = optionalStringValue(nullableString(code))
	item.Model = optionalStringValue(nullableString(model))
	item.Brand = optionalStringValue(nullableString(brand))
	item.Position = optionalStringValue(nullableString(position))
	item.IPAddr = nullableString(ipaddr)
	item.Status = nullableString(status)
	item.Serial = nullableString(serial)
	return item, nil
}

func validateCameraRequired(code string, model string, brand string, position string, ipaddr *string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(model) == "" || strings.TrimSpace(brand) == "" || strings.TrimSpace(position) == "" {
		return fmt.Errorf("camera_required_fields")
	}
	return validateOptionalIP(ipaddr)
}

func validateOptionalIP(value *string) error {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	if net.ParseIP(strings.TrimSpace(*value)) == nil {
		return fmt.Errorf("invalid_camera_ip")
	}
	return nil
}

func cameraPatch(body CameraPatch) ([]string, []any, error) {
	if err := validateOptionalIP(body.IPAddr); err != nil {
		return nil, nil, err
	}
	sets := []string{}
	args := []any{}
	required := []struct {
		column string
		value  *string
	}{
		{"code", body.Code},
		{"model", body.Model},
		{"brand", body.Brand},
		{"position", body.Position},
	}
	for _, field := range required {
		if field.value != nil {
			if strings.TrimSpace(*field.value) == "" {
				return nil, nil, fmt.Errorf("camera_required_fields")
			}
			sets = append(sets, field.column+" = ?")
			args = append(args, strings.TrimSpace(*field.value))
		}
	}
	optional := []struct {
		column string
		value  *string
	}{
		{"ipaddr", body.IPAddr},
		{"status", body.Status},
		{"serial", body.Serial},
	}
	for _, field := range optional {
		if field.value != nil {
			sets = append(sets, field.column+" = ?")
			args = append(args, optionalTrimmed(field.value))
		}
	}
	return sets, args, nil
}
