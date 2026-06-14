package raenad

import (
	"database/sql"
	"time"
)

type SQLStore struct {
	mistra   *sql.DB
	configDB *sql.DB
}

func NewSQLStore(mistra, configDB *sql.DB) *SQLStore {
	if mistra == nil && configDB == nil {
		return nil
	}
	return &SQLStore{
		mistra:   mistra,
		configDB: configDB,
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

const quoteScanColumns = `
	id,
	quote_number,
	created_at,
	updated_at,
	created_by,
	updated_by,
	authoring_status,
	hubspot_sync_status,
	hubspot_sync_error,
	hubspot_synced_at,
	hubspot_company_id,
	hubspot_contact_id,
	hubspot_deal_id,
	hubspot_pipeline_id,
	hubspot_pipeline_label,
	hubspot_dealstage_id,
	hubspot_dealstage_label,
	customer_name,
	customer_vat,
	customer_tax_code,
	customer_pec,
	customer_email,
	customer_address,
	customer_zip,
	customer_city,
	customer_province,
	customer_country,
	customer_language,
	numero_azienda_snapshot,
	contact_first_name,
	contact_last_name,
	contact_full_name,
	contact_email,
	contact_role,
	document_date::text,
	payment_method_code,
	payment_method_label,
	payment_bank_details,
	description,
	internal_notes,
	total_net::text,
	total_vat::text,
	total_gross::text,
	total_purchase::text,
	total_gain::text`

const quoteLineScanColumns = `
	id,
	quote_id,
	position,
	line_type,
	item_code,
	item_description,
	description,
	unit_of_measure,
	qta::text,
	unit_price::text,
	discounts,
	cod_iva,
	iva_percent_snapshot::text,
	purchase_unit_price::text,
	line_net::text,
	line_vat::text,
	line_gross::text,
	line_purchase::text,
	line_gain::text`

func scanQuote(scanner rowScanner) (quoteResponse, error) {
	var (
		item                  quoteResponse
		createdAt             time.Time
		updatedAt             time.Time
		hubSpotSyncError      sql.NullString
		hubSpotSyncedAt       sql.NullTime
		hubSpotCompanyID      sql.NullString
		hubSpotContactID      sql.NullString
		hubSpotDealID         sql.NullString
		hubSpotPipelineID     sql.NullString
		hubSpotPipelineLabel  sql.NullString
		hubSpotDealstageID    sql.NullString
		hubSpotDealstageLabel sql.NullString
		customerName          sql.NullString
		customerVAT           sql.NullString
		customerTaxCode       sql.NullString
		customerPEC           sql.NullString
		customerEmail         sql.NullString
		customerAddress       sql.NullString
		customerZIP           sql.NullString
		customerCity          sql.NullString
		customerProvince      sql.NullString
		customerCountry       sql.NullString
		customerLanguage      sql.NullString
		numeroAzienda         sql.NullString
		contactFirstName      sql.NullString
		contactLastName       sql.NullString
		contactFullName       sql.NullString
		contactEmail          sql.NullString
		contactRole           sql.NullString
		documentDate          sql.NullString
		paymentMethodCode     sql.NullString
		paymentMethodLabel    sql.NullString
		paymentBankDetails    sql.NullString
		description           sql.NullString
		internalNotes         sql.NullString
	)

	if err := scanner.Scan(
		&item.ID,
		&item.QuoteNumber,
		&createdAt,
		&updatedAt,
		&item.CreatedBy,
		&item.UpdatedBy,
		&item.AuthoringStatus,
		&item.HubSpotSyncStatus,
		&hubSpotSyncError,
		&hubSpotSyncedAt,
		&hubSpotCompanyID,
		&hubSpotContactID,
		&hubSpotDealID,
		&hubSpotPipelineID,
		&hubSpotPipelineLabel,
		&hubSpotDealstageID,
		&hubSpotDealstageLabel,
		&customerName,
		&customerVAT,
		&customerTaxCode,
		&customerPEC,
		&customerEmail,
		&customerAddress,
		&customerZIP,
		&customerCity,
		&customerProvince,
		&customerCountry,
		&customerLanguage,
		&numeroAzienda,
		&contactFirstName,
		&contactLastName,
		&contactFullName,
		&contactEmail,
		&contactRole,
		&documentDate,
		&paymentMethodCode,
		&paymentMethodLabel,
		&paymentBankDetails,
		&description,
		&internalNotes,
		&item.TotalNet,
		&item.TotalVAT,
		&item.TotalGross,
		&item.TotalPurchase,
		&item.TotalGain,
	); err != nil {
		return quoteResponse{}, err
	}

	item.CreatedAt = formatTimestamp(createdAt)
	item.UpdatedAt = formatTimestamp(updatedAt)
	item.HubSpotSyncError = nullStringPtr(hubSpotSyncError)
	item.HubSpotSyncedAt = nullTimestampPtr(hubSpotSyncedAt)
	item.HubSpotCompanyID = nullStringPtr(hubSpotCompanyID)
	item.HubSpotContactID = nullStringPtr(hubSpotContactID)
	item.HubSpotDealID = nullStringPtr(hubSpotDealID)
	item.HubSpotPipelineID = nullStringPtr(hubSpotPipelineID)
	item.HubSpotPipelineLabel = nullStringPtr(hubSpotPipelineLabel)
	item.HubSpotDealstageID = nullStringPtr(hubSpotDealstageID)
	item.HubSpotDealstageLabel = nullStringPtr(hubSpotDealstageLabel)
	item.CustomerName = nullStringPtr(customerName)
	item.DocumentDate = nullStringPtr(documentDate)
	item.Description = nullStringPtr(description)
	item.InternalNotes = nullStringPtr(internalNotes)
	item.Customer = customerSnapshot{customerSnapshotInput{
		Name:                  nullStringPtr(customerName),
		VAT:                   nullStringPtr(customerVAT),
		TaxCode:               nullStringPtr(customerTaxCode),
		PEC:                   nullStringPtr(customerPEC),
		Email:                 nullStringPtr(customerEmail),
		Address:               nullStringPtr(customerAddress),
		ZIP:                   nullStringPtr(customerZIP),
		City:                  nullStringPtr(customerCity),
		Province:              nullStringPtr(customerProvince),
		Country:               nullStringPtr(customerCountry),
		Language:              nullStringPtr(customerLanguage),
		NumeroAziendaSnapshot: nullStringPtr(numeroAzienda),
	}}
	item.Contact = contactSnapshot{contactSnapshotInput{
		FirstName: nullStringPtr(contactFirstName),
		LastName:  nullStringPtr(contactLastName),
		FullName:  nullStringPtr(contactFullName),
		Email:     nullStringPtr(contactEmail),
		Role:      nullStringPtr(contactRole),
	}}
	item.Payment = paymentSnapshot{
		MethodCode:  nullStringPtr(paymentMethodCode),
		MethodLabel: nullStringPtr(paymentMethodLabel),
		BankDetails: nullStringPtr(paymentBankDetails),
	}
	return item, nil
}

func scanQuoteLine(scanner rowScanner) (quoteLine, error) {
	var (
		item               quoteLine
		itemCode           sql.NullString
		itemDescription    sql.NullString
		description        sql.NullString
		unitOfMeasure      sql.NullString
		qta                sql.NullString
		unitPrice          sql.NullString
		discounts          sql.NullString
		codIVA             sql.NullString
		ivaPercentSnapshot sql.NullString
		purchaseUnitPrice  sql.NullString
		lineNet            sql.NullString
		lineVAT            sql.NullString
		lineGross          sql.NullString
		linePurchase       sql.NullString
		lineGain           sql.NullString
	)
	if err := scanner.Scan(
		&item.ID,
		&item.QuoteID,
		&item.Position,
		&item.LineType,
		&itemCode,
		&itemDescription,
		&description,
		&unitOfMeasure,
		&qta,
		&unitPrice,
		&discounts,
		&codIVA,
		&ivaPercentSnapshot,
		&purchaseUnitPrice,
		&lineNet,
		&lineVAT,
		&lineGross,
		&linePurchase,
		&lineGain,
	); err != nil {
		return quoteLine{}, err
	}
	item.ItemCode = nullStringPtr(itemCode)
	item.ItemDescription = nullStringPtr(itemDescription)
	item.Description = nullStringPtr(description)
	item.UnitOfMeasure = nullStringPtr(unitOfMeasure)
	item.Qta = nullStringPtr(qta)
	item.UnitPrice = nullStringPtr(unitPrice)
	item.Discounts = nullStringPtr(discounts)
	item.CodIVA = nullStringPtr(codIVA)
	item.IVAPercentSnapshot = nullStringPtr(ivaPercentSnapshot)
	item.PurchaseUnitPrice = nullStringPtr(purchaseUnitPrice)
	item.LineNet = nullStringPtr(lineNet)
	item.LineVAT = nullStringPtr(lineVAT)
	item.LineGross = nullStringPtr(lineGross)
	item.LinePurchase = nullStringPtr(linePurchase)
	item.LineGain = nullStringPtr(lineGain)
	return item, nil
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullTimestampPtr(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := formatTimestamp(value.Time)
	return &formatted
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	trimmed := value.String
	return &trimmed
}
