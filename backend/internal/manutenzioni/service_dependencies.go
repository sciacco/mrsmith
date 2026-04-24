package manutenzioni

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleListServiceDependencies(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	args := []any{}
	where := []string{}
	active := strings.TrimSpace(r.URL.Query().Get("active"))
	if active == "" || active == "true" || active == "active" {
		where = append(where, "sd.is_active = true")
	} else if active == "false" || active == "inactive" {
		where = append(where, "sd.is_active = false")
	}
	if id := queryInt64(r, "upstream_service_id"); id > 0 {
		where = append(where, "sd.upstream_service_id = "+placeholder(&args, id))
	}
	if id := queryInt64(r, "downstream_service_id"); id > 0 {
		where = append(where, "sd.downstream_service_id = "+placeholder(&args, id))
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		pattern := "%" + q + "%"
		where = append(where, `(us.code ILIKE `+placeholder(&args, pattern)+`
			OR us.name_it ILIKE `+placeholder(&args, pattern)+`
			OR ds.code ILIKE `+placeholder(&args, pattern)+`
			OR ds.name_it ILIKE `+placeholder(&args, pattern)+`
			OR sd.dependency_type ILIKE `+placeholder(&args, pattern)+`)`)
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := h.maintenance.QueryContext(r.Context(), serviceDependencySelect()+whereSQL+serviceDependencyOrder(), args...)
	if err != nil {
		h.dbFailure(w, r, "service_dependency_list", err)
		return
	}
	defer rows.Close()
	items := []ServiceDependency{}
	for rows.Next() {
		item, err := scanServiceDependency(rows)
		if err != nil {
			h.dbFailure(w, r, "service_dependency_scan", err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "service_dependency_rows", err)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleGetServiceDependency(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_service_dependency_id")
		return
	}
	item, err := h.loadServiceDependency(r, id)
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "service_dependency_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "service_dependency_get", err, "service_dependency_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) handleCreateServiceDependency(w http.ResponseWriter, r *http.Request) {
	h.handleServiceDependencyMutation(w, r, 0)
}

func (h *Handler) handleUpdateServiceDependency(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_service_dependency_id")
		return
	}
	h.handleServiceDependencyMutation(w, r, id)
}

func (h *Handler) handleServiceDependencyMutation(w http.ResponseWriter, r *http.Request, id int64) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	var body serviceDependencyRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := validateServiceDependencyRequest(body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_service_dependency")
		return
	}
	var row *sql.Row
	if id == 0 {
		row = h.maintenance.QueryRowContext(
			r.Context(),
			serviceDependencyMutationSelect(`INSERT INTO maintenance.service_dependency (
					upstream_service_id, downstream_service_id, dependency_type, is_redundant,
					default_severity, source, is_active, metadata
				) VALUES ($1, $2, $3, $4, $5, 'manual', true, $6::jsonb)
				RETURNING *`),
			body.UpstreamServiceID,
			body.DownstreamServiceID,
			body.DependencyType,
			body.IsRedundant,
			body.DefaultSeverity,
			rawJSONOrDefault(body.Metadata),
		)
	} else {
		row = h.maintenance.QueryRowContext(
			r.Context(),
			serviceDependencyMutationSelect(`UPDATE maintenance.service_dependency
				SET upstream_service_id = $1,
					downstream_service_id = $2,
					dependency_type = $3,
					is_redundant = $4,
					default_severity = $5,
					metadata = $6::jsonb,
					updated_at = now()
				WHERE service_dependency_id = $7
				RETURNING *`),
			body.UpstreamServiceID,
			body.DownstreamServiceID,
			body.DependencyType,
			body.IsRedundant,
			body.DefaultSeverity,
			rawJSONOrDefault(body.Metadata),
			id,
		)
	}
	item, err := scanServiceDependency(row)
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "service_dependency_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "service_dependency_save", err, "service_dependency_id", id)
		return
	}
	status := http.StatusOK
	if id == 0 {
		status = http.StatusCreated
	}
	httputil.JSON(w, status, item)
}

