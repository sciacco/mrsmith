export interface AlyanteInvoiceRow {
  DO11_DITTA_CG18: number | null;
  DO11_NUMREG_CO99: number | null;
  DO11_DOCUM_MG36: string | null;
  MG36_DESCDOCUM: string | null;
  DO11_NUMDOC: string | null;
  DO11_SEZDOC: string | null;
  DO11_DATADOC: string | null;
  DO11_CLIFOR_CG44: number | null;
  RAGIONE_SOCIALE: string | null;
  DO11_NUMDOCORIG: string | null;
  DO11_NOTEDOCUM: string | null;
  DO13_TOTDOCUMENTO: number | null;
  DO13_TOTAPAGARE: number | null;
  NUM_RATE_APERTE: number | null;
  TOTRATE: number | null;
  EF01_SCADE_S: string | null;
  EF01_IMPEFFORIG: number | null;
  RESIDUO: number | null;
  PAGATO_SU_RESIDUO: number | null;
  IN_SCADENZIARIO: boolean;
  DO30_PROGRIGA: number | null;
  DO30_PROGVISUASTA: number | null;
  DO30_INDTIPORIGA: number | null;
  DO30_CODART_MG66: string | null;
  DO30_DESCART: string | null;
  DO30_UM1: string | null;
  DO30_QTA1: number | null;
  DO30_PREZZO1: number | null;
  DO30_SCPER1: number | null;
  DO30_SCPER2: number | null;
  DO30_SCPER3: number | null;
  DO30_SCIMP: number | null;
  DO30_IMPORTO: number | null;
  DO30_IMPNETSCP: number | null;
  DO30_ALIVA_CG28: string | null;
  DO30_IMPORTOIVA: number | null;
}

export interface MatchingFunnelProfileCounts {
  one_time: number;
  recurring: number;
  mixed: number;
  unknown: number;
}

export type MatchingFunnelProfile = 'one_time' | 'recurring' | 'mixed' | 'unknown';

export type MatchingFunnelReferenceOutcome =
  | 'no_note'
  | 'no_code'
  | 'no_rda_declared'
  | 'legacy_only'
  | 'unresolved'
  | 'ambiguous'
  | 'one'
  | 'multiple';

export interface MatchingFunnelReferencedRDA {
  code: string;
  legacy_code: string;
  id: number | null;
  state: string;
  supplier_erp_id: number | null;
  resolution: 'resolved' | 'successor' | 'unresolved' | 'ambiguous';
  supplier_match: boolean | null;
  in_candidates: boolean;
  total: number | null;
}

export interface MatchingFunnelReference {
  note: string;
  afc_status: string;
  outcome: MatchingFunnelReferenceOutcome;
  rdas: MatchingFunnelReferencedRDA[];
  legacy_codes: string[];
}

export interface MatchingFunnelReferenceSummary {
  no_note: number;
  no_code: number;
  no_rda_declared: number;
  legacy_only: number;
  unresolved: number;
  ambiguous: number;
  one_rda: number;
  multiple_rdas: number;
  arak_and_legacy: number;
  legacy_promoted: number;
  supplier_mismatch: number;
}

export interface MatchingFunnelProposal {
  codes: string[];
  ids: number[];
}

export type MatchingFunnelRuleVerdict =
  | 'match'
  | 'ambiguous'
  | 'wrong'
  | 'none'
  | 'false_positive'
  | 'silent_ok'
  | 'no_truth';

export interface MatchingFunnelRuleResult {
  rule: '' | 'full' | 'installment' | 'sum';
  proposals: MatchingFunnelProposal[];
  verdict: MatchingFunnelRuleVerdict;
  near_miss: number | null;
}

export interface MatchingFunnelRulesSummary {
  truth_invoices: number;
  match: number;
  ambiguous: number;
  wrong: number;
  none: number;
  no_rda_invoices: number;
  false_positive: number;
  silent_ok: number;
  no_truth_invoices: number;
  no_truth_proposals: number;
  match_by_full: number;
  match_by_installment: number;
  match_by_sum: number;
  near_misses: number;
}

export interface MatchingFunnelInvoice {
  registration: number;
  document_number: string;
  document_date: string | null;
  supplier_reference: string;
  taxable_amount: number | null;
  reference: MatchingFunnelReference;
  rules: MatchingFunnelRuleResult;
}

export interface MatchingFunnelRDA {
  id: number;
  code: string;
  state: string;
  object: string;
  total: number | null;
  currency: string;
  created: string | null;
  profile: MatchingFunnelProfile;
}

export interface MatchingFunnelSupplier {
  supplier_erp_id: number | null;
  alyante_supplier_name: string | null;
  provider_name: string | null;
  invoice_count: number;
  candidate_count: number;
  outcome: 'none' | 'one' | 'multiple';
  profiles: MatchingFunnelProfileCounts;
  reference: MatchingFunnelReferenceSummary;
  invoices: MatchingFunnelInvoice[];
  candidates: MatchingFunnelRDA[];
}

export interface MatchingFunnelResponse {
  summary: {
    invoice_count: number;
    no_candidates: number;
    one_candidate: number;
    multiple_candidates: number;
    rda_count: number;
    rdas_without_erp_id: number;
    duplicate_rda_codes: number;
    rdas_with_legacy_predecessor: number;
  };
  profiles: MatchingFunnelProfileCounts;
  reference: MatchingFunnelReferenceSummary;
  rules: MatchingFunnelRulesSummary;
  suppliers: MatchingFunnelSupplier[];
}

export interface ArakRDARow {
  id: number | null;
  type: string | null;
  project: string | null;
  description: string | null;
  note: string | null;
  object: string | null;
  requester_id: number | null;
  code: string | null;
  payment_method: string | null;
  budget_id: number | null;
  cost_center: string | null;
  budget_user_id: number | null;
  provider_id: number | null;
  currency: string | null;
  leasing: boolean | null;
  total_price: number | null;
  state: string | null;
  created_document: string | null;
  created: string | null;
  updated: string | null;
  deleted: string | null;
  budget_increment_id: number | null;
  subtracted_from_budget: boolean | null;
  provider_offer_date: string | null;
  provider_offer_code: string | null;
  reference_warehouse: string | null;
  advance_payment: boolean | null;
  provider_company_name: string | null;
  erp_id: string | null;
  provider_state: string | null;
  budget_name: string | null;
  budget_year: number | null;
  requester_email: string | null;
  payment_method_code: string | null;
  payment_method_description: string | null;
  current_approval_level: number | null;
  row_id: number | null;
  product_code: string | null;
  row_type: string | null;
  qty: number | null;
  nrc: number | null;
  mrc: number | null;
  product_description: string | null;
  row_description: string | null;
  total: number | null;
  price: number | null;
  row_is_recurrent: boolean | null;
  row_advance_payment: boolean | null;
  row_month_recursion: number | null;
  row_start_at_date: string | null;
  row_start_pay_at_activation_date: boolean | null;
  row_automatic_renew: boolean | null;
  row_initial_subscription_months: number | null;
  row_next_subscription_months: number | null;
  row_cancellation_advice_days: number | null;
}
