package manutenzioni

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type resourceKind string

const (
	resourceStandard resourceKind = "standard"
	resourceSite     resourceKind = "site"
	resourceService  resourceKind = "service"
)

type resourceMeta struct {
	Key          string
	Table        string
	IDColumn     string
	Kind         resourceKind
	BusinessName string
}

var resourceMetas = map[string]resourceMeta{
	"sites":             {Key: "sites", Table: "maintenance.site", IDColumn: "site_id", Kind: resourceSite, BusinessName: "Siti"},
	"technical-domains": {Key: "technical-domains", Table: "maintenance.technical_domain", IDColumn: "technical_domain_id", Kind: resourceStandard, BusinessName: "Domini tecnici"},
	"maintenance-kinds": {Key: "maintenance-kinds", Table: "maintenance.maintenance_kind", IDColumn: "maintenance_kind_id", Kind: resourceStandard, BusinessName: "Tipi manutenzione"},
	"customer-scopes":   {Key: "customer-scopes", Table: "maintenance.customer_scope", IDColumn: "customer_scope_id", Kind: resourceStandard, BusinessName: "Ambiti clienti"},
	"reason-classes":    {Key: "reason-classes", Table: "maintenance.reason_class", IDColumn: "reason_class_id", Kind: resourceStandard, BusinessName: "Motivi"},
	"impact-effects":    {Key: "impact-effects", Table: "maintenance.impact_effect", IDColumn: "impact_effect_id", Kind: resourceStandard, BusinessName: "Effetti impatto"},
	"quality-flags":     {Key: "quality-flags", Table: "maintenance.quality_flag", IDColumn: "quality_flag_id", Kind: resourceStandard, BusinessName: "Segnali qualita"},
	"target-types":      {Key: "target-types", Table: "maintenance.target_type", IDColumn: "target_type_id", Kind: resourceStandard, BusinessName: "Tipi target"},
	"notice-channels":   {Key: "notice-channels", Table: "maintenance.notice_channel", IDColumn: "notice_channel_id", Kind: resourceStandard, BusinessName: "Canali comunicazione"},
	"service-taxonomy":  {Key: "service-taxonomy", Table: "maintenance.service_taxonomy", IDColumn: "service_taxonomy_id", Kind: resourceService, BusinessName: "Servizi"},
}

func (h *Handler) handleReferenceData(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}

	selected := map[string][]int64{}
	if raw := strings.TrimSpace(r.URL.Query().Get("maintenance_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && id > 0 {
			var loadErr error
			selected, loadErr = h.selectedReferenceIDs(r.Context(), id)
			if loadErr != nil && !errors.Is(loadErr, sql.ErrNoRows) {
				h.dbFailure(w, r, "reference_selected_ids", loadErr, "maintenance_id", id)
				return
			}
		}
	}

	bundle := ReferenceData{}
	var err error
	if bundle.Sites, err = h.loadReferenceItems(r.Context(), resourceMetas["sites"], true, selected["sites"]); err != nil {
		h.dbFailure(w, r, "reference_sites", err)
		return
	}
	if bundle.TechnicalDomains, err = h.loadReferenceItems(r.Context(), resourceMetas["technical-domains"], true, selected["technical-domains"]); err != nil {
		h.dbFailure(w, r, "reference_technical_domains", err)
		return
	}
	if bundle.MaintenanceKinds, err = h.loadReferenceItems(r.Context(), resourceMetas["maintenance-kinds"], true, selected["maintenance-kinds"]); err != nil {
		h.dbFailure(w, r, "reference_kinds", err)
		return
	}
	if bundle.CustomerScopes, err = h.loadReferenceItems(r.Context(), resourceMetas["customer-scopes"], true, selected["customer-scopes"]); err != nil {
		h.dbFailure(w, r, "reference_scopes", err)
		return
	}
	if bundle.ServiceTaxonomy, err = h.loadReferenceItems(r.Context(), resourceMetas["service-taxonomy"], true, selected["service-taxonomy"]); err != nil {
		h.dbFailure(w, r, "reference_service_taxonomy", err)
		return
	}
	if bundle.ReasonClasses, err = h.loadReferenceItems(r.Context(), resourceMetas["reason-classes"], true, selected["reason-classes"]); err != nil {
		h.dbFailure(w, r, "reference_reasons", err)
		return
	}
	if bundle.ImpactEffects, err = h.loadReferenceItems(r.Context(), resourceMetas["impact-effects"], true, selected["impact-effects"]); err != nil {
		h.dbFailure(w, r, "reference_impacts", err)
		return
	}
	if bundle.QualityFlags, err = h.loadReferenceItems(r.Context(), resourceMetas["quality-flags"], true, selected["quality-flags"]); err != nil {
		h.dbFailure(w, r, "reference_quality_flags", err)
		return
	}
	if bundle.TargetTypes, err = h.loadReferenceItems(r.Context(), resourceMetas["target-types"], true, selected["target-types"]); err != nil {
		h.dbFailure(w, r, "reference_target_types", err)
		return
	}
	if bundle.NoticeChannels, err = h.loadReferenceItems(r.Context(), resourceMetas["notice-channels"], true, selected["notice-channels"]); err != nil {
		h.dbFailure(w, r, "reference_notice_channels", err)
		return
	}

	httputil.JSON(w, http.StatusOK, bundle)
}

