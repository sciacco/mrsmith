// ── Customer variants ──
export interface CustomerWithInvoices {
  numero_azienda: number;
  ragione_sociale: string;
}

export interface CustomerWithOrders {
  numero_azienda: number;
  ragione_sociale: string;
}

export interface CustomerWithAccessLines {
  id: number;
  intestazione: string;
}

// ── Orders: Detail ──
export interface OrderDetailRow {
  // Anagrafica
  ragione_sociale: string;
  data_ordine: string | null;
  nome_testata_ordine: string;
  cliente: string | null;
  numero_azienda: number;
  id_gamma: string | null;
  commerciale: string | null;
  data_documento: string | null;
  data_conferma: string | null;
  stato_ordine: string;
  tipo_ordine: string | null;
  tipo_documento: string | null;
  sost_ord: string | null;
  riferimento_odv_cliente: string | null;
  durata_servizio: string | null;
  tacito_rinnovo: string | null;
  durata_rinnovo: string | null;
  tempi_rilascio: string | null;
  metodo_pagamento: string | null;
  note_legali: string | null;
  // Referenti
  referente_amm_nome: string | null;
  referente_amm_mail: string | null;
  referente_amm_tel: string | null;
  referente_tech_nome: string | null;
  referente_tech_mail: string | null;
  referente_tech_tel: string | null;
  referente_altro_nome: string | null;
  referente_altro_mail: string | null;
  referente_altro_tel: string | null;
  // Date testata
  data_creazione: string | null;
  data_variazione: string | null;
  sostituito_da: string | null;
  // Riga
  quantita: number | null;
  codice_kit: string | null;
  codice_prodotto: string | null;
  descrizione_prodotto: string | null;
  descrizione_estesa: string | null;
  serialnumber: string | null;
  setup: number;
  canone: number;
  valuta: string | null;
  costo_cessazione: number;
  data_attivazione: string | null;
  data_disdetta: string | null;
  data_cessazione: string | null;
  raggruppamento_fatturazione: string | null;
  intervallo_fatt_attivazione: string | null;
  intervallo_fatt_canone: string | null;
  data_ultima_fatt: string | null;
  data_fine_fatt: string | null;
  system_odv_row: string | null;
  id_gamma_testata: string | null;
  progressivo_riga: number;
  annullato: number;
  data_scadenza_ordine: string | null;
  mrc: number | null;
  // Prodotto
  famiglia: string | null;
  sotto_famiglia: string | null;
  conto_ricavo: string | null;
  stato_riga: string;
  intestazione_ordine: string | null;
  descrizione_long: string | null;
  storico: string | null;
}

// ── Invoices ──
export interface InvoiceLine {
  progressivo_riga: number;
  descrizione_riga: string | null;
  qta: number | null;
  prezzo_unitario: number | null;
  prezzo_totale_netto: number | null;
  codice_articolo: string | null;
  serialnumber: string | null;
  riferimento_ordine_cliente: string | null;
  condizione_pagamento: string | null;
  scadenza: string | null;
  desc_conto_ricavo: string | null;
  gruppo: string | null;
  sottogruppo: string | null;
}

export interface InvoiceDocument {
  id: string;
  anno_documento: number;
  mese_documento: number;
  tipo_documento: string;
  num_documento: string;
  doc: string;
  data_documento: string | null;
  id_cliente: number;
  segno: number;
  line_count: number;
  totale_netto: number | null;
  lines: InvoiceLine[];
}

export interface InvoiceDocumentsResponse {
  items: InvoiceDocument[];
  total_documents: number;
  page: number;
  page_size: number;
}

// ── Access Lines ──
export interface AccessLine {
  tipo_conn: string;
  fornitore: string | null;
  provincia: string | null;
  comune: string | null;
  tipo: string | null;
  profilo_commerciale: string | null;
  intestatario: string | null;
  ordine: string | null;
  fatturato_fino_al: string | null;
  stato_riga: string | null;
  stato_ordine: string | null;
  stato: string;
  id: number;
  codice_ordine: string | null;
  serialnumber: string | null;
  id_anagrafica: number | null;
}

// ── IaaS ──
export interface IaaSAccount {
  intestazione: string;
  credito: number;
  cloudstack_domain: string;
  id_cli_fatturazione: number;
  abbreviazione: string | null;
  codice_ordine: string | null;
  serialnumber: string | null;
  data_attivazione: string | null;
}

// Aggregated charge time series point. `bucket` is an anchor date (YYYY-MM-DD)
// whose meaning depends on the requested aggregation group.
export interface ChargeSeriesPoint {
  bucket: string;
  total_importo: number;
}

// Charge composition by macro-category over a period.
// Categories: VM, Storage, Licenze Windows, Altro (usage_type 9999 excluded).
export interface CategoryAmount {
  category: string;
  amount: number;
}

export interface ChargeCategoryBreakdown {
  categories: CategoryAmount[];
  total: number;
}

export interface WindowsLicense {
  x: string;
  y: number;
}

// ── Timoo ──
export interface TimooTenant {
  as7_tenant_id: number;
  name: string;
}

export interface PbxRow {
  pbx_name: string;
  pbx_id: number;
  users: number;
  service_extensions: number;
  totale: number;
}

export interface PbxStatsResponse {
  rows: PbxRow[];
  totalUsers: number;
  totalSE: number;
}
