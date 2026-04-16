package rdf

type pagedResponse[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type Deal struct {
	ID            int64   `json:"id"`
	Codice        string  `json:"codice"`
	DealName      string  `json:"deal_name"`
	CompanyName   *string `json:"company_name"`
	OwnerEmail    *string `json:"owner_email"`
	PipelineLabel *string `json:"pipeline_label"`
	StageLabel    *string `json:"stage_label"`
	StageOrder    *int    `json:"stage_order"`
}

type LookupItem struct {
	ID   int    `json:"id"`
	Nome string `json:"nome"`
}

type Richiesta struct {
	ID                     int      `json:"id"`
	DealID                 *int64   `json:"deal_id"`
	DataRichiesta          string   `json:"data_richiesta"`
	Descrizione            string   `json:"descrizione"`
	Indirizzo              string   `json:"indirizzo"`
	Stato                  string   `json:"stato"`
	AnnotazioniRichiedente *string  `json:"annotazioni_richiedente"`
	AnnotazioniCarrier     *string  `json:"annotazioni_carrier"`
	CreatedBy              *string  `json:"created_by"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              *string  `json:"updated_at"`
	FornitoriPreferiti     []int    `json:"fornitori_preferiti"`
	CodiceDeal             string   `json:"codice_deal"`
	PreferredSupplierNames []string `json:"preferred_supplier_names,omitempty"`
}

type Fattibilita struct {
	ID                   int      `json:"id"`
	RichiestaID          int      `json:"richiesta_id"`
	FornitoreID          int      `json:"fornitore_id"`
	FornitoreNome        string   `json:"fornitore_nome"`
	DataRichiesta        string   `json:"data_richiesta"`
	TecnologiaID         int      `json:"tecnologia_id"`
	TecnologiaNome       string   `json:"tecnologia_nome"`
	Descrizione          *string  `json:"descrizione"`
	ContattoFornitore    *string  `json:"contatto_fornitore"`
	RiferimentoFornitore *string  `json:"riferimento_fornitore"`
	Stato                string   `json:"stato"`
	Annotazioni          *string  `json:"annotazioni"`
	EsitoRicevutoIl      *string  `json:"esito_ricevuto_il"`
	DaOrdinare           bool     `json:"da_ordinare"`
	ProfiloFornitore     *string  `json:"profilo_fornitore"`
	NRC                  *float64 `json:"nrc"`
	MRC                  *float64 `json:"mrc"`
	DurataMesi           *int     `json:"durata_mesi"`
	AderenzaBudget       int      `json:"aderenza_budget"`
	Copertura            bool     `json:"copertura"`
	GiorniRilascio       *int     `json:"giorni_rilascio"`
}

type FattibilitaCounts struct {
	Bozza       int `json:"bozza"`
	Inviata     int `json:"inviata"`
	Sollecitata int `json:"sollecitata"`
	Completata  int `json:"completata"`
	Annullata   int `json:"annullata"`
	Totale      int `json:"totale"`
}

type RichiestaSummary struct {
	Richiesta
	DealName    *string           `json:"deal_name"`
	CompanyName *string           `json:"company_name"`
	OwnerEmail  *string           `json:"owner_email"`
	Counts      FattibilitaCounts `json:"counts"`
}

type RichiestaFull struct {
	Richiesta
	Deal        *Deal             `json:"deal"`
	DealName    *string           `json:"deal_name"`
	CompanyName *string           `json:"company_name"`
	OwnerEmail  *string           `json:"owner_email"`
	Fattibilita []Fattibilita     `json:"fattibilita"`
	Counts      FattibilitaCounts `json:"counts"`
}

type analysisAction struct {
	Azione     string  `json:"azione"`
	Fornitore  string  `json:"fornitore"`
	Tecnologia *string `json:"tecnologia,omitempty"`
	Motivo     string  `json:"motivo"`
}

type analysisValutazione struct {
	Fornitore      string  `json:"fornitore"`
	Tecnologia     string  `json:"tecnologia"`
	Stato          string  `json:"stato"`
	Copertura      *string `json:"copertura,omitempty"`
	AderenzaBudget *string `json:"aderenza_budget,omitempty"`
	DurataMesi     *int    `json:"durata_mesi,omitempty"`
	GiorniRilascio *int    `json:"giorni_rilascio,omitempty"`
	Preferito      *bool   `json:"preferito,omitempty"`
	Criticita      *string `json:"criticita,omitempty"`
}

type analysisJSON struct {
	AzioniRaccomandate []analysisAction      `json:"azioni_raccomandate"`
	Valutazioni        []analysisValutazione `json:"valutazioni"`
}
