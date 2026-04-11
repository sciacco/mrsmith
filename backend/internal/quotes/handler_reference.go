package quotes

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// ── Templates ──

func (h *Handler) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	templateType := r.URL.Query().Get("type")
	lang := r.URL.Query().Get("lang")
	isColoStr := r.URL.Query().Get("is_colo")

	query := `SELECT template_id, description, lang, template_type, kit_id, service_category_id, is_colo, is_active
	          FROM quotes.template WHERE is_active = true`
	args := []any{}
	argIdx := 0

	if templateType != "" {
		argIdx++
		query += ` AND template_type = $` + strconv.Itoa(argIdx)
		args = append(args, templateType)
	}
	if lang != "" {
		argIdx++
		query += ` AND lang = $` + strconv.Itoa(argIdx)
		args = append(args, lang)
	}
	if isColoStr != "" {
		argIdx++
		query += ` AND is_colo = $` + strconv.Itoa(argIdx)
		args = append(args, isColoStr == "true")
	}

	query += ` ORDER BY description`

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_templates", err)
		return
	}
	defer rows.Close()

	type tmpl struct {
		TemplateID        string `json:"template_id"`
		Description       string `json:"description"`
		Lang              string `json:"lang"`
		TemplateType      string `json:"template_type"`
		KitID             *int64 `json:"kit_id"`
		ServiceCategoryID *int   `json:"service_category_id"`
		IsColo            bool   `json:"is_colo"`
		IsActive          bool   `json:"is_active"`
	}

	result := []tmpl{}
	for rows.Next() {
		var t tmpl
		if err := rows.Scan(&t.TemplateID, &t.Description, &t.Lang, &t.TemplateType, &t.KitID, &t.ServiceCategoryID, &t.IsColo, &t.IsActive); err != nil {
			h.dbFailure(w, r, "list_templates_scan", err)
			return
		}
		result = append(result, t)
	}
	if !h.rowsDone(w, r, rows, "list_templates") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ── Product Categories ──

func (h *Handler) handleListCategories(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	excludeStandard := r.URL.Query().Get("exclude_standard") == "true"

	query := `SELECT id, name FROM products.product_category`
	args := []any{}

	if excludeStandard {
		query += ` WHERE id NOT IN (12, 13, 14, 15)`
	}
	query += ` ORDER BY name`

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.dbFailure(w, r, "list_categories", err)
		return
	}
	defer rows.Close()

	type category struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	result := []category{}
	for rows.Next() {
		var c category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			h.dbFailure(w, r, "list_categories_scan", err)
			return
		}
		result = append(result, c)
	}
	if !h.rowsDone(w, r, rows, "list_categories") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ── Kits ──

