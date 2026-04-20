// DTOs shared with the Go backend under /api/afc-tools/*.
// Field names mirror `backend/internal/afctools/*.go` json tags.

export interface WhmcsTransaction {
  cliente: string | null;
  fattura: string | null;
  invoiceid: number | null;
  userid: number | null;
  payment_method: string | null;
  date: string | null;
  description: string | null;
  amountin: number | null;
  fees: number | null;
  amountout: number | null;
  rate: number | null;
  transid: string | null;
  refundid: number | null;
  accountsid: number | null;
}

export interface WhmcsInvoiceLine {
  raggruppamento: string | null;
  ragionesocialecliente: string | null;
  nomecliente: string | null;
  cognomecliente: string | null;
  partitaiva: string | null;
  codicefiscale: string | null;
  codiceiso: string | null;
  flagpersonafisica: string | null;
  indirizzo: string | null;
  numerocivico: string | null;
  cap: string | null;
  comune: string | null;
  provincia: string | null;
  nazione: string | null;
  numerodocumento: string | null;
  datadocumento: string | null;
  causale: string | null;
  numerolinea: number | null;
  quantita: number | null;
  descrizioneriga: string | null;
  prezzo: number | null;
  datainizioperiodo: string | null;
  datafineperiodo: string | null;
  modalitapagamento: string | null;
  ivariga: number | null;
  bollo: number | null;
  codiceclienteerp: string | null;
  tipo: string | null;
  invoiceid: number | null;
  id: number;
}

export interface MissingArticle {
  code: string;
  categoria: string | null;
  nrc: number | null;
  mrc: number | null;
  descrizione_it: string | null;
  descrizione_en: string | null;
}

export interface XConnectOrder {
  id_ordine: number;
  codice_ordine: string | null;
  cliente: string | null;
  data_creazione: string;
}

export interface EnergiaColoPivotRow {
  customer: string | null;
  gennaio_a: number | null;
  gennaio_kw: number | null;
  febbraio_a: number | null;
  febbraio_kw: number | null;
  marzo_a: number | null;
  marzo_kw: number | null;
  aprile_a: number | null;
  aprile_kw: number | null;
  maggio_a: number | null;
  maggio_kw: number | null;
  giugno_a: number | null;
  giugno_kw: number | null;
  luglio_a: number | null;
  luglio_kw: number | null;
  agosto_a: number | null;
  agosto_kw: number | null;
  settembre_a: number | null;
  settembre_kw: number | null;
  ottobre_a: number | null;
  ottobre_kw: number | null;
  novembre_a: number | null;
  novembre_kw: number | null;
  dicembre_a: number | null;
  dicembre_kw: number | null;
}

export interface EnergiaColoDetailRow {
  customer: string | null;
  start_period: string | null;
  end_period: string | null;
  consumo: number | null;
  amount: number | null;
  pun: number | null;
  coefficiente: number | null;
  fisso_cu: number | null;
  eccedenti: number | null;
  importo_eccedenti: number | null;
  tipo_variabile: string | null;
}

export interface SalesOrderSummary {
  id: number;
  cdlan_tipodoc: string | null;
  cdlan_ndoc: string | null;
  cdlan_anno: number | null;
  codice_ordine: string | null;
  cdlan_sost_ord: string | null;
  cdlan_cliente: string | null;
  cdlan_datadoc: string | null;
  tipo_di_servizi: string | null;
  tipo_di_ordine: string | null;
  cdlan_dataconferma: string | null;
  cdlan_stato: string | null;
  dal_cp: string | null;
}

export interface OrderHeader {
  id: number;
  cdlan_systemodv: string | null;
  cdlan_tipodoc: string | null;
  cdlan_ndoc: string | null;
  cdlan_datadoc: string | null;
  cdlan_cliente: string | null;
  cdlan_commerciale: string | null;
  cdlan_cod_termini_pag: number | null;
  cdlan_note: string | null;
  cdlan_tipo_ord: string | null;
  cdlan_dur_rin: number | null;
  cdlan_tacito_rin: number | null;
  cdlan_sost_ord: string | null;
  cdlan_tempi_ril: string | null;
  cdlan_durata_servizio: string | null;
  cdlan_dataconferma: string | null;
  cdlan_rif_ordcli: string | null;
  cdlan_rif_tech_nom: string | null;
  cdlan_rif_tech_tel: string | null;
  cdlan_rif_tech_email: string | null;
  cdlan_rif_altro_tech_nom: string | null;
  cdlan_rif_altro_tech_tel: string | null;
  cdlan_rif_altro_tech_email: string | null;
  cdlan_rif_adm_nom: string | null;
  cdlan_rif_adm_tech_tel: string | null;
  cdlan_rif_adm_tech_email: string | null;
  cdlan_int_fatturazione_desc: string | null;
  cdlan_int_fatturazione: number | null;
  cdlan_int_fatturazione_att_desc: string | null;
  cdlan_int_fatturazione_att: number | null;
  cdlan_stato: string | null;
  cdlan_evaso: number | null;
  cdlan_chiuso: number | null;
  cdlan_anno: number | null;
  cdlan_valuta: string | null;
  written_by: string | null;
  profile_iva: string | null;
  profile_cf: string | null;
  profile_address: string | null;
  profile_city: string | null;
  profile_cap: string | null;
  profile_pv: string | null;
  profile_sdi: string | null;
  profile_lang: string | null;
  cdlan_cliente_id: number | null;
  service_type: string | null;
  data_decorrenza: string | null;
  cdlan_tacito_rin_in_pdf: number | null;
  is_colo: string | null;
  origin_cod_termini_pag: number | null;
  is_arxivar: number | null;
  from_cp: number | null;
  arx_doc_number: string | null;
}

export interface OrderRow {
  id_riga: number;
  system_odv_riga: string | null;
  codice_articolo_bundle: string | null;
  codice_articolo: string | null;
  descrizione_articolo: string | null;
  canone: number | null;
  attivazione: number | null;
  quantita: number | null;
  prezzo_cessazione: number | null;
  codice_raggruppamento_fatturazione: string | null;
  data_attivazione: string | null;
  numero_seriale: string | null;
  confirm_data_attivazione: number | null;
  data_annullamento: string | null;
}

// DDT cespiti: dynamic columns (preserve 1:1 Appsmith binding behavior).
export type DdtCespitoRow = Record<string, unknown>;

export interface TransactionsExportResponse {
  renderId: string;
  renderUrl: string;
  reportName: string;
}
