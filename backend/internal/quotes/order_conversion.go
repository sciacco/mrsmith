package quotes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	hubSpotPortalID             = "26622471"
	orderConversionFolder       = "/deal-documents"
	orderConversionPDFMediaType = "application/pdf"
)

type orderConversionStep struct {
	Step   int    `json:"step"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

type orderConversionStatus struct {
	Converted      bool    `json:"converted"`
	OrderID        *int64  `json:"order_id"`
	OrderCode      *string `json:"order_code"`
	HubSpotDealID  *string `json:"hubspot_deal_id"`
	HubSpotDealURL *string `json:"hubspot_deal_url"`
	Conflict       bool    `json:"conflict,omitempty"`
	ConflictOrder  *int64  `json:"conflict_order_id,omitempty"`
}

type orderConversionResponse struct {
	Success        bool                  `json:"success"`
	OrderID        *int64                `json:"order_id,omitempty"`
	OrderCode      *string               `json:"order_code,omitempty"`
	HubSpotDealID  *string               `json:"hubspot_deal_id,omitempty"`
	HubSpotDealURL *string               `json:"hubspot_deal_url,omitempty"`
	FileID         *string               `json:"file_id,omitempty"`
	NoteID         *int64                `json:"note_id,omitempty"`
	Steps          []orderConversionStep `json:"steps"`
}

type orderConversionHubSpotMetadata struct {
	DealID         string `json:"deal_id,omitempty"`
	DealURL        string `json:"deal_url,omitempty"`
	FileID         string `json:"file_id,omitempty"`
	NoteID         int64  `json:"note_id,omitempty"`
	Filename       string `json:"filename,omitempty"`
	FolderPath     string `json:"folder_path,omitempty"`
	FileUploadedAt string `json:"file_uploaded_at,omitempty"`
	NoteCreatedAt  string `json:"note_created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

type conversionRequestError struct {
	status int
	code   string
}

func (e *conversionRequestError) Error() string { return e.code }

func (h *Handler) handleGetOrderConversion(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	status, err := h.getOrderConversionStatus(r.Context(), quoteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.Error(w, http.StatusNotFound, "quote_not_found")
			return
		}
		h.dbFailure(w, r, "get_order_conversion", err)
		return
	}
	httputil.JSON(w, http.StatusOK, status)
}

func (h *Handler) handleConvertOrder(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) || !h.requireVodka(w) || !h.requireArak(w) || !h.requireHS(w) {
		return
	}

	quoteID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_quote_id")
		return
	}

	result, reqErr, err := h.convertQuoteToOrder(r.Context(), quoteID)
	if reqErr != nil {
		logging.AddAccessLogAttrs(r.Context(), "conversion_reject", reqErr.code)
		httputil.Error(w, reqErr.status, reqErr.code)
		return
	}
	if err != nil {
		h.dbFailure(w, r, "convert_order", err, "quote_id", quoteID)
		return
	}
	if !result.Success {
		failed := lastErrorStep(result.Steps)
		logging.FromContext(r.Context()).Error("order conversion failed",
			"component", "quotes",
			"operation", "convert_order",
			"quote_id", quoteID,
			"order_id", result.OrderID,
			"failed_step", failed.Name,
			"error", failed.Error,
		)
		logging.AddAccessLogAttrs(r.Context(),
			"conversion_success", false,
			"conversion_failed_step", failed.Name,
		)
	}
	httputil.JSON(w, http.StatusOK, result)
}

// lastErrorStep returns the conversion step that failed (the one with status
// "error"). failStep always appends it last, so iterate from the end.
func lastErrorStep(steps []orderConversionStep) orderConversionStep {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Status == "error" {
			return steps[i]
		}
	}
	return orderConversionStep{}
}

func (h *Handler) getOrderConversionStatus(ctx context.Context, quoteID int) (*orderConversionStatus, error) {
	source, err := h.loadQuoteOrderSource(ctx, quoteID)
	if err != nil {
		return nil, err
	}

	status := &orderConversionStatus{}
	if source.HSDealID.Valid {
		dealID := strconv.FormatInt(source.HSDealID.Int64, 10)
		status.HubSpotDealID = &dealID
		dealURL := hubspotDealURL(dealID)
		status.HubSpotDealURL = &dealURL
	}

	bridge, err := h.findLegacyOrder(ctx, quoteID)
	if err != nil {
		return nil, err
	}
	if bridge != nil {
		status.Converted = true
		status.OrderID = &bridge.VodkaID
		if code := orderCode(source.CdlanNdoc, source.CdlanAnno); code != "" {
			status.OrderCode = &code
		}
		return status, nil
	}

	if h.vodkaDB != nil {
		cdlanNdoc, cdlanAnno, err := parseDealOrderCode(nullStringValue(source.DealNumber))
		if err == nil {
			existingID, err := h.findVodkaOrderByCode(ctx, cdlanNdoc, cdlanAnno)
			if err != nil {
				return nil, err
			}
			if existingID.Valid {
				status.Conflict = true
				status.ConflictOrder = &existingID.Int64
			}
		}
	}

	return status, nil
}