func (h *Handler) handleListKits(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	query := `SELECT k.id, k.internal_name, k.nrc, k.mrc, k.category_id, pc.name as category_name,
	                 k.is_active, k.ecommerce, k.quotable
	          FROM products.kit k
	          LEFT JOIN products.product_category pc ON pc.id = k.category_id
	          WHERE k.is_active = true AND k.ecommerce = false AND k.quotable = true
	          ORDER BY pc.name, k.internal_name`

	rows, err := h.db.QueryContext(r.Context(), query)
	if err != nil {
		h.dbFailure(w, r, "list_kits", err)
		return
	}
	defer rows.Close()

	type kit struct {
		ID           int64          `json:"id"`
		InternalName string         `json:"internal_name"`
		NRC          float64        `json:"nrc"`
		MRC          float64        `json:"mrc"`
		CategoryID   sql.NullInt32  `json:"-"`
		CategoryName sql.NullString `json:"-"`
		Category     *string        `json:"category_name"`
		CategoryIDV  *int           `json:"category_id"`
		IsActive     bool           `json:"is_active"`
		Ecommerce    bool           `json:"ecommerce"`
		Quotable     bool           `json:"quotable"`
	}

	result := []kit{}
	for rows.Next() {
		var k kit
		if err := rows.Scan(&k.ID, &k.InternalName, &k.NRC, &k.MRC, &k.CategoryID, &k.CategoryName, &k.IsActive, &k.Ecommerce, &k.Quotable); err != nil {
			h.dbFailure(w, r, "list_kits_scan", err)
			return
		}
		if k.CategoryName.Valid {
			k.Category = &k.CategoryName.String
		}
		if k.CategoryID.Valid {
			v := int(k.CategoryID.Int32)
			k.CategoryIDV = &v
		}
		result = append(result, k)
	}
	if !h.rowsDone(w, r, rows, "list_kits") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ── Customers (HubSpot companies from loader) ──

func (h *Handler) handleListCustomers(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	query := `SELECT id, name, numero_azienda FROM loader.hubs_company ORDER BY name`

	rows, err := h.db.QueryContext(r.Context(), query)
	if err != nil {
		h.dbFailure(w, r, "list_customers", err)
		return
	}
	defer rows.Close()

	type customer struct {
		ID             int64          `json:"id"`
		Name           string         `json:"name"`
		NumeroAzienda  sql.NullString `json:"-"`
		NumeroAziendaV *string        `json:"numero_azienda"`
	}

	result := []customer{}
	for rows.Next() {
		var c customer
		if err := rows.Scan(&c.ID, &c.Name, &c.NumeroAzienda); err != nil {
			h.dbFailure(w, r, "list_customers_scan", err)
			return
		}
		if c.NumeroAzienda.Valid {
			c.NumeroAziendaV = &c.NumeroAzienda.String
		}
		result = append(result, c)
	}
	if !h.rowsDone(w, r, rows, "list_customers") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ── Deals ──

// Pipeline and stage filtering constants from Appsmith get_potentials / get_deals SQL.
// These are backend constants per spec decision A6 (not configurable in v1).
// Source: Appsmith export — pages/Nuova Proposta/queries/get_potentials.txt
// Stages are coupled per pipeline — the WHERE clause must pair them.
var (
	standardPipeline = "255768766"
	standardStages   = []string{"424443344", "424502259", "424502261", "424502262"}
	iaasPipeline     = "255768768"
	iaasStages       = []string{"424443381", "424443586", "424443588", "424443587", "424443589"}
)

// quoteStageInClause renders a `('a','b',...)` SQL IN list from a stage slice.
// The stage IDs are hardcoded backend constants (not user input), so direct
// interpolation is safe and matches the Appsmith source query verbatim.
func quoteStageInClause(stages []string) string {
	quoted := make([]string, len(stages))
	for i, s := range stages {
		quoted[i] = "'" + s + "'"
	}
	return "(" + strings.Join(quoted, ",") + ")"
}

// listDealsQuery reproduces the Appsmith `get_potentials` eligibility rules:
// (standard pipeline + its stages) OR (iaas pipeline + its stages), AND a
// non-empty `codice`. Ordering is kept deterministic by id desc to match the
// Appsmith source (`order by id desc`).
var listDealsQuery = `SELECT d.id, d.name, d.pipeline, d.dealstage,
                             c.id as company_id, c.name as company_name
                      FROM loader.hubs_deal d
                      LEFT JOIN loader.hubs_company c ON c.id = d.company_id
                      WHERE ((d.pipeline = '` + standardPipeline + `' AND d.dealstage IN ` + quoteStageInClause(standardStages) + `)
                          OR (d.pipeline = '` + iaasPipeline + `' AND d.dealstage IN ` + quoteStageInClause(iaasStages) + `))
                        AND d.codice <> ''
                      ORDER BY d.id DESC`

func (h *Handler) handleListDeals(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	rows, err := h.db.QueryContext(r.Context(), listDealsQuery)
	if err != nil {
		h.dbFailure(w, r, "list_deals", err)
		return
	}
	defer rows.Close()

	type deal struct {
		ID           int64          `json:"id"`
		DealName     string         `json:"name"`
		Pipeline     sql.NullString `json:"-"`
		PipelineV    *string        `json:"pipeline"`
		DealStage    sql.NullString `json:"-"`
		DealStageV   *string        `json:"dealstage"`
		CompanyID    sql.NullInt64  `json:"-"`
		CompanyIDV   *int64         `json:"company_id"`
		CompanyName  sql.NullString `json:"-"`
		CompanyNameV *string        `json:"company_name"`
	}

	result := []deal{}
	for rows.Next() {
		var d deal
		if err := rows.Scan(&d.ID, &d.DealName, &d.Pipeline, &d.DealStage, &d.CompanyID, &d.CompanyName); err != nil {
			h.dbFailure(w, r, "list_deals_scan", err)
			return
		}
		if d.Pipeline.Valid {
			d.PipelineV = &d.Pipeline.String
		}
		if d.DealStage.Valid {
			d.DealStageV = &d.DealStage.String
		}
		if d.CompanyID.Valid {
			d.CompanyIDV = &d.CompanyID.Int64
		}
		if d.CompanyName.Valid {
			d.CompanyNameV = &d.CompanyName.String
		}
		result = append(result, d)
	}
	if !h.rowsDone(w, r, rows, "list_deals") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ── Deal detail ──

func (h *Handler) handleGetDeal(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	dealID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_deal_id")
		return
	}

	type dealDetail struct {
		ID            int64   `json:"id"`
		DealName      string  `json:"name"`
		Pipeline      *string `json:"pipeline"`
		DealStage     *string `json:"dealstage"`
		CompanyID     *int64  `json:"company_id"`
		CompanyName   *string `json:"company_name"`
		NumeroAzienda *string `json:"numero_azienda"`
	}

	var d dealDetail
	err = h.db.QueryRowContext(r.Context(),
		`SELECT d.id, d.name, d.pipeline, d.dealstage,
		        c.id, c.name, c.numero_azienda
		 FROM loader.hubs_deal d
		 LEFT JOIN loader.hubs_company c ON c.id = d.company_id
		 WHERE d.id = $1`, dealID).Scan(
		&d.ID, &d.DealName, &d.Pipeline, &d.DealStage, &d.CompanyID, &d.CompanyName, &d.NumeroAzienda)

	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "deal_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_deal", err)
		return
	}

	httputil.JSON(w, http.StatusOK, d)
}

