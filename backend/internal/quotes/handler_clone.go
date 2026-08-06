package quotes

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// cloneQuoteHeader is deliberately limited to the commercial fields which are
// safe to carry from one quote to another. In particular, it does not contain
// the source owner, status, notes, dates, or HubSpot quote state.
type cloneQuoteHeader struct {
	CustomerID        sql.NullInt64
	RagioneSociale    sql.NullString
	DealNumber        sql.NullString
	DocumentType      sql.NullString
	Template          sql.NullString
	Services          sql.NullString
	ProposalType      sql.NullString
	InitialTermMonths int
	NextTermMonths    int
	BillMonths        int
	DeliveredInDays   int
	NRCChargeTime     int
	Description       sql.NullString
	HSDealID          sql.NullInt64
	PaymentMethod     sql.NullString
	Trial             sql.NullString
	RifOrdcli         sql.NullString
	RifTechNom        sql.NullString
	RifTechTel        sql.NullString
	RifTechEmail      sql.NullString
	RifAltroTechNom   sql.NullString
	RifAltroTechTel   sql.NullString
	RifAltroTechEmail sql.NullString
	RifAdmNom         sql.NullString
	RifAdmTechTel     sql.NullString
	RifAdmTechEmail   sql.NullString
	DocumentDate      string
}

type cloneKitRow struct {
	SourceID        int
	KitID           int64
	InternalName    sql.NullString
	BundlePrefixRow sql.NullString
	Position        sql.NullInt64
}

type cloneProduct struct {
	ProductCode         string
	Minimum             int
	Maximum             int
	Required            bool
	NRC                 string
	MRC                 string
	Position            int
	GroupName           sql.NullString
	Included            bool
	Quantity            string
	ExtendedDescription string
	MainProduct         bool
}