func (h *Handler) selectedReferenceIDs(ctx context.Context, maintenanceID int64) (map[string][]int64, error) {
	result := map[string][]int64{}
	var kindID, domainID, scopeID int64
	var siteID sql.NullInt64
	if err := h.maintenance.QueryRowContext(
		ctx,
		`SELECT maintenance_kind_id, technical_domain_id, customer_scope_id, site_id
		FROM maintenance.maintenance
		WHERE maintenance_id = $1`,
		maintenanceID,
	).Scan(&kindID, &domainID, &scopeID, &siteID); err != nil {
		return result, err
	}
	result["maintenance-kinds"] = []int64{kindID}
	result["technical-domains"] = []int64{domainID}
	result["customer-scopes"] = []int64{scopeID}
	if siteID.Valid {
		result["sites"] = []int64{siteID.Int64}
	}
	type selectedQuery struct {
		key   string
		query string
	}
	queries := []selectedQuery{
		{"service-taxonomy", `SELECT service_taxonomy_id FROM maintenance.maintenance_service_taxonomy WHERE maintenance_id = $1`},
		{"reason-classes", `SELECT reason_class_id FROM maintenance.maintenance_reason_class WHERE maintenance_id = $1`},
		{"impact-effects", `SELECT impact_effect_id FROM maintenance.maintenance_impact_effect WHERE maintenance_id = $1`},
		{"quality-flags", `SELECT quality_flag_id FROM maintenance.maintenance_quality_flag WHERE maintenance_id = $1`},
		{"target-types", `SELECT target_type_id FROM maintenance.maintenance_target WHERE maintenance_id = $1`},
		{"notice-channels", `SELECT notice_channel_id FROM maintenance.notice WHERE maintenance_id = $1`},
	}
	for _, item := range queries {
		ids, err := h.queryIDs(ctx, item.query, maintenanceID)
		if err != nil {
			return result, err
		}
		result[item.key] = ids
	}
	return result, nil
}