// ── Owners ──

func (h *Handler) handleListOwners(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	query := `SELECT id, first_name, last_name, email FROM loader.hubs_owner WHERE archived = FALSE ORDER BY last_name, first_name`

	rows, err := h.db.QueryContext(r.Context(), query)
	if err != nil {
		h.dbFailure(w, r, "list_owners", err)
		return
	}
	defer rows.Close()

	type owner struct {
		ID         string         `json:"id"`
		FirstName  sql.NullString `json:"-"`
		LastName   sql.NullString `json:"-"`
		Email      sql.NullString `json:"-"`
		FirstNameV *string        `json:"firstname"`
		LastNameV  *string        `json:"lastname"`
		EmailV     *string        `json:"email"`
	}

	result := []owner{}
	for rows.Next() {
		var o owner
		if err := rows.Scan(&o.ID, &o.FirstName, &o.LastName, &o.Email); err != nil {
			h.dbFailure(w, r, "list_owners_scan", err)
			return
		}
		if o.FirstName.Valid {
			o.FirstNameV = &o.FirstName.String
		}
		if o.LastName.Valid {
			o.LastNameV = &o.LastName.String
		}
		if o.Email.Valid {
			o.EmailV = &o.Email.String
		}
		result = append(result, o)
	}
	if !h.rowsDone(w, r, rows, "list_owners") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ── Payment Methods ──

func (h *Handler) handleListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	query := `SELECT cod_pagamento, desc_pagamento FROM loader.erp_metodi_pagamento WHERE selezionabile IS TRUE ORDER BY desc_pagamento`

	rows, err := h.db.QueryContext(r.Context(), query)
	if err != nil {
		h.dbFailure(w, r, "list_payment_methods", err)
		return
	}
	defer rows.Close()

	type paymentMethod struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	}

	result := []paymentMethod{}
	for rows.Next() {
		var pm paymentMethod
		if err := rows.Scan(&pm.Code, &pm.Description); err != nil {
			h.dbFailure(w, r, "list_payment_methods_scan", err)
			return
		}
		result = append(result, pm)
	}
	if !h.rowsDone(w, r, rows, "list_payment_methods") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}