func nullCloneString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullCloneInt(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

const cloneProductInsertSQL = `
	INSERT INTO quotes.quote_rows_products
		(quote_row_id, product_code, minimum, maximum, required, nrc, mrc,
		 position, group_name, included, quantity, extended_description, main_product)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

// handleDuplicateQuote clones the commercial configuration of a visible quote.
// All reads and writes, including the owner resolution, are performed on the
// same repeatable-read transaction so a partially expanded quote can never be
// committed and all source children come from one snapshot.
func (h *Handler) handleDuplicateQuote(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || quoteID <= 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		h.dbFailure(w, r, "duplicate_quote_tx", err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Load the source before allocating a number. The duplicate endpoint follows
	// the same visibility as the Quotes list/get endpoints; those endpoints do
	// not exclude any quote status.
	var source cloneQuoteHeader
	err = tx.QueryRowContext(r.Context(), `
		SELECT q.customer_id, q.ragione_sociale, q.deal_number,
		       q.document_type, q.template, q.services, q.proposal_type,
		       q.initial_term_months, q.next_term_months, q.bill_months,
		       q.delivered_in_days, q.nrc_charge_time, q.description,
		       q.hs_deal_id, q.payment_method, q.trial,
		       q.rif_ordcli, q.rif_tech_nom, q.rif_tech_tel, q.rif_tech_email,
		       q.rif_altro_tech_nom, q.rif_altro_tech_tel, q.rif_altro_tech_email,
		       q.rif_adm_nom, q.rif_adm_tech_tel, q.rif_adm_tech_email,
		       CURRENT_DATE::text
		FROM quotes.quote q
		WHERE q.id = $1
		FOR SHARE`, quoteID).Scan(
		&source.CustomerID, &source.RagioneSociale, &source.DealNumber,
		&source.DocumentType, &source.Template, &source.Services, &source.ProposalType,
		&source.InitialTermMonths, &source.NextTermMonths, &source.BillMonths,
		&source.DeliveredInDays, &source.NRCChargeTime, &source.Description,
		&source.HSDealID, &source.PaymentMethod, &source.Trial,
		&source.RifOrdcli, &source.RifTechNom, &source.RifTechTel, &source.RifTechEmail,
		&source.RifAltroTechNom, &source.RifAltroTechTel, &source.RifAltroTechEmail,
		&source.RifAdmNom, &source.RifAdmTechTel, &source.RifAdmTechEmail,
		&source.DocumentDate)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "quote_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "duplicate_quote_source_load", err)
		return
	}

	// The browser never supplies the destination owner. Resolve it from the
	// authenticated identity and require an active HubSpot owner mapping.
	var ownerID string
	err = tx.QueryRowContext(r.Context(), `
		SELECT id::text
		FROM loader.hubs_owner
		WHERE lower(email) = lower($1) AND archived = FALSE
		ORDER BY id
		LIMIT 1`, claims.Email).Scan(&ownerID)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusUnprocessableEntity, "quote_owner_mapping_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "duplicate_quote_owner_lookup", err)
		return
	}

	var quoteNumber string
	if err := tx.QueryRowContext(r.Context(), `SELECT common.new_document_number('SP-')`).Scan(&quoteNumber); err != nil {
		h.dbFailure(w, r, "duplicate_quote_number", err)
		return
	}

	// Keep the payload explicit. ins_quote_head ignores fields not handled by
	// its INSERT, while the reference fields are restored below without using
	// the full-overwrite update procedure.
	payload := map[string]any{
		"quote_number":         quoteNumber,
		"customer_id":          nullCloneInt(source.CustomerID),
		"ragione_sociale":      nullCloneString(source.RagioneSociale),
		"deal_number":          nullCloneString(source.DealNumber),
		"owner":                ownerID,
		"document_date":        source.DocumentDate,
		"document_type":        nullCloneString(source.DocumentType),
		"replace_orders":       nil,
		"template":             nullCloneString(source.Template),
		"services":             nullCloneString(source.Services),
		"proposal_type":        nullCloneString(source.ProposalType),
		"initial_term_months":  source.InitialTermMonths,
		"next_term_months":     source.NextTermMonths,
		"bill_months":          source.BillMonths,
		"delivered_in_days":    source.DeliveredInDays,
		"date_sent":            nil,
		"status":               "DRAFT",
		"notes":                nil,
		"nrc_charge_time":      source.NRCChargeTime,
		"hs_deal_id":           nullCloneInt(source.HSDealID),
		"description":          nullCloneString(source.Description),
		"hs_quote_id":          nil,
		"payment_method":       nullCloneString(source.PaymentMethod),
		"trial":                nullCloneString(source.Trial),
		"hs_esign_enabled":     nil,
		"hs_esign_contacts":    nil,
		"hs_sign_status":       nil,
		"hs_esign_date":        nil,
		"rif_ordcli":           nullCloneString(source.RifOrdcli),
		"rif_tech_nom":         nullCloneString(source.RifTechNom),
		"rif_tech_tel":         nullCloneString(source.RifTechTel),
		"rif_tech_email":       nullCloneString(source.RifTechEmail),
		"rif_altro_tech_nom":   nullCloneString(source.RifAltroTechNom),
		"rif_altro_tech_tel":   nullCloneString(source.RifAltroTechTel),
		"rif_altro_tech_email": nullCloneString(source.RifAltroTechEmail),
		"rif_adm_nom":          nullCloneString(source.RifAdmNom),
		"rif_adm_tech_tel":     nullCloneString(source.RifAdmTechTel),
		"rif_adm_tech_email":   nullCloneString(source.RifAdmTechEmail),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		h.dbFailure(w, r, "duplicate_quote_header_marshal", err)
		return
	}

	var headResult json.RawMessage
	if err := tx.QueryRowContext(r.Context(),
		`SELECT quotes.ins_quote_head($1::json)`, string(payloadJSON)).Scan(&headResult); err != nil {
		h.dbFailure(w, r, "duplicate_quote_header_insert", err)
		return
	}
	var inserted struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(headResult, &inserted); err != nil {
		h.dbFailure(w, r, "duplicate_quote_header_parse", err)
		return
	}
	if inserted.ID <= 0 || inserted.Status == "ERROR" {
		h.dbFailure(w, r, "duplicate_quote_header_result", errors.New("quote header insert returned an error"))
		return
	}

	// ins_quote_head does not handle the newer HubSpot/e-signature columns.
	// Reset them explicitly, and also protect the excluded legacy fields from
	// schema defaults or future procedure changes.
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE quotes.quote
		SET replace_orders = NULL, date_sent = NULL, notes = NULL,
		    hs_quote_id = NULL, hs_esign_enabled = NULL,
		    hs_esign_contacts = NULL, hs_sign_status = NULL, hs_esign_date = NULL
		WHERE id = $1`, inserted.ID); err != nil {
		h.dbFailure(w, r, "duplicate_quote_header_reset", err)
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE quotes.quote
		SET rif_ordcli = $1, rif_tech_nom = $2, rif_tech_tel = $3,
		    rif_tech_email = $4, rif_altro_tech_nom = $5, rif_altro_tech_tel = $6,
		    rif_altro_tech_email = $7, rif_adm_nom = $8, rif_adm_tech_tel = $9,
		    rif_adm_tech_email = $10
		WHERE id = $11`,
		nullCloneString(source.RifOrdcli), nullCloneString(source.RifTechNom),
		nullCloneString(source.RifTechTel), nullCloneString(source.RifTechEmail),
		nullCloneString(source.RifAltroTechNom), nullCloneString(source.RifAltroTechTel),
		nullCloneString(source.RifAltroTechEmail), nullCloneString(source.RifAdmNom),
		nullCloneString(source.RifAdmTechTel), nullCloneString(source.RifAdmTechEmail), inserted.ID); err != nil {
		h.dbFailure(w, r, "duplicate_quote_reference_data", err)
		return
	}

	kitRows, err := tx.QueryContext(r.Context(), `
		SELECT qr.id, qr.kit_id, qr.internal_name, qr.bundle_prefix_row, qr.position
		FROM quotes.quote_rows qr
		WHERE qr.quote_id = $1
		ORDER BY qr.position, qr.id`, quoteID)
	if err != nil {
		h.dbFailure(w, r, "duplicate_quote_kit_rows_load", err)
		return
	}
	var sourceRows []cloneKitRow
	for kitRows.Next() {
		var row cloneKitRow
		if err := kitRows.Scan(&row.SourceID, &row.KitID, &row.InternalName, &row.BundlePrefixRow, &row.Position); err != nil {
			_ = kitRows.Close()
			h.dbFailure(w, r, "duplicate_quote_kit_row_scan", err)
			return
		}
		sourceRows = append(sourceRows, row)
	}
	if err := kitRows.Err(); err != nil {
		_ = kitRows.Close()
		h.dbFailure(w, r, "duplicate_quote_kit_rows_rows", err)
		return
	}
	_ = kitRows.Close()

	for _, sourceRow := range sourceRows {
		var newRowID int
		if err := tx.QueryRowContext(r.Context(), `
			INSERT INTO quotes.quote_rows (quote_id, kit_id, position)
			VALUES ($1, $2, $3) RETURNING id`, inserted.ID, sourceRow.KitID, nullableClonePosition(sourceRow.Position)).Scan(&newRowID); err != nil {
			h.dbFailure(w, r, "duplicate_quote_kit_row_insert", err)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`DELETE FROM quotes.quote_rows_products WHERE quote_row_id = $1`, newRowID); err != nil {
			h.dbFailure(w, r, "duplicate_quote_generated_products_delete", err)
			return
		}

		products, err := tx.QueryContext(r.Context(), `
			SELECT product_code, minimum, maximum, required, nrc, mrc, position,
			       group_name, included, quantity, extended_description, main_product
			FROM quotes.quote_rows_products
			WHERE quote_row_id = $1
			ORDER BY position, id`, sourceRow.SourceID)
		if err != nil {
			h.dbFailure(w, r, "duplicate_quote_products_load", err)
			return
		}
		var sourceProducts []cloneProduct
		for products.Next() {
			var product cloneProduct
			if err := products.Scan(&product.ProductCode, &product.Minimum, &product.Maximum,
				&product.Required, &product.NRC, &product.MRC, &product.Position,
				&product.GroupName, &product.Included, &product.Quantity,
				&product.ExtendedDescription, &product.MainProduct); err != nil {
				_ = products.Close()
				h.dbFailure(w, r, "duplicate_quote_product_scan", err)
				return
			}
			sourceProducts = append(sourceProducts, product)
		}
		if err := products.Err(); err != nil {
			_ = products.Close()
			h.dbFailure(w, r, "duplicate_quote_products_rows", err)
			return
		}
		_ = products.Close()

		for _, product := range sourceProducts {
			if _, err := tx.ExecContext(r.Context(), cloneProductInsertSQL,
				newRowID, product.ProductCode, product.Minimum, product.Maximum, product.Required,
				product.NRC, product.MRC, product.Position, nullCloneString(product.GroupName),
				product.Included, product.Quantity, product.ExtendedDescription, product.MainProduct); err != nil {
				h.dbFailure(w, r, "duplicate_quote_product_insert", err)
				return
			}
		}

		// The kit-row trigger replaces these values with catalog values. A quote
		// clone must retain the source's customized row labels instead.
		if _, err := tx.ExecContext(r.Context(), `
			UPDATE quotes.quote_rows
			SET internal_name = $1, bundle_prefix_row = $2,
			    hs_line_item_id = NULL, hs_line_item_nrc = NULL
			WHERE id = $3`, nullCloneString(sourceRow.InternalName),
			nullCloneString(sourceRow.BundlePrefixRow), newRowID); err != nil {
			h.dbFailure(w, r, "duplicate_quote_kit_row_restore", err)
			return
		}
	}

	// Kit triggers may append legal notes. The duplicate intentionally starts
	// with no notes regardless of catalog metadata.
	if _, err := tx.ExecContext(r.Context(),
		`UPDATE quotes.quote SET notes = NULL WHERE id = $1`, inserted.ID); err != nil {
		h.dbFailure(w, r, "duplicate_quote_notes_clear", err)
		return
	}

	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "duplicate_quote_commit", err)
		return
	}

	httputil.JSON(w, http.StatusCreated, map[string]any{
		"id":           inserted.ID,
		"quote_number": quoteNumber,
		"status":       "DRAFT",
	})
}

func nullableClonePosition(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
