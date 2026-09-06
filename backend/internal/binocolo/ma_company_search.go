package binocolo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// The corpus and fiscal grouping are shared by the search and its local area
// catalog. No lookup here refreshes a vendor cache or calls an external service.
const maCompanyCorpusSQL = `
WITH source_base AS (
  SELECT
    'ricerca'::text AS source_kind,
    t.id::text AS entity_id,
    session.id::text AS context_id,
    COALESCE(session.title, '') AS context_title,
    COALESCE(t.company_key, '') AS company_key,
    COALESCE(t.company_name, '') AS company_name,
    COALESCE(t.vat_code, '') AS vat_code,
    COALESCE(t.tax_code, '') AS tax_code,
    COALESCE(t.province, '') AS province,
    COALESCE(t.town, '') AS town,
    ''::text AS domain,
    t.created_at AS seen_at,
    ''::text AS card_state,
    ''::text AS card_esito
  FROM binocolo.ma_target t
  JOIN binocolo.ma_session session ON session.id = t.session_id
  WHERE session.deleted_at IS NULL AND session.purged_at IS NULL

  UNION ALL

  SELECT
    'iniziativa', c.initiative_id::text || ':' || c.company_key, initiative.id::text,
    COALESCE(initiative.title, ''), upper(btrim(c.company_key)), COALESCE(c.company_name, ''),
    COALESCE(c.vat_code, ''), COALESCE(c.tax_code, ''), COALESCE(c.province, ''), '', '',
    c.updated_at, c.state, COALESCE(c.esito, '')
  FROM binocolo.ma_initiative_card c
  JOIN binocolo.ma_initiative initiative ON initiative.id = c.initiative_id
  WHERE initiative.deleted_at IS NULL AND initiative.purged_at IS NULL

  UNION ALL

  SELECT
    'registro', d.company_key, '', '', upper(btrim(d.company_key)), COALESCE(d.company_name, ''),
    COALESCE(d.vat_code, ''), COALESCE(d.tax_code, ''), '', '', COALESCE(d.domain, ''),
    d.updated_at, '', ''
  FROM binocolo.ma_company_domain d
), source_clean AS (
  -- Le primitive della 120, non una loro copia scritta a mano: erano
  -- bit-identiche, ma tenute in sincronia solo da un commento. F0 esisteva
  -- proprio per togliere di mezzo le espressioni ricopiate (issue #86).
  SELECT source_base.*,
         binocolo.ma_normalize_fiscal(vat_code) AS vat_clean,
         binocolo.ma_normalize_fiscal(tax_code) AS tax_clean
  FROM source_base
), source_rows AS (
  SELECT source_clean.*,
         COALESCE(
           NULLIF(binocolo.ma_stable_vat(vat_code), ''),
           NULLIF(tax_clean, ''),
           company_key
         ) AS stable_key
  FROM source_clean
  WHERE company_key <> ''
)`

type maCompanySearchOptions struct {
	Kind             string   `json:"kind"`
	Query            string   `json:"query"`
	Name             string   `json:"name"`
	VAT              string   `json:"vat"`
	Tax              string   `json:"tax"`
	Annotation       string   `json:"annotation"`
	NDA              string   `json:"nda"`
	Include          []string `json:"include"`
	Exclude          []string `json:"exclude"`
	TurnoverMin      *float64 `json:"turnoverMin"`
	TurnoverMax      *float64 `json:"turnoverMax"`
	EmployeesMin     *int     `json:"employeesMin"`
	EmployeesMax     *int     `json:"employeesMax"`
	TurnoverMissing  bool     `json:"turnoverMissing"`
	EmployeesMissing bool     `json:"employeesMissing"`
	Page             int      `json:"-"`
	PageSize         int      `json:"-"`
	Sort             string   `json:"-"`
	Direction        string   `json:"-"`
}

