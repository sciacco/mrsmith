package listini

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// handleListKits returns all active, non-ecommerce kits grouped by category.
func (h *Handler) handleListKits(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
		SELECT k.id, k.internal_name, k.billing_period,
		       k.initial_subscription_months, k.next_subscription_months,
		       k.activation_time_days, k.category_id,
		       pc.name AS category_name, pc.color AS category_color,
		       k.is_main_prd_sellable, k.sconto_massimo,
		       k.variable_billing, k.h24_assurance,
		       k.sla_resolution_hours, k.notes
		FROM products.kit k
		JOIN products.product_category pc ON pc.id = k.category_id
		WHERE k.is_active = true AND k.ecommerce = false
		ORDER BY pc.name, k.internal_name`)
	if err != nil {
		h.dbFailure(w, r, "list_kits", err)
		return
	}
	defer rows.Close()

	type kit struct {
		ID                        int     `json:"id"`
		InternalName              string  `json:"internal_name"`
		BillingPeriod             string  `json:"billing_period"`
		InitialSubscriptionMonths int     `json:"initial_subscription_months"`
		NextSubscriptionMonths    int     `json:"next_subscription_months"`
		ActivationTimeDays        int     `json:"activation_time_days"`
		CategoryID                int     `json:"category_id"`
		CategoryName              string  `json:"category_name"`
		CategoryColor             string  `json:"category_color"`
		IsMainPrdSellable         bool    `json:"is_main_prd_sellable"`
		ScontoMassimo             float64 `json:"sconto_massimo"`
		VariableBilling           bool    `json:"variable_billing"`
		H24Assurance              bool    `json:"h24_assurance"`
		SLAResolutionHours        int     `json:"sla_resolution_hours"`
		Notes                     *string `json:"notes"`
	}

	var result []kit
	for rows.Next() {
		var k kit
		if err := rows.Scan(
			&k.ID, &k.InternalName, &k.BillingPeriod,
			&k.InitialSubscriptionMonths, &k.NextSubscriptionMonths,
			&k.ActivationTimeDays, &k.CategoryID,
			&k.CategoryName, &k.CategoryColor,
			&k.IsMainPrdSellable, &k.ScontoMassimo,
			&k.VariableBilling, &k.H24Assurance,
			&k.SLAResolutionHours, &k.Notes,
		); err != nil {
			h.dbFailure(w, r, "list_kits_scan", err)
			return
		}
		result = append(result, k)
	}
	if !h.rowsDone(w, r, rows, "list_kits") {
		return
	}
	if result == nil {
		result = []kit{}
	}

	httputil.JSON(w, http.StatusOK, result)
}

// handleGetKitProducts returns products for a specific kit, grouped by group_name.
func (h *Handler) handleGetKitProducts(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	kitID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_kit_id")
		return
	}

	type product struct {
		GroupName   *string `json:"group_name"`
		Name        string  `json:"internal_name"`
		NRC         float64 `json:"nrc"`
		MRC         float64 `json:"mrc"`
		Minimum     int     `json:"minimum"`
		Maximum     *int    `json:"maximum"`
		Required    bool    `json:"required"`
		Position    int     `json:"position"`
		ProductCode string  `json:"product_code"`
		Notes       *string `json:"notes"`
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
		(
			SELECT kp.group_name, p.internal_name,
			       kp.minimum, kp.maximum, kp.required,
			       kp.nrc, kp.mrc, kp.position,
			       kp.product_code, kp.notes
			FROM products.kit_product kp
			LEFT JOIN products.product p ON kp.product_code = p.code
			WHERE kp.kit_id = $1
		)
		UNION ALL
		(
			SELECT 'Articolo principale' AS group_name,
			       p.internal_name,
			       1 AS minimum, NULL AS maximum, TRUE AS required,
			       k.nrc, k.mrc, -1 AS position,
			       k.main_product_code AS product_code,
			       '' AS notes
			FROM products.kit k
			LEFT JOIN products.product p ON k.main_product_code = p.code
			WHERE k.id = $1 AND k.is_main_prd_sellable
		)
		ORDER BY position, group_name, internal_name`, kitID)
	if err != nil {
		h.dbFailure(w, r, "get_kit_products", err)
		return
	}
	defer rows.Close()

	var products []product
	for rows.Next() {
		var p product
		if err := rows.Scan(
			&p.GroupName, &p.Name,
			&p.Minimum, &p.Maximum, &p.Required,
			&p.NRC, &p.MRC, &p.Position,
			&p.ProductCode, &p.Notes,
		); err != nil {
			h.dbFailure(w, r, "get_kit_products_scan", err)
			return
		}
		products = append(products, p)
	}
	if !h.rowsDone(w, r, rows, "get_kit_products") {
		return
	}
	if products == nil {
		products = []product{}
	}

	httputil.JSON(w, http.StatusOK, products)
}

// handleGetKitHelpURL returns the help URL for a kit.
func (h *Handler) handleGetKitHelpURL(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) {
		return
	}

	kitID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_kit_id")
		return
	}

	var helpURL *string
	err = h.mistraDB.QueryRowContext(r.Context(),
		`SELECT help_url FROM products.kit_help WHERE kit_id = $1`, kitID).Scan(&helpURL)
	if err != nil && err != sql.ErrNoRows {
		h.dbFailure(w, r, "get_kit_help_url", err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]*string{"help_url": helpURL})
}