func (h *Handler) convertQuoteToOrder(ctx context.Context, quoteID int) (*orderConversionResponse, *conversionRequestError, error) {
	steps := make([]orderConversionStep, 0, 5)
	addStep := func(name, status, detail string) {
		steps = append(steps, orderConversionStep{
			Step:   len(steps) + 1,
			Name:   name,
			Status: status,
			Detail: detail,
		})
	}
	failStep := func(name string, err error) *orderConversionResponse {
		steps = append(steps, orderConversionStep{
			Step:   len(steps) + 1,
			Name:   name,
			Status: "error",
			Error:  err.Error(),
		})
		return &orderConversionResponse{Success: false, Steps: steps}
	}

	source, err := h.loadQuoteOrderSource(ctx, quoteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &conversionRequestError{status: http.StatusNotFound, code: "quote_not_found"}, nil
		}
		return nil, nil, err
	}
	if !canConvertQuoteToOrder(source.Status) {
		return nil, &conversionRequestError{status: http.StatusConflict, code: "quote_status_not_approved"}, nil
	}
	cdlanNdoc, cdlanAnno, err := parseDealOrderCode(nullStringValue(source.DealNumber))
	if err != nil {
		return nil, &conversionRequestError{status: http.StatusUnprocessableEntity, code: "deal_number_invalid"}, nil
	}
	source.CdlanNdoc = cdlanNdoc
	source.CdlanAnno = cdlanAnno

	dealID, err := h.resolveHubSpotDealID(ctx, source)
	if err != nil {
		return nil, &conversionRequestError{status: http.StatusUnprocessableEntity, code: "hubspot_deal_not_found"}, nil
	}

	bridge, err := h.findLegacyOrder(ctx, quoteID)
	if err != nil {
		return nil, nil, err
	}
	metadata := bridge.hubSpotMetadata()

	var orderID int64
	var orderCodeValue = orderCode(cdlanNdoc, cdlanAnno)
	if bridge != nil {
		orderID = bridge.VodkaID
		addStep("order", "skipped", fmt.Sprintf("Ordine %d gia presente", orderID))
	} else {
		existingID, err := h.findVodkaOrderByCode(ctx, cdlanNdoc, cdlanAnno)
		if err != nil {
			return nil, nil, err
		}
		if existingID.Valid {
			return nil, &conversionRequestError{status: http.StatusConflict, code: "order_already_exists_without_bridge"}, nil
		}

		categories, err := h.categoryNamesByID(ctx, parseServiceCategoryIDs(nullStringValue(source.Services)))
		if err != nil {
			return nil, nil, err
		}
		rows, err := h.loadQuoteOrderRows(ctx, quoteID)
		if err != nil {
			return nil, nil, err
		}
		if len(rows) == 0 {
			return nil, &conversionRequestError{status: http.StatusUnprocessableEntity, code: "quote_has_no_included_products"}, nil
		}

		header, reqErr, err := h.buildVodkaOrderHeader(ctx, source, categories)
		if reqErr != nil || err != nil {
			return nil, reqErr, err
		}
		orderID, err = h.insertVodkaOrder(ctx, quoteID, header, rows)
		if err != nil {
			return nil, nil, err
		}
		addStep("order", "completed", fmt.Sprintf("Ordine %d creato", orderID))

		if err := h.insertLegacyOrder(ctx, quoteID, orderID, header); err != nil {
			return failStep("bridge", err), nil, nil
		}
		addStep("bridge", "completed", "Ordine collegato alla proposta")
	}

	dealURL := hubspotDealURL(dealID)
	if metadata == nil {
		metadata = &orderConversionHubSpotMetadata{}
	}
	if metadata.DealID == "" {
		metadata.DealID = dealID
	}
	if metadata.DealURL == "" {
		metadata.DealURL = dealURL
	}

	fileID := metadata.FileID
	if fileID != "" {
		addStep("pdf", "skipped", "PDF ordine gia presente su HubSpot")
		addStep("hubspot_file", "skipped", "PDF HubSpot gia presente")
	} else {
		pdf, err := h.generateOrderPDF(ctx, orderID)
		if err != nil {
			return failStep("pdf", err), nil, nil
		}
		addStep("pdf", "completed", "PDF ordine generato")

		filename := orderPDFFilename(orderCodeValue, time.Now())
		upload, err := h.hs.UploadFile(ctx, filename, pdf, orderConversionFolder, map[string]any{"access": "PRIVATE"})
		if err != nil {
			return failStep("hubspot_file", err), nil, nil
		}
		fileID = upload.ID
		metadata.FileID = upload.ID
		metadata.Filename = filename
		metadata.FolderPath = orderConversionFolder
		metadata.FileUploadedAt = time.Now().UTC().Format(time.RFC3339)
		if err := h.updateLegacyOrderHubSpotMetadata(ctx, quoteID, orderID, metadata); err != nil {
			return failStep("hubspot_file", fmt.Errorf("persist HubSpot file metadata: %w", err)), nil, nil
		}
		addStep("hubspot_file", "completed", "PDF caricato su HubSpot")
	}

	noteID := metadata.NoteID
	if noteID > 0 {
		addStep("hubspot_note", "skipped", "Nota HubSpot gia presente")
	} else {
		noteID, err = h.hs.CreateNoteWithAttachment(ctx, dealID, fileID, orderID)
		if err != nil {
			return failStep("hubspot_note", err), nil, nil
		}
		metadata.NoteID = noteID
		metadata.NoteCreatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := h.updateLegacyOrderHubSpotMetadata(ctx, quoteID, orderID, metadata); err != nil {
			return failStep("hubspot_note", fmt.Errorf("persist HubSpot note metadata: %w", err)), nil, nil
		}
		addStep("hubspot_note", "completed", "Nota creata sul deal")
	}

	var responseFileID *string
	if fileID != "" {
		responseFileID = &fileID
	}
	var responseNoteID *int64
	if noteID > 0 {
		responseNoteID = &noteID
	}
	return &orderConversionResponse{
		Success:        true,
		OrderID:        &orderID,
		OrderCode:      &orderCodeValue,
		HubSpotDealID:  &dealID,
		HubSpotDealURL: &dealURL,
		FileID:         responseFileID,
		NoteID:         responseNoteID,
		Steps:          steps,
	}, nil, nil
}

