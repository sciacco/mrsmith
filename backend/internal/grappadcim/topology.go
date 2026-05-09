package grappadcim

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleGetFiberRingTopology(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	ring, found, err := h.getFiberRing(r.Context(), id)
	if err != nil {
		h.dbFailure(w, r, "get_fiber_ring_topology_ring", err, "ring_id", id)
		return
	}
	if !found {
		httputil.Error(w, http.StatusNotFound, "fiber_ring_not_found")
		return
	}
	nodes, err := h.listFiberRingNodes(r.Context(), id)
	if err != nil {
		h.dbFailure(w, r, "list_fiber_ring_nodes", err, "ring_id", id)
		return
	}
	arcs, err := h.listFiberRingArcs(r.Context(), id)
	if err != nil {
		h.dbFailure(w, r, "list_fiber_ring_arcs", err, "ring_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, FiberRingTopology{Ring: ring, Nodes: nodes, Arcs: arcs})
}

func (h *Handler) handleUpdateFiberRingNode(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	ringID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	nodeID, err := parsePathInt(r, "nodeId")
	if err != nil {
		invalidRequest(w, "invalid_node_id")
		return
	}
	var body FiberRingNodePatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_node_payload")
		return
	}
	sets := []string{}
	args := []any{}
	addString := func(column string, value *string) {
		if value != nil {
			sets = append(sets, column+" = ?")
			args = append(args, optionalTrimmed(value))
		}
	}
	addStringRequired := func(column string, value *string) {
		if value != nil {
			sets = append(sets, column+" = ?")
			args = append(args, strings.TrimSpace(*value))
		}
	}
	addInt := func(column string, value *int) {
		if value != nil {
			sets = append(sets, column+" = ?")
			args = append(args, *value)
		}
	}
	addFloat := func(column string, value *float64) {
		if value != nil {
			sets = append(sets, column+" = ?")
			args = append(args, *value)
		}
	}
	addStringRequired("identificativo", body.Identifier)
	addStringRequired("indirizzo", body.Address)
	addInt("id_foglio_linee", body.LineSheetID)
	addInt("id_anagrafica", body.CustomerID)
	addFloat("longitudine", body.Longitude)
	addFloat("latitudine", body.Latitude)
	addInt("posizione", body.Position)
	addString("modello_switch", body.SwitchModel)
	addString("serial_number_switch", body.SwitchSerialNumber)
	addString("mac_address_switch", body.SwitchMacAddress)
	addString("indirizzo_ip", body.IPAddress)
	addString("indirizzo_ip_ups", body.UPSIPAddress)
	addString("eaps_master_node", body.EAPSMasterNode)
	addInt("id_nodo_est", body.EastNodeID)
	addString("porta_est", body.EastPort)
	addString("primary_porta_est", body.PrimaryEastPort)
	addString("secondary_porta_est", body.SecondaryEastPort)
	addString("tipo_transceiver_est", body.EastTransceiverType)
	addInt("id_nodo_ovest", body.WestNodeID)
	addString("porta_ovest", body.WestPort)
	addString("primary_porta_ovest", body.PrimaryWestPort)
	addString("secondary_porta_ovest", body.SecondaryWestPort)
	addString("tipo_transceiver_ovest", body.WestTransceiverType)
	addString("note", body.Note)
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: nodeID, Message: "Nessuna modifica."})
		return
	}
	args = append(args, nodeID, ringID)
	result, err := h.grappa.ExecContext(r.Context(), "UPDATE nodi SET "+strings.Join(sets, ", ")+" WHERE id_nodo = ? AND id_anello = ?", args...)
	if err != nil {
		h.dbFailure(w, r, "update_fiber_ring_node", err, "ring_id", ringID, "node_id", nodeID)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "fiber_ring_node_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: nodeID, Message: "Nodo aggiornato."})
}

func (h *Handler) handleUpdateFiberRingArc(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	ringID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	arcID, err := parsePathInt(r, "arcId")
	if err != nil {
		invalidRequest(w, "invalid_arc_id")
		return
	}
	var body FiberRingArcPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_arc_payload")
		return
	}
	sets := []string{}
	args := []any{}
	if body.Distance != nil {
		sets = append(sets, "distanza = ?")
		args = append(args, *body.Distance)
	}
	if body.Attenuation != nil {
		sets = append(sets, "attenuazione = ?")
		args = append(args, *body.Attenuation)
	}
	if body.Reference != nil {
		sets = append(sets, "riferimento = ?")
		args = append(args, optionalTrimmed(body.Reference))
	}
	if body.MetrowebReference != nil {
		sets = append(sets, "riferimento_metroweb = ?")
		args = append(args, optionalTrimmed(body.MetrowebReference))
	}
	if body.ReleasedAt != nil {
		sets = append(sets, "data_rilascio = ?")
		args = append(args, optionalTrimmed(body.ReleasedAt))
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: arcID, Message: "Nessuna modifica."})
		return
	}
	args = append(args, arcID, ringID)
	result, err := h.grappa.ExecContext(r.Context(), "UPDATE archi SET "+strings.Join(sets, ", ")+" WHERE id_arco = ? AND id_anello = ?", args...)
	if err != nil {
		h.dbFailure(w, r, "update_fiber_ring_arc", err, "ring_id", ringID, "arc_id", arcID)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "fiber_ring_arc_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: arcID, Message: "Tratta aggiornata."})
}