func (h *Handler) handleDeactivateServiceDependency(w http.ResponseWriter, r *http.Request) {
	h.handleServiceDependencyActive(w, r, false)
}

func (h *Handler) handleReactivateServiceDependency(w http.ResponseWriter, r *http.Request) {
	h.handleServiceDependencyActive(w, r, true)
}

func (h *Handler) handleServiceDependencyActive(w http.ResponseWriter, r *http.Request, active bool) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_service_dependency_id")
		return
	}
	item, err := scanServiceDependency(h.maintenance.QueryRowContext(
		r.Context(),
		serviceDependencyMutationSelect(`UPDATE maintenance.service_dependency
			SET is_active = $1, updated_at = now()
			WHERE service_dependency_id = $2
			RETURNING *`),
		active,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "service_dependency_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "service_dependency_active", err, "service_dependency_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) loadServiceDependency(r *http.Request, id int64) (ServiceDependency, error) {
	return scanServiceDependency(h.maintenance.QueryRowContext(
		r.Context(),
		serviceDependencySelect()+" WHERE sd.service_dependency_id = $1"+serviceDependencyOrder(),
		id,
	))
}

func validateServiceDependencyRequest(body serviceDependencyRequest) error {
	if body.UpstreamServiceID <= 0 || body.DownstreamServiceID <= 0 || body.UpstreamServiceID == body.DownstreamServiceID {
		return errBadRequest
	}
	if !validDependencyType(body.DependencyType) || !validSeverity(body.DefaultSeverity) {
		return errBadRequest
	}
	return nil
}

func validDependencyType(value string) bool {
	switch strings.TrimSpace(value) {
	case "runs_on", "connects_through", "consumes", "depends_on":
		return true
	default:
		return false
	}
}

func queryInt64(r *http.Request, key string) int64 {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func serviceDependencySelect() string {
	return `SELECT
		sd.service_dependency_id,
		sd.upstream_service_id,
		us.code, us.name_it, us.name_en, us.description, us.sort_order, us.is_active,
		us.technical_domain_id, utd.name_it, us.target_type_id, utt.name_it, us.audience,
		sd.downstream_service_id,
		ds.code, ds.name_it, ds.name_en, ds.description, ds.sort_order, ds.is_active,
		ds.technical_domain_id, dtd.name_it, ds.target_type_id, dtt.name_it, ds.audience,
		sd.dependency_type,
		sd.is_redundant,
		sd.default_severity,
		sd.source,
		sd.is_active,
		sd.metadata,
		sd.created_at,
		sd.updated_at
	FROM maintenance.service_dependency sd
	JOIN maintenance.service_taxonomy us ON us.service_taxonomy_id = sd.upstream_service_id
	JOIN maintenance.technical_domain utd ON utd.technical_domain_id = us.technical_domain_id
	JOIN maintenance.target_type utt ON utt.target_type_id = us.target_type_id
	JOIN maintenance.service_taxonomy ds ON ds.service_taxonomy_id = sd.downstream_service_id
	JOIN maintenance.technical_domain dtd ON dtd.technical_domain_id = ds.technical_domain_id
	JOIN maintenance.target_type dtt ON dtt.target_type_id = ds.target_type_id`
}

func serviceDependencyMutationSelect(mutation string) string {
	return `WITH changed AS (` + mutation + `)
	SELECT
		sd.service_dependency_id,
		sd.upstream_service_id,
		us.code, us.name_it, us.name_en, us.description, us.sort_order, us.is_active,
		us.technical_domain_id, utd.name_it, us.target_type_id, utt.name_it, us.audience,
		sd.downstream_service_id,
		ds.code, ds.name_it, ds.name_en, ds.description, ds.sort_order, ds.is_active,
		ds.technical_domain_id, dtd.name_it, ds.target_type_id, dtt.name_it, ds.audience,
		sd.dependency_type,
		sd.is_redundant,
		sd.default_severity,
		sd.source,
		sd.is_active,
		sd.metadata,
		sd.created_at,
		sd.updated_at
	FROM changed sd
	JOIN maintenance.service_taxonomy us ON us.service_taxonomy_id = sd.upstream_service_id
	JOIN maintenance.technical_domain utd ON utd.technical_domain_id = us.technical_domain_id
	JOIN maintenance.target_type utt ON utt.target_type_id = us.target_type_id
	JOIN maintenance.service_taxonomy ds ON ds.service_taxonomy_id = sd.downstream_service_id
	JOIN maintenance.technical_domain dtd ON dtd.technical_domain_id = ds.technical_domain_id
	JOIN maintenance.target_type dtt ON dtt.target_type_id = ds.target_type_id`
}

func serviceDependencyOrder() string {
	return ` ORDER BY us.sort_order, us.name_it, ds.sort_order, ds.name_it, sd.service_dependency_id`
}

func scanServiceDependency(scanner interface {
	Scan(dest ...any) error
}) (ServiceDependency, error) {
	var item ServiceDependency
	var upstream, downstream ReferenceItem
	var upstreamNameEN, upstreamDescription, upstreamDomainName, upstreamTargetTypeName, upstreamAudience sql.NullString
	var downstreamNameEN, downstreamDescription, downstreamDomainName, downstreamTargetTypeName, downstreamAudience sql.NullString
	var upstreamDomainID, upstreamTargetTypeID, downstreamDomainID, downstreamTargetTypeID sql.NullInt64
	var metadata []byte
	err := scanner.Scan(
		&item.ServiceDependencyID,
		&item.UpstreamServiceID,
		&upstream.Code,
		&upstream.NameIT,
		&upstreamNameEN,
		&upstreamDescription,
		&upstream.SortOrder,
		&upstream.IsActive,
		&upstreamDomainID,
		&upstreamDomainName,
		&upstreamTargetTypeID,
		&upstreamTargetTypeName,
		&upstreamAudience,
		&item.DownstreamServiceID,
		&downstream.Code,
		&downstream.NameIT,
		&downstreamNameEN,
		&downstreamDescription,
		&downstream.SortOrder,
		&downstream.IsActive,
		&downstreamDomainID,
		&downstreamDomainName,
		&downstreamTargetTypeID,
		&downstreamTargetTypeName,
		&downstreamAudience,
		&item.DependencyType,
		&item.IsRedundant,
		&item.DefaultSeverity,
		&item.Source,
		&item.IsActive,
		&metadata,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return item, err
	}
	upstream.ID = item.UpstreamServiceID
	upstream.NameEN = nullStringValue(upstreamNameEN)
	upstream.Description = nullStringValue(upstreamDescription)
	upstream.TechnicalDomainID = nullInt64Value(upstreamDomainID)
	upstream.TechnicalDomainName = nullStringValue(upstreamDomainName)
	upstream.TargetTypeID = nullInt64Value(upstreamTargetTypeID)
	upstream.TargetTypeName = nullStringValue(upstreamTargetTypeName)
	upstream.Audience = nullStringValue(upstreamAudience)
	downstream.ID = item.DownstreamServiceID
	downstream.NameEN = nullStringValue(downstreamNameEN)
	downstream.Description = nullStringValue(downstreamDescription)
	downstream.TechnicalDomainID = nullInt64Value(downstreamDomainID)
	downstream.TechnicalDomainName = nullStringValue(downstreamDomainName)
	downstream.TargetTypeID = nullInt64Value(downstreamTargetTypeID)
	downstream.TargetTypeName = nullStringValue(downstreamTargetTypeName)
	downstream.Audience = nullStringValue(downstreamAudience)
	item.UpstreamService = upstream
	item.DownstreamService = downstream
	item.Metadata = rawJSONFromBytes(metadata)
	return item, nil
}
