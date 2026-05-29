package ordini

import "time"

const component = "ordini"

type OrderState string

const (
	OrderStateBozza     OrderState = "BOZZA"
	OrderStateInviato   OrderState = "INVIATO"
	OrderStateAttivo    OrderState = "ATTIVO"
	OrderStatePerso     OrderState = "PERSO"
	OrderStateAnnullato OrderState = "ANNULLATO"
)

type OrderSummary struct {
	ID                int64    `json:"id"`
	CdlanSystemODV    *string  `json:"cdlan_systemodv"`
	CdlanTipodoc      *string  `json:"cdlan_tipodoc"`
	CdlanNdoc         *string  `json:"cdlan_ndoc"`
	CdlanAnno         *int64   `json:"cdlan_anno"`
	CodiceOrdine      *string  `json:"codice_ordine"`
	CdlanSostOrd      *string  `json:"cdlan_sost_ord"`
	CdlanCliente      *string  `json:"cdlan_cliente"`
	CdlanClienteID    *int64   `json:"cdlan_cliente_id"`
	CdlanDatadoc      NullDate `json:"cdlan_datadoc"`
	ServiceType       *string  `json:"service_type"`
	IsColo            *string  `json:"is_colo"`
	CdlanTipoOrd      *string  `json:"cdlan_tipo_ord"`
	CdlanDataconferma NullDate `json:"cdlan_dataconferma"`
	CdlanStato        *string  `json:"cdlan_stato"`
	ProfileLang       *string  `json:"profile_lang"`
	CdlanEvaso        *int64   `json:"cdlan_evaso"`
	FromCP            *int64   `json:"from_cp"`
	ArxDocNumber      *string  `json:"arx_doc_number"`
}

type OrderDetail struct {
	OrderSummary
	CdlanCommerciale        *string      `json:"cdlan_commerciale"`
	CdlanCodTerminiPag      *string      `json:"cdlan_cod_termini_pag"`
	CdlanNote               *string      `json:"cdlan_note"`
	CdlanDurRin             *string      `json:"cdlan_dur_rin"`
	CdlanTacitoRin          *string      `json:"cdlan_tacito_rin"`
	CdlanTempiRil           *string      `json:"cdlan_tempi_ril"`
	CdlanDurataServizio     *string      `json:"cdlan_durata_servizio"`
	CdlanRifOrdcli          *string      `json:"cdlan_rif_ordcli"`
	CdlanRifTechNom         *string      `json:"cdlan_rif_tech_nom"`
	CdlanRifTechTel         *string      `json:"cdlan_rif_tech_tel"`
	CdlanRifTechEmail       *string      `json:"cdlan_rif_tech_email"`
	CdlanRifAltroTechNom    *string      `json:"cdlan_rif_altro_tech_nom"`
	CdlanRifAltroTechTel    *string      `json:"cdlan_rif_altro_tech_tel"`
	CdlanRifAltroTechEmail  *string      `json:"cdlan_rif_altro_tech_email"`
	CdlanRifAdmNom          *string      `json:"cdlan_rif_adm_nom"`
	CdlanRifAdmTechTel      *string      `json:"cdlan_rif_adm_tech_tel"`
	CdlanRifAdmTechEmail    *string      `json:"cdlan_rif_adm_tech_email"`
	CdlanIntFatturazione    *string      `json:"cdlan_int_fatturazione"`
	CdlanIntFatturazioneAtt *string      `json:"cdlan_int_fatturazione_att"`
	CdlanChiuso             *int64       `json:"cdlan_chiuso"`
	CdlanValuta             *string      `json:"cdlan_valuta"`
	WrittenBy               *string      `json:"written_by"`
	ProfileIVA              *string      `json:"profile_iva"`
	ProfileCF               *string      `json:"profile_cf"`
	ProfileAddress          *string      `json:"profile_address"`
	ProfileCity             *string      `json:"profile_city"`
	ProfileCAP              *string      `json:"profile_cap"`
	ProfilePV               *string      `json:"profile_pv"`
	ProfileSDI              *string      `json:"profile_sdi"`
	DataDecorrenza          NullDate     `json:"data_decorrenza"`
	CdlanTacitoRinInPDF     *string      `json:"cdlan_tacito_rin_in_pdf"`
	OriginCodTerminiPag     *string      `json:"origin_cod_termini_pag"`
	IsArxivar               *int64       `json:"is_arxivar"`
	Origin                  *OrderOrigin `json:"origin,omitempty"`
}

type OrderOrigin struct {
	Type      string  `json:"type"`
	QuoteID   int64   `json:"quote_id"`
	QuoteCode *string `json:"quote_code,omitempty"`
	QuoteURL  string  `json:"quote_url"`
}

