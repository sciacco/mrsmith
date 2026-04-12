// ── Shared enums ──
export type QuoteStatus = 'DRAFT' | 'PENDING_APPROVAL' | 'APPROVED' | 'APPROVAL_NOT_NEEDED' | 'ESIGN_COMPLETED';
export type DocumentType = 'TSC-ORDINE-RIC' | 'TSC-ORDINE';
export type ProposalType = 'NUOVO' | 'SOSTITUZIONE' | 'RINNOVO';
export type TemplateType = 'standard' | 'iaas' | 'legacy';

// ── Reference data ──
export interface Template {
  template_id: string;
  description: string;
  lang: string;
  template_type: TemplateType;
  kit_id: number | null;
  service_category_id: number | null;
  is_colo: boolean;
  is_active: boolean;
}

export interface ProductCategory {
  id: number;
  name: string;
}

export interface Kit {
  id: number;
  internal_name: string;
  nrc: number;
  mrc: number;
  category_id: number | null;
  category_name: string | null;
  is_active: boolean;
  ecommerce: boolean;
  quotable: boolean;
}

export interface Customer {
  id: number;
  name: string;
  numero_azienda: string | null;
}

export interface Deal {
  id: number;
  name: string;
  pipeline: string | null;
  dealstage: string | null;
  dealtype?: string | null;
  created_at?: string | null;
  updated_at?: string | null;
  company_id: number | null;
  company_name: string | null;
  company_lingua: string | null;
}

export interface Owner {
  id: string;
  firstname: string | null;
  lastname: string | null;
  email: string | null;
}

export interface PaymentMethod {
  code: string;
  description: string;
}

export interface CustomerPayment {
  payment_code: string;
}

export interface CustomerOrder {
  name: string;
}

// ── Quote entities ──
export interface Quote {
  id: number;
  quote_number: string;
  customer_id: number | null;
  deal_number: string | null;
  owner: string | null;
  document_date: string | null;
  document_type: DocumentType;
  replace_orders: string | null;
  template: string | null;
  services: string | null;
  proposal_type: ProposalType;
  initial_term_months: number;
  next_term_months: number;
  bill_months: number;
  delivered_in_days: number;
  date_sent: string | null;
  status: QuoteStatus;
  notes: string | null;
  nrc_charge_time: number;
  created_at: string;
  updated_at: string;
  description: string;
  hs_deal_id: number | null;
  hs_quote_id: number | null;
  payment_method: string | null;
  trial: string | null;
  rif_ordcli: string | null;
  rif_tech_nom: string | null;
  rif_tech_tel: string | null;
  rif_tech_email: string | null;
  rif_altro_tech_nom: string | null;
  rif_altro_tech_tel: string | null;
  rif_altro_tech_email: string | null;
  rif_adm_nom: string | null;
  rif_adm_tech_tel: string | null;
  rif_adm_tech_email: string | null;
  // Joined fields (from list endpoint)
  customer_name?: string;
  deal_name?: string;
  owner_name?: string;
}

export interface QuoteListResponse {
  quotes: Quote[];
  total: number;
  page: number;
  page_size: number;
}

export interface QuoteRow {
  id: number;
  quote_id: number;
  kit_id: number;
  internal_name: string;
  nrc_row: number;
  mrc_row: number;
  bundle_prefix_row: string;
  hs_line_item_id: number | null;
  hs_line_item_nrc: number | null;
  position: number;
  hs_mrc?: string;
  hs_nrc?: string;
}

export interface ProductVariant {
  id: number;
  product_code: string;
  product_name: string;
  minimum: number;
  maximum: number;
  required: boolean;
  nrc: number;
  mrc: number;
  position: number;
  group_name: string;
  included: boolean;
  main_product: boolean;
  quantity: number;
  extended_description: string | null;
}

export interface ProductGroup {
  group_name: string;
  quote_row_id: number;
  products: ProductVariant[];
  count: number;
  required: boolean;
  main_product: boolean;
  position: number;
  included_product: ProductVariant | null;
}

export interface HSStatus {
  hs_quote_id: number | null;
  status: QuoteStatus;
  hs_status: string | null;
  hs_locked: boolean | null;
  quote_url: string | null;
  pdf_url: string | null;
  sign_status: string | null;
}

export interface PublishPrecheck {
  invalid_required_groups: number;
  has_missing_required_products: boolean;
}
