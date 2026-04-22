package manutenzioni

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	meta, ok := resourceMetas[r.PathValue("resource")]
	if !ok {
		appError(w, http.StatusNotFound, "config_resource_not_found")
		return
	}
	active := strings.TrimSpace(r.URL.Query().Get("active"))
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	args := []any{}
	where := []string{}
	if active == "" || active == "active" {
		where = append(where, "x.is_active = true")
	} else if active == "inactive" {
		where = append(where, "x.is_active = false")
	}
	if q != "" {
		pattern := "%" + q + "%"
		where = append(where, `(COALESCE(x.code, '') ILIKE `+placeholder(&args, pattern)+`
			OR COALESCE(x.name_it, '') ILIKE `+placeholder(&args, pattern)+`
			OR COALESCE(x.name_en, '') ILIKE `+placeholder(&args, pattern)+`
			OR COALESCE(x.description, '') ILIKE `+placeholder(&args, pattern)+`
			OR COALESCE(x.technical_domain_name, '') ILIKE `+placeholder(&args, pattern)+`)`)
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	query := configListQuery(meta, whereSQL)
	rows, err := h.maintenance.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "config_list", err, "resource", meta.Key)
		return
	}
	defer rows.Close()
	items := []ReferenceItem{}
	for rows.Next() {
		item, err := scanReferenceItem(rows)
		if err != nil {
			h.dbFailure(w, r, "config_list_scan", err, "resource", meta.Key)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "config_list_rows", err, "resource", meta.Key)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func configListQuery(meta resourceMeta, whereSQL string) string {
	switch meta.Kind {
	case resourceSite:
		return `SELECT * FROM (
			SELECT site_id AS id, code, name AS name_it, NULL::text AS name_en, NULL::text AS description,
				100 AS sort_order, is_active, city, country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name
			FROM maintenance.site
		) x` + whereSQL + ` ORDER BY x.code, x.name_it, x.id`
	case resourceService:
		return `SELECT * FROM (
			SELECT st.service_taxonomy_id AS id, st.code, st.name_it, st.name_en, st.description,
				st.sort_order, st.is_active, NULL::text AS city, NULL::text AS country_code,
				st.technical_domain_id, td.name_it AS technical_domain_name
			FROM maintenance.service_taxonomy st
			JOIN maintenance.technical_domain td ON td.technical_domain_id = st.technical_domain_id
		) x` + whereSQL + ` ORDER BY x.sort_order, x.name_it, x.id`
	default:
		return fmt.Sprintf(`SELECT * FROM (
			SELECT %s AS id, code, name_it, name_en, description, sort_order, is_active,
				NULL::text AS city, NULL::text AS country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name
			FROM %s
		) x`, meta.IDColumn, meta.Table) + whereSQL + ` ORDER BY x.sort_order, x.name_it, x.id`
	}
}

func (h *Handler) handleCreateConfig(w http.ResponseWriter, r *http.Request) {
	h.handleConfigMutation(w, r, 0)
}

func (h *Handler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_config_id")
		return
	}
	h.handleConfigMutation(w, r, id)
}

func (h *Handler) handleConfigMutation(w http.ResponseWriter, r *http.Request, id int64) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	meta, ok := resourceMetas[r.PathValue("resource")]
	if !ok {
		appError(w, http.StatusNotFound, "config_resource_not_found")
		return
	}
	var body configItemRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	body.Code = strings.TrimSpace(body.Code)
	body.NameIT = strings.TrimSpace(body.NameIT)
	if id == 0 && !validateCode(body.Code, meta.Kind == resourceSite) {
		appError(w, http.StatusBadRequest, "invalid_config_code")
		return
	}
	if body.NameIT == "" {
		appError(w, http.StatusBadRequest, "required_fields_missing")
		return
	}
	sortOrder := 100
	if body.SortOrder != nil {
		sortOrder = *body.SortOrder
	}
	isActive := true
	if body.IsActive != nil {
		isActive = *body.IsActive
	}

	var item ReferenceItem
	var err error
	if id == 0 {
		item, err = h.insertConfigItem(r, meta, body, sortOrder, isActive)
	} else {
		item, err = h.updateConfigItem(r, meta, id, body, sortOrder, isActive)
	}
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "config_item_not_found")
		return
	}
	if errors.Is(err, errBadRequest) {
		appError(w, http.StatusBadRequest, "invalid_config_item")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "config_save", err, "resource", meta.Key, "config_id", id)
		return
	}
	status := http.StatusOK
	if id == 0 {
		status = http.StatusCreated
	}
	httputil.JSON(w, status, item)
}