// ── Alyante: Customer Default Payment ──

func (h *Handler) handleCustomerPayment(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	if customerID == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_customer_id")
		return
	}

	type payment struct {
		PaymentCode string `json:"payment_code"`
	}
	defaultPayment := payment{PaymentCode: "402"}

	// Alyante not configured — return documented fallback (not 503)
	if h.alyanteDB == nil {
		httputil.JSON(w, http.StatusOK, defaultPayment)
		return
	}

	// Step 1: Resolve ERP bridge — customerId is the HubSpot company ID,
	// not the Alyante NUMERO_AZIENDA. Look up the ERP link first.
	if !h.requireDB(w) {
		return
	}
	var erpID sql.NullString
	err := h.db.QueryRowContext(r.Context(),
		`SELECT numero_azienda FROM loader.hubs_company WHERE id = $1`,
		customerID).Scan(&erpID)
	if err != nil && err != sql.ErrNoRows {
		h.dbFailure(w, r, "customer_payment_bridge", err)
		return
	}
	if err == sql.ErrNoRows || !erpID.Valid {
		httputil.JSON(w, http.StatusOK, defaultPayment)
		return
	}

	// Step 2: Query Alyante with the resolved ERP ID
	var p payment
	err = h.alyanteDB.QueryRowContext(r.Context(),
		`SELECT TOP 1 LTRIM(RTRIM(AN_CONDPAG)) as payment_code
		 FROM Tsmi_Anagrafiche_clienti
		 WHERE NUMERO_AZIENDA = @p1`, erpID.String).Scan(&p.PaymentCode)

	if err == sql.ErrNoRows {
		httputil.JSON(w, http.StatusOK, defaultPayment)
		return
	}
	if err != nil {
		h.dbFailure(w, r, "customer_payment", err)
		return
	}

	httputil.JSON(w, http.StatusOK, p)
}

// ── Alyante: Customer Orders (for SOSTITUZIONE) ──

func (h *Handler) handleCustomerOrders(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	if customerID == "" {
		httputil.Error(w, http.StatusBadRequest, "missing_customer_id")
		return
	}

	// Alyante not configured — return documented fallback (not 503)
	if h.alyanteDB == nil {
		httputil.JSON(w, http.StatusOK, []struct{}{})
		return
	}

	// Step 1: Resolve ERP bridge — customerId is the HubSpot company ID.
	if !h.requireDB(w) {
		return
	}
	var erpID sql.NullString
	err := h.db.QueryRowContext(r.Context(),
		`SELECT numero_azienda FROM loader.hubs_company WHERE id = $1`,
		customerID).Scan(&erpID)
	if err != nil && err != sql.ErrNoRows {
		h.dbFailure(w, r, "customer_orders_bridge", err)
		return
	}
	if err == sql.ErrNoRows || !erpID.Valid {
		httputil.JSON(w, http.StatusOK, []struct{}{})
		return
	}

	// Step 2: Query Alyante with the resolved ERP ID.
	// IMPORTANT: Always filter by customer ID — this fixes the Appsmith bug
	// where cli_orders was unscoped (bug A7 in spec).
	query := `SELECT TOP 500 LTRIM(RTRIM(NOME)) as order_name
	          FROM Tsmi_Ordini
	          WHERE NUMERO_AZIENDA = @p1
	          ORDER BY NOME DESC`

	rows, err := h.alyanteDB.QueryContext(r.Context(), query, erpID.String)
	if err != nil {
		h.dbFailure(w, r, "customer_orders", err)
		return
	}
	defer rows.Close()

	type order struct {
		Name string `json:"name"`
	}

	result := []order{}
	for rows.Next() {
		var o order
		if err := rows.Scan(&o.Name); err != nil {
			h.dbFailure(w, r, "customer_orders_scan", err)
			return
		}
		result = append(result, o)
	}
	if !h.rowsDone(w, r, rows, "customer_orders") {
		return
	}

	httputil.JSON(w, http.StatusOK, result)
}
