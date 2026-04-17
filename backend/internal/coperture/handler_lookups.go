package coperture

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type locationOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type coverageDetailType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func parsePositivePathInt(r *http.Request, key string) (int, error) {
	value := strings.TrimSpace(r.PathValue(key))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, strconv.ErrSyntax
	}
	return parsed, nil
}

func (h *Handler) handleListStates(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	const query = `SELECT coperture.get_states()`
	var payload []byte
	if err := h.db.QueryRowContext(r.Context(), query).Scan(&payload); err != nil {
		h.dbFailure(w, r, "list_states", err)
		return
	}

	var states []locationOption
	if err := json.Unmarshal(payload, &states); err != nil {
		h.dbFailure(w, r, "decode_states", err)
		return
	}
	if states == nil {
		states = []locationOption{}
	}

	httputil.JSON(w, http.StatusOK, states)
}

func (h *Handler) handleListCities(w http.ResponseWriter, r *http.Request) {
	stateID, err := parsePositivePathInt(r, "stateId")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_state_id")
		return
	}
	h.handleLocationList(w, r,
		`SELECT id, name FROM coperture.network_coverage_cities WHERE network_coverage_state_id = $1 ORDER BY name`,
		stateID,
		"list_cities",
	)
}

func (h *Handler) handleListAddresses(w http.ResponseWriter, r *http.Request) {
	cityID, err := parsePositivePathInt(r, "cityId")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_city_id")
		return
	}
	h.handleLocationList(w, r,
		`SELECT id, name FROM coperture.network_coverage_addresses WHERE network_coverage_city_id = $1 ORDER BY name`,
		cityID,
		"list_addresses",
	)
}

func (h *Handler) handleListHouseNumbers(w http.ResponseWriter, r *http.Request) {
	addressID, err := parsePositivePathInt(r, "addressId")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_address_id")
		return
	}
	h.handleLocationList(w, r,
		`SELECT id, name FROM coperture.network_coverage_house_numbers WHERE network_coverage_address_id = $1 ORDER BY name`,
		addressID,
		"list_house_numbers",
	)
}

func (h *Handler) handleLocationList(w http.ResponseWriter, r *http.Request, query string, id int, operation string) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.db.QueryContext(r.Context(), query, id)
	if err != nil {
		h.dbFailure(w, r, operation, err)
		return
	}
	defer rows.Close()

	items := make([]locationOption, 0)
	for rows.Next() {
		var item locationOption
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			h.dbFailure(w, r, operation+"_scan", err)
			return
		}
		items = append(items, item)
	}
	if !h.rowsDone(w, r, rows, operation) {
		return
	}

	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) loadDetailTypes(r *http.Request) (map[int]string, error) {
	const query = `SELECT coperture.get_coverage_details_types()`
	var payload []byte
	if err := h.db.QueryRowContext(r.Context(), query).Scan(&payload); err != nil {
		return nil, err
	}

	var types []coverageDetailType
	if err := json.Unmarshal(payload, &types); err != nil {
		return nil, err
	}

	index := make(map[int]string, len(types))
	for _, item := range types {
		index[item.ID] = item.Name
	}
	return index, nil
}