func (h *Handler) handleReplaceFiberRingRoutes(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	ringID, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_fiber_ring_id")
		return
	}
	var body FiberRingRoutesInput
	if err := decodeJSONBody(r, &body); err != nil || body.ArcID <= 0 {
		invalidRequest(w, "invalid_routes_payload")
		return
	}
	if err := withTx(r.Context(), h.grappa, func(tx *sql.Tx) error {
		var lockedArcID int
		if err := tx.QueryRowContext(r.Context(), `SELECT id_arco FROM archi WHERE id_arco = ? AND id_anello = ? FOR UPDATE`, body.ArcID, ringID).Scan(&lockedArcID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM archi_tratta WHERE id_arco = ?`, body.ArcID); err != nil {
			return err
		}
		for _, route := range body.Routes {
			if _, err := tx.ExecContext(r.Context(), `
				INSERT INTO archi_tratta (
					id_arco, identificativo, p_armadio, p_livello, p_cavo, p_fibre, p_segmento_ottico,
					d_armadio, d_livello, d_cavo, d_fibre, d_segmento_ottico, lunghezza_tratta_m, lunghezza_drop_m
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				body.ArcID, optionalTrimmed(route.Identifier), optionalTrimmed(route.SourceCabinet), optionalTrimmed(route.SourceLevel),
				optionalTrimmed(route.SourceCable), optionalTrimmed(route.SourceFibers), optionalTrimmed(route.SourceOpticalSegment),
				optionalTrimmed(route.DestinationCabinet), optionalTrimmed(route.DestinationLevel), optionalTrimmed(route.DestinationCable),
				optionalTrimmed(route.DestinationFibers), optionalTrimmed(route.DestinationOpticalSegment), route.RouteLengthMeters, route.DropLengthMeters); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			httputil.Error(w, http.StatusNotFound, "fiber_ring_arc_not_found")
			return
		}
		h.dbFailure(w, r, "replace_fiber_ring_routes", err, "ring_id", ringID, "arc_id", body.ArcID)
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: body.ArcID, Message: "Dettagli tratta aggiornati."})
}

func createFiberRingTopologyTx(ctx context.Context, tx *sql.Tx, ringID int, nodeCount int) error {
	nodeIDs := make([]int, 0, nodeCount)
	for idx := 1; idx <= nodeCount; idx++ {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO nodi (identificativo, indirizzo, id_anello, posizione)
			VALUES (?, '', ?, ?)`, strconvItoa(idx), ringID, idx*100)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		nodeIDs = append(nodeIDs, int(id))
	}
	for idx, fromID := range nodeIDs {
		toID := nodeIDs[(idx+1)%len(nodeIDs)]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO archi (id_anello, id_nodo_da, id_nodo_a, distanza, attenuazione)
			VALUES (?, ?, ?, 0, 0)`, ringID, fromID, toID); err != nil {
			return err
		}
	}
	return nil
}