func (h *Handler) insertConfigItem(r *http.Request, meta resourceMeta, body configItemRequest, sortOrder int, isActive bool) (ReferenceItem, error) {
	switch meta.Kind {
	case resourceSite:
		return h.queryConfigItem(
			r,
			`INSERT INTO maintenance.site (code, name, city, country_code, is_active)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING site_id AS id, code, name AS name_it, NULL::text AS name_en, NULL::text AS description,
				100 AS sort_order, is_active, city, country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name`,
			body.Code,
			body.NameIT,
			nullStringPtr(body.City),
			nullStringPtr(body.CountryCode),
			isActive,
		)
	case resourceService:
		if body.TechnicalDomainID == nil || *body.TechnicalDomainID <= 0 {
			return ReferenceItem{}, errBadRequest
		}
		return h.queryConfigItem(
			r,
			`WITH inserted AS (
				INSERT INTO maintenance.service_taxonomy (code, technical_domain_id, name_it, name_en, description, sort_order, is_active)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				RETURNING *
			)
			SELECT i.service_taxonomy_id AS id, i.code, i.name_it, i.name_en, i.description, i.sort_order, i.is_active,
				NULL::text AS city, NULL::text AS country_code, i.technical_domain_id, td.name_it AS technical_domain_name
			FROM inserted i
			JOIN maintenance.technical_domain td ON td.technical_domain_id = i.technical_domain_id`,
			body.Code,
			*body.TechnicalDomainID,
			body.NameIT,
			nullStringPtr(body.NameEN),
			nullStringPtr(body.Description),
			sortOrder,
			isActive,
		)
	default:
		return h.queryConfigItem(
			r,
			fmt.Sprintf(`INSERT INTO %s (code, name_it, name_en, description, sort_order, is_active)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING %s AS id, code, name_it, name_en, description, sort_order, is_active,
				NULL::text AS city, NULL::text AS country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name`, meta.Table, meta.IDColumn),
			body.Code,
			body.NameIT,
			nullStringPtr(body.NameEN),
			nullStringPtr(body.Description),
			sortOrder,
			isActive,
		)
	}
}

func (h *Handler) updateConfigItem(r *http.Request, meta resourceMeta, id int64, body configItemRequest, sortOrder int, isActive bool) (ReferenceItem, error) {
	switch meta.Kind {
	case resourceSite:
		return h.queryConfigItem(
			r,
			`UPDATE maintenance.site SET name = $1, city = $2, country_code = $3, is_active = $4
			WHERE site_id = $5
			RETURNING site_id AS id, code, name AS name_it, NULL::text AS name_en, NULL::text AS description,
				100 AS sort_order, is_active, city, country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name`,
			body.NameIT,
			nullStringPtr(body.City),
			nullStringPtr(body.CountryCode),
			isActive,
			id,
		)
	case resourceService:
		if body.TechnicalDomainID == nil || *body.TechnicalDomainID <= 0 {
			return ReferenceItem{}, errBadRequest
		}
		return h.queryConfigItem(
			r,
			`WITH updated AS (
				UPDATE maintenance.service_taxonomy
				SET technical_domain_id = $1, name_it = $2, name_en = $3, description = $4, sort_order = $5, is_active = $6
				WHERE service_taxonomy_id = $7
				RETURNING *
			)
			SELECT u.service_taxonomy_id AS id, u.code, u.name_it, u.name_en, u.description, u.sort_order, u.is_active,
				NULL::text AS city, NULL::text AS country_code, u.technical_domain_id, td.name_it AS technical_domain_name
			FROM updated u
			JOIN maintenance.technical_domain td ON td.technical_domain_id = u.technical_domain_id`,
			*body.TechnicalDomainID,
			body.NameIT,
			nullStringPtr(body.NameEN),
			nullStringPtr(body.Description),
			sortOrder,
			isActive,
			id,
		)
	default:
		return h.queryConfigItem(
			r,
			fmt.Sprintf(`UPDATE %s
			SET name_it = $1, name_en = $2, description = $3, sort_order = $4, is_active = $5
			WHERE %s = $6
			RETURNING %s AS id, code, name_it, name_en, description, sort_order, is_active,
				NULL::text AS city, NULL::text AS country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name`, meta.Table, meta.IDColumn, meta.IDColumn),
			body.NameIT,
			nullStringPtr(body.NameEN),
			nullStringPtr(body.Description),
			sortOrder,
			isActive,
			id,
		)
	}
}

func (h *Handler) queryConfigItem(r *http.Request, query string, args ...any) (ReferenceItem, error) {
	return scanReferenceItem(h.maintenance.QueryRowContext(r.Context(), query, args...))
}

func (h *Handler) handleDeactivateConfig(w http.ResponseWriter, r *http.Request) {
	h.handleConfigActive(w, r, false)
}

func (h *Handler) handleReactivateConfig(w http.ResponseWriter, r *http.Request) {
	h.handleConfigActive(w, r, true)
}

func (h *Handler) handleConfigActive(w http.ResponseWriter, r *http.Request, active bool) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	meta, ok := resourceMetas[r.PathValue("resource")]
	if !ok {
		appError(w, http.StatusNotFound, "config_resource_not_found")
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_config_id")
		return
	}
	var query string
	switch meta.Kind {
	case resourceSite:
		query = `UPDATE maintenance.site SET is_active = $1 WHERE site_id = $2
			RETURNING site_id AS id, code, name AS name_it, NULL::text AS name_en, NULL::text AS description,
				100 AS sort_order, is_active, city, country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name`
	case resourceService:
		query = `WITH updated AS (
				UPDATE maintenance.service_taxonomy SET is_active = $1 WHERE service_taxonomy_id = $2 RETURNING *
			)
			SELECT u.service_taxonomy_id AS id, u.code, u.name_it, u.name_en, u.description, u.sort_order, u.is_active,
				NULL::text AS city, NULL::text AS country_code, u.technical_domain_id, td.name_it AS technical_domain_name
			FROM updated u
			JOIN maintenance.technical_domain td ON td.technical_domain_id = u.technical_domain_id`
	default:
		query = fmt.Sprintf(`UPDATE %s SET is_active = $1 WHERE %s = $2
			RETURNING %s AS id, code, name_it, name_en, description, sort_order, is_active,
				NULL::text AS city, NULL::text AS country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name`, meta.Table, meta.IDColumn, meta.IDColumn)
	}
	item, err := h.queryConfigItem(r, query, active, id)
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "config_item_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "config_active", err, "resource", meta.Key, "config_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func configIDPlaceholder(id int64) string {
	return strconv.FormatInt(id, 10)
}
