package manutenzioni

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type maintenanceListFilterOptions struct {
	includeScheduledRange bool
}

const maintenanceListFrom = ` FROM maintenance.maintenance m
		JOIN maintenance.maintenance_kind mk ON mk.maintenance_kind_id = m.maintenance_kind_id
		JOIN maintenance.technical_domain td ON td.technical_domain_id = m.technical_domain_id
		LEFT JOIN maintenance.customer_scope cs ON cs.customer_scope_id = m.customer_scope_id
		LEFT JOIN maintenance.site s ON s.site_id = m.site_id
		LEFT JOIN maintenance.v_current_window vcw ON vcw.maintenance_id = m.maintenance_id`

const maintenanceListSelect = `SELECT
			m.maintenance_id,
			COALESCE(m.code, ''),
			m.title_it,
			m.title_en,
			m.status,
			mk.maintenance_kind_id, mk.code, mk.name_it, mk.name_en, mk.description, mk.sort_order, mk.is_active,
			td.technical_domain_id, td.code, td.name_it, td.name_en, td.description, td.sort_order, td.is_active,
			cs.customer_scope_id, cs.code, cs.name_it, cs.name_en, cs.description, cs.sort_order, cs.is_active,
			s.site_id, s.code, s.name, s.city, s.country_code, s.is_active, s.scope,
			vcw.maintenance_window_id, vcw.seq_no, vcw.window_status, vcw.scheduled_start_at, vcw.scheduled_end_at, vcw.expected_downtime_minutes,
			COALESCE(primary_service.name_it, primary_impact.name_it, ''),
			COALESCE(notice_counts.statuses, '[]'::jsonb),
			m.created_at,
			m.updated_at`

const maintenanceListLateralJoins = `
		LEFT JOIN LATERAL (
			SELECT st.name_it
			FROM maintenance.maintenance_service_taxonomy mst
			JOIN maintenance.service_taxonomy st ON st.service_taxonomy_id = mst.service_taxonomy_id
			WHERE mst.maintenance_id = m.maintenance_id
			ORDER BY mst.is_primary DESC, st.sort_order, st.name_it, st.service_taxonomy_id
			LIMIT 1
		) primary_service ON true
		LEFT JOIN LATERAL (
			SELECT ie.name_it
			FROM maintenance.maintenance_impact_effect mie
			JOIN maintenance.impact_effect ie ON ie.impact_effect_id = mie.impact_effect_id
			WHERE mie.maintenance_id = m.maintenance_id
			ORDER BY mie.is_primary DESC, ie.sort_order, ie.name_it, ie.impact_effect_id
			LIMIT 1
		) primary_impact ON true
		LEFT JOIN LATERAL (
			SELECT jsonb_agg(jsonb_build_object('status', send_status, 'count', cnt) ORDER BY send_status) AS statuses
			FROM (
				SELECT send_status, COUNT(*)::int AS cnt
				FROM maintenance.notice n
				WHERE n.maintenance_id = m.maintenance_id
				GROUP BY send_status
			) x
		) notice_counts ON true`

const maintenanceListOrder = `
		ORDER BY
			CASE WHEN vcw.scheduled_start_at IS NOT NULL AND vcw.scheduled_start_at >= now() THEN 0 ELSE 1 END,
			vcw.scheduled_start_at ASC NULLS LAST,
			m.updated_at DESC,
			m.maintenance_id DESC`