func parseMACompanySearch(values url.Values) (maCompanySearchOptions, error) {
	o := maCompanySearchOptions{Page: 1, PageSize: 25, Sort: "name", Direction: "asc", Include: []string{}, Exclude: []string{}}
	invalid := func(field string) (maCompanySearchOptions, error) {
		return o, fmt.Errorf("%w: %s", errMAStrategyInvalid, field)
	}
	mode := values.Get("mode")
	if mode != "" && mode != "simple" && mode != "advanced" {
		return invalid("mode")
	}
	var err error
	o.Kind, o.Query, err = normalizeMACompanySearch(values.Get("query"))
	if err != nil && mode != "advanced" {
		return o, err
	}
	if mode == "advanced" {
		o.Kind, o.Query = maCompanySearchRecent, ""
		for key, dest := range map[string]*string{"name": &o.Name, "vat": &o.VAT, "tax": &o.Tax, "annotation": &o.Annotation} {
			*dest = strings.TrimSpace(values.Get(key))
			if len(*dest) > 500 {
				return invalid(key)
			}
		}
		o.Name = escapeMACompanySearchText(o.Name)
		o.Annotation = escapeMACompanySearchText(o.Annotation)
		normalizeCode := func(s string) string {
			return strings.ToUpper(strings.Join(strings.Fields(strings.ReplaceAll(s, ".", "")), ""))
		}
		o.VAT, o.Tax = normalizeCode(o.VAT), normalizeCode(o.Tax)
		if strings.HasPrefix(o.VAT, "IT") {
			o.VAT = strings.TrimPrefix(o.VAT, "IT")
		}
		if o.VAT != "" && (len(o.VAT) != 11 || !isMACompanySearchDigits(o.VAT)) {
			return invalid("vat")
		}
		if o.Tax != "" && !((len(o.Tax) == 11 && isMACompanySearchDigits(o.Tax)) || (len(o.Tax) == 16 && isMACompanySearchAlphanumeric(o.Tax))) {
			return invalid("tax")
		}
		o.NDA = strings.TrimSpace(values.Get("nda"))
		switch o.NDA {
		case "", "any", "active", "expired_only", "none":
		default:
			return invalid("nda")
		}
		for key, dest := range map[string]*[]string{"include": &o.Include, "exclude": &o.Exclude} {
			for _, value := range values[key] {
				if len(value) > 300 || !(strings.HasPrefix(value, "region:") || strings.HasPrefix(value, "province:")) {
					return invalid(key)
				}
				*dest = append(*dest, value)
			}
		}
		for key, dest := range map[string]**float64{"turnoverMin": &o.TurnoverMin, "turnoverMax": &o.TurnoverMax} {
			if v := values.Get(key); v != "" {
				n, e := strconv.ParseFloat(v, 64)
				if e != nil || n < 0 || n > 1e15 || n != n {
					return invalid(key)
				}
				*dest = &n
			}
		}
		for key, dest := range map[string]**int{"employeesMin": &o.EmployeesMin, "employeesMax": &o.EmployeesMax} {
			if v := values.Get(key); v != "" {
				n, e := strconv.Atoi(v)
				if e != nil || n < 0 || n > 2147483647 {
					return invalid(key)
				}
				*dest = &n
			}
		}
		if o.TurnoverMin != nil && o.TurnoverMax != nil && *o.TurnoverMin > *o.TurnoverMax {
			return invalid("turnover")
		}
		if o.EmployeesMin != nil && o.EmployeesMax != nil && *o.EmployeesMin > *o.EmployeesMax {
			return invalid("employees")
		}
		for key, dest := range map[string]*bool{"turnoverMissing": &o.TurnoverMissing, "employeesMissing": &o.EmployeesMissing} {
			if v := values.Get(key); v != "" {
				b, e := strconv.ParseBool(v)
				if e != nil {
					return invalid(key)
				}
				*dest = b
			}
		}
	}
	if v := values.Get("page"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 {
			return invalid("page")
		}
		o.Page = n
	}
	if v := values.Get("pageSize"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 || n > 100 {
			return invalid("pageSize")
		}
		o.PageSize = n
	}
	if v := values.Get("sort"); v != "" {
		if v != "name" && v != "turnover" && v != "employees" {
			return invalid("sort")
		}
		o.Sort = v
	}
	if v := values.Get("direction"); v != "" {
		if v != "asc" && v != "desc" {
			return invalid("direction")
		}
		o.Direction = v
	}
	return o, nil
}

func escapeMACompanySearchText(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}

func maCompanySearchOrder(o maCompanySearchOptions) string {
	column := "lower(company_name)"
	if o.Sort == "turnover" {
		column = "turnover"
	} else if o.Sort == "employees" {
		column = "employees"
	}
	direction := "ASC"
	if o.Direction == "desc" {
		direction = "DESC"
	}
	return column + " " + direction + " NULLS LAST, stable_key ASC"
}

