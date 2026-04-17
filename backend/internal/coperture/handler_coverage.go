package coperture

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type coverageProfile struct {
	Name string `json:"name"`
}

type coverageDetail struct {
	TypeID   int    `json:"type_id"`
	TypeName string `json:"type_name"`
	Value    string `json:"value"`
}

type coverageResult struct {
	CoverageID   string            `json:"coverage_id"`
	OperatorID   int               `json:"operator_id"`
	OperatorName string            `json:"operator_name"`
	LogoURL      string            `json:"logo_url"`
	Tech         string            `json:"tech"`
	Profiles     []coverageProfile `json:"profiles"`
	Details      []coverageDetail  `json:"details"`
}

type sourceCoverageDetail struct {
	TypeID   int    `json:"type_id"`
	TypeName string `json:"type_name"`
	Value    string `json:"value"`
}

func (h *Handler) handleListCoverage(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	houseNumberID, err := parsePositivePathInt(r, "houseNumberId")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_house_number_id")
		return
	}

	detailTypes, err := h.loadDetailTypes(r)
	if err != nil {
		h.dbFailure(w, r, "load_detail_types", err)
		return
	}

	const query = `SELECT
  v.coverage_id::text,
  v.operator_id,
  COALESCE(o.name, '') AS operator_name,
  COALESCE(o.logo_url, '') AS logo_url,
  COALESCE(v.tech, '') AS tech,
  COALESCE(v.profiles::text, '[]') AS profiles,
  COALESCE(v.details::text, '[]') AS details
FROM coperture.v_get_coverage AS v
LEFT JOIN coperture.network_coverage_operators AS o ON o.id = v.operator_id
WHERE v.house_number_id = $1
ORDER BY v.operator, v.tech`

	rows, err := h.db.QueryContext(r.Context(), query, houseNumberID)
	if err != nil {
		h.dbFailure(w, r, "list_coverage", err)
		return
	}
	defer rows.Close()

	results := make([]coverageResult, 0)
	for rows.Next() {
		var (
			result      coverageResult
			profilesRaw []byte
			detailsRaw  []byte
		)

		if err := rows.Scan(
			&result.CoverageID,
			&result.OperatorID,
			&result.OperatorName,
			&result.LogoURL,
			&result.Tech,
			&profilesRaw,
			&detailsRaw,
		); err != nil {
			h.dbFailure(w, r, "list_coverage_scan", err)
			return
		}

		profiles, err := decodeProfiles(profilesRaw)
		if err != nil {
			h.dbFailure(w, r, "decode_profiles", err)
			return
		}
		details, err := decodeDetails(detailsRaw, detailTypes)
		if err != nil {
			h.dbFailure(w, r, "decode_details", err)
			return
		}

		result.Profiles = profiles
		result.Details = details
		results = append(results, result)
	}
	if !h.rowsDone(w, r, rows, "list_coverage") {
		return
	}

	httputil.JSON(w, http.StatusOK, results)
}

func decodeProfiles(raw []byte) ([]coverageProfile, error) {
	if len(raw) == 0 {
		return []coverageProfile{}, nil
	}

	var profiles []coverageProfile
	if err := json.Unmarshal(raw, &profiles); err != nil {
		return nil, err
	}
	if profiles == nil {
		return []coverageProfile{}, nil
	}
	return profiles, nil
}

func decodeDetails(raw []byte, detailTypes map[int]string) ([]coverageDetail, error) {
	if len(raw) == 0 {
		return []coverageDetail{}, nil
	}

	var source []sourceCoverageDetail
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	if source == nil {
		return []coverageDetail{}, nil
	}

	details := make([]coverageDetail, 0, len(source))
	for _, item := range source {
		typeName := item.TypeName
		if typeName == "" {
			typeName = detailTypes[item.TypeID]
		}
		details = append(details, coverageDetail{
			TypeID:   item.TypeID,
			TypeName: typeName,
			Value:    strings.TrimSuffix(item.Value, "0000"),
		})
	}
	return details, nil
}