func (h *Handler) handleListMaintenances(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}

	page := queryPositiveInt(r, "page", 1)
	pageSize := queryPositiveInt(r, "page_size", 20)
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	args := []any{}
	where := maintenanceListWhere(r, &args)

	var total int
	if err := h.maintenance.QueryRowContext(r.Context(), `SELECT COUNT(*)`+maintenanceListFrom+where, args...).Scan(&total); err != nil {
		h.dbFailure(w, r, "list_maintenances_count", err)
		return
	}

	listArgs := append([]any{}, args...)
	limit := placeholder(&listArgs, pageSize)
	off := placeholder(&listArgs, offset)
	query := maintenanceListSelect + maintenanceListFrom + maintenanceListLateralJoins + where + maintenanceListOrder + `
		LIMIT ` + limit + ` OFFSET ` + off

	rows, err := h.maintenance.QueryContext(r.Context(), query, listArgs...)
	if err != nil {
		h.dbFailure(w, r, "list_maintenances", err)
		return
	}
	defer rows.Close()

	items := make([]MaintenanceListItem, 0)
	for rows.Next() {
		item, err := scanMaintenanceListItem(rows)
		if err != nil {
			h.dbFailure(w, r, "list_maintenances_scan", err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "list_maintenances_rows", err)
		return
	}

	httputil.JSON(w, http.StatusOK, pagedResponse[MaintenanceListItem]{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

func maintenanceListWhere(r *http.Request, args *[]any) string {
	return maintenanceListWhereWithOptions(r, args, maintenanceListFilterOptions{includeScheduledRange: true})
}

func maintenanceListWhereWithOptions(r *http.Request, args *[]any, options maintenanceListFilterOptions) string {
	parts := []string{"1=1"}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q != "" {
		pattern := "%" + q + "%"
		parts = append(parts, `(COALESCE(m.code, '') ILIKE `+placeholder(args, pattern)+`
			OR COALESCE(m.title_it, '') ILIKE `+placeholder(args, pattern)+`
			OR COALESCE(m.title_en, '') ILIKE `+placeholder(args, pattern)+`
			OR COALESCE(m.reason_it, '') ILIKE `+placeholder(args, pattern)+`
			OR COALESCE(m.residual_service_it, '') ILIKE `+placeholder(args, pattern)+`
			OR COALESCE(mk.name_it, '') ILIKE `+placeholder(args, pattern)+`
			OR COALESCE(td.name_it, '') ILIKE `+placeholder(args, pattern)+`
			OR COALESCE(cs.name_it, '') ILIKE `+placeholder(args, pattern)+`
			OR COALESCE(s.name, '') ILIKE `+placeholder(args, pattern)+`
			OR EXISTS (
				SELECT 1 FROM maintenance.maintenance_target mt
				WHERE mt.maintenance_id = m.maintenance_id
				AND COALESCE(mt.display_name, '') ILIKE `+placeholder(args, pattern)+`
			))`)
	}
	if statuses := splitQueryList(r.URL.Query().Get("status")); len(statuses) > 0 {
		holders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			holders = append(holders, placeholder(args, status))
		}
		parts = append(parts, "m.status IN ("+strings.Join(holders, ", ")+")")
	}
	if options.includeScheduledRange {
		if raw := strings.TrimSpace(r.URL.Query().Get("scheduled_from")); raw != "" {
			parts = append(parts, "vcw.scheduled_start_at::date >= "+placeholder(args, raw))
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("scheduled_to")); raw != "" {
			parts = append(parts, "vcw.scheduled_start_at::date <= "+placeholder(args, raw))
		}
	}
	addIDFilter := func(param string, column string) {
		raw := strings.TrimSpace(r.URL.Query().Get(param))
		if raw == "" {
			return
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && id > 0 {
			parts = append(parts, column+" = "+placeholder(args, id))
		}
	}
	addIDFilter("technical_domain_id", "m.technical_domain_id")
	addIDFilter("maintenance_kind_id", "m.maintenance_kind_id")
	addIDFilter("customer_scope_id", "m.customer_scope_id")
	addIDFilter("site_id", "m.site_id")

	return " WHERE " + strings.Join(parts, " AND ")
}

func (h *Handler) handleMaintenanceRadar(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}

	now := time.Now().Local()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	next7DaysTo := today.AddDate(0, 0, 7)
	next45DaysFrom := today.AddDate(0, 0, 8)
	next45DaysTo := today.AddDate(0, 0, 52)
	sixMonthsTo := today.AddDate(0, 6, 0)

	args := []any{}
	where := maintenanceListWhereWithOptions(r, &args, maintenanceListFilterOptions{includeScheduledRange: false})
	todayParam := placeholder(&args, today.Format("2006-01-02"))
	sixMonthsParam := placeholder(&args, sixMonthsTo.Format("2006-01-02"))
	where += ` AND (vcw.maintenance_window_id IS NULL OR vcw.scheduled_start_at::date BETWEEN ` + todayParam + ` AND ` + sixMonthsParam + `)`

	query := maintenanceListSelect + maintenanceListFrom + maintenanceListLateralJoins + where + maintenanceListOrder
	rows, err := h.maintenance.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "maintenance_radar", err)
		return
	}
	defer rows.Close()

	items := make([]MaintenanceListItem, 0)
	for rows.Next() {
		item, err := scanMaintenanceListItem(rows)
		if err != nil {
			h.dbFailure(w, r, "maintenance_radar_scan", err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "maintenance_radar_rows", err)
		return
	}

	httputil.JSON(w, http.StatusOK, maintenanceRadarResponse{
		Items:          items,
		Today:          today.Format("2006-01-02"),
		Next7DaysTo:    next7DaysTo.Format("2006-01-02"),
		Next45DaysFrom: next45DaysFrom.Format("2006-01-02"),
		Next45DaysTo:   next45DaysTo.Format("2006-01-02"),
		SixMonthsTo:    sixMonthsTo.Format("2006-01-02"),
	})
}

func splitQueryList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	values := strings.Split(raw, ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func scanMaintenanceListItem(rows *sql.Rows) (MaintenanceListItem, error) {
	var item MaintenanceListItem
	var titleEN sql.NullString
	var kindID, domainID int64
	var scopeID sql.NullInt64
	var kindCode, kindName, domainCode, domainName string
	var scopeCode, scopeName sql.NullString
	var kindNameEN, kindDescription, domainNameEN, domainDescription, scopeNameEN, scopeDescription sql.NullString
	var kindSort, domainSort int
	var scopeSort sql.NullInt64
	var kindActive, domainActive bool
	var scopeActive sql.NullBool
	var siteID sql.NullInt64
	var siteCode, siteName, siteCity, siteCountry, siteScope sql.NullString
	var siteActive sql.NullBool
	var windowID sql.NullInt64
	var seqNo sql.NullInt64
	var windowStatus sql.NullString
	var scheduledStart, scheduledEnd sql.NullTime
	var expectedDowntime sql.NullInt64
	var primaryImpact sql.NullString
	var noticeRaw []byte

	err := rows.Scan(
		&item.MaintenanceID,
		&item.Code,
		&item.TitleIT,
		&titleEN,
		&item.Status,
		&kindID, &kindCode, &kindName, &kindNameEN, &kindDescription, &kindSort, &kindActive,
		&domainID, &domainCode, &domainName, &domainNameEN, &domainDescription, &domainSort, &domainActive,
		&scopeID, &scopeCode, &scopeName, &scopeNameEN, &scopeDescription, &scopeSort, &scopeActive,
		&siteID, &siteCode, &siteName, &siteCity, &siteCountry, &siteActive, &siteScope,
		&windowID, &seqNo, &windowStatus, &scheduledStart, &scheduledEnd, &expectedDowntime,
		&primaryImpact,
		&noticeRaw,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return item, err
	}
	item.TitleEN = nullStringValue(titleEN)
	item.MaintenanceKind = ReferenceItem{ID: kindID, Code: kindCode, NameIT: kindName, NameEN: nullStringValue(kindNameEN), Description: nullStringValue(kindDescription), SortOrder: kindSort, IsActive: kindActive}
	item.TechnicalDomain = ReferenceItem{ID: domainID, Code: domainCode, NameIT: domainName, NameEN: nullStringValue(domainNameEN), Description: nullStringValue(domainDescription), SortOrder: domainSort, IsActive: domainActive}
	item.CustomerScope = nullableReferenceItem(scopeID, scopeCode, scopeName, scopeNameEN, scopeDescription, scopeSort, scopeActive)
	if siteID.Valid {
		item.Site = &ReferenceItem{
			ID:          siteID.Int64,
			Code:        siteCode.String,
			NameIT:      siteName.String,
			IsActive:    siteActive.Bool,
			City:        nullStringValue(siteCity),
			CountryCode: nullStringValue(siteCountry),
			SortOrder:   100,
			Scope:       nullStringValue(siteScope),
		}
	}
	if windowID.Valid && scheduledStart.Valid && scheduledEnd.Valid {
		item.CurrentWindow = &WindowSummary{
			MaintenanceWindowID:     windowID.Int64,
			SeqNo:                   int(seqNo.Int64),
			WindowStatus:            windowStatus.String,
			ScheduledStartAt:        scheduledStart.Time,
			ScheduledEndAt:          scheduledEnd.Time,
			ExpectedDowntimeMinutes: nullIntValue(expectedDowntime),
		}
	}
	item.PrimaryImpactLabel = nullStringValue(primaryImpact)
	if len(noticeRaw) > 0 {
		_ = json.Unmarshal(noticeRaw, &item.NoticeStatuses)
	}
	if item.NoticeStatuses == nil {
		item.NoticeStatuses = []StatusCount{}
	}
	return item, nil
}

func nullableReferenceItem(
	id sql.NullInt64,
	code sql.NullString,
	name sql.NullString,
	nameEN sql.NullString,
	description sql.NullString,
	sortOrder sql.NullInt64,
	isActive sql.NullBool,
) *ReferenceItem {
	if !id.Valid {
		return nil
	}
	item := ReferenceItem{
		ID:          id.Int64,
		Code:        code.String,
		NameIT:      name.String,
		NameEN:      nullStringValue(nameEN),
		Description: nullStringValue(description),
		IsActive:    isActive.Valid && isActive.Bool,
	}
	if sortOrder.Valid {
		item.SortOrder = int(sortOrder.Int64)
	}
	return &item
}

func (h *Handler) handleGetMaintenance(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	detail, err := h.loadMaintenanceDetail(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "maintenance_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_maintenance", err, "maintenance_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

func (h *Handler) loadMaintenanceDetail(ctx context.Context, id int64) (MaintenanceDetail, error) {
	var detail MaintenanceDetail
	var titleEN, descriptionIT, descriptionEN, reasonIT, reasonEN, residualIT, residualEN sql.NullString
	var metadata []byte
	var kindID, domainID int64
	var scopeID sql.NullInt64
	var kindCode, kindName, domainCode, domainName string
	var scopeCode, scopeName sql.NullString
	var kindNameEN, kindDescription, domainNameEN, domainDescription, scopeNameEN, scopeDescription sql.NullString
	var kindSort, domainSort int
	var scopeSort sql.NullInt64
	var kindActive, domainActive bool
	var scopeActive sql.NullBool
	var siteID sql.NullInt64
	var siteCode, siteName, siteCity, siteCountry, siteScope sql.NullString
	var siteActive sql.NullBool
	var windowID sql.NullInt64
	var seqNo sql.NullInt64
	var windowStatus sql.NullString
	var scheduledStart, scheduledEnd sql.NullTime
	var expectedDowntime sql.NullInt64

	err := h.maintenance.QueryRowContext(
		ctx,
		`SELECT
			m.maintenance_id,
			COALESCE(m.code, ''),
			m.title_it,
			m.title_en,
			m.description_it,
			m.description_en,
			m.status,
			mk.maintenance_kind_id, mk.code, mk.name_it, mk.name_en, mk.description, mk.sort_order, mk.is_active,
			td.technical_domain_id, td.code, td.name_it, td.name_en, td.description, td.sort_order, td.is_active,
			cs.customer_scope_id, cs.code, cs.name_it, cs.name_en, cs.description, cs.sort_order, cs.is_active,
			s.site_id, s.code, s.name, s.city, s.country_code, s.is_active, s.scope,
			m.reason_it,
			m.reason_en,
			m.residual_service_it,
			m.residual_service_en,
			vcw.maintenance_window_id, vcw.seq_no, vcw.window_status, vcw.scheduled_start_at, vcw.scheduled_end_at, vcw.expected_downtime_minutes,
			m.created_at,
			m.updated_at,
			m.metadata
		FROM maintenance.maintenance m
		JOIN maintenance.maintenance_kind mk ON mk.maintenance_kind_id = m.maintenance_kind_id
		JOIN maintenance.technical_domain td ON td.technical_domain_id = m.technical_domain_id
		LEFT JOIN maintenance.customer_scope cs ON cs.customer_scope_id = m.customer_scope_id
		LEFT JOIN maintenance.site s ON s.site_id = m.site_id
		LEFT JOIN maintenance.v_current_window vcw ON vcw.maintenance_id = m.maintenance_id
		WHERE m.maintenance_id = $1`,
		id,
	).Scan(
		&detail.MaintenanceID,
		&detail.Code,
		&detail.TitleIT,
		&titleEN,
		&descriptionIT,
		&descriptionEN,
		&detail.Status,
		&kindID, &kindCode, &kindName, &kindNameEN, &kindDescription, &kindSort, &kindActive,
		&domainID, &domainCode, &domainName, &domainNameEN, &domainDescription, &domainSort, &domainActive,
		&scopeID, &scopeCode, &scopeName, &scopeNameEN, &scopeDescription, &scopeSort, &scopeActive,
		&siteID, &siteCode, &siteName, &siteCity, &siteCountry, &siteActive, &siteScope,
		&reasonIT,
		&reasonEN,
		&residualIT,
		&residualEN,
		&windowID, &seqNo, &windowStatus, &scheduledStart, &scheduledEnd, &expectedDowntime,
		&detail.CreatedAt,
		&detail.UpdatedAt,
		&metadata,
	)
	if err != nil {
		return detail, err
	}

	detail.TitleEN = nullStringValue(titleEN)
	detail.DescriptionIT = nullStringValue(descriptionIT)
	detail.DescriptionEN = nullStringValue(descriptionEN)
	detail.ReasonIT = nullStringValue(reasonIT)
	detail.ReasonEN = nullStringValue(reasonEN)
	detail.ResidualServiceIT = nullStringValue(residualIT)
	detail.ResidualServiceEN = nullStringValue(residualEN)
	detail.Metadata = rawJSONFromBytes(metadata)
	detail.MaintenanceKind = ReferenceItem{ID: kindID, Code: kindCode, NameIT: kindName, NameEN: nullStringValue(kindNameEN), Description: nullStringValue(kindDescription), SortOrder: kindSort, IsActive: kindActive}
	detail.TechnicalDomain = ReferenceItem{ID: domainID, Code: domainCode, NameIT: domainName, NameEN: nullStringValue(domainNameEN), Description: nullStringValue(domainDescription), SortOrder: domainSort, IsActive: domainActive}
	detail.CustomerScope = nullableReferenceItem(scopeID, scopeCode, scopeName, scopeNameEN, scopeDescription, scopeSort, scopeActive)
	if siteID.Valid {
		detail.Site = &ReferenceItem{
			ID:          siteID.Int64,
			Code:        siteCode.String,
			NameIT:      siteName.String,
			IsActive:    siteActive.Bool,
			City:        nullStringValue(siteCity),
			CountryCode: nullStringValue(siteCountry),
			SortOrder:   100,
			Scope:       nullStringValue(siteScope),
		}
	}
	if windowID.Valid && scheduledStart.Valid && scheduledEnd.Valid {
		detail.CurrentWindow = &WindowSummary{
			MaintenanceWindowID:     windowID.Int64,
			SeqNo:                   int(seqNo.Int64),
			WindowStatus:            windowStatus.String,
			ScheduledStartAt:        scheduledStart.Time,
			ScheduledEndAt:          scheduledEnd.Time,
			ExpectedDowntimeMinutes: nullIntValue(expectedDowntime),
		}
	}

	var loadErr error
	if detail.Windows, loadErr = h.loadWindows(ctx, id); loadErr != nil {
		return detail, loadErr
	}
	if detail.ServiceTaxonomy, loadErr = h.loadClassifications(ctx, serviceTaxonomyClass, id); loadErr != nil {
		return detail, loadErr
	}
	if detail.ReasonClasses, loadErr = h.loadClassifications(ctx, reasonClassClass, id); loadErr != nil {
		return detail, loadErr
	}
	if detail.ImpactEffects, loadErr = h.loadClassifications(ctx, impactEffectClass, id); loadErr != nil {
		return detail, loadErr
	}
	if detail.QualityFlags, loadErr = h.loadClassifications(ctx, qualityFlagClass, id); loadErr != nil {
		return detail, loadErr
	}
	if detail.Targets, loadErr = h.loadTargets(ctx, id); loadErr != nil {
		return detail, loadErr
	}
	if detail.ImpactedCustomers, loadErr = h.loadImpactedCustomers(ctx, id); loadErr != nil {
		return detail, loadErr
	}
	if detail.Notices, loadErr = h.loadNotices(ctx, id); loadErr != nil {
		return detail, loadErr
	}
	if detail.Events, loadErr = h.loadEvents(ctx, id); loadErr != nil {
		return detail, loadErr
	}

	return detail, nil
}

func (h *Handler) loadWindows(ctx context.Context, maintenanceID int64) ([]MaintenanceWindow, error) {
	rows, err := h.maintenance.QueryContext(
		ctx,
		`SELECT
			maintenance_window_id,
			maintenance_id,
			seq_no,
			window_status,
			scheduled_start_at,
			scheduled_end_at,
			expected_downtime_minutes,
			actual_start_at,
			actual_end_at,
			actual_downtime_minutes,
			cancellation_reason_it,
			cancellation_reason_en,
			announced_at,
			last_notice_at,
			created_at
		FROM maintenance.maintenance_window
		WHERE maintenance_id = $1
		ORDER BY scheduled_start_at ASC, seq_no ASC`,
		maintenanceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]MaintenanceWindow, 0)
	for rows.Next() {
		var item MaintenanceWindow
		var expected, actualDowntime sql.NullInt64
		var actualStart, actualEnd, announcedAt, lastNoticeAt sql.NullTime
		var cancellationIT, cancellationEN sql.NullString
		if err := rows.Scan(
			&item.MaintenanceWindowID,
			&item.MaintenanceID,
			&item.SeqNo,
			&item.WindowStatus,
			&item.ScheduledStartAt,
			&item.ScheduledEndAt,
			&expected,
			&actualStart,
			&actualEnd,
			&actualDowntime,
			&cancellationIT,
			&cancellationEN,
			&announcedAt,
			&lastNoticeAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.ExpectedDowntimeMinutes = nullIntValue(expected)
		item.ActualStartAt = nullTimeValue(actualStart)
		item.ActualEndAt = nullTimeValue(actualEnd)
		item.ActualDowntimeMinutes = nullIntValue(actualDowntime)
		item.CancellationReasonIT = nullStringValue(cancellationIT)
		item.CancellationReasonEN = nullStringValue(cancellationEN)
		item.AnnouncedAt = nullTimeValue(announcedAt)
		item.LastNoticeAt = nullTimeValue(lastNoticeAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *Handler) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	items, err := h.loadEvents(r.Context(), id)
	if err != nil {
		h.dbFailure(w, r, "get_events", err, "maintenance_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) loadEvents(ctx context.Context, maintenanceID int64) ([]MaintenanceEvent, error) {
	rows, err := h.maintenance.QueryContext(
		ctx,
		`SELECT maintenance_event_id, maintenance_id, maintenance_window_id, event_type, actor_type, event_at, summary, payload
		FROM maintenance.maintenance_event
		WHERE maintenance_id = $1
		ORDER BY event_at DESC, maintenance_event_id DESC`,
		maintenanceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]MaintenanceEvent, 0)
	for rows.Next() {
		var item MaintenanceEvent
		var windowID sql.NullInt64
		var summary sql.NullString
		var payload []byte
		if err := rows.Scan(&item.MaintenanceEventID, &item.MaintenanceID, &windowID, &item.EventType, &item.ActorType, &item.EventAt, &summary, &payload); err != nil {
			return nil, err
		}
		item.MaintenanceWindowID = nullInt64Value(windowID)
		item.Summary = nullStringValue(summary)
		item.Payload = rawJSONFromBytes(payload)
		items = append(items, item)
	}
	return items, rows.Err()
}