// Even an expired province catalog is useful reference data. Do not use the
// normal cache reader here: it updates counters and can trigger vendor refresh.
const maCompanySearchGeographySQL = `,
 province_catalog AS (
  SELECT DISTINCT upper(btrim(p->>'sigla')) AS province,
    btrim(p->>'provincia') AS province_name, btrim(p->>'regione') AS region
  FROM binocolo.province_cache cache
  CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(cache.response->'data') = 'array' THEN cache.response->'data' ELSE '[]'::jsonb END) p
 ), location_observations AS (
  SELECT stable_key, upper(btrim(province)) AS province, btrim(town) AS town, seen_at
  FROM source_rows
  UNION ALL
  SELECT keys.stable_key,
   upper(btrim(COALESCE(root#>>'{address,registeredOffice,province}', root#>>'{address,province,code}', ''))),
   btrim(COALESCE(root#>>'{address,registeredOffice,town}', root#>>'{address,town}', '')),
   COALESCE(deep.requested_at, deep.updated_at)
  FROM (SELECT DISTINCT stable_key, company_key FROM source_rows) keys
  JOIN binocolo.ma_deep_analysis deep ON upper(btrim(deep.company_key)) = keys.company_key
  CROSS JOIN LATERAL (SELECT CASE WHEN jsonb_typeof(deep.itfull_payload->'data') = 'object' THEN deep.itfull_payload->'data' ELSE deep.itfull_payload END AS root) payload
 ), latest_locations AS (
  SELECT DISTINCT ON (stable_key) * FROM location_observations
  WHERE province <> '' OR town <> ''
  ORDER BY stable_key, seen_at DESC, (province <> '') DESC, (town <> '') DESC, province, town
 ), company_locations AS (
  SELECT loc.stable_key, loc.province, COALESCE(NULLIF(loc.town, ''), (
   SELECT old.town FROM location_observations old
   WHERE old.stable_key = loc.stable_key AND old.province = loc.province AND old.town <> ''
   ORDER BY old.seen_at DESC, old.town LIMIT 1
  ), '') AS town
  FROM latest_locations loc
 ), company_geography AS (
  SELECT loc.*, COALESCE(pc.region, '') AS region
  FROM company_locations loc LEFT JOIN province_catalog pc USING (province)
 )`

// Numeric observations retain their own year. Undated headcounts are a
// fallback, not the employeeRange code and not dated using the turnover year.
const maCompanySearchFactsSQL = maCompanySearchGeographySQL + `,
 company_keys AS (SELECT DISTINCT stable_key, company_key FROM source_rows),
 payloads AS (
  SELECT k.stable_key, t.vendor_payload AS payload, t.created_at AS observed_at, 'target:' || t.id::text AS source_id
  FROM company_keys k JOIN binocolo.ma_target t ON t.company_key = k.company_key
  JOIN binocolo.ma_session s ON s.id = t.session_id AND s.deleted_at IS NULL AND s.purged_at IS NULL
  UNION ALL
  SELECT k.stable_key, d.itfull_payload, COALESCE(d.requested_at, d.updated_at), 'deep:' || d.company_key
  FROM company_keys k JOIN binocolo.ma_deep_analysis d ON upper(btrim(d.company_key)) = k.company_key
  WHERE d.itfull_payload IS NOT NULL
  UNION ALL
  SELECT k.stable_key, v.payload, v.fetched_at, 'vintage:' || v.company_key || ':' || v.balance_sheet_date::text
  FROM company_keys k JOIN binocolo.ma_deep_payload_vintage v ON upper(btrim(v.company_key)) = k.company_key
 ), roots AS (
  SELECT *, CASE WHEN jsonb_typeof(payload->'data') = 'object' THEN payload->'data' ELSE payload END AS root FROM payloads
 ), observations AS (
  SELECT stable_key, observed_at, source_id, sheet->>'year' AS year,
    sheet->>'turnover' AS turnover, sheet->>'employees' AS employees
  FROM roots CROSS JOIN LATERAL jsonb_array_elements(
   CASE WHEN jsonb_typeof(root#>'{balanceSheets,all}') = 'array' THEN root#>'{balanceSheets,all}' ELSE '[]'::jsonb END
   || jsonb_build_array(root#>'{balanceSheets,last}')
  ) sheet
  UNION ALL
  SELECT stable_key, observed_at, source_id, root#>>'{ecofin,turnoverYear}', root#>>'{ecofin,turnover}', NULL FROM roots
  UNION ALL
  SELECT stable_key, observed_at, source_id, NULL, NULL,
   COALESCE(root#>>'{employees,employee}', CASE WHEN jsonb_typeof(root->'employees') = 'number' THEN root->>'employees' END)
  FROM roots
  UNION ALL
  -- Preserve normalized facts even when the retained payload is sparse. A
  -- legacy employeeRange code is not a headcount and must not leak back in.
  SELECT k.stable_key, t.created_at, 'target:' || t.id::text, t.turnover_year::text, t.turnover::text, NULL
  FROM company_keys k JOIN binocolo.ma_target t ON t.company_key = k.company_key
  JOIN binocolo.ma_session s ON s.id = t.session_id AND s.deleted_at IS NULL AND s.purged_at IS NULL
  UNION ALL
  SELECT k.stable_key, t.created_at, 'target:' || t.id::text, NULL, NULL, t.employees::text
  FROM company_keys k JOIN binocolo.ma_target t ON t.company_key = k.company_key
  JOIN binocolo.ma_session s ON s.id = t.session_id AND s.deleted_at IS NULL AND s.purged_at IS NULL
  WHERE t.vendor_payload#>'{employees,employeeRange}' IS NULL
    AND t.vendor_payload#>'{data,employees,employeeRange}' IS NULL
 ), numeric_observations AS (
  SELECT *, CASE WHEN year ~ '^[12][0-9]{3}$' THEN year::integer END AS metric_year,
   CASE WHEN turnover ~ '^[0-9]{1,15}(\.[0-9]+)?$' THEN turnover::numeric END AS revenue_value,
   CASE WHEN employees ~ '^[0-9]{1,9}$' THEN employees::integer END AS employees_value
  FROM observations
 ), latest_revenue AS (
  SELECT DISTINCT ON (stable_key) stable_key, revenue_value AS turnover, metric_year AS turnover_year
  FROM numeric_observations WHERE revenue_value IS NOT NULL
  ORDER BY stable_key, metric_year DESC NULLS LAST, observed_at DESC, source_id, revenue_value DESC
 ), latest_employees AS (
  SELECT DISTINCT ON (stable_key) stable_key, employees_value AS employees, metric_year AS employees_year
  FROM numeric_observations WHERE employees_value IS NOT NULL
  ORDER BY stable_key, metric_year DESC NULLS LAST, observed_at DESC, source_id, employees_value DESC
 ), company_summary AS (
  SELECT identity.stable_key, identity.company_name, COALESCE(geo.province, '') AS province,
   COALESCE(geo.town, '') AS town, COALESCE(geo.region, '') AS region,
   revenue.turnover, revenue.turnover_year, employees.employees, employees.employees_year
  FROM (
   SELECT DISTINCT ON (stable_key) stable_key, company_name
   FROM source_rows
   ORDER BY stable_key, CASE WHEN source_kind = 'registro' THEN 2 ELSE 1 END, seen_at DESC,
    CASE source_kind WHEN 'ricerca' THEN 1 WHEN 'iniziativa' THEN 2 ELSE 3 END, company_key, entity_id
  ) identity
  LEFT JOIN company_geography geo USING (stable_key)
  LEFT JOIN latest_revenue revenue USING (stable_key)
  LEFT JOIN latest_employees employees USING (stable_key)
 )`