type quoteOrderSource struct {
	ID                      int
	QuoteNumber             string
	CustomerID              sql.NullInt64
	DealNumber              sql.NullString
	Owner                   sql.NullString
	DocumentType            sql.NullString
	ReplaceOrders           sql.NullString
	Template                sql.NullString
	Services                sql.NullString
	ProposalType            sql.NullString
	InitialTermMonths       int
	NextTermMonths          int
	BillMonths              int
	DeliveredInDays         int
	Status                  string
	Notes                   sql.NullString
	Trial                   sql.NullString
	NrcChargeTime           int
	HSDealID                sql.NullInt64
	Description             sql.NullString
	PaymentMethod           sql.NullString
	CustomerName            sql.NullString
	CustomerNumber          sql.NullString
	PartitaIVA              sql.NullString
	OwnerName               sql.NullString
	City                    sql.NullString
	ZIP                     sql.NullString
	Country                 sql.NullString
	ProvinciaDiFatturazione sql.NullString
	CodiceFiscale           sql.NullString
	Address                 sql.NullString
	Lingua                  sql.NullString
	TemplateDescription     sql.NullString
	TemplateIsColo          bool
	RifOrdcli               sql.NullString
	RifTechNom              sql.NullString
	RifTechTel              sql.NullString
	RifTechEmail            sql.NullString
	RifAltroTechNom         sql.NullString
	RifAltroTechTel         sql.NullString
	RifAltroTechEmail       sql.NullString
	RifAdmNom               sql.NullString
	RifAdmTechTel           sql.NullString
	RifAdmTechEmail         sql.NullString
	CdlanNdoc               string
	CdlanAnno               string
}