// handleGenerateKitPDF generates a PDF for a kit using Carbone.
func (h *Handler) handleGenerateKitPDF(w http.ResponseWriter, r *http.Request) {
	if h.carbone == nil {
		httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "pdf_generation_unavailable"})
		return
	}
	if !h.requireMistra(w) {
		return
	}

	kitID, err := pathID(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_kit_id")
		return
	}

	logger := logging.FromContext(r.Context())

	// Fetch kit metadata — use DB column names to match Carbone template placeholders
	var kitName, billingPeriod, categoryName, categoryColor string
	var initialMonths, nextMonths, activationDays, slaHours, categoryID int
	var scontoMassimo float64
	var variableBilling, h24, isMainPrdSellable bool
	var notes *string
	err = h.mistraDB.QueryRowContext(r.Context(), `
		SELECT k.internal_name, k.billing_period,
		       k.initial_subscription_months, k.next_subscription_months,
		       k.activation_time_days, k.sconto_massimo,
		       k.variable_billing, k.h24_assurance,
		       k.sla_resolution_hours, k.notes,
		       k.is_main_prd_sellable, k.category_id,
		       pc.name AS category_name, pc.color AS category_color
		FROM products.kit k
		JOIN products.product_category pc ON pc.id = k.category_id
		WHERE k.id = $1`, kitID).Scan(
		&kitName, &billingPeriod,
		&initialMonths, &nextMonths,
		&activationDays, &scontoMassimo,
		&variableBilling, &h24,
		&slaHours, &notes,
		&isMainPrdSellable, &categoryID,
		&categoryName, &categoryColor,
	)
	if h.rowError(w, r, "get_kit_for_pdf", err) {
		return
	}

	// Fetch products via same UNION ALL as the list endpoint
	type pdfProduct struct {
		GroupName    *string `json:"group_name"`
		InternalName *string `json:"internal_name"`
		Minimum      int     `json:"minimum"`
		Maximum      *int    `json:"maximum"`
		Required     bool    `json:"required"`
		NRC          float64 `json:"nrc"`
		MRC          float64 `json:"mrc"`
		Position     int     `json:"position"`
		ProductCode  string  `json:"product_code"`
		Notes        *string `json:"notes"`
	}

	rows, err := h.mistraDB.QueryContext(r.Context(), `
		(
			SELECT kp.group_name, p.internal_name,
			       kp.minimum, kp.maximum, kp.required,
			       kp.nrc, kp.mrc, kp.position,
			       kp.product_code, kp.notes
			FROM products.kit_product kp
			LEFT JOIN products.product p ON kp.product_code = p.code
			WHERE kp.kit_id = $1
		)
		UNION ALL
		(
			SELECT 'Articolo principale' AS group_name,
			       p.internal_name,
			       1 AS minimum, NULL AS maximum, TRUE AS required,
			       k.nrc, k.mrc, -1 AS position,
			       k.main_product_code AS product_code,
			       '' AS notes
			FROM products.kit k
			LEFT JOIN products.product p ON k.main_product_code = p.code
			WHERE k.id = $1 AND k.is_main_prd_sellable
		)
		ORDER BY position, group_name, internal_name`, kitID)
	if err != nil {
		h.dbFailure(w, r, "get_kit_products_for_pdf", err)
		return
	}
	defer rows.Close()

	var products []pdfProduct
	for rows.Next() {
		var p pdfProduct
		if err := rows.Scan(
			&p.GroupName, &p.InternalName,
			&p.Minimum, &p.Maximum, &p.Required,
			&p.NRC, &p.MRC, &p.Position,
			&p.ProductCode, &p.Notes,
		); err != nil {
			h.dbFailure(w, r, "get_kit_products_for_pdf_scan", err)
			return
		}
		products = append(products, p)
	}
	if !h.rowsDone(w, r, rows, "get_kit_products_for_pdf") {
		return
	}

	// Boolean → Italian "SI"/"NO" (matches Appsmith conversion for Carbone template)
	vb := "NO"
	if variableBilling {
		vb = "SI"
	}
	h24Str := "NO"
	if h24 {
		h24Str = "SI"
	}

	// Build Carbone payload — field names match DB column names as Appsmith passes them
	payload := map[string]any{
		// Kit top-level fields (from tbl_kit.selectedRow in Appsmith)
		"id":                          kitID,
		"internal_name":               kitName,
		"billing_period":              billingPeriod,
		"initial_subscription_months": initialMonths,
		"next_subscription_months":    nextMonths,
		"activation_time_days":        activationDays,
		"sconto_massimo":              scontoMassimo,
		"variable_billing":            vb,
		"h24_assurance":               h24Str,
		"sla_resolution_hours":        slaHours,
		"notes":                       notes,
		"is_main_prd_sellable":        isMainPrdSellable,
		"category_id":                 categoryID,
		"category_name":               categoryName,
		"category_color":              categoryColor,
		// Products array (from get_kit_products.data in Appsmith)
		"products": products,
	}

	pdfBytes, _, err := h.carbone.GeneratePDF(r.Context(), payload)
	if err != nil {
		logger.Error("pdf generation failed", "component", "listini", "kit_id", kitID, "error", err)
		httputil.Error(w, http.StatusInternalServerError, "pdf_generation_failed")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="kit-%d.pdf"`, kitID))
	w.Write(pdfBytes)
}