const maCompanySearchFilterSQL = `,
 filters AS (SELECT $1::jsonb AS f),
 identity_matches AS (
  -- Aggregate once across aliases; a correlated scan per company made narrow
  -- name searches quadratic in the size of the internal corpus.
  SELECT sr.stable_key FROM source_rows sr CROSS JOIN filters GROUP BY sr.stable_key
  HAVING bool_or(
    f->>'kind' = 'recent'
    OR (f->>'kind' = 'name' AND sr.company_name ILIKE ('%' || (f->>'query') || '%') ESCAPE '\')
    OR (f->>'kind' = 'vat' AND (sr.stable_key = f->>'query' OR sr.tax_clean = f->>'query'))
    OR (f->>'kind' = 'tax' AND sr.tax_clean = f->>'query')
  )
  AND bool_or(f->>'name' = '' OR sr.company_name ILIKE ('%' || (f->>'name') || '%') ESCAPE '\')
  AND bool_or(f->>'vat' = '' OR binocolo.ma_stable_vat(sr.vat_code) = f->>'vat')
  AND bool_or(f->>'tax' = '' OR sr.tax_clean = f->>'tax')
 ), matched_keys AS (
  SELECT c.* FROM company_summary c JOIN identity_matches USING (stable_key) CROSS JOIN filters
  WHERE (f->>'annotation' = '' OR EXISTS (
   SELECT 1 FROM binocolo.ma_target_outcome note JOIN company_keys k ON k.company_key = upper(btrim(note.company_key))
   WHERE k.stable_key = c.stable_key AND note.event = 'nota' AND note.deleted_at IS NULL
    AND note.note ILIKE ('%' || (f->>'annotation') || '%') ESCAPE '\'
  ))
  AND (f->>'nda' = '' OR (
   -- expires_on inclusivo: un accordo è ancora attivo se scade oggi o non scade mai.
   CASE f->>'nda'
    WHEN 'any' THEN EXISTS (
     SELECT 1 FROM binocolo.ma_company_agreement a JOIN company_keys k ON k.company_key = upper(btrim(a.company_key))
     WHERE k.stable_key = c.stable_key AND a.kind = 'nda' AND a.deleted_at IS NULL
    )
    WHEN 'active' THEN EXISTS (
     SELECT 1 FROM binocolo.ma_company_agreement a JOIN company_keys k ON k.company_key = upper(btrim(a.company_key))
     WHERE k.stable_key = c.stable_key AND a.kind = 'nda' AND a.deleted_at IS NULL
      AND (a.expires_on IS NULL OR a.expires_on >= current_date)
    )
    WHEN 'expired_only' THEN EXISTS (
     SELECT 1 FROM binocolo.ma_company_agreement a JOIN company_keys k ON k.company_key = upper(btrim(a.company_key))
     WHERE k.stable_key = c.stable_key AND a.kind = 'nda' AND a.deleted_at IS NULL
    ) AND NOT EXISTS (
     SELECT 1 FROM binocolo.ma_company_agreement a JOIN company_keys k ON k.company_key = upper(btrim(a.company_key))
     WHERE k.stable_key = c.stable_key AND a.kind = 'nda' AND a.deleted_at IS NULL
      AND (a.expires_on IS NULL OR a.expires_on >= current_date)
    )
    WHEN 'none' THEN NOT EXISTS (
     SELECT 1 FROM binocolo.ma_company_agreement a JOIN company_keys k ON k.company_key = upper(btrim(a.company_key))
     WHERE k.stable_key = c.stable_key AND a.kind = 'nda' AND a.deleted_at IS NULL
    )
    ELSE false
   END))
  AND (jsonb_array_length(f->'include') = 0 OR EXISTS (
   SELECT 1 FROM jsonb_array_elements_text(f->'include') area WHERE area IN ('region:' || lower(c.region), 'province:' || c.province)
  ))
  AND NOT EXISTS (
   SELECT 1 FROM jsonb_array_elements_text(f->'exclude') area WHERE area IN ('region:' || lower(c.region), 'province:' || c.province)
  )
  AND ((f->>'turnoverMin' IS NULL AND f->>'turnoverMax' IS NULL)
   OR (c.turnover IS NULL AND (f->>'turnoverMissing')::boolean)
   OR (c.turnover IS NOT NULL AND (f->>'turnoverMin' IS NULL OR c.turnover >= (f->>'turnoverMin')::numeric) AND (f->>'turnoverMax' IS NULL OR c.turnover <= (f->>'turnoverMax')::numeric)))
  AND ((f->>'employeesMin' IS NULL AND f->>'employeesMax' IS NULL)
   OR (c.employees IS NULL AND (f->>'employeesMissing')::boolean)
   OR (c.employees IS NOT NULL AND (f->>'employeesMin' IS NULL OR c.employees >= (f->>'employeesMin')::integer) AND (f->>'employeesMax' IS NULL OR c.employees <= (f->>'employeesMax')::integer)))
 )`