func (h *Handler) loadQuoteOrderSource(ctx context.Context, quoteID int) (*quoteOrderSource, error) {
	row := h.db.QueryRowContext(ctx, `
SELECT q.id, q.quote_number, q.customer_id, q.deal_number, q.owner,
       q.document_type, q.replace_orders, q.template, q.services,
       q.proposal_type, q.initial_term_months, q.next_term_months, q.bill_months,
       q.delivered_in_days, q.status, q.notes, q.trial, q.nrc_charge_time,
       q.hs_deal_id, q.description, RTRIM(q.payment_method) AS payment_method,
       hc.name AS customer_name, hc.numero_azienda AS customer_number, hc.partita_iva,
       COALESCE(ho.first_name || ' ' || ho.last_name, '') AS owner_name,
       hc.city, hc.zip, hc.country, hc.provincia_di_fatturazione,
       hc.codice_fiscale, hc.address, hc.lingua,
       t.description AS template_description,
       COALESCE(t.is_colo, false) AS template_is_colo,
       q.rif_ordcli, q.rif_tech_nom, q.rif_tech_tel, q.rif_tech_email,
       q.rif_altro_tech_nom, q.rif_altro_tech_tel, q.rif_altro_tech_email,
       q.rif_adm_nom, q.rif_adm_tech_tel, q.rif_adm_tech_email
FROM quotes.quote q
LEFT JOIN loader.hubs_company hc ON hc.id = q.customer_id
LEFT JOIN loader.hubs_owner ho ON ho.id::text = q.owner
LEFT JOIN quotes.template t ON t.template_id = q.template
WHERE q.id = $1`, quoteID)

	var q quoteOrderSource
	err := row.Scan(
		&q.ID, &q.QuoteNumber, &q.CustomerID, &q.DealNumber, &q.Owner,
		&q.DocumentType, &q.ReplaceOrders, &q.Template, &q.Services,
		&q.ProposalType, &q.InitialTermMonths, &q.NextTermMonths, &q.BillMonths,
		&q.DeliveredInDays, &q.Status, &q.Notes, &q.Trial, &q.NrcChargeTime,
		&q.HSDealID, &q.Description, &q.PaymentMethod,
		&q.CustomerName, &q.CustomerNumber, &q.PartitaIVA,
		&q.OwnerName, &q.City, &q.ZIP, &q.Country, &q.ProvinciaDiFatturazione,
		&q.CodiceFiscale, &q.Address, &q.Lingua, &q.TemplateDescription,
		&q.TemplateIsColo,
		&q.RifOrdcli, &q.RifTechNom, &q.RifTechTel, &q.RifTechEmail,
		&q.RifAltroTechNom, &q.RifAltroTechTel, &q.RifAltroTechEmail,
		&q.RifAdmNom, &q.RifAdmTechTel, &q.RifAdmTechEmail,
	)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

type legacyOrderBridge struct {
	VodkaID int64
	JData   sql.NullString
}

func (b *legacyOrderBridge) hubSpotMetadata() *orderConversionHubSpotMetadata {
	if b == nil || !b.JData.Valid || strings.TrimSpace(b.JData.String) == "" {
		return nil
	}
	var payload struct {
		HubSpot *orderConversionHubSpotMetadata `json:"hubspot"`
	}
	if err := json.Unmarshal([]byte(b.JData.String), &payload); err != nil || payload.HubSpot == nil {
		return nil
	}
	return payload.HubSpot.normalized()
}

func (m *orderConversionHubSpotMetadata) normalized() *orderConversionHubSpotMetadata {
	if m == nil {
		return nil
	}
	m.DealID = strings.TrimSpace(m.DealID)
	m.DealURL = strings.TrimSpace(m.DealURL)
	m.FileID = strings.TrimSpace(m.FileID)
	m.Filename = strings.TrimSpace(m.Filename)
	m.FolderPath = strings.TrimSpace(m.FolderPath)
	return m
}

func (h *Handler) findLegacyOrder(ctx context.Context, quoteID int) (*legacyOrderBridge, error) {
	rows, err := h.db.QueryContext(ctx, `
SELECT vodka_id, jdata::text
FROM orders.legacy_orders
WHERE quote_id = $1
ORDER BY vodka_id DESC
`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var bridge legacyOrderBridge
		if err := rows.Scan(&bridge.VodkaID, &bridge.JData); err != nil {
			return nil, err
		}
		if h.vodkaDB == nil {
			return &bridge, nil
		}
		exists, err := h.vodkaOrderExists(ctx, bridge.VodkaID)
		if err != nil {
			return nil, err
		}
		if exists {
			return &bridge, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *Handler) vodkaOrderExists(ctx context.Context, orderID int64) (bool, error) {
	var id int64
	err := h.vodkaDB.QueryRowContext(ctx, `
SELECT id
FROM orders
WHERE id = ?
LIMIT 1`, orderID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (h *Handler) resolveHubSpotDealID(ctx context.Context, source *quoteOrderSource) (string, error) {
	if source.HSDealID.Valid {
		return strconv.FormatInt(source.HSDealID.Int64, 10), nil
	}
	var dealID string
	err := h.db.QueryRowContext(ctx, `
SELECT id::text
FROM loader.hubs_deal
WHERE codice = $1
LIMIT 1`, nullStringValue(source.DealNumber)).Scan(&dealID)
	if err != nil {
		return "", err
	}
	dealID = strings.TrimSpace(dealID)
	if dealID == "" {
		return "", sql.ErrNoRows
	}
	return dealID, nil
}

func (h *Handler) findVodkaOrderByCode(ctx context.Context, cdlanNdoc, cdlanAnno string) (sql.NullInt64, error) {
	var id sql.NullInt64
	err := h.vodkaDB.QueryRowContext(ctx, `
SELECT id
FROM orders
WHERE cdlan_ndoc = ? AND cdlan_anno = ?
ORDER BY id DESC
LIMIT 1`, cdlanNdoc, cdlanAnno).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.NullInt64{}, nil
	}
	return id, err
}

func (h *Handler) categoryNamesByID(ctx context.Context, ids []int) (map[int]string, error) {
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	args := make([]any, 0, len(ids))
	placeholders := make([]string, 0, len(ids))
	for i, id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, "$"+strconv.Itoa(i+1))
	}
	rows, err := h.db.QueryContext(ctx, `
SELECT id, name
FROM products.product_category
WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int]string, len(ids))
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

type quoteOrderRowSource struct {
	RowID               int64
	QuoteID             int64
	KitID               int64
	InternalName        sql.NullString
	BundlePrefixRow     sql.NullString
	QRPID               int64
	ProductCode         string
	NRC                 float64
	MRC                 float64
	GroupName           sql.NullString
	Quantity            float64
	ExtendedDescription sql.NullString
	MainProduct         bool
	Translations        json.RawMessage
}

func (h *Handler) loadQuoteOrderRows(ctx context.Context, quoteID int) ([]quoteOrderRowSource, error) {
	rows, err := h.db.QueryContext(ctx, `
SELECT qr.id AS row_id, qr.quote_id, qr.kit_id, qr.internal_name, qr.bundle_prefix_row,
       qrp.id AS qrp_id, qrp.product_code, qrp.nrc, qrp.mrc, qrp.group_name,
       qrp.quantity, qrp.extended_description, qrp.main_product,
       COALESCE((
         SELECT json_agg(x)
         FROM (
           SELECT *
           FROM common.translation t
           WHERE t.translation_uuid = p.translation_uuid
         ) AS x
       ), '[]'::json) AS translations
FROM quotes.quote_rows qr
JOIN quotes.quote_rows_products qrp ON qr.id = qrp.quote_row_id
JOIN products.product p ON qrp.product_code = p.code
WHERE qr.quote_id = $1
  AND qrp.included = true
ORDER BY qr.position, qr.id, qrp.position`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []quoteOrderRowSource{}
	for rows.Next() {
		var row quoteOrderRowSource
		if err := rows.Scan(
			&row.RowID, &row.QuoteID, &row.KitID, &row.InternalName, &row.BundlePrefixRow,
			&row.QRPID, &row.ProductCode, &row.NRC, &row.MRC, &row.GroupName,
			&row.Quantity, &row.ExtendedDescription, &row.MainProduct, &row.Translations,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type vodkaOrderHeader struct {
	CdlanSystemODV        int64                           `json:"cdlan_systemodv"`
	CdlanTipodoc          string                          `json:"cdlan_tipodoc"`
	CdlanNdoc             string                          `json:"cdlan_ndoc"`
	CdlanDatadoc          string                          `json:"cdlan_datadoc"`
	CdlanCliente          *string                         `json:"cdlan_cliente"`
	CdlanCommerciale      *string                         `json:"cdlan_commerciale"`
	CdlanCodTerminiPag    string                          `json:"cdlan_cod_termini_pag"`
	CdlanNote             *string                         `json:"cdlan_note"`
	CdlanTipoOrd          string                          `json:"cdlan_tipo_ord"`
	CdlanDurRin           string                          `json:"cdlan_dur_rin"`
	CdlanTacitoRin        string                          `json:"cdlan_tacito_rin"`
	CdlanSostOrd          *string                         `json:"cdlan_sost_ord"`
	CdlanTempiRil         string                          `json:"cdlan_tempi_ril"`
	CdlanDurataServizio   string                          `json:"cdlan_durata_servizio"`
	CdlanDataconferma     *string                         `json:"cdlan_dataconferma"`
	CdlanRifOrdcli        *string                         `json:"cdlan_rif_ordcli"`
	CdlanRifTechNom       *string                         `json:"cdlan_rif_tech_nom"`
	CdlanRifTechTel       *string                         `json:"cdlan_rif_tech_tel"`
	CdlanRifTechEmail     *string                         `json:"cdlan_rif_tech_email"`
	CdlanRifAltroTechNom  *string                         `json:"cdlan_rif_altro_tech_nom"`
	CdlanRifAltroTechTel  *string                         `json:"cdlan_rif_altro_tech_tel"`
	CdlanRifAltroTechMail *string                         `json:"cdlan_rif_altro_tech_email"`
	CdlanRifAdmNom        *string                         `json:"cdlan_rif_adm_nom"`
	CdlanRifAdmTechTel    *string                         `json:"cdlan_rif_adm_tech_tel"`
	CdlanRifAdmTechEmail  *string                         `json:"cdlan_rif_adm_tech_email"`
	CdlanIntFatturazione  string                          `json:"cdlan_int_fatturazione"`
	CdlanIntFattAtt       string                          `json:"cdlan_int_fatturazione_att"`
	CdlanStato            string                          `json:"cdlan_stato"`
	CdlanEvaso            int                             `json:"cdlan_evaso"`
	CdlanChiuso           int                             `json:"cdlan_chiuso"`
	CdlanAnno             string                          `json:"cdlan_anno"`
	CdlanValuta           string                          `json:"cdlan_valuta"`
	WrittenBy             *string                         `json:"written_by"`
	ProfileIVA            *string                         `json:"profile_iva"`
	ProfileCF             *string                         `json:"profile_cf"`
	ProfileAddress        *string                         `json:"profile_address"`
	ProfileCity           *string                         `json:"profile_city"`
	ProfileCAP            *string                         `json:"profile_cap"`
	ProfilePV             *string                         `json:"profile_pv"`
	ProfileSDI            *string                         `json:"profile_sdi"`
	ProfileLang           string                          `json:"profile_lang"`
	CdlanClienteID        *int64                          `json:"cdlan_cliente_id"`
	ServiceType           string                          `json:"service_type"`
	DataDecorrenza        string                          `json:"data_decorrenza"`
	CdlanTacitoRinInPDF   string                          `json:"cdlan_tacito_rin_in_pdf"`
	IsColo                string                          `json:"is_colo"`
	HubSpot               *orderConversionHubSpotMetadata `json:"hubspot,omitempty"`
}

func (h *Handler) buildVodkaOrderHeader(ctx context.Context, source *quoteOrderSource, categoryNames map[int]string) (*vodkaOrderHeader, *conversionRequestError, error) {
	var systemODV int64
	if err := h.db.QueryRowContext(ctx, `SELECT nextval('orders.system_odv_alyante')`).Scan(&systemODV); err != nil {
		return nil, nil, err
	}
	header, reqErr := buildVodkaOrderHeader(source, categoryNames, systemODV, time.Now())
	return header, reqErr, nil
}

func buildVodkaOrderHeader(source *quoteOrderSource, categoryNames map[int]string, systemODV int64, now time.Time) (*vodkaOrderHeader, *conversionRequestError) {
	documentType := strings.TrimSpace(nullStringValue(source.DocumentType))
	if documentType == "" {
		return nil, &conversionRequestError{status: http.StatusUnprocessableEntity, code: "quote_document_type_required"}
	}
	proposalType := strings.TrimSpace(nullStringValue(source.ProposalType))
	cdlanTipoOrd, ok := mapProposalTypeToLegacyOrderType(proposalType)
	if !ok {
		return nil, &conversionRequestError{status: http.StatusUnprocessableEntity, code: "quote_proposal_type_invalid"}
	}

	note := quoteOrderNote(nullStringValue(source.Trial), nullStringValue(source.Notes))
	cdlanTacitoRin := "1"
	if documentType == "TSC-ORDINE" {
		cdlanTacitoRin = "0"
	}

	return &vodkaOrderHeader{
		CdlanSystemODV:        systemODV,
		CdlanTipodoc:          documentType,
		CdlanNdoc:             source.CdlanNdoc,
		CdlanDatadoc:          now.Format("2006-01-02"),
		CdlanCliente:          nullStringPtr(source.CustomerName),
		CdlanCommerciale:      nil,
		CdlanCodTerminiPag:    legacyPaymentMethod(source.PaymentMethod),
		CdlanNote:             emptyStringPtr(note),
		CdlanTipoOrd:          cdlanTipoOrd,
		CdlanDurRin:           strconv.Itoa(source.NextTermMonths),
		CdlanTacitoRin:        cdlanTacitoRin,
		CdlanSostOrd:          nullStringPtr(source.ReplaceOrders),
		CdlanTempiRil:         strconv.Itoa(source.DeliveredInDays),
		CdlanDurataServizio:   strconv.Itoa(source.InitialTermMonths),
		CdlanDataconferma:     nil,
		CdlanRifOrdcli:        nullStringPtr(source.RifOrdcli),
		CdlanRifTechNom:       nullStringPtr(source.RifTechNom),
		CdlanRifTechTel:       nullStringPtr(source.RifTechTel),
		CdlanRifTechEmail:     nullStringPtr(source.RifTechEmail),
		CdlanRifAltroTechNom:  nullStringPtr(source.RifAltroTechNom),
		CdlanRifAltroTechTel:  nullStringPtr(source.RifAltroTechTel),
		CdlanRifAltroTechMail: nullStringPtr(source.RifAltroTechEmail),
		CdlanRifAdmNom:        nullStringPtr(source.RifAdmNom),
		CdlanRifAdmTechTel:    nullStringPtr(source.RifAdmTechTel),
		CdlanRifAdmTechEmail:  nullStringPtr(source.RifAdmTechEmail),
		CdlanIntFatturazione:  strconv.Itoa(source.BillMonths),
		CdlanIntFattAtt:       strconv.Itoa(source.NrcChargeTime),
		CdlanStato:            "BOZZA",
		CdlanEvaso:            0,
		CdlanChiuso:           0,
		CdlanAnno:             source.CdlanAnno,
		CdlanValuta:           "EURO",
		WrittenBy:             nullStringPtr(source.OwnerName),
		ProfileIVA:            nullStringPtr(source.PartitaIVA),
		ProfileCF:             nullStringPtr(source.CodiceFiscale),
		ProfileAddress:        nullStringPtr(source.Address),
		ProfileCity:           nullStringPtr(source.City),
		ProfileCAP:            nullStringPtr(source.ZIP),
		ProfilePV:             provincePrefix(nullStringValue(source.ProvinciaDiFatturazione)),
		ProfileSDI:            nil,
		ProfileLang:           normalizeLegacyQuoteLanguage(nullStringValue(source.Lingua)),
		CdlanClienteID:        nil,
		ServiceType:           serviceNamesForLegacy(source.Services, categoryNames),
		DataDecorrenza:        "",
		CdlanTacitoRinInPDF:   "0",
		IsColo:                legacyIsColo(source.TemplateIsColo),
	}, nil
}

func (h *Handler) insertVodkaOrder(ctx context.Context, quoteID int, header *vodkaOrderHeader, sourceRows []quoteOrderRowSource) (int64, error) {
	tx, err := h.vodkaDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, insertVodkaOrderQuery, header.values()...)
	if err != nil {
		return 0, err
	}
	orderID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	legacyRows, err := h.buildVodkaOrderRows(ctx, orderID, header.ProfileLang, sourceRows)
	if err != nil {
		return 0, err
	}
	for _, row := range legacyRows {
		if _, err := tx.ExecContext(ctx, insertVodkaOrderRowQuery, row.values()...); err != nil {
			return 0, fmt.Errorf("insert order row for quote %d product %s: %w", quoteID, row.CdlanCodart, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return orderID, nil
}

func (h *Handler) insertLegacyOrder(ctx context.Context, quoteID int, orderID int64, header *vodkaOrderHeader) error {
	jdata, err := json.Marshal(header)
	if err != nil {
		return err
	}
	_, err = h.db.ExecContext(ctx, `
INSERT INTO orders.legacy_orders (quote_id, vodka_id, jdata)
VALUES ($1, $2, $3::jsonb)
ON CONFLICT (quote_id, vodka_id)
	DO UPDATE SET jdata = EXCLUDED.jdata`, quoteID, orderID, string(jdata))
	return err
}

func (h *Handler) updateLegacyOrderHubSpotMetadata(ctx context.Context, quoteID int, orderID int64, metadata *orderConversionHubSpotMetadata) error {
	if metadata == nil {
		return nil
	}
	metadata.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	jdata, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	res, err := h.db.ExecContext(ctx, `
	UPDATE orders.legacy_orders
	SET jdata = jsonb_set(COALESCE(jdata, '{}'::jsonb), '{hubspot}', $3::jsonb, true)
	WHERE quote_id = $1 AND vodka_id = $2`, quoteID, orderID, string(jdata))
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type vodkaOrderRow struct {
	OrdersID               int64   `json:"orders_id"`
	CdlanSystemODVRow      int64   `json:"cdlan_systemodv_row"`
	CdlanCodart            string  `json:"cdlan_codart"`
	CdlanDescart           string  `json:"cdlan_descart"`
	CdlanQta               string  `json:"cdlan_qta"`
	CdlanSerialNumber      int64   `json:"cdlan_serialnumber"`
	CdlanPrezzo            string  `json:"cdlan_prezzo"`
	CdlanPrezzoAttivazione string  `json:"cdlan_prezzo_attivazione"`
	CdlanPrezzoCessazione  string  `json:"cdlan_prezzo_cessazione"`
	CdlanRaggFatturazione  string  `json:"cdlan_ragg_fatturazione"`
	CdlanDataAttivazione   *string `json:"cdlan_data_attivazione"`
	CdlanCodiceKit         *string `json:"cdlan_codice_kit"`
	IndexKit               int64   `json:"index_kit"`
	NoteTecnici            *string `json:"note_tecnici"`
	DataAnnullamento       *string `json:"data_annullamento"`
	ConfirmDataAttivazione int     `json:"confirm_data_attivazione"`
}

func (h *Handler) buildVodkaOrderRows(ctx context.Context, orderID int64, language string, sourceRows []quoteOrderRowSource) ([]vodkaOrderRow, error) {
	out := make([]vodkaOrderRow, 0, len(sourceRows))
	for _, source := range sourceRows {
		var systemODV int64
		if err := h.db.QueryRowContext(ctx, `SELECT nextval('orders.system_odv_alyante')`).Scan(&systemODV); err != nil {
			return nil, err
		}
		var serialNumber int64
		if err := h.db.QueryRowContext(ctx, `SELECT nextval('orders.serial_number')`).Scan(&serialNumber); err != nil {
			return nil, err
		}
		out = append(out, buildVodkaOrderRow(orderID, language, source, systemODV, serialNumber))
	}
	return out, nil
}

func buildVodkaOrderRow(orderID int64, language string, source quoteOrderRowSource, systemODV, serialNumber int64) vodkaOrderRow {
	return vodkaOrderRow{
		OrdersID:               orderID,
		CdlanSystemODVRow:      systemODV,
		CdlanCodart:            source.ProductCode,
		CdlanDescart:           legacyRowDescription(language, source),
		CdlanQta:               formatLegacyPlainDecimal(source.Quantity),
		CdlanSerialNumber:      serialNumber,
		CdlanPrezzo:            formatLegacyCommaDecimal(source.MRC),
		CdlanPrezzoAttivazione: formatLegacyPlainDecimal(source.NRC),
		CdlanPrezzoCessazione:  "0",
		CdlanRaggFatturazione:  "A",
		CdlanDataAttivazione:   nil,
		CdlanCodiceKit:         nullStringPtr(source.BundlePrefixRow),
		IndexKit:               source.RowID,
		NoteTecnici:            nil,
		DataAnnullamento:       nil,
		ConfirmDataAttivazione: 0,
	}
}

func (h *Handler) generateOrderPDF(ctx context.Context, orderID int64) ([]byte, error) {
	path := "/orders/v1/order/pdf/" + url.PathEscape(strconv.FormatInt(orderID, 10)) + "/generate"
	resp, err := h.arak.Do(http.MethodGet, path, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gateway PDF HTTP %d: %s", resp.StatusCode, compactGatewayBody(body))
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("gateway PDF response empty")
	}
	return body, nil
}

const insertVodkaOrderQuery = `
INSERT INTO orders (
  cdlan_systemodv, cdlan_tipodoc, cdlan_ndoc, cdlan_datadoc, cdlan_cliente, cdlan_commerciale,
  cdlan_cod_termini_pag, cdlan_note, cdlan_tipo_ord, cdlan_dur_rin, cdlan_tacito_rin, cdlan_sost_ord,
  cdlan_tempi_ril, cdlan_durata_servizio, cdlan_dataconferma, cdlan_rif_ordcli, cdlan_rif_tech_nom,
  cdlan_rif_tech_tel, cdlan_rif_tech_email, cdlan_rif_altro_tech_nom, cdlan_rif_altro_tech_tel,
  cdlan_rif_altro_tech_email, cdlan_rif_adm_nom, cdlan_rif_adm_tech_tel, cdlan_rif_adm_tech_email,
  cdlan_int_fatturazione, cdlan_int_fatturazione_att, cdlan_stato, cdlan_evaso, cdlan_chiuso,
  cdlan_anno, cdlan_valuta, written_by, profile_iva, profile_cf, profile_address, profile_city,
  profile_cap, profile_pv, profile_sdi, profile_lang, cdlan_cliente_id, service_type, data_decorrenza,
  cdlan_tacito_rin_in_pdf, is_colo
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)`

func (h *vodkaOrderHeader) values() []any {
	return []any{
		h.CdlanSystemODV, h.CdlanTipodoc, h.CdlanNdoc, h.CdlanDatadoc, h.CdlanCliente, h.CdlanCommerciale,
		h.CdlanCodTerminiPag, h.CdlanNote, h.CdlanTipoOrd, h.CdlanDurRin, h.CdlanTacitoRin, h.CdlanSostOrd,
		h.CdlanTempiRil, h.CdlanDurataServizio, h.CdlanDataconferma, h.CdlanRifOrdcli, h.CdlanRifTechNom,
		h.CdlanRifTechTel, h.CdlanRifTechEmail, h.CdlanRifAltroTechNom, h.CdlanRifAltroTechTel,
		h.CdlanRifAltroTechMail, h.CdlanRifAdmNom, h.CdlanRifAdmTechTel, h.CdlanRifAdmTechEmail,
		h.CdlanIntFatturazione, h.CdlanIntFattAtt, h.CdlanStato, h.CdlanEvaso, h.CdlanChiuso,
		h.CdlanAnno, h.CdlanValuta, h.WrittenBy, h.ProfileIVA, h.ProfileCF, h.ProfileAddress, h.ProfileCity,
		h.ProfileCAP, h.ProfilePV, h.ProfileSDI, h.ProfileLang, h.CdlanClienteID, h.ServiceType, h.DataDecorrenza,
		h.CdlanTacitoRinInPDF, h.IsColo,
	}
}

const insertVodkaOrderRowQuery = `
INSERT INTO orders_rows (
  orders_id, cdlan_systemodv_row, cdlan_codart, cdlan_descart, cdlan_qta, cdlan_serialnumber,
  cdlan_prezzo, cdlan_prezzo_attivazione, cdlan_prezzo_cessazione, cdlan_ragg_fatturazione,
  cdlan_data_attivazione, cdlan_codice_kit, index_kit, note_tecnici, data_annullamento,
  confirm_data_attivazione
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func (r vodkaOrderRow) values() []any {
	return []any{
		r.OrdersID, r.CdlanSystemODVRow, r.CdlanCodart, r.CdlanDescart, r.CdlanQta, r.CdlanSerialNumber,
		r.CdlanPrezzo, r.CdlanPrezzoAttivazione, r.CdlanPrezzoCessazione, r.CdlanRaggFatturazione,
		r.CdlanDataAttivazione, r.CdlanCodiceKit, r.IndexKit, r.NoteTecnici, r.DataAnnullamento,
		r.ConfirmDataAttivazione,
	}
}

func parseDealOrderCode(dealNumber string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(dealNumber), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("deal number must use numero/anno format")
	}
	ndoc := strings.TrimSpace(parts[0])
	anno := strings.TrimSpace(parts[1])
	if ndoc == "" || anno == "" {
		return "", "", fmt.Errorf("deal number must include numero and anno")
	}
	return ndoc, anno, nil
}

func mapProposalTypeToLegacyOrderType(proposalType string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(proposalType)) {
	case "NUOVO":
		return "N", true
	case "SOSTITUZIONE":
		return "A", true
	case "RINNOVO":
		return "R", true
	default:
		return "", false
	}
}

func canConvertQuoteToOrder(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "APPROVED")
}

func parseServiceCategoryIDs(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var values []any
		if err := json.Unmarshal([]byte(raw), &values); err == nil {
			out := make([]int, 0, len(values))
			for _, value := range values {
				switch v := value.(type) {
				case float64:
					out = append(out, int(v))
				case string:
					if id, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
						out = append(out, id)
					}
				}
			}
			return out
		}
		raw = strings.Trim(raw, "[]")
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil {
			out = append(out, id)
		}
	}
	return out
}

