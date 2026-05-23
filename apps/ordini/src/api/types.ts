export type OrderState = 'BOZZA' | 'INVIATO' | 'ATTIVO' | 'PERSO' | 'ANNULLATO' | string;

export interface OrderSummary {
  id: number;
  cdlan_systemodv: string | null;
  cdlan_tipodoc: string | null;
  cdlan_ndoc: string | null;
  cdlan_anno: number | null;
  codice_ordine: string | null;
  cdlan_sost_ord: string | null;
  cdlan_cliente: string | null;
  cdlan_cliente_id: number | null;
  cdlan_datadoc: string | null;
  service_type: string | null;
  is_colo: string | null;
  cdlan_tipo_ord: string | null;
  cdlan_dataconferma: string | null;
  cdlan_stato: OrderState | null;
  profile_lang: string | null;
  cdlan_evaso: number | null;
  from_cp: number | null;
  arx_doc_number: string | null;
}

export interface OrderOrigin {
  type: 'quote';
  quote_id: number;
  quote_code?: string;
  quote_url: string;
}

export interface OrderDetail extends OrderSummary {
  cdlan_commerciale: string | null;
  cdlan_cod_termini_pag: string | null;
  cdlan_note: string | null;
  cdlan_dur_rin: string | null;
  cdlan_tacito_rin: string | null;
  cdlan_tempi_ril: string | null;
  cdlan_durata_servizio: string | null;
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
  cdlan_int_fatturazione: string | null;
  cdlan_int_fatturazione_att: string | null;
  cdlan_chiuso: number | null;
  cdlan_valuta: string | null;
  written_by: string | null;
  profile_iva: string | null;
  profile_cf: string | null;
  profile_address: string | null;
  profile_city: string | null;
  profile_cap: string | null;
  profile_pv: string | null;
  profile_sdi: string | null;
  data_decorrenza: string | null;
  cdlan_tacito_rin_in_pdf: string | null;
  origin_cod_termini_pag: string | null;
  is_arxivar: number | null;
  origin?: OrderOrigin;
}

export interface OrderRow {
  id: number;
  orders_id: number;
  cdlan_systemodv_row: number | null;
  cdlan_codice_kit: string | null;
  index_kit: number | null;
  bundle_code: string | null;
  cdlan_codart: string | null;
  cdlan_descart: string | null;
  cdlan_qta: number | null;
  canone: number | null;
  activation_price: number | null;
  termination_price: number | null;
  cdlan_ragg_fatturazione: string | null;
  cdlan_data_attivazione: string | null;
  cdlan_serialnumber: string | null;
  confirm_data_attivazione: number | null;
  data_annullamento: string | null;
}

export interface TechnicalRow {
  id: number;
  cdlan_systemodv_row: number | null;
  bundle_code: string | null;
  cdlan_codart: string | null;
  cdlan_descart: string | null;
  note_tecnici: string | null;
  data_annullamento: string | null;
}

export interface CustomerRef {
  id: number;
  name: string;
}

export interface UpdateHeaderPayload {
  customer_po: string;
  confirmation_date: string;
  customer_id: number;
}

export interface UpdateReferentsPayload {
  technical_name: string;
  technical_phone: string;
  technical_email: string;
  other_technical_name: string;
  other_technical_phone: string;
  other_technical_email: string;
  admin_name: string;
  admin_phone: string;
  admin_email: string;
}

export interface ActivationResponse {
  order_state: OrderState;
  row: OrderRow;
}

export interface SendToERPRowOutcome {
  rowId: number;
  cdlan_systemodv_row: number | null;
  status: 'ok' | 'error';
  error?: string;
}

export interface SendToERPResponse {
  rows: SendToERPRowOutcome[];
  stateTransitioned: boolean;
  arxivarUploaded: boolean;
  warning?: string;
}