func (s *SQLStore) ListMACompanySearchAreas(ctx context.Context) (MACompanySearchAreas, error) {
	out := MACompanySearchAreas{Items: []MACompanySearchArea{}}
	if s == nil || s.db == nil {
		return out, errors.New("binocolo ma store not configured")
	}
	rows, err := s.db.QueryContext(ctx, maCompanyCorpusSQL+maCompanySearchGeographySQL+`
 SELECT value, label FROM (
  SELECT 'region:' || lower(region) AS value, region AS label FROM province_catalog WHERE region <> ''
  UNION
  SELECT 'province:' || province, 'Provincia di ' || COALESCE(NULLIF(province_name, ''), province) || ' (' || province || ')' FROM province_catalog WHERE province <> ''
  UNION
  SELECT 'province:' || province, 'Provincia di ' || province FROM company_locations WHERE province <> '' AND province NOT IN (SELECT province FROM province_catalog)
 ) areas ORDER BY label, value`)
	if err != nil {
		return out, fmt.Errorf("list company search areas: %w", err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var item MACompanySearchArea
		if err := rows.Scan(&item.Value, &item.Label); err != nil {
			return out, err
		}
		if !seen[item.Value] {
			out.Items = append(out.Items, item)
			seen[item.Value] = true
		}
		if strings.HasPrefix(item.Value, "region:") {
			out.RegionsAvailable = true
		}
	}
	return out, rows.Err()
}

func (s *SQLStore) SearchMACompanies(ctx context.Context, options maCompanySearchOptions) (MACompanySearchResponse, error) {
	if s == nil || s.db == nil {
		return MACompanySearchResponse{}, errors.New("binocolo ma store not configured")
	}
	raw, err := json.Marshal(options)
	if err != nil {
		return MACompanySearchResponse{}, err
	}
	// Count and page must see the same corpus, including an empty/out-of-range page.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return MACompanySearchResponse{}, fmt.Errorf("begin company search: %w", err)
	}
	defer tx.Rollback()
	var total int
	if err := tx.QueryRowContext(ctx, maCompanyCorpusSQL+maCompanySearchFactsSQL+maCompanySearchFilterSQL+" SELECT count(*) FROM matched_keys", string(raw)).Scan(&total); err != nil {
		return MACompanySearchResponse{}, fmt.Errorf("count ma companies: %w", err)
	}
	lastPage := max(1, (total+options.PageSize-1)/options.PageSize)
	options.Page = min(options.Page, lastPage)
	rows, err := tx.QueryContext(ctx, maCompanyCorpusSQL+maCompanySearchFactsSQL+maCompanySearchFilterSQL+`,
 selected_keys AS (
   SELECT *, row_number() OVER (ORDER BY `+maCompanySearchOrder(options)+`) AS position
   FROM matched_keys
   ORDER BY `+maCompanySearchOrder(options)+`
   LIMIT $5 OFFSET $6
 ), key_map AS (
   SELECT DISTINCT sr.stable_key, sr.company_key
   FROM source_rows sr JOIN selected_keys sk USING (stable_key)
 )
SELECT
  identity.company_name, identity.vat_code, identity.tax_code, sk.province, sk.town,
  COALESCE(domain_row.domain, ''),
  (SELECT COUNT(DISTINCT sr.context_id) FROM source_rows sr WHERE sr.stable_key = sk.stable_key AND sr.source_kind = 'ricerca'),
  (SELECT COUNT(DISTINCT sr.context_id) FROM source_rows sr WHERE sr.stable_key = sk.stable_key AND sr.source_kind = 'iniziativa'),
  COALESCE(last_context.seen_at, identity.seen_at),
  COALESCE(last_context.source_kind, ''), COALESCE(last_context.context_id, ''), COALESCE(last_context.context_title, ''),
  COALESCE((
    SELECT jsonb_agg(keys.company_key ORDER BY keys.last_seen DESC)
    FROM (
      SELECT sr.company_key, MAX(sr.seen_at) AS last_seen
      FROM source_rows sr
      WHERE sr.stable_key = sk.stable_key
      GROUP BY sr.company_key
    ) keys
  ), '[]'::jsonb),
  COALESCE(last_context.company_key, identity.company_key),
  COALESCE(active_card.card_state, ''), COALESCE(active_card.context_title, ''),
  COALESCE(closed_card.card_state, ''), COALESCE(closed_card.card_esito, ''), COALESCE(closed_card.context_title, ''),
  latest_rating.rating, COALESCE(latest_rating.reason, ''),
  latest_target.id, latest_target.run_id, COALESCE(latest_target.company_key, ''), COALESCE(latest_target.company_name, ''),
  COALESCE(latest_target.origin, ''), COALESCE(latest_target.vat_code, ''), COALESCE(latest_target.province, ''),
  COALESCE(latest_target.town, ''), COALESCE(latest_target.ateco_code, ''), latest_target.score,
  latest_target.score_version, COALESCE(latest_target.match_state, ''), COALESCE(latest_target.confidence, ''),
  COALESCE(latest_target.flags, '[]'::jsonb), COALESCE(latest_target.enrichment_level, ''), latest_target.rating,
  COALESCE(latest_target.thesis, ''), COALESCE(latest_target.has_validation, false),
  COALESCE(latest_target.web_validation_state, ''), COALESCE(latest_target.final_action, ''),
  COALESCE(latest_target.selected_domain, ''), COALESCE(latest_target.final_reason, ''),
  COALESCE(latest_target.group_domain, ''), COALESCE(latest_target.group_identifier, ''),
  COALESCE(latest_target.candidate_count, 0), latest_target.outside_revenue, latest_target.outside_shareholders,
  EXISTS (
    SELECT 1
    FROM binocolo.ma_deep_analysis deep
    WHERE deep.status = 'ready'
      AND (
        EXISTS (SELECT 1 FROM key_map km WHERE km.stable_key = sk.stable_key AND km.company_key = upper(btrim(deep.company_key)))
        OR binocolo.ma_normalize_fiscal(deep.vat_code) = sk.stable_key
        OR binocolo.ma_normalize_fiscal(deep.tax_code) = sk.stable_key
      )
  ) AS has_deep,
  sk.turnover, sk.turnover_year, sk.employees, sk.employees_year
FROM selected_keys sk
JOIN LATERAL (
  SELECT sr.*
  FROM source_rows sr
  WHERE sr.stable_key = sk.stable_key
  ORDER BY CASE WHEN sr.source_kind = 'registro' THEN 2 ELSE 1 END, sr.seen_at DESC,
           CASE sr.source_kind WHEN 'ricerca' THEN 1 WHEN 'iniziativa' THEN 2 ELSE 3 END, sr.company_key, sr.entity_id
  LIMIT 1
) identity ON TRUE
LEFT JOIN LATERAL (
  SELECT sr.*
  FROM source_rows sr
  WHERE sr.stable_key = sk.stable_key AND sr.source_kind IN ('ricerca', 'iniziativa')
  ORDER BY sr.seen_at DESC, sr.source_kind
  LIMIT 1
) last_context ON TRUE
LEFT JOIN LATERAL (
  SELECT sr.domain
  FROM source_rows sr
  WHERE sr.stable_key = sk.stable_key AND sr.domain <> ''
  ORDER BY sr.seen_at DESC
  LIMIT 1
) domain_row ON TRUE
LEFT JOIN LATERAL (
  SELECT sr.card_state, sr.context_title
  FROM source_rows sr
  WHERE sr.stable_key = sk.stable_key AND sr.source_kind = 'iniziativa'
    AND sr.card_state NOT IN ('won', 'ko_nostro', 'ko_target', 'rimossa')
  ORDER BY sr.seen_at DESC
  LIMIT 1
) active_card ON TRUE
LEFT JOIN LATERAL (
  SELECT sr.card_state, sr.card_esito, sr.context_title
  FROM source_rows sr
  WHERE sr.stable_key = sk.stable_key AND sr.source_kind = 'iniziativa'
    AND sr.card_state IN ('won', 'ko_nostro', 'ko_target')
  ORDER BY sr.seen_at DESC
  LIMIT 1
) closed_card ON TRUE
LEFT JOIN LATERAL (
  SELECT rating.rating, rating.reason
  FROM binocolo.ma_target_rating rating
  JOIN binocolo.ma_session session ON session.id = rating.session_id
  WHERE session.deleted_at IS NULL AND session.purged_at IS NULL
    AND EXISTS (
      SELECT 1 FROM key_map km
      WHERE km.stable_key = sk.stable_key AND km.company_key = upper(btrim(rating.company_key))
    )
  ORDER BY rating.rated_at DESC
  LIMIT 1
) latest_rating ON TRUE
LEFT JOIN LATERAL (
  SELECT
    target.id::text AS id, target.run_id::text AS run_id, sr.company_key, target.company_name,
    COALESCE(target.origin, 'search') AS origin, target.vat_code, target.province, target.town, target.ateco_code,
    target.score, target.score_version, target.match_state, target.confidence, target.flags,
    COALESCE(target.enrichment_level, 'advanced') AS enrichment_level, rating.rating,
    COALESCE(strategy.strategy->>'thesis', '') AS thesis,
    validation.company_key IS NOT NULL AS has_validation,
    validation.web_validation_state, validation.final_action, validation.selected_domain,
    validation.final_decision->>'reason' AS final_reason,
    validation.domain_response->'groupSiteHint'->>'domain' AS group_domain,
    validation.domain_response->'groupSiteHint'->>'identifier' AS group_identifier,
    jsonb_array_length(CASE WHEN jsonb_typeof(validation.domain_response->'candidates') = 'array' THEN validation.domain_response->'candidates' ELSE '[]'::jsonb END) AS candidate_count,
    outside_filter.revenue_per_employee_value AS outside_revenue,
    outside_filter.max_shareholders_value AS outside_shareholders
  FROM source_rows sr
  JOIN binocolo.ma_target target ON target.id::text = sr.entity_id
  JOIN binocolo.ma_session session ON session.id::text = sr.context_id
  LEFT JOIN binocolo.ma_strategy_version strategy ON strategy.id = session.active_strategy_id
  LEFT JOIN binocolo.ma_target_rating rating ON rating.session_id = session.id AND upper(btrim(rating.company_key)) = sr.company_key
  LEFT JOIN binocolo.ma_target_web_validation validation ON validation.session_id = session.id AND validation.company_key = sr.company_key
  LEFT JOIN LATERAL (
    SELECT
      MAX(COALESCE(evidence.value, '')) FILTER (WHERE evidence.criterion = $2) AS revenue_per_employee_value,
      MAX(COALESCE(evidence.value, '')) FILTER (WHERE evidence.criterion = $3) AS max_shareholders_value
    FROM binocolo.ma_evidence evidence
    WHERE evidence.target_id = target.id AND evidence.status = $4
      AND evidence.criterion IN ($2, $3)
  ) outside_filter ON TRUE
  WHERE sr.stable_key = sk.stable_key AND sr.source_kind = 'ricerca'
  ORDER BY sr.seen_at DESC
  LIMIT 1
) latest_target ON TRUE
ORDER BY sk.position
`, string(raw), maPostFilterRevenuePerEmployeeMin, maPostFilterMaxShareholders, maEvidenceOutside, options.PageSize, (options.Page-1)*options.PageSize)
	if err != nil {
		return MACompanySearchResponse{}, fmt.Errorf("search ma companies: %w", err)
	}
	defer rows.Close()
	out := []MACompanySearchRow{}
	for rows.Next() {
		var item MACompanySearchRow
		var contextType, contextID, contextTitle string
		var companyKeysRaw, flagsRaw []byte
		var hydration maCompanySearchHydration
		var latestRating, targetRating sql.NullInt64
		var targetID, targetRunID sql.NullString
		var targetScore, targetScoreVersion sql.NullInt64
		var targetCompanyKey, targetCompanyName, targetOrigin, targetVAT, targetProvince, targetTown, targetAteco string
		var targetMatchState, targetConfidence, targetEnrichment, targetThesis string
		var hasValidation bool
		var webState, finalAction, selectedDomain, finalReason, groupDomain, groupIdentifier string
		var candidateCount int
		var outsideRevenue, outsideShareholders sql.NullString
		if err := rows.Scan(
			&item.CompanyName, &item.VATCode, &item.TaxCode, &item.Province, &item.Town, &item.Domain,
			&item.SessionCount, &item.InitiativeCount, &item.LastSeenAt,
			&contextType, &contextID, &contextTitle, &companyKeysRaw, &item.PrimaryCompanyKey,
			&hydration.ActiveCardState, &hydration.ActiveCardInitiative,
			&hydration.ClosedCardState, &hydration.ClosedCardEsito, &hydration.ClosedCardInitiative,
			&latestRating, &hydration.LatestRatingReason,
			&targetID, &targetRunID, &targetCompanyKey, &targetCompanyName, &targetOrigin, &targetVAT,
			&targetProvince, &targetTown, &targetAteco, &targetScore, &targetScoreVersion,
			&targetMatchState, &targetConfidence, &flagsRaw, &targetEnrichment, &targetRating, &targetThesis,
			&hasValidation, &webState, &finalAction, &selectedDomain, &finalReason, &groupDomain, &groupIdentifier,
			&candidateCount, &outsideRevenue, &outsideShareholders, &item.HasDeep,
			&item.Turnover, &item.TurnoverYear, &item.Employees, &item.EmployeesYear,
		); err != nil {
			return MACompanySearchResponse{}, fmt.Errorf("scan ma company search row: %w", err)
		}
		if err := json.Unmarshal(companyKeysRaw, &item.CompanyKeys); err != nil {
			return MACompanySearchResponse{}, fmt.Errorf("decode ma company search keys: %w", err)
		}
		if contextID != "" {
			item.LastContext = &MACompanySearchContext{Type: contextType, ID: contextID, Title: contextTitle}
		}
		if latestRating.Valid {
			value := int(latestRating.Int64)
			hydration.LatestRating = &value
		}
		if targetID.Valid {
			target := MATargetRow{
				ID: targetID.String, RunID: targetRunID.String, CompanyKey: targetCompanyKey,
				CompanyName: targetCompanyName, Origin: targetOrigin, VATCode: targetVAT,
				Province: targetProvince, Town: targetTown, AtecoCode: targetAteco,
				MatchState: targetMatchState, Confidence: targetConfidence, EnrichmentLevel: targetEnrichment,
			}
			if targetScore.Valid {
				target.Score = int(targetScore.Int64)
			}
			if targetScoreVersion.Valid {
				value := int(targetScoreVersion.Int64)
				target.ScoreVersion = &value
			}
			if targetRating.Valid {
				value := int(targetRating.Int64)
				target.Rating = &value
			}
			if len(flagsRaw) > 0 {
				_ = json.Unmarshal(flagsRaw, &target.Flags)
			}
			if outsideRevenue.Valid {
				value := outsideRevenue.String
				target.OutsideRevenuePerEmployeeValue = &value
			}
			if outsideShareholders.Valid {
				value := outsideShareholders.String
				target.OutsideMaxShareholdersValue = &value
			}
			if hasValidation {
				target.WebValidation = &MATargetRowWeb{
					WebValidationState: webState, FinalAction: finalAction, SelectedDomain: selectedDomain,
					FinalDecision: MATargetRowFinalDecision{Reason: finalReason}, GroupSiteDomain: groupDomain,
					GroupSiteIdentifier: groupIdentifier, CandidateCount: candidateCount,
				}
			}
			hydration.LatestTarget = &target
			hydration.LatestTargetThesis = targetThesis
		}
		item.Status = resolveMACompanySearchStatus(hydration)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return MACompanySearchResponse{}, fmt.Errorf("iterate ma company search rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return MACompanySearchResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return MACompanySearchResponse{}, err
	}
	return MACompanySearchResponse{Items: out, Total: total, Page: options.Page, PageSize: options.PageSize}, nil
}