func increaseFiberRingNodesTx(ctx context.Context, tx *sql.Tx, ringID int, currentNodes int, targetNodes int) error {
	nodes, err := listFiberRingNodesTx(ctx, tx, ringID)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return createFiberRingTopologyTx(ctx, tx, ringID, targetNodes)
	}
	firstNodeID := nodes[0].ID
	lastNodeID := nodes[len(nodes)-1].ID
	var closingArcID int
	var distance, attenuation sql.NullFloat64
	var ref, metro sql.NullString
	var released sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT id_arco, distanza, attenuazione, riferimento, riferimento_metroweb, data_rilascio
		FROM archi
		WHERE id_anello = ? AND id_nodo_da = ? AND id_nodo_a = ?
		FOR UPDATE`, ringID, lastNodeID, firstNodeID).Scan(&closingArcID, &distance, &attenuation, &ref, &metro, &released); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
		closingArcID = 0
	}
	if closingArcID > 0 {
		var routeCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM archi_tratta WHERE id_arco = ?`, closingArcID).Scan(&routeCount); err != nil {
			return err
		}
		if routeCount > 0 || (distance.Valid && distance.Float64 != 0) || (attenuation.Valid && attenuation.Float64 != 0) || nullableString(ref) != nil || nullableString(metro) != nil || released.Valid {
			return errFiberRingClosingArcInUse
		}
	}
	newNodeIDs := []int{}
	for idx := currentNodes + 1; idx <= targetNodes; idx++ {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO nodi (identificativo, indirizzo, id_anello, posizione)
			VALUES (?, '', ?, ?)`, strconvItoa(idx), ringID, idx*100)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		newNodeIDs = append(newNodeIDs, int(id))
	}
	if len(newNodeIDs) == 0 {
		return nil
	}
	if closingArcID > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE archi SET id_nodo_a = ? WHERE id_arco = ?`, newNodeIDs[0], closingArcID); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO archi (id_anello, id_nodo_da, id_nodo_a, distanza, attenuazione)
			VALUES (?, ?, ?, 0, 0)`, ringID, lastNodeID, newNodeIDs[0]); err != nil {
			return err
		}
	}
	for idx, fromID := range newNodeIDs {
		toID := firstNodeID
		if idx < len(newNodeIDs)-1 {
			toID = newNodeIDs[idx+1]
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO archi (id_anello, id_nodo_da, id_nodo_a, distanza, attenuazione)
			VALUES (?, ?, ?, 0, 0)`, ringID, fromID, toID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE anelli_fibra SET n_nodi = ? WHERE id_anello = ?`, targetNodes, ringID)
	return err
}

func (h *Handler) listFiberRingNodes(ctx context.Context, ringID int) ([]FiberRingNode, error) {
	return listFiberRingNodesTx(ctx, h.grappa, ringID)
}

func listFiberRingNodesTx(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, ringID int) ([]FiberRingNode, error) {
	rows, err := q.QueryContext(ctx, fiberRingNodeSelectSQL()+` WHERE n.id_anello = ? ORDER BY COALESCE(n.posizione, 0), n.id_nodo`, ringID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []FiberRingNode{}
	for rows.Next() {
		item, err := scanFiberRingNode(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *Handler) listFiberRingArcs(ctx context.Context, ringID int) ([]FiberRingArc, error) {
	rows, err := h.grappa.QueryContext(ctx, fiberRingArcSelectSQL()+` WHERE a.id_anello = ? ORDER BY a.id_arco`, ringID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []FiberRingArc{}
	arcIDs := []int{}
	for rows.Next() {
		item, err := scanFiberRingArc(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		arcIDs = append(arcIDs, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	routes, err := h.routesByArc(ctx, arcIDs)
	if err != nil {
		return nil, err
	}
	for idx := range items {
		items[idx].Routes = routes[items[idx].ID]
	}
	return items, nil
}

func fiberRingNodeSelectSQL() string {
	return `
		SELECT n.id_nodo, n.identificativo, n.indirizzo, n.id_foglio_linee, n.id_anagrafica, n.id_anello,
		       n.longitudine, n.latitudine, n.posizione, n.modello_switch, n.serial_number_switch,
		       n.mac_address_switch, n.indirizzo_ip, n.indirizzo_ip_ups, n.eaps_master_node,
		       n.id_nodo_est, n.porta_est, n.primary_porta_est, n.secondary_porta_est, n.tipo_transceiver_est,
		       n.id_nodo_ovest, n.porta_ovest, n.primary_porta_ovest, n.secondary_porta_ovest, n.tipo_transceiver_ovest,
		       n.note
		FROM nodi n`
}

type fiberRingNodeScanner interface {
	Scan(dest ...any) error
}

func scanFiberRingNode(scanner fiberRingNodeScanner) (FiberRingNode, error) {
	var item FiberRingNode
	var line, customer, position, eastNode, westNode sql.NullInt64
	var longitude, latitude sql.NullFloat64
	var switchModel, switchSerial, switchMac, ip, upsIP, eaps, eastPort, primaryEast, secondaryEast, eastTransceiver sql.NullString
	var westPort, primaryWest, secondaryWest, westTransceiver, note sql.NullString
	if err := scanner.Scan(
		&item.ID, &item.Identifier, &item.Address, &line, &customer, &item.RingID, &longitude, &latitude, &position,
		&switchModel, &switchSerial, &switchMac, &ip, &upsIP, &eaps, &eastNode, &eastPort, &primaryEast,
		&secondaryEast, &eastTransceiver, &westNode, &westPort, &primaryWest, &secondaryWest, &westTransceiver, &note,
	); err != nil {
		return item, err
	}
	item.LineSheetID = nullableInt(line)
	item.CustomerID = nullableInt(customer)
	item.Longitude = nullableFloat(longitude)
	item.Latitude = nullableFloat(latitude)
	item.Position = nullableInt(position)
	item.SwitchModel = nullableString(switchModel)
	item.SwitchSerialNumber = nullableString(switchSerial)
	item.SwitchMacAddress = nullableString(switchMac)
	item.IPAddress = nullableString(ip)
	item.UPSIPAddress = nullableString(upsIP)
	item.EAPSMasterNode = nullableString(eaps)
	item.EastNodeID = nullableInt(eastNode)
	item.EastPort = nullableString(eastPort)
	item.PrimaryEastPort = nullableString(primaryEast)
	item.SecondaryEastPort = nullableString(secondaryEast)
	item.EastTransceiverType = nullableString(eastTransceiver)
	item.WestNodeID = nullableInt(westNode)
	item.WestPort = nullableString(westPort)
	item.PrimaryWestPort = nullableString(primaryWest)
	item.SecondaryWestPort = nullableString(secondaryWest)
	item.WestTransceiverType = nullableString(westTransceiver)
	item.Note = nullableString(note)
	return item, nil
}

func fiberRingArcSelectSQL() string {
	return `
		SELECT a.id_arco, a.id_anello, a.id_nodo_da, a.id_nodo_a, nd.identificativo, na.identificativo,
		       a.distanza, a.attenuazione, a.riferimento, a.riferimento_metroweb, a.data_rilascio
		FROM archi a
		LEFT JOIN nodi nd ON nd.id_nodo = a.id_nodo_da
		LEFT JOIN nodi na ON na.id_nodo = a.id_nodo_a`
}

type fiberRingArcScanner interface {
	Scan(dest ...any) error
}

func scanFiberRingArc(scanner fiberRingArcScanner) (FiberRingArc, error) {
	var item FiberRingArc
	var fromIdentifier, toIdentifier, ref, metro sql.NullString
	var distance, attenuation sql.NullFloat64
	var released sql.NullTime
	if err := scanner.Scan(&item.ID, &item.RingID, &item.FromNodeID, &item.ToNodeID, &fromIdentifier, &toIdentifier, &distance, &attenuation, &ref, &metro, &released); err != nil {
		return item, err
	}
	item.FromIdentifier = nullableString(fromIdentifier)
	item.ToIdentifier = nullableString(toIdentifier)
	item.Distance = nullableFloat(distance)
	item.Attenuation = nullableFloat(attenuation)
	item.Reference = nullableString(ref)
	item.MetrowebReference = nullableString(metro)
	item.ReleasedAt = nullableDate(released)
	return item, nil
}

func (h *Handler) routesByArc(ctx context.Context, arcIDs []int) (map[int][]FiberRingRoute, error) {
	result := map[int][]FiberRingRoute{}
	if len(arcIDs) == 0 {
		return result, nil
	}
	query := `SELECT id_tratta, id_arco, identificativo, p_armadio, p_livello, p_cavo, p_fibre, p_segmento_ottico,
	                 d_armadio, d_livello, d_cavo, d_fibre, d_segmento_ottico, lunghezza_tratta_m, lunghezza_drop_m
	          FROM archi_tratta WHERE id_arco IN (` + placeholders(len(arcIDs)) + `) ORDER BY id_arco, id_tratta`
	args := make([]any, 0, len(arcIDs))
	for _, id := range arcIDs {
		args = append(args, id)
	}
	rows, err := h.grappa.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item FiberRingRoute
		var identifier, pArmadio, pLivello, pCavo, pFibre, pSegmento sql.NullString
		var dArmadio, dLivello, dCavo, dFibre, dSegmento sql.NullString
		var routeLength, dropLength sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.ArcID, &identifier, &pArmadio, &pLivello, &pCavo, &pFibre, &pSegmento, &dArmadio, &dLivello, &dCavo, &dFibre, &dSegmento, &routeLength, &dropLength); err != nil {
			return result, err
		}
		item.Identifier = nullableString(identifier)
		item.SourceCabinet = nullableString(pArmadio)
		item.SourceLevel = nullableString(pLivello)
		item.SourceCable = nullableString(pCavo)
		item.SourceFibers = nullableString(pFibre)
		item.SourceOpticalSegment = nullableString(pSegmento)
		item.DestinationCabinet = nullableString(dArmadio)
		item.DestinationLevel = nullableString(dLivello)
		item.DestinationCable = nullableString(dCavo)
		item.DestinationFibers = nullableString(dFibre)
		item.DestinationOpticalSegment = nullableString(dSegmento)
		item.RouteLengthMeters = nullableFloat(routeLength)
		item.DropLengthMeters = nullableFloat(dropLength)
		result[item.ArcID] = append(result[item.ArcID], item)
	}
	return result, rows.Err()
}

func strconvItoa(value int) string {
	return strings.TrimSpace(strconvFormatInt(int64(value)))
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