func (h *Handler) queryIDs(ctx context.Context, query string, id int64) ([]int64, error) {
	rows, err := h.maintenance.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []int64{}
	for rows.Next() {
		var value int64
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (h *Handler) loadReferenceItems(ctx context.Context, meta resourceMeta, activeOnly bool, includeIDs []int64) ([]ReferenceItem, error) {
	whereParts := []string{}
	args := []any{}
	if activeOnly {
		whereParts = append(whereParts, "(x.is_active = true"+includeIDCondition(&args, "x.id", includeIDs)+")")
	}
	where := ""
	if len(whereParts) > 0 {
		where = " WHERE " + strings.Join(whereParts, " AND ")
	}

	var query string
	switch meta.Kind {
	case resourceSite:
		query = `SELECT * FROM (
			SELECT site_id AS id, code, name AS name_it, NULL::text AS name_en, NULL::text AS description,
				100 AS sort_order, is_active, city, country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name
			FROM maintenance.site
		) x` + where + ` ORDER BY x.code, x.name_it, x.id`
	case resourceService:
		query = `SELECT * FROM (
			SELECT st.service_taxonomy_id AS id, st.code, st.name_it, st.name_en, st.description,
				st.sort_order, st.is_active, NULL::text AS city, NULL::text AS country_code,
				st.technical_domain_id, td.name_it AS technical_domain_name
			FROM maintenance.service_taxonomy st
			JOIN maintenance.technical_domain td ON td.technical_domain_id = st.technical_domain_id
		) x` + where + ` ORDER BY x.sort_order, x.name_it, x.id`
	default:
		query = fmt.Sprintf(`SELECT * FROM (
			SELECT %s AS id, code, name_it, name_en, description, sort_order, is_active,
				NULL::text AS city, NULL::text AS country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name
			FROM %s
		) x`, meta.IDColumn, meta.Table) + where + ` ORDER BY x.sort_order, x.name_it, x.id`
	}

	rows, err := h.maintenance.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ReferenceItem{}
	for rows.Next() {
		item, err := scanReferenceItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func includeIDCondition(args *[]any, column string, ids []int64) string {
	clean := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return ")"
	}
	holders := make([]string, 0, len(clean))
	for _, id := range clean {
		holders = append(holders, placeholder(args, id))
	}
	return " OR " + column + " IN (" + strings.Join(holders, ", ") + "))"
}

func scanReferenceItem(scanner interface {
	Scan(dest ...any) error
}) (ReferenceItem, error) {
	var item ReferenceItem
	var nameEN, description, city, country, domainName sql.NullString
	var domainID sql.NullInt64
	err := scanner.Scan(
		&item.ID,
		&item.Code,
		&item.NameIT,
		&nameEN,
		&description,
		&item.SortOrder,
		&item.IsActive,
		&city,
		&country,
		&domainID,
		&domainName,
	)
	if err != nil {
		return item, err
	}
	item.NameEN = nullStringValue(nameEN)
	item.Description = nullStringValue(description)
	item.City = nullStringValue(city)
	item.CountryCode = nullStringValue(country)
	item.TechnicalDomainID = nullInt64Value(domainID)
	item.TechnicalDomainName = nullStringValue(domainName)
	return item, nil
}

func (h *Handler) handleSearchCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistraDB(w) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		httputil.JSON(w, http.StatusOK, []CustomerSearchItem{})
		return
	}
	pageSize := queryPositiveInt(r, "page_size", 20)
	if pageSize > 50 {
		pageSize = 50
	}
	pattern := "%" + q + "%"
	rows, err := h.mistra.QueryContext(
		r.Context(),
		`SELECT id, COALESCE(name, '')
		FROM customers.customer
		WHERE CAST(id AS text) ILIKE $1 OR COALESCE(name, '') ILIKE $1
		ORDER BY name NULLS LAST, id
		LIMIT $2`,
		pattern,
		pageSize,
	)
	if err != nil {
		h.dbFailure(w, r, "search_customers", err)
		return
	}
	defer rows.Close()

	items := []CustomerSearchItem{}
	for rows.Next() {
		var item CustomerSearchItem
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			h.dbFailure(w, r, "search_customers_scan", err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "search_customers_rows", err)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) enrichCustomerNames(ctx context.Context, ids []int64) map[int64]string {
	result := map[int64]string{}
	for _, id := range ids {
		if id > 0 {
			result[id] = fmt.Sprintf("Cliente #%d", id)
		}
	}
	if h.mistra == nil || len(result) == 0 {
		return result
	}

	args := []any{}
	holders := make([]string, 0, len(result))
	for id := range result {
		holders = append(holders, placeholder(&args, id))
	}
	rows, err := h.mistra.QueryContext(
		ctx,
		`SELECT id, COALESCE(name, '') FROM customers.customer WHERE id IN (`+strings.Join(holders, ", ")+`)`,
		args...,
	)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err == nil && strings.TrimSpace(name) != "" {
			result[id] = name
		}
	}
	return result
}
