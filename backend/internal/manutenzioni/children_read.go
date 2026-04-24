package manutenzioni

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type classificationMeta struct {
	Resource          resourceMeta
	RelationTable     string
	RelationIDColumn  string
	ReferenceIDColumn string
	HasPrimary        bool
	EventLabel        string
}

var (
	serviceTaxonomyClass = classificationMeta{
		Resource:          resourceMetas["service-taxonomy"],
		RelationTable:     "maintenance.maintenance_service_taxonomy",
		RelationIDColumn:  "maintenance_service_taxonomy_id",
		ReferenceIDColumn: "service_taxonomy_id",
		HasPrimary:        true,
		EventLabel:        "Servizio aggiornato",
	}
	reasonClassClass = classificationMeta{
		Resource:          resourceMetas["reason-classes"],
		RelationTable:     "maintenance.maintenance_reason_class",
		RelationIDColumn:  "maintenance_reason_class_id",
		ReferenceIDColumn: "reason_class_id",
		HasPrimary:        true,
		EventLabel:        "Motivo aggiornato",
	}
	impactEffectClass = classificationMeta{
		Resource:          resourceMetas["impact-effects"],
		RelationTable:     "maintenance.maintenance_impact_effect",
		RelationIDColumn:  "maintenance_impact_effect_id",
		ReferenceIDColumn: "impact_effect_id",
		HasPrimary:        true,
		EventLabel:        "Impatto aggiornato",
	}
	qualityFlagClass = classificationMeta{
		Resource:          resourceMetas["quality-flags"],
		RelationTable:     "maintenance.maintenance_quality_flag",
		RelationIDColumn:  "maintenance_quality_flag_id",
		ReferenceIDColumn: "quality_flag_id",
		HasPrimary:        false,
		EventLabel:        "Controlli qualita aggiornati",
	}
)

