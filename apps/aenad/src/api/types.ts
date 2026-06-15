export interface DocumentTypeOption {
  tipoDoc: string;
  label: string;
}

export interface AenadDocument {
  IDDoc: number;
  TipoDoc: string | null;
  IDAnagr: number | null;
  Anagr_Nome: string | null;
  CodDest_IDAnagr: number | null;
  CodDest: string | null;
  Data: string | null;
  Num: number | null;
  DataDoc: string | null;
  NumDoc: string | null;
  DescDoc: string | null;
  TotNetto: string | null;
  TotDoc: string | null;
  TotPrezzoAcquisto: string | null;
  TotGuadagno: string | null;
  Pagamento: string | null;
  Pagam_CoordBancarie: string | null;
  NoteInterne: string | null;
  Anagr_Indirizzo: string | null;
  Anagr_Cap: string | null;
  Anagr_Citta: string | null;
  Anagr_Prov: string | null;
  Anagr_Nazione: string | null;
  Anagr_CodiceFiscale: string | null;
  Anagr_PartitaIva: string | null;
  Anagr_DestNome: string | null;
  Anagr_DestIndirizzo: string | null;
  Anagr_DestCap: string | null;
  Anagr_DestCitta: string | null;
  Anagr_DestProv: string | null;
  Anagr_DestNazione: string | null;
}

export interface AenadDocumentsPage {
  items: AenadDocument[];
  total: number;
  page: number;
  pageSize: number;
}

export interface ArchiveDocumentFilters {
  tipoDoc: string;
  dateFrom: string;
  dateTo: string;
  idAnagr?: number;
  page: number;
  pageSize: number;
}

export interface CustomerOption {
  idAnagr: number;
  nome: string;
}

export interface AenadDocumentRow {
  IDDocRiga: number;
  IDDoc: number;
  CodArticolo: string | null;
  Desc: string | null;
  Qta: string | null;
  Udm: string | null;
  PrezzoNetto: string | null;
  Sconti: string | null;
  ImportoNettoRiga: string | null;
}

export interface PaymentMethodOption {
  nomePagamento: string;
  categPagamento: string | null;
  rate: string | null;
}

export interface QuoteSummary {
  id: number;
  quote_number: string;
  created_at: string;
  updated_at: string;
  created_by: string;
  updated_by: string;
  authoring_status: 'draft' | 'ready' | 'archived';
  hubspot_sync_status: 'pending' | 'succeeded' | 'failed';
  hubspot_sync_error: string | null;
  hubspot_synced_at: string | null;
  hubspot_company_id: string | null;
  hubspot_contact_id: string | null;
  hubspot_deal_id: string | null;
  hubspot_pipeline_id: string | null;
  hubspot_pipeline_label: string | null;
  hubspot_dealstage_id: string | null;
  hubspot_dealstage_label: string | null;
  customer_name: string | null;
  document_date: string | null;
  description: string | null;
  total_net: string;
  total_vat: string;
  total_gross: string;
  total_purchase: string;
  total_gain: string;
}

export interface QuoteListResponse {
  items: QuoteSummary[];
  total: number;
  page: number;
  page_size: number;
}

export interface CustomerSnapshotInput {
  name: string | null;
  vat: string | null;
  tax_code: string | null;
  pec: string | null;
  email: string | null;
  address: string | null;
  zip: string | null;
  city: string | null;
  province: string | null;
  country: string | null;
  language: string | null;
  numero_azienda_snapshot: string | null;
}

export interface ContactSnapshotInput {
  first_name: string | null;
  last_name: string | null;
  full_name: string | null;
  email: string | null;
  role: string | null;
}

export interface PaymentSnapshot {
  method_code: string | null;
  method_label: string | null;
  bank_details: string | null;
}

export interface QuoteLineInput {
  position: number;
  line_type: 'spacer' | 'description' | 'item';
  item_code: string | null;
  item_description: string | null;
  description: string | null;
  unit_of_measure: string | null;
  qta: string | null;
  unit_price: string | null;
  discounts: string | null;
  cod_iva: string | null;
  iva_percent_snapshot: string | null;
  purchase_unit_price: string | null;
}

export interface QuoteLine extends QuoteLineInput {
  id: number;
  quote_id: number;
  line_net: string | null;
  line_vat: string | null;
  line_gross: string | null;
  line_purchase: string | null;
  line_gain: string | null;
}

export interface QuoteEvent {
  id: number;
  quote_id: number;
  event_type: string;
  actor_subject: string;
  payload: any;
  created_at: string;
}

export interface QuoteResponse extends QuoteSummary {
  customer: CustomerSnapshotInput;
  contact: ContactSnapshotInput;
  payment: PaymentSnapshot;
  internal_notes: string | null;
  lines: QuoteLine[];
  events: QuoteEvent[];
}

export interface SaveQuotePayload {
  hubspot_company_id: string | null;
  hubspot_contact_id: string | null;
  hubspot_pipeline_id: string | null;
  hubspot_pipeline_label: string | null;
  hubspot_dealstage_id: string | null;
  hubspot_dealstage_label: string | null;
  customer: CustomerSnapshotInput;
  contact: ContactSnapshotInput;
  document_date: string | null;
  payment_method_code: string | null;
  payment_method_label: string | null;
  payment_bank_details: string | null;
  description: string | null;
  internal_notes: string | null;
  lines: QuoteLineInput[];
}

export interface CustomerSelection {
  hubspot_company_id: string;
  customer: CustomerSnapshotInput;
  domain: string | null;
  alyante_id_anagrafica: string | null;
}

export interface ProspectPayload {
  email: string | null;
  first_name: string | null;
  last_name: string | null;
  full_name: string | null;
  role: string | null;
  company_name: string | null;
  domain: string | null;
  customer: CustomerSnapshotInput;
}

export interface ProspectResponse {
  hubspot_company_id: string;
  hubspot_contact_id: string | null;
  customer: CustomerSnapshotInput;
  contact: ContactSnapshotInput;
}

export interface ArticleLineInitializer {
  line_type: 'item';
  item_code: string;
  item_description: string | null;
  description: string | null;
  unit_of_measure: string | null;
  qta: string | null;
  unit_price: string | null;
  discounts: string | null;
  cod_iva: string | null;
  purchase_unit_price: string | null;
}

export interface PaymentMethodSelection {
  cod_pagamento: string;
  desc_pagamento: string;
}

export interface QuoteDefaultsResponse {
  line: {
    cod_iva: string | null;
  };
}

export interface StageSelection {
  id: string;
  label: string | null;
  pipeline: string;
  pipeline_label: string | null;
  display_order: number | null;
  is_initial: boolean;
}

export interface StagesResponse {
  pipeline_id: string;
  pipeline_label: string | null;
  initial_dealstage_id: string;
  items: StageSelection[];
}

export interface PdfExport {
  id: number;
  quote_id: number;
  revision: number;
  filename: string;
  content_type: string;
  checksum_sha256: string | null;
  render_payload: any;
  created_at: string;
  created_by: string;
  hubspot_attachment_status: string;
  hubspot_file_id: string | null;
  hubspot_note_id: string | null;
  hubspot_deal_id: string | null;
  hubspot_attached_at: string | null;
  hubspot_error: string | null;
  is_stale: boolean;
}