type OrderRow struct {
	ID                     int64     `json:"id"`
	OrderID                int64     `json:"orders_id"`
	CdlanSystemODVRow      *int64    `json:"cdlan_systemodv_row"`
	CdlanCodiceKit         *string   `json:"cdlan_codice_kit"`
	IndexKit               *int64    `json:"index_kit"`
	BundleCode             *string   `json:"bundle_code"`
	CdlanCodart            *string   `json:"cdlan_codart"`
	CdlanDescart           *string   `json:"cdlan_descart"`
	CdlanQta               NullFloat `json:"cdlan_qta"`
	Canone                 NullFloat `json:"canone"`
	ActivationPrice        NullFloat `json:"activation_price"`
	TerminationPrice       NullFloat `json:"termination_price"`
	CdlanRaggFatturazione  *string   `json:"cdlan_ragg_fatturazione"`
	CdlanDataAttivazione   NullDate  `json:"cdlan_data_attivazione"`
	CdlanSerialNumber      *string   `json:"cdlan_serialnumber"`
	ConfirmDataAttivazione *int64    `json:"confirm_data_attivazione"`
	DataAnnullamento       NullDate  `json:"data_annullamento"`
}

type TechnicalRow struct {
	ID                int64    `json:"id"`
	CdlanSystemODVRow *int64   `json:"cdlan_systemodv_row"`
	BundleCode        *string  `json:"bundle_code"`
	CdlanCodart       *string  `json:"cdlan_codart"`
	CdlanDescart      *string  `json:"cdlan_descart"`
	NoteTecnici       *string  `json:"note_tecnici"`
	DataAnnullamento  NullDate `json:"data_annullamento"`
}

type CustomerRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type UpdateHeaderRequest struct {
	CustomerPO       string `json:"customer_po"`
	ConfirmationDate string `json:"confirmation_date"`
	CustomerID       int64  `json:"customer_id"`
}

type UpdateReferentsRequest struct {
	TechnicalName       string `json:"technical_name"`
	TechnicalPhone      string `json:"technical_phone"`
	TechnicalEmail      string `json:"technical_email"`
	OtherTechnicalName  string `json:"other_technical_name"`
	OtherTechnicalPhone string `json:"other_technical_phone"`
	OtherTechnicalEmail string `json:"other_technical_email"`
	AdminName           string `json:"admin_name"`
	AdminPhone          string `json:"admin_phone"`
	AdminEmail          string `json:"admin_email"`
}

type UpdateSerialRequest struct {
	SerialNumber string `json:"serial_number"`
}

type UpdateTechnicalNotesRequest struct {
	TechnicalNotes string `json:"technical_notes"`
}

type ActivateRowRequest struct {
	ActivationDate string `json:"activation_date"`
}

type ActivationResponse struct {
	OrderState string   `json:"order_state"`
	Row        OrderRow `json:"row"`
}

type SendToERPRowOutcome struct {
	RowID             int64   `json:"rowId"`
	CdlanSystemODVRow *int64  `json:"cdlan_systemodv_row"`
	Status            string  `json:"status"`
	Error             *string `json:"error,omitempty"`
}

type SendToERPResponse struct {
	Rows              []SendToERPRowOutcome `json:"rows"`
	StateTransitioned bool                  `json:"stateTransitioned"`
	ArxivarUploaded   bool                  `json:"arxivarUploaded"`
	Warning           string                `json:"warning,omitempty"`
}

type RevertConversionResponse struct {
	Reverted      bool                           `json:"reverted"`
	OrderID       int64                          `json:"order_id"`
	QuoteID       int64                          `json:"quote_id"`
	OrderCode     *string                        `json:"order_code,omitempty"`
	DeletedRows   int64                          `json:"deleted_rows"`
	BridgeDeleted bool                           `json:"bridge_deleted"`
	HubSpot       RevertConversionHubSpotCleanup `json:"hubspot"`
	Warnings      []string                       `json:"warnings,omitempty"`
	Warning       string                         `json:"warning,omitempty"`
}

type RevertConversionHubSpotCleanup struct {
	Attempted   bool     `json:"attempted"`
	NoteID      *int64   `json:"note_id,omitempty"`
	FileID      *string  `json:"file_id,omitempty"`
	NoteDeleted bool     `json:"note_deleted"`
	FileDeleted bool     `json:"file_deleted"`
	Warnings    []string `json:"warnings,omitempty"`
}

func stateOf(order *OrderDetail) OrderState {
	if order == nil || order.CdlanStato == nil {
		return ""
	}
	return OrderState(*order.CdlanStato)
}

func todayString() string {
	return time.Now().Format("2006-01-02")
}