func serviceNamesForLegacy(services sql.NullString, categoryNames map[int]string) string {
	ids := parseServiceCategoryIDs(nullStringValue(services))
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name := strings.TrimSpace(categoryNames[id]); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func normalizeLegacyQuoteLanguage(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "EN", "ENG", "ING":
		return "en"
	default:
		return "it"
	}
}

func legacyPaymentMethod(value sql.NullString) string {
	payment := strings.TrimSpace(nullStringValue(value))
	if payment == "" {
		return "402"
	}
	return payment
}

func legacyIsColo(isColo bool) string {
	if isColo {
		return "Colocation variabile"
	}
	return "0"
}

func quoteOrderNote(trial, notes string) string {
	if strings.TrimSpace(trial) == "" {
		return notes
	}
	return trial + notes
}

func legacyRowDescription(language string, source quoteOrderRowSource) string {
	short := translationShort(source.Translations, language)
	if strings.TrimSpace(short) == "" {
		short = nullStringValue(source.InternalName)
	}
	if strings.TrimSpace(short) == "" {
		short = source.ProductCode
	}
	extended := strings.TrimSpace(nullStringValue(source.ExtendedDescription))
	if extended == "" {
		return strings.TrimSpace(short)
	}
	return strings.TrimSpace(short) + "\r\n" + extended
}