func (h *Handler) loadClassifications(ctx context.Context, meta classificationMeta, maintenanceID int64) ([]ClassificationItem, error) {
	var refSelect string
	var refJoin string
	var serviceSelect string
	switch meta.Resource.Kind {
	case resourceService:
		refSelect = `r.code, r.name_it, r.name_en, r.description, r.sort_order, r.is_active,
			NULL::text AS city, NULL::text AS country_code, r.technical_domain_id, td.name_it AS technical_domain_name,
			r.target_type_id, tt.name_it AS target_type_name, r.audience`
		refJoin = fmt.Sprintf(`JOIN %s r ON r.%s = c.%s JOIN maintenance.technical_domain td ON td.technical_domain_id = r.technical_domain_id JOIN maintenance.target_type tt ON tt.target_type_id = r.target_type_id`, meta.Resource.Table, meta.Resource.IDColumn, meta.ReferenceIDColumn)
		serviceSelect = `c.role, c.expected_severity, c.expected_audience`
	default:
		refSelect = `r.code, r.name_it, r.name_en, r.description, r.sort_order, r.is_active,
			NULL::text AS city, NULL::text AS country_code, NULL::bigint AS technical_domain_id, NULL::text AS technical_domain_name,
			NULL::bigint AS target_type_id, NULL::text AS target_type_name, NULL::text AS audience`
		refJoin = fmt.Sprintf(`JOIN %s r ON r.%s = c.%s`, meta.Resource.Table, meta.Resource.IDColumn, meta.ReferenceIDColumn)
		serviceSelect = `NULL::text AS role, NULL::text AS expected_severity, NULL::text AS expected_audience`
	}

	query := fmt.Sprintf(`SELECT
			c.%s,
			c.maintenance_id,
			r.%s,
			%s,
			c.source,
			c.confidence::float8,
			%s,
			%s,
			c.metadata
		FROM %s c
		%s
		WHERE c.maintenance_id = $1
		ORDER BY %s DESC, r.sort_order, r.name_it, r.%s`,
		meta.RelationIDColumn,
		meta.Resource.IDColumn,
		refSelect,
		classPrimarySelect(meta),
		serviceSelect,
		meta.RelationTable,
		refJoin,
		classPrimaryOrder(meta),
		meta.Resource.IDColumn,
	)

	rows, err := h.maintenance.QueryContext(ctx, query, maintenanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ClassificationItem{}
	for rows.Next() {
		var item ClassificationItem
		var ref ReferenceItem
		var nameEN, description, city, country, domainName, targetTypeName, audience sql.NullString
		var role, expectedSeverity, expectedAudience sql.NullString
		var domainID, targetTypeID sql.NullInt64
		var confidence sql.NullFloat64
		var metadata []byte
		if err := rows.Scan(
			&item.ID,
			&item.MaintenanceID,
			&ref.ID,
			&ref.Code,
			&ref.NameIT,
			&nameEN,
			&description,
			&ref.SortOrder,
			&ref.IsActive,
			&city,
			&country,
			&domainID,
			&domainName,
			&targetTypeID,
			&targetTypeName,
			&audience,
			&item.Source,
			&confidence,
			&item.IsPrimary,
			&role,
			&expectedSeverity,
			&expectedAudience,
			&metadata,
		); err != nil {
			return nil, err
		}
		ref.NameEN = nullStringValue(nameEN)
		ref.Description = nullStringValue(description)
		ref.City = nullStringValue(city)
		ref.CountryCode = nullStringValue(country)
		ref.TechnicalDomainID = nullInt64Value(domainID)
		ref.TechnicalDomainName = nullStringValue(domainName)
		ref.TargetTypeID = nullInt64Value(targetTypeID)
		ref.TargetTypeName = nullStringValue(targetTypeName)
		ref.Audience = nullStringValue(audience)
		item.Reference = ref
		item.Confidence = nullFloatValue(confidence)
		item.Role = nullStringValue(role)
		item.ExpectedSeverity = nullStringValue(expectedSeverity)
		item.ExpectedAudience = nullStringValue(expectedAudience)
		item.Metadata = rawJSONFromBytes(metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func classPrimarySelect(meta classificationMeta) string {
	if meta.HasPrimary {
		return "c.is_primary"
	}
	return "false AS is_primary"
}

func classPrimaryOrder(meta classificationMeta) string {
	if meta.HasPrimary {
		return "c.is_primary"
	}
	return "false"
}

func (h *Handler) loadTargets(ctx context.Context, maintenanceID int64) ([]MaintenanceTarget, error) {
	rows, err := h.maintenance.QueryContext(
		ctx,
		`SELECT
			mt.maintenance_target_id,
			mt.maintenance_id,
			tt.target_type_id, tt.code, tt.name_it, tt.name_en, tt.description, tt.sort_order, tt.is_active,
			mt.service_taxonomy_id,
			st.code, st.name_it, st.name_en, st.description, st.sort_order, st.is_active,
			st.technical_domain_id, td.name_it, st.target_type_id, stt.name_it, st.audience,
			mt.ref_table,
			mt.ref_id,
			mt.external_key,
			mt.display_name,
			mt.source,
			mt.confidence::float8,
			mt.is_primary,
			mt.metadata
		FROM maintenance.maintenance_target mt
		JOIN maintenance.target_type tt ON tt.target_type_id = mt.target_type_id
		LEFT JOIN maintenance.service_taxonomy st ON st.service_taxonomy_id = mt.service_taxonomy_id
		LEFT JOIN maintenance.technical_domain td ON td.technical_domain_id = st.technical_domain_id
		LEFT JOIN maintenance.target_type stt ON stt.target_type_id = st.target_type_id
		WHERE mt.maintenance_id = $1
		ORDER BY mt.is_primary DESC, mt.display_name, mt.maintenance_target_id`,
		maintenanceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []MaintenanceTarget{}
	for rows.Next() {
		var item MaintenanceTarget
		var ref ReferenceItem
		var serviceRef ReferenceItem
		var nameEN, description, refTable, externalKey sql.NullString
		var serviceCode, serviceName, serviceNameEN, serviceDescription, serviceDomainName, serviceTargetTypeName, serviceAudience sql.NullString
		var refID, serviceID, serviceDomainID, serviceTargetTypeID sql.NullInt64
		var serviceSort sql.NullInt64
		var serviceActive sql.NullBool
		var confidence sql.NullFloat64
		var metadata []byte
		if err := rows.Scan(
			&item.MaintenanceTargetID,
			&item.MaintenanceID,
			&ref.ID,
			&ref.Code,
			&ref.NameIT,
			&nameEN,
			&description,
			&ref.SortOrder,
			&ref.IsActive,
			&serviceID,
			&serviceCode,
			&serviceName,
			&serviceNameEN,
			&serviceDescription,
			&serviceSort,
			&serviceActive,
			&serviceDomainID,
			&serviceDomainName,
			&serviceTargetTypeID,
			&serviceTargetTypeName,
			&serviceAudience,
			&refTable,
			&refID,
			&externalKey,
			&item.DisplayName,
			&item.Source,
			&confidence,
			&item.IsPrimary,
			&metadata,
		); err != nil {
			return nil, err
		}
		ref.NameEN = nullStringValue(nameEN)
		ref.Description = nullStringValue(description)
		item.TargetType = ref
		item.ServiceTaxonomyID = nullInt64Value(serviceID)
		if serviceID.Valid {
			serviceRef.ID = serviceID.Int64
			serviceRef.Code = serviceCode.String
			serviceRef.NameIT = serviceName.String
			serviceRef.NameEN = nullStringValue(serviceNameEN)
			serviceRef.Description = nullStringValue(serviceDescription)
			if serviceSort.Valid {
				serviceRef.SortOrder = int(serviceSort.Int64)
			}
			serviceRef.IsActive = serviceActive.Bool
			serviceRef.TechnicalDomainID = nullInt64Value(serviceDomainID)
			serviceRef.TechnicalDomainName = nullStringValue(serviceDomainName)
			serviceRef.TargetTypeID = nullInt64Value(serviceTargetTypeID)
			serviceRef.TargetTypeName = nullStringValue(serviceTargetTypeName)
			serviceRef.Audience = nullStringValue(serviceAudience)
			item.ServiceTaxonomy = &serviceRef
		}
		item.ReferenceTable = nullStringValue(refTable)
		item.ReferenceID = nullInt64Value(refID)
		item.ExternalKey = nullStringValue(externalKey)
		item.Confidence = nullFloatValue(confidence)
		item.Metadata = rawJSONFromBytes(metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *Handler) loadImpactedCustomers(ctx context.Context, maintenanceID int64) ([]ImpactedCustomer, error) {
	rows, err := h.maintenance.QueryContext(
		ctx,
		`SELECT
			maintenance_impacted_customer_id,
			maintenance_id,
			customer_id,
			order_id,
			service_id,
			impact_scope,
			derivation_source,
			confidence::float8,
			reason,
			metadata,
			created_at
		FROM maintenance.maintenance_impacted_customer
		WHERE maintenance_id = $1
		ORDER BY customer_id, maintenance_impacted_customer_id`,
		maintenanceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ImpactedCustomer{}
	customerIDs := []int64{}
	for rows.Next() {
		var item ImpactedCustomer
		var orderID, serviceID sql.NullInt64
		var confidence sql.NullFloat64
		var reason sql.NullString
		var metadata []byte
		if err := rows.Scan(
			&item.MaintenanceImpactedCustomerID,
			&item.MaintenanceID,
			&item.CustomerID,
			&orderID,
			&serviceID,
			&item.ImpactScope,
			&item.DerivationSource,
			&confidence,
			&reason,
			&metadata,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.OrderID = nullInt64Value(orderID)
		item.ServiceID = nullInt64Value(serviceID)
		item.Confidence = nullFloatValue(confidence)
		item.Reason = nullStringValue(reason)
		item.Metadata = rawJSONFromBytes(metadata)
		items = append(items, item)
		customerIDs = append(customerIDs, item.CustomerID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	names := h.enrichCustomerNames(ctx, customerIDs)
	for i := range items {
		items[i].CustomerName = names[items[i].CustomerID]
		if strings.TrimSpace(items[i].CustomerName) == "" {
			items[i].CustomerName = fmt.Sprintf("Cliente #%d", items[i].CustomerID)
		}
	}
	return items, nil
}

func (h *Handler) loadNotices(ctx context.Context, maintenanceID int64) ([]Notice, error) {
	rows, err := h.maintenance.QueryContext(
		ctx,
		`SELECT
			n.notice_id,
			n.maintenance_id,
			n.maintenance_window_id,
			n.notice_type,
			n.audience,
			nc.notice_channel_id, nc.code, nc.name_it, nc.name_en, nc.description, nc.sort_order, nc.is_active,
			n.template_code,
			n.template_version,
			n.generation_source,
			n.send_status,
			n.scheduled_send_at,
			n.sent_at,
			n.created_at,
			n.metadata
		FROM maintenance.notice n
		JOIN maintenance.notice_channel nc ON nc.notice_channel_id = n.notice_channel_id
		WHERE n.maintenance_id = $1
		ORDER BY n.created_at DESC, n.notice_id DESC`,
		maintenanceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Notice{}
	for rows.Next() {
		var item Notice
		var windowID sql.NullInt64
		var channel ReferenceItem
		var nameEN, description, templateCode sql.NullString
		var templateVersion sql.NullInt64
		var scheduledSendAt, sentAt sql.NullTime
		var metadata []byte
		if err := rows.Scan(
			&item.NoticeID,
			&item.MaintenanceID,
			&windowID,
			&item.NoticeType,
			&item.Audience,
			&channel.ID,
			&channel.Code,
			&channel.NameIT,
			&nameEN,
			&description,
			&channel.SortOrder,
			&channel.IsActive,
			&templateCode,
			&templateVersion,
			&item.GenerationSource,
			&item.SendStatus,
			&scheduledSendAt,
			&sentAt,
			&item.CreatedAt,
			&metadata,
		); err != nil {
			return nil, err
		}
		channel.NameEN = nullStringValue(nameEN)
		channel.Description = nullStringValue(description)
		item.MaintenanceWindowID = nullInt64Value(windowID)
		item.NoticeChannel = channel
		item.TemplateCode = nullStringValue(templateCode)
		item.TemplateVersion = nullIntValue(templateVersion)
		item.ScheduledSendAt = nullTimeValue(scheduledSendAt)
		item.SentAt = nullTimeValue(sentAt)
		item.Metadata = rawJSONFromBytes(metadata)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		locales, err := h.loadNoticeLocales(ctx, items[i].NoticeID)
		if err != nil {
			return nil, err
		}
		flags, err := h.loadNoticeQualityFlags(ctx, items[i].NoticeID)
		if err != nil {
			return nil, err
		}
		items[i].Locales = locales
		items[i].QualityFlags = flags
	}
	return items, nil
}

func (h *Handler) loadNoticeLocales(ctx context.Context, noticeID int64) ([]NoticeLocale, error) {
	rows, err := h.maintenance.QueryContext(
		ctx,
		`SELECT notice_locale_id, notice_id, locale, subject, body_html, body_text
		FROM maintenance.notice_locale
		WHERE notice_id = $1
		ORDER BY CASE locale WHEN 'it' THEN 0 ELSE 1 END`,
		noticeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []NoticeLocale{}
	for rows.Next() {
		var item NoticeLocale
		var bodyHTML, bodyText sql.NullString
		if err := rows.Scan(&item.NoticeLocaleID, &item.NoticeID, &item.Locale, &item.Subject, &bodyHTML, &bodyText); err != nil {
			return nil, err
		}
		item.BodyHTML = nullStringValue(bodyHTML)
		item.BodyText = nullStringValue(bodyText)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *Handler) loadNoticeQualityFlags(ctx context.Context, noticeID int64) ([]NoticeQualityFlag, error) {
	rows, err := h.maintenance.QueryContext(
		ctx,
		`SELECT
			nqf.notice_quality_flag_id,
			nqf.notice_id,
			qf.quality_flag_id, qf.code, qf.name_it, qf.name_en, qf.description, qf.sort_order, qf.is_active,
			nqf.source,
			nqf.confidence::float8,
			nqf.metadata
		FROM maintenance.notice_quality_flag nqf
		JOIN maintenance.quality_flag qf ON qf.quality_flag_id = nqf.quality_flag_id
		WHERE nqf.notice_id = $1
		ORDER BY qf.sort_order, qf.name_it`,
		noticeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []NoticeQualityFlag{}
	for rows.Next() {
		var item NoticeQualityFlag
		var ref ReferenceItem
		var nameEN, description sql.NullString
		var confidence sql.NullFloat64
		var metadata []byte
		if err := rows.Scan(
			&item.ID,
			&item.NoticeID,
			&ref.ID,
			&ref.Code,
			&ref.NameIT,
			&nameEN,
			&description,
			&ref.SortOrder,
			&ref.IsActive,
			&item.Source,
			&confidence,
			&metadata,
		); err != nil {
			return nil, err
		}
		ref.NameEN = nullStringValue(nameEN)
		ref.Description = nullStringValue(description)
		item.Reference = ref
		item.Confidence = nullFloatValue(confidence)
		item.Metadata = rawJSONFromBytes(metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func payloadSummary(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}
	return string(payload)
}
