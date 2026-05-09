package grappadcim

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListBuildings(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	where := []string{"1=1"}
	args := []any{}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(db.name LIKE ? OR db.address LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status == "active" {
		where = append(where, activeStateSQL("db.status"))
	} else if status != "" && status != "all" {
		where = append(where, "db.status = ?")
		args = append(args, status)
	}

	query := fmt.Sprintf(`
		SELECT db.id, db.name, db.address, db.status, db.portale_clienti, db.n_rack,
		       db.created_at, db.updated_at, db.ceased_at,
		       COUNT(DISTINCT d.id_datacenter) AS datacenter_count,
		       COUNT(DISTINCT r.id_rack) AS rack_count
		FROM dc_build db
		LEFT JOIN datacenter d ON d.dc_build_id = db.id
		LEFT JOIN racks r ON r.id_datacenter = d.id_datacenter AND %s
		WHERE %s
		GROUP BY db.id, db.name, db.address, db.status, db.portale_clienti, db.n_rack,
		         db.created_at, db.updated_at, db.ceased_at
		ORDER BY db.name ASC, db.id ASC`, activeStateSQL("r.stato"), strings.Join(where, " AND "))

	rows, err := h.grappa.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_buildings", err)
		return
	}
	defer rows.Close()

	items := []Building{}
	for rows.Next() {
		item, err := scanBuilding(rows)
		if err != nil {
			h.dbFailure(w, r, "list_buildings_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_buildings") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetBuilding(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_building_id")
		return
	}
	item, found, err := h.getBuilding(r, id)
	if err != nil {
		h.dbFailure(w, r, "get_building", err, "building_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "building_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateBuilding(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body BuildingInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_building_payload")
		return
	}
	if err := validateBuildingInput(body); err != nil {
		invalidRequest(w, err.Error())
		return
	}
	status := strings.TrimSpace(body.Status)
	result, err := h.grappa.ExecContext(
		r.Context(),
		`INSERT INTO dc_build (name, address, status, portale_clienti, n_rack) VALUES (?, ?, ?, ?, ?)`,
		strings.TrimSpace(body.Name),
		strings.TrimSpace(body.Address),
		status,
		boolInt(body.PortalEnabled),
		body.RackCapacity,
	)
	if err != nil {
		h.dbFailure(w, r, "create_building", err)
		return
	}
	id, _ := result.LastInsertId()
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Edificio creato."})
}

func (h *Handler) handleUpdateBuilding(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_building_id")
		return
	}
	var body BuildingPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_building_payload")
		return
	}
	sets, args, err := buildingPatch(body)
	if err != nil {
		invalidRequest(w, err.Error())
		return
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE dc_build SET `+strings.Join(sets, ", ")+`, updated_at = NOW() WHERE id = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_building", err, "building_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "building_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Edificio aggiornato."})
}

func (h *Handler) handleCeaseBuilding(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_building_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	deps, err := h.buildingDependencies(r, id)
	if err != nil {
		h.dbFailure(w, r, "building_cease_dependencies", err, "building_id", id)
		return
	}
	if !deps.Allowed {
		httputil.JSON(w, http.StatusConflict, deps)
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE dc_build SET status = 'Cessato', ceased_at = COALESCE(ceased_at, NOW()), updated_at = NOW() WHERE id = ?`, id)
	if err != nil {
		h.dbFailure(w, r, "cease_building", err, "building_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "building_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Edificio cessato."})
}

func (h *Handler) handleDeleteBuilding(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_building_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	deps, err := h.buildingDependencies(r, id)
	if err != nil {
		h.dbFailure(w, r, "building_delete_dependencies", err, "building_id", id)
		return
	}
	if !deps.Allowed {
		httputil.JSON(w, http.StatusConflict, deps)
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `DELETE FROM dc_build WHERE id = ?`, id)
	if err != nil {
		h.dbFailure(w, r, "delete_building", err, "building_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "building_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Edificio eliminato."})
}

func (h *Handler) getBuilding(r *http.Request, id int) (Building, bool, error) {
	row := h.grappa.QueryRowContext(r.Context(), `
		SELECT db.id, db.name, db.address, db.status, db.portale_clienti, db.n_rack,
		       db.created_at, db.updated_at, db.ceased_at,
		       COUNT(DISTINCT d.id_datacenter) AS datacenter_count,
		       COUNT(DISTINCT r.id_rack) AS rack_count
		FROM dc_build db
		LEFT JOIN datacenter d ON d.dc_build_id = db.id
		LEFT JOIN racks r ON r.id_datacenter = d.id_datacenter AND `+activeStateSQL("r.stato")+`
		WHERE db.id = ?
		GROUP BY db.id, db.name, db.address, db.status, db.portale_clienti, db.n_rack,
		         db.created_at, db.updated_at, db.ceased_at`, id)
	item, err := scanBuilding(row)
	if err == sql.ErrNoRows {
		return Building{}, false, nil
	}
	return item, err == nil, err
}

type buildingScanner interface {
	Scan(dest ...any) error
}

func scanBuilding(scanner buildingScanner) (Building, error) {
	var item Building
	var portal int
	var createdAt, updatedAt, ceasedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Address,
		&item.Status,
		&portal,
		&item.RackCapacity,
		&createdAt,
		&updatedAt,
		&ceasedAt,
		&item.DatacenterCount,
		&item.RackCount,
	); err != nil {
		return item, err
	}
	item.PortalEnabled = portal == 1
	item.CreatedAt = nullableTime(createdAt)
	item.UpdatedAt = nullableTime(updatedAt)
	item.CeasedAt = nullableTime(ceasedAt)
	return item, nil
}

func validateBuildingInput(body BuildingInput) error {
	if strings.TrimSpace(body.Name) == "" {
		return fmt.Errorf("building_name_required")
	}
	if strings.TrimSpace(body.Address) == "" {
		return fmt.Errorf("building_address_required")
	}
	status := strings.TrimSpace(body.Status)
	if status == "" {
		return fmt.Errorf("building_status_required")
	}
	if status != "Attivo" && status != "Cessato" {
		return fmt.Errorf("invalid_building_status")
	}
	if body.RackCapacity < 0 {
		return fmt.Errorf("invalid_rack_capacity")
	}
	return nil
}

func buildingPatch(body BuildingPatch) ([]string, []any, error) {
	sets := []string{}
	args := []any{}
	if body.Name != nil {
		if strings.TrimSpace(*body.Name) == "" {
			return nil, nil, fmt.Errorf("building_name_required")
		}
		sets = append(sets, "name = ?")
		args = append(args, strings.TrimSpace(*body.Name))
	}
	if body.Address != nil {
		if strings.TrimSpace(*body.Address) == "" {
			return nil, nil, fmt.Errorf("building_address_required")
		}
		sets = append(sets, "address = ?")
		args = append(args, strings.TrimSpace(*body.Address))
	}
	if body.Status != nil {
		status := strings.TrimSpace(*body.Status)
		if status != "Attivo" && status != "Cessato" {
			return nil, nil, fmt.Errorf("invalid_building_status")
		}
		sets = append(sets, "status = ?")
		args = append(args, status)
	}
	if body.PortalEnabled != nil {
		sets = append(sets, "portale_clienti = ?")
		args = append(args, boolInt(*body.PortalEnabled))
	}
	if body.RackCapacity != nil {
		if *body.RackCapacity < 0 {
			return nil, nil, fmt.Errorf("invalid_rack_capacity")
		}
		sets = append(sets, "n_rack = ?")
		args = append(args, *body.RackCapacity)
	}
	return sets, args, nil
}

func (h *Handler) buildingDependencies(r *http.Request, id int) (DependencySummary, error) {
	checks := []dependencyCheck{
		{Key: "datacenters", Label: "Sale e MMR attivi", Query: `SELECT COUNT(*) FROM datacenter WHERE dc_build_id = ? AND ` + activeStateSQL("stato") + ` AND data_cessazione IS NULL`},
		{Key: "portal", Label: "Esposizioni sul portale clienti", Query: `SELECT COUNT(*) FROM datacenter WHERE dc_build_id = ? AND portale_clienti = '1'`},
		{Key: "racks", Label: "Rack attivi", Query: `SELECT COUNT(*) FROM racks r JOIN datacenter d ON d.id_datacenter = r.id_datacenter WHERE d.dc_build_id = ? AND ` + activeStateSQL("r.stato") + ` AND r.data_cessazione IS NULL`},
		{Key: "apparati", Label: "Apparati collegati", Query: `SELECT COUNT(*) FROM apparato a JOIN racks r ON r.id_rack = a.id_rack JOIN datacenter d ON d.id_datacenter = r.id_datacenter WHERE d.dc_build_id = ? AND ` + activeStateSQL("a.stato")},
		{Key: "servers", Label: "Server collegati", Query: `SELECT COUNT(*) FROM server s JOIN racks r ON r.id_rack = s.id_rack JOIN datacenter d ON d.id_datacenter = r.id_datacenter WHERE d.dc_build_id = ? AND ` + activeStateSQL("s.stato")},
		{Key: "optical", Label: "Cassetti ottici collegati", Query: `SELECT COUNT(*) FROM cassetti_ottici co JOIN datacenter d ON d.id_datacenter IN (co.id_datacenter, co.id_datacenter_coll) WHERE d.dc_build_id = ? AND ` + activeStateSQL("co.stato")},
	}
	return h.runDependencyChecks(r, id, checks)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
