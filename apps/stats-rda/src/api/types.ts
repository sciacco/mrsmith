// Types mirroring the backend contracts in IMPLEMENTATION-PLAN.md.

export interface IssueSummary {
  issue_key: string;
  summary: string;
  numero_ordine: string | null;
  status: string | null;
  issue_type: string | null;
  importo_totale: number | null;
  valuta: string | null;
  budget_di_riferimento: string | null;
  reporter_name: string | null;
  fornitore_selezionato: string | null;
  created: string | null;
}

export interface IssueListResponse {
  items: IssueSummary[];
  total: number;
  page: number;
  limit: number;
}

export interface FilterOption {
  value: string;
  count: number;
}

export interface FiltersResponse {
  budget: FilterOption[];
  stati: FilterOption[];
  tipi: FilterOption[];
  valute: FilterOption[];
  range_date: { min: string | null; max: string | null };
}

export interface AutocompleteResponse {
  items: FilterOption[];
}

export type PeriodPreset =
  | 'this_month'
  | 'previous_month'
  | 'this_quarter'
  | 'previous_quarter'
  | 'current_year'
  | 'previous_year';

export type RiepilogoPeriodSelection = PeriodPreset | 'custom';

export interface RiepilogoPeriod {
  preset: RiepilogoPeriodSelection;
  from: string;
  to: string;
}

export interface RiepilogoTotals {
  order_count: number;
  budget_count: number;
  amount: number;
}

export interface RiepilogoBudget {
  budget: string;
  order_count: number;
  amount: number;
  percentage: number;
}

export interface RiepilogoDetail {
  issue_key: string;
  numero_ordine: string | null;
  summary: string;
  budget_di_riferimento: string;
  importo_totale: number;
  valuta: string | null;
  reporter_name: string | null;
  fornitore_selezionato: string | null;
  status: string | null;
  resolution: string | null;
  created: string | null;
}

export interface RiepilogoResponse {
  period: RiepilogoPeriod;
  totals: RiepilogoTotals;
  budgets: RiepilogoBudget[];
  details: RiepilogoDetail[];
}

export interface RiepilogoRdaBudget {
  budget_key: string;
  budget: string;
  budget_id: number | null;
  budget_name: string | null;
  budget_year: number | null;
  order_count: number;
  amount: number;
  percentage: number;
}

export interface RiepilogoRdaDetail {
  code: string;
  project: string | null;
  object: string | null;
  requester: string | null;
  budget_id: number | null;
  budget_name: string | null;
  budget_year: number | null;
  cost_center: string | null;
  currency: string | null;
  total_price: number;
  state: string | null;
  created: string | null;
  company_name: string | null;
}

export interface RiepilogoRdaResponse {
  period: RiepilogoPeriod;
  totals: RiepilogoTotals;
  budgets: RiepilogoRdaBudget[];
  details: RiepilogoRdaDetail[];
}

export interface IssueHeader {
  issue_key: string;
  summary: string;
  numero_ordine: string | null;
  issue_type: string | null;
  status: string | null;
  stato: string | null;
  priority: string | null;
  resolution: string | null;
  valuta: string | null;
  created: string | null;
  updated: string | null;
  resolution_date: string | null;
  due_date: string | null;
  reporter_name: string | null;
  reporter_email: string | null;
  assignee_name: string | null;
  creator_name: string | null;
}

export interface PurchaseSection {
  importo_totale: number | null;
  importo_totale_merci: number | null;
  importo_totale_servizi: number | null;
  importo_totale_leasing: number | null;
  fornitore_selezionato: string | null;
  tipo_di_ordine: string | null;
  tipo_documento: string | null;
  budget_di_riferimento: string | null;
  budget_corrente: number | null;
  budget_totale: number | null;
  limite_approvazione: number | null;
  percentuale_approvazione: number | null;
  inviato_in_approvazione: string | null;
  approvato: string | null;
  ricorrente: string | null;
}

export interface LineItem {
  grid: string;
  row_no: number;
  articolo_name: string | null;
  vendor: string | null;
  part_number: string | null;
  descrizione: string | null;
  quantita: number | null;
  importo: number | null;
  prezzo_totale: number | null;
  pagamento_name: string | null;
  durata_name: string | null;
  data_pagamento: string | null;
  vendita: string | null;
  rinnovo: string | null;
}

export interface Comment {
  created: string | null;
  author_name: string | null;
  body: string | null;
}

export interface Attachment {
  filename: string;
  mimetype: string | null;
  filesize: number | null;
  created: string | null;
  author_name: string | null;
}

export interface IssueLink {
  source_key: string;
  destination_key: string;
  link_name: string | null;
  inward: string | null;
  outward: string | null;
}

export interface HistoryEntry {
  created: string | null;
  author_name: string | null;
  field: string | null;
  old_string: string | null;
  new_string: string | null;
}

export interface IssueDetail {
  issue: IssueHeader;
  purchase: PurchaseSection;
  description: string | null;
  line_items_by_grid: Record<string, LineItem[]>;
  comments: Comment[];
  attachments: Attachment[];
  links: IssueLink[];
  history: HistoryEntry[];
  history_total: number;
}
