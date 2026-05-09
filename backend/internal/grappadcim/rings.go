package grappadcim

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

var (
	errFiberRingDecreaseBlocked = errors.New("fiber ring node decrease blocked")
	errFiberRingDeleteBlocked   = errors.New("fiber ring delete blocked")
	errFiberRingClosingArcInUse = errors.New("fiber ring closing arc in use")
)

type sqlQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (h *Handler) handleListFiberRings(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	where := []string{"1=1"}
	args := []any{}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where = append(where, "(af.nome LIKE ? OR af.note LIKE ? OR af.serialnumber LIKE ? OR af.codice_ordine LIKE ? OR CAST(af.id_anagrafica AS CHAR) LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like, like)
	}
	if customerID := strings.TrimSpace(r.URL.Query().Get("customerId")); customerID != "" {
		id, err := parsePositiveString(customerID)
		if err != nil {
			invalidRequest(w, "invalid_customer_id")
			return
		}
		where = append(where, "af.id_anagrafica = ?")
		args = append(args, id)
	}
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))) {
	case "active", "attivo":
		where = append(where, activeStateSQL("af.stato"))
	case "all", "":
	default:
		where = append(where, "LOWER(TRIM(af.stato)) = LOWER(TRIM(?))")
		args = append(args, r.URL.Query().Get("status"))
	}

	rows, err := h.grappa.QueryContext(r.Context(), fiberRingSelectSQL()+" WHERE "+strings.Join(where, " AND ")+" ORDER BY af.nome ASC, af.id_anello ASC", args...)
	if err != nil {
		h.dbFailure(w, r, "list_fiber_rings", err)
		return
	}
	defer rows.Close()
	items := []FiberRing{}
	for rows.Next() {
		item, err := scanFiberRing(rows)
		if err != nil {
			h.dbFailure(w, r, "list_fiber_rings_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, "list_fiber_rings") {
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetFiberRing(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	item, found, err := h.getFiberRing(r.Context(), id)
	if err != nil {
		h.dbFailure(w, r, "get_fiber_ring", err, "ring_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "fiber_ring_not_found")
		return
	}
	summary, err := h.fiberRingDeleteSummary(r.Context(), h.grappa, item)
	if err != nil {
		h.dbFailure(w, r, "fiber_ring_delete_summary", err, "ring_id", id)
		return
	}
	item.DeleteCheck = &summary
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateFiberRing(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	var body FiberRingInput
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_fiber_ring_payload")
		return
	}
	if strings.TrimSpace(body.Name) == "" || body.NodeCount <= 0 {
		invalidRequest(w, "fiber_ring_name_nodes_required")
		return
	}
	status := "Attivo"
	if body.Status != nil && strings.TrimSpace(*body.Status) != "" {
		status = strings.TrimSpace(*body.Status)
	}

	var id int64
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(r.Context(), `
			INSERT INTO anelli_fibra (nome, id_anagrafica, n_nodi, note, serialnumber, codice_ordine, stato)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			strings.TrimSpace(body.Name), body.CustomerID, body.NodeCount, optionalTrimmed(body.Note),
			optionalTrimmed(body.SerialNumber), optionalTrimmed(body.OrderCode), status)
		if err != nil {
			return err
		}
		id, _ = result.LastInsertId()
		return createFiberRingTopologyTx(r.Context(), tx, int(id), body.NodeCount)
	}); err != nil {
		h.dbFailure(w, r, "create_fiber_ring", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, MutationResponse{ID: int(id), Message: "Anello fibra creato con topologia circolare."})
}

func (h *Handler) handleUpdateFiberRing(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	var body FiberRingPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_fiber_ring_payload")
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		var currentNodes int
		var currentName string
		if err := tx.QueryRowContext(r.Context(), `SELECT n_nodi, nome FROM anelli_fibra WHERE id_anello = ? FOR UPDATE`, id).Scan(&currentNodes, &currentName); err != nil {
			return err
		}
		if body.NodeCount != nil {
			if *body.NodeCount < currentNodes {
				return errFiberRingDecreaseBlocked
			}
			if *body.NodeCount > currentNodes {
				if err := increaseFiberRingNodesTx(r.Context(), tx, id, currentNodes, *body.NodeCount); err != nil {
					return err
				}
			}
		}
		sets := []string{}
		args := []any{}
		if body.Name != nil {
			newName := strings.TrimSpace(*body.Name)
			if newName == "" {
				return errBadRequest
			}
			if err := updateFiberRingKMLAssociationTx(r.Context(), tx, currentName, newName); err != nil {
				return err
			}
			sets = append(sets, "nome = ?")
			args = append(args, newName)
		}
		if body.CustomerID != nil {
			sets = append(sets, "id_anagrafica = ?")
			args = append(args, *body.CustomerID)
		}
		if body.Note != nil {
			sets = append(sets, "note = ?")
			args = append(args, optionalTrimmed(body.Note))
		}
		if body.SerialNumber != nil {
			sets = append(sets, "serialnumber = ?")
			args = append(args, optionalTrimmed(body.SerialNumber))
		}
		if body.OrderCode != nil {
			sets = append(sets, "codice_ordine = ?")
			args = append(args, optionalTrimmed(body.OrderCode))
		}
		if body.Status != nil {
			if strings.TrimSpace(*body.Status) == "" {
				return errBadRequest
			}
			sets = append(sets, "stato = ?")
			args = append(args, strings.TrimSpace(*body.Status))
		}
		if len(sets) == 0 {
			return nil
		}
		args = append(args, id)
		result, err := tx.ExecContext(r.Context(), "UPDATE anelli_fibra SET "+strings.Join(sets, ", ")+" WHERE id_anello = ?", args...)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return sql.ErrNoRows
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "fiber_ring_not_found")
			return
		}
		if err == errFiberRingDecreaseBlocked {
			httputil.Error(w, http.StatusConflict, "fiber_ring_node_decrease_blocked")
			return
		}
		if err == errFiberRingClosingArcInUse {
			httputil.Error(w, http.StatusConflict, "fiber_ring_closing_arc_has_route")
			return
		}
		if err == errBadRequest {
			invalidRequest(w, "invalid_fiber_ring_payload")
			return
		}
		h.dbFailure(w, r, "update_fiber_ring", err, "ring_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Anello fibra aggiornato."})
}

func (h *Handler) handleIncreaseFiberRingNodes(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	var body IncreaseFiberRingNodesRequest
	if err := decodeJSONBody(r, &body); err != nil || body.NodeCount <= 0 {
		invalidRequest(w, "invalid_node_count")
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		var currentNodes int
		if err := tx.QueryRowContext(r.Context(), `SELECT n_nodi FROM anelli_fibra WHERE id_anello = ? FOR UPDATE`, id).Scan(&currentNodes); err != nil {
			return err
		}
		if body.NodeCount < currentNodes {
			return errFiberRingDecreaseBlocked
		}
		if body.NodeCount == currentNodes {
			return nil
		}
		return increaseFiberRingNodesTx(r.Context(), tx, id, currentNodes, body.NodeCount)
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "fiber_ring_not_found")
			return
		}
		if err == errFiberRingDecreaseBlocked {
			httputil.Error(w, http.StatusConflict, "fiber_ring_node_decrease_blocked")
			return
		}
		if err == errFiberRingClosingArcInUse {
			httputil.Error(w, http.StatusConflict, "fiber_ring_closing_arc_has_route")
			return
		}
		h.dbFailure(w, r, "increase_fiber_ring_nodes", err, "ring_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Numero nodi aumentato."})
}

func (h *Handler) handleCeaseFiberRing(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE anelli_fibra SET stato = 'Cessato' WHERE id_anello = ?`, id)
	if err != nil {
		h.dbFailure(w, r, "cease_fiber_ring", err, "ring_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "fiber_ring_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Anello fibra cessato."})
}

func (h *Handler) handleDeleteFiberRing(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	if _, err := decodeDestructiveBody(r); err != nil {
		invalidRequest(w, "double_confirmation_required")
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		item, found, err := getFiberRingTx(r.Context(), tx, id)
		if err != nil {
			return err
		}
		if !found {
			return sql.ErrNoRows
		}
		summary, err := h.fiberRingDeleteSummary(r.Context(), tx, item)
		if err != nil {
			return err
		}
		if !summary.Allowed {
			return errFiberRingDeleteBlocked
		}
		if _, err := tx.ExecContext(r.Context(), `DELETE tr FROM archi_tratta tr JOIN archi a ON a.id_arco = tr.id_arco WHERE a.id_anello = ?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM archi WHERE id_anello = ?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM nodi WHERE id_anello = ?`, id); err != nil {
			return err
		}
		result, err := tx.ExecContext(r.Context(), `DELETE FROM anelli_fibra WHERE id_anello = ?`, id)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return sql.ErrNoRows
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "fiber_ring_not_found")
			return
		}
		if err == errFiberRingDeleteBlocked {
			httputil.Error(w, http.StatusConflict, "fiber_ring_delete_blocked")
			return
		}
		h.dbFailure(w, r, "delete_fiber_ring", err, "ring_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Anello fibra eliminato."})
}

func (h *Handler) getFiberRing(ctx context.Context, id int) (FiberRing, bool, error) {
	return getFiberRingTx(ctx, h.grappa, id)
}

func getFiberRingTx(ctx context.Context, q sqlQueryer, id int) (FiberRing, bool, error) {
	row := q.QueryRowContext(ctx, fiberRingSelectSQL()+" WHERE af.id_anello = ?", id)
	item, err := scanFiberRing(row)
	if err == sql.ErrNoRows {
		return FiberRing{}, false, nil
	}
	return item, err == nil, err
}

func fiberRingSelectSQL() string {
	return `
		SELECT af.id_anello, af.nome, af.id_anagrafica, af.n_nodi, af.note, af.serialnumber, af.codice_ordine,
		       COALESCE(NULLIF(TRIM(af.stato), ''), 'Attivo') AS stato,
		       CASE WHEN COALESCE(TRIM(af.kml_file_path), '') <> '' THEN 1 ELSE 0 END AS kml_file_present,
		       (SELECT COUNT(*) FROM nodi n WHERE n.id_anello = af.id_anello) AS node_total,
		       (SELECT COUNT(*) FROM archi a WHERE a.id_anello = af.id_anello) AS arc_total,
		       (SELECT COUNT(*) FROM archi_tratta tr JOIN archi a2 ON a2.id_arco = tr.id_arco WHERE a2.id_anello = af.id_anello) AS route_total,
		       (SELECT COUNT(*) FROM mappa_tracciati_anelli m WHERE ` + fiberRingKMLAssociationSQL("m", "af") + `) AS kml_artifact_total
		FROM anelli_fibra af`
}

type fiberRingScanner interface {
	Scan(dest ...any) error
}

func scanFiberRing(scanner fiberRingScanner) (FiberRing, error) {
	var item FiberRing
	var customer sql.NullInt64
	var note, serial, order sql.NullString
	var kmlPresent int
	if err := scanner.Scan(&item.ID, &item.Name, &customer, &item.NodeCount, &note, &serial, &order, &item.Status, &kmlPresent, &item.NodeTotal, &item.ArcTotal, &item.RouteTotal, &item.KMLArtifactTotal); err != nil {
		return item, err
	}
	item.CustomerID = nullableInt(customer)
	item.Note = nullableString(note)
	item.SerialNumber = nullableString(serial)
	item.OrderCode = nullableString(order)
	item.KMLFilePresent = kmlPresent == 1
	item.TopologyConsistent = item.NodeCount == item.NodeTotal && item.NodeTotal == item.ArcTotal
	return item, nil
}

func (h *Handler) fiberRingDeleteSummary(ctx context.Context, q sqlQueryer, ring FiberRing) (DependencySummary, error) {
	summary := DependencySummary{
		Allowed: true,
		Counts:  map[string]int{},
		Details: []DependencyDetail{},
	}
	checks := []struct {
		key   string
		label string
		query string
	}{
		{"ringReferences", "Dati anello", `SELECT COUNT(*) FROM anelli_fibra WHERE id_anello = ? AND (id_anagrafica IS NOT NULL OR COALESCE(TRIM(note), '') <> '' OR COALESCE(TRIM(serialnumber), '') <> '' OR COALESCE(TRIM(codice_ordine), '') <> '')`},
		{"kml", "KML", `SELECT COUNT(*) FROM anelli_fibra af WHERE af.id_anello = ? AND (COALESCE(TRIM(af.kml_file_path), '') <> '' OR EXISTS (SELECT 1 FROM mappa_tracciati_anelli m WHERE ` + fiberRingKMLAssociationSQL("m", "af") + `))`},
		{"routes", "Dettagli tratte", `SELECT COUNT(*) FROM archi_tratta tr JOIN archi a ON a.id_arco = tr.id_arco WHERE a.id_anello = ?`},
		{"coordinates", "Coordinate nodi", `SELECT COUNT(*) FROM nodi WHERE id_anello = ? AND (latitudine IS NOT NULL OR longitudine IS NOT NULL)`},
		{"nodeReferences", "Dati nodi", `SELECT COUNT(*) FROM nodi WHERE id_anello = ? AND (id_foglio_linee IS NOT NULL OR id_anagrafica IS NOT NULL OR id_nodo_est IS NOT NULL OR id_nodo_ovest IS NOT NULL OR COALESCE(TRIM(indirizzo), '') <> '' OR COALESCE(TRIM(modello_switch), '') <> '' OR COALESCE(TRIM(serial_number_switch), '') <> '' OR COALESCE(TRIM(mac_address_switch), '') <> '' OR COALESCE(TRIM(indirizzo_ip), '') <> '' OR COALESCE(TRIM(indirizzo_ip_ups), '') <> '' OR COALESCE(TRIM(porta_est), '') <> '' OR COALESCE(TRIM(porta_ovest), '') <> '' OR COALESCE(TRIM(note), '') <> '')`},
		{"arcReferences", "Dati tratte", `SELECT COUNT(*) FROM archi WHERE id_anello = ? AND (COALESCE(distanza, 0) <> 0 OR COALESCE(attenuazione, 0) <> 0 OR COALESCE(TRIM(riferimento), '') <> '' OR COALESCE(TRIM(riferimento_metroweb), '') <> '' OR data_rilascio IS NOT NULL)`},
	}
	for _, check := range checks {
		var count int
		if err := q.QueryRowContext(ctx, check.query, ring.ID).Scan(&count); err != nil {
			return summary, err
		}
		summary.Counts[check.key] = count
		if count > 0 {
			summary.Allowed = false
			summary.Details = append(summary.Details, DependencyDetail{Label: check.label, Count: count})
		}
	}
	if !summary.Allowed {
		summary.Message = "Eliminazione bloccata: usa la cessazione per anelli con dati operativi."
	}
	return summary, nil
}

func fiberRingKMLAssociationSQL(kmlAlias string, ringAlias string) string {
	return kmlAlias + ".nome_anello = " + ringAlias + ".nome OR " + kmlAlias + ".nome = " + ringAlias + ".nome"
}

func fiberRingKMLAssociationByNameSQL(kmlAlias string) string {
	return kmlAlias + ".nome_anello = ? OR " + kmlAlias + ".nome = ?"
}

func updateFiberRingKMLAssociationTx(ctx context.Context, tx *sql.Tx, oldName string, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE mappa_tracciati_anelli
		SET nome_anello = ?
		WHERE nome_anello = ? OR nome = ?`, newName, oldName, oldName)
	return err
}