func translationShort(raw json.RawMessage, language string) string {
	if len(raw) == 0 {
		return ""
	}
	var translations []struct {
		Language string `json:"language"`
		Short    string `json:"short"`
	}
	if err := json.Unmarshal(raw, &translations); err != nil {
		return ""
	}
	for _, translation := range translations {
		if strings.EqualFold(strings.TrimSpace(translation.Language), language) {
			return strings.TrimSpace(translation.Short)
		}
	}
	return ""
}

func formatLegacyCommaDecimal(value float64) string {
	return strings.ReplaceAll(formatLegacyPlainDecimal(value), ".", ",")
}

func formatLegacyPlainDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func provincePrefix(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if len(raw) > 2 {
		raw = raw[:2]
	}
	return &raw
}

func emptyStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func orderCode(cdlanNdoc, cdlanAnno string) string {
	if cdlanNdoc == "" || cdlanAnno == "" {
		return ""
	}
	return cdlanNdoc + "/" + cdlanAnno
}

func hubspotDealURL(dealID string) string {
	return "https://app-eu1.hubspot.com/contacts/" + hubSpotPortalID + "/record/0-3/" + url.PathEscape(dealID)
}

func orderPDFFilename(orderCode string, now time.Time) string {
	code := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(orderCode)
	return "order_" + code + "_" + now.Format("2006-01-02") + ".pdf"
}

func compactGatewayBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 256 {
		return text[:256] + "..."
	}
	return text
}
